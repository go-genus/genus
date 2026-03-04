package playground

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"
)

// PlaygroundConfig configuração do playground.
type PlaygroundConfig struct {
	Port         int
	DB           *sql.DB
	Driver       string
	ReadOnly     bool
	MaxQueryTime time.Duration
	MaxResults   int
}

// PlaygroundServer servidor do playground de queries.
type PlaygroundServer struct {
	config  PlaygroundConfig
	mux     *http.ServeMux
	history []QueryHistory
	mu      sync.RWMutex
}

// QueryHistory histórico de uma query.
type QueryHistory struct {
	ID        int           `json:"id"`
	Query     string        `json:"query"`
	Duration  time.Duration `json:"duration"`
	RowCount  int           `json:"row_count"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// QueryRequest requisição de query.
type QueryRequest struct {
	Query   string `json:"query"`
	Explain bool   `json:"explain"`
}

// QueryResponse resposta de uma query.
type QueryResponse struct {
	Columns  []string                 `json:"columns"`
	Rows     []map[string]interface{} `json:"rows"`
	RowCount int                      `json:"row_count"`
	Duration string                   `json:"duration"`
	Error    string                   `json:"error,omitempty"`
	Explain  string                   `json:"explain,omitempty"`
}

// NewPlaygroundServer cria um novo servidor de playground.
func NewPlaygroundServer(config PlaygroundConfig) *PlaygroundServer {
	if config.Port == 0 {
		config.Port = 8765
	}
	if config.MaxQueryTime == 0 {
		config.MaxQueryTime = 30 * time.Second
	}
	if config.MaxResults == 0 {
		config.MaxResults = 1000
	}

	server := &PlaygroundServer{
		config: config,
		mux:    http.NewServeMux(),
	}

	// Registra rotas
	server.mux.HandleFunc("/", server.handleIndex)
	server.mux.HandleFunc("/api/query", server.handleQuery)
	server.mux.HandleFunc("/api/schema", server.handleSchema)
	server.mux.HandleFunc("/api/history", server.handleHistory)
	server.mux.HandleFunc("/api/tables", server.handleTables)
	server.mux.HandleFunc("/api/describe/", server.handleDescribe)
	server.mux.HandleFunc("/api/export", server.handleExport)

	return server
}

// Start inicia o servidor.
func (s *PlaygroundServer) Start() error {
	addr := fmt.Sprintf(":%d", s.config.Port)
	fmt.Printf("🎮 Genus Playground running at http://localhost%s\n", addr)
	return http.ListenAndServe(addr, s.mux)
}

// Handler retorna o handler HTTP para uso em servidores existentes.
func (s *PlaygroundServer) Handler() http.Handler {
	return s.mux
}

func (s *PlaygroundServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	tmpl := template.Must(template.New("index").Parse(playgroundHTML))
	tmpl.Execute(w, map[string]interface{}{
		"Driver":   s.config.Driver,
		"ReadOnly": s.config.ReadOnly,
	})
}

func (s *PlaygroundServer) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Verifica modo read-only
	if s.config.ReadOnly {
		upper := strings.ToUpper(strings.TrimSpace(req.Query))
		if !strings.HasPrefix(upper, "SELECT") &&
			!strings.HasPrefix(upper, "SHOW") &&
			!strings.HasPrefix(upper, "DESCRIBE") &&
			!strings.HasPrefix(upper, "EXPLAIN") {
			s.jsonError(w, "Only SELECT queries allowed in read-only mode", http.StatusForbidden)
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.config.MaxQueryTime)
	defer cancel()

	start := time.Now()

	// Executa EXPLAIN se solicitado
	var explainResult string
	if req.Explain {
		explainResult = s.runExplain(ctx, req.Query)
	}

	// Detecta tipo de query
	upper := strings.ToUpper(strings.TrimSpace(req.Query))
	isSelect := strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "DESCRIBE") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "WITH")

	var response QueryResponse
	var queryErr error

	if isSelect {
		response, queryErr = s.executeSelect(ctx, req.Query)
	} else {
		response, queryErr = s.executeExec(ctx, req.Query)
	}

	response.Duration = time.Since(start).String()
	response.Explain = explainResult

	if queryErr != nil {
		response.Error = queryErr.Error()
	}

	// Adiciona ao histórico
	s.addHistory(req.Query, time.Since(start), response.RowCount, queryErr)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (s *PlaygroundServer) executeSelect(ctx context.Context, query string) (QueryResponse, error) {
	var response QueryResponse

	rows, err := s.config.DB.QueryContext(ctx, query)
	if err != nil {
		return response, err
	}
	defer rows.Close()

	// Obtém colunas
	columns, err := rows.Columns()
	if err != nil {
		return response, err
	}
	response.Columns = columns

	// Lê resultados
	count := 0
	for rows.Next() && count < s.config.MaxResults {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return response, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Converte bytes para string
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		response.Rows = append(response.Rows, row)
		count++
	}

	response.RowCount = count
	return response, nil
}

func (s *PlaygroundServer) executeExec(ctx context.Context, query string) (QueryResponse, error) {
	var response QueryResponse

	result, err := s.config.DB.ExecContext(ctx, query)
	if err != nil {
		return response, err
	}

	affected, _ := result.RowsAffected()
	response.RowCount = int(affected)
	response.Columns = []string{"affected_rows"}
	response.Rows = []map[string]interface{}{
		{"affected_rows": affected},
	}

	return response, nil
}

func (s *PlaygroundServer) runExplain(ctx context.Context, query string) string {
	var explainQuery string
	switch s.config.Driver {
	case "postgres":
		explainQuery = "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) " + query
	case "mysql":
		explainQuery = "EXPLAIN ANALYZE " + query
	default:
		explainQuery = "EXPLAIN " + query
	}

	rows, err := s.config.DB.QueryContext(ctx, explainQuery)
	if err != nil {
		return fmt.Sprintf("EXPLAIN error: %v", err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			// Tenta múltiplas colunas
			cols, _ := rows.Columns()
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			if err := rows.Scan(valuePtrs...); err == nil {
				for i, col := range cols {
					lines = append(lines, fmt.Sprintf("%s: %v", col, values[i]))
				}
			}
			continue
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (s *PlaygroundServer) handleSchema(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	schema, err := s.loadSchema(ctx)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schema)
}

func (s *PlaygroundServer) handleTables(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tables, err := s.loadTables(ctx)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tables)
}

func (s *PlaygroundServer) handleDescribe(w http.ResponseWriter, r *http.Request) {
	tableName := strings.TrimPrefix(r.URL.Path, "/api/describe/")
	if tableName == "" {
		s.jsonError(w, "Table name required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	columns, err := s.describeTable(ctx, tableName)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(columns)
}

func (s *PlaygroundServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.history)
}

func (s *PlaygroundServer) handleExport(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	query := r.URL.Query().Get("query")

	if query == "" {
		s.jsonError(w, "Query required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	response, err := s.executeSelect(ctx, query)
	if err != nil {
		s.jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	switch format {
	case "csv":
		s.exportCSV(w, response)
	case "json":
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Disposition", "attachment; filename=export.json")
		json.NewEncoder(w).Encode(response.Rows)
	default:
		s.jsonError(w, "Invalid format. Use csv or json", http.StatusBadRequest)
	}
}

func (s *PlaygroundServer) exportCSV(w http.ResponseWriter, response QueryResponse) {
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=export.csv")

	// Cabeçalho
	fmt.Fprintln(w, strings.Join(response.Columns, ","))

	// Dados
	for _, row := range response.Rows {
		values := make([]string, len(response.Columns))
		for i, col := range response.Columns {
			val := fmt.Sprintf("%v", row[col])
			// Escapa vírgulas e aspas
			if strings.Contains(val, ",") || strings.Contains(val, "\"") {
				val = "\"" + strings.ReplaceAll(val, "\"", "\"\"") + "\""
			}
			values[i] = val
		}
		fmt.Fprintln(w, strings.Join(values, ","))
	}
}

type TableInfo struct {
	Name    string       `json:"name"`
	Columns []ColumnInfo `json:"columns"`
}

type ColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

func (s *PlaygroundServer) loadSchema(ctx context.Context) ([]TableInfo, error) {
	tables, err := s.loadTables(ctx)
	if err != nil {
		return nil, err
	}

	var schema []TableInfo
	for _, tableName := range tables {
		columns, err := s.describeTable(ctx, tableName)
		if err != nil {
			continue
		}
		schema = append(schema, TableInfo{
			Name:    tableName,
			Columns: columns,
		})
	}

	return schema, nil
}

func (s *PlaygroundServer) loadTables(ctx context.Context) ([]string, error) {
	var query string
	switch s.config.Driver {
	case "postgres":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'"
	case "mysql":
		query = "SELECT table_name FROM information_schema.tables WHERE table_schema = DATABASE()"
	case "sqlite3":
		query = "SELECT name FROM sqlite_master WHERE type='table'"
	default:
		return nil, fmt.Errorf("unsupported driver: %s", s.config.Driver)
	}

	rows, err := s.config.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		tables = append(tables, name)
	}

	return tables, nil
}

func (s *PlaygroundServer) describeTable(ctx context.Context, tableName string) ([]ColumnInfo, error) {
	var query string
	switch s.config.Driver {
	case "postgres":
		query = fmt.Sprintf(`
			SELECT column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = '%s'
			ORDER BY ordinal_position`, tableName)
	case "mysql":
		query = fmt.Sprintf(`
			SELECT column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_schema = DATABASE() AND table_name = '%s'
			ORDER BY ordinal_position`, tableName)
	case "sqlite3":
		query = fmt.Sprintf("PRAGMA table_info(%s)", tableName)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", s.config.Driver)
	}

	rows, err := s.config.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []ColumnInfo

	if s.config.Driver == "sqlite3" {
		for rows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue interface{}
			if err := rows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				continue
			}
			columns = append(columns, ColumnInfo{
				Name:     name,
				Type:     colType,
				Nullable: notNull == 0,
			})
		}
	} else {
		for rows.Next() {
			var name, colType, isNullable string
			if err := rows.Scan(&name, &colType, &isNullable); err != nil {
				continue
			}
			columns = append(columns, ColumnInfo{
				Name:     name,
				Type:     colType,
				Nullable: isNullable == "YES",
			})
		}
	}

	return columns, nil
}

func (s *PlaygroundServer) addHistory(query string, duration time.Duration, rowCount int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	errStr := ""
	if err != nil {
		errStr = err.Error()
	}

	s.history = append(s.history, QueryHistory{
		ID:        len(s.history) + 1,
		Query:     query,
		Duration:  duration,
		RowCount:  rowCount,
		Error:     errStr,
		Timestamp: time.Now(),
	})

	// Mantém apenas os últimos 100
	if len(s.history) > 100 {
		s.history = s.history[1:]
	}
}

func (s *PlaygroundServer) jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

const playgroundHTML = `<!DOCTYPE html>
<html>
<head>
    <title>Genus Query Playground</title>
    <style>
        * {
            box-sizing: border-box;
            margin: 0;
            padding: 0;
        }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1e1e1e;
            color: #d4d4d4;
            height: 100vh;
            display: flex;
            flex-direction: column;
        }
        header {
            background: #252526;
            padding: 15px 20px;
            display: flex;
            align-items: center;
            justify-content: space-between;
            border-bottom: 1px solid #333;
        }
        .logo {
            font-size: 20px;
            font-weight: bold;
            color: #569cd6;
        }
        .badge {
            background: #4ec9b0;
            color: #1e1e1e;
            padding: 4px 8px;
            border-radius: 4px;
            font-size: 12px;
            margin-left: 10px;
        }
        .badge.readonly {
            background: #dcdcaa;
        }
        main {
            display: flex;
            flex: 1;
            overflow: hidden;
        }
        .sidebar {
            width: 250px;
            background: #252526;
            border-right: 1px solid #333;
            overflow-y: auto;
        }
        .sidebar-section {
            padding: 15px;
            border-bottom: 1px solid #333;
        }
        .sidebar-title {
            font-size: 12px;
            text-transform: uppercase;
            color: #888;
            margin-bottom: 10px;
        }
        .table-item {
            padding: 8px 10px;
            cursor: pointer;
            border-radius: 4px;
            margin-bottom: 4px;
        }
        .table-item:hover {
            background: #2d2d2d;
        }
        .table-item.active {
            background: #094771;
        }
        .editor-container {
            flex: 1;
            display: flex;
            flex-direction: column;
        }
        .query-editor {
            height: 200px;
            border-bottom: 1px solid #333;
            position: relative;
        }
        #query-input {
            width: 100%;
            height: 100%;
            background: #1e1e1e;
            color: #d4d4d4;
            border: none;
            padding: 15px;
            font-family: 'Fira Code', 'Consolas', monospace;
            font-size: 14px;
            resize: none;
        }
        #query-input:focus {
            outline: none;
        }
        .toolbar {
            display: flex;
            padding: 10px;
            gap: 10px;
            background: #252526;
        }
        button {
            background: #0e639c;
            color: white;
            border: none;
            padding: 8px 16px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 13px;
        }
        button:hover {
            background: #1177bb;
        }
        button:disabled {
            background: #444;
            cursor: not-allowed;
        }
        button.secondary {
            background: #333;
        }
        button.secondary:hover {
            background: #444;
        }
        .checkbox-label {
            display: flex;
            align-items: center;
            gap: 5px;
            font-size: 13px;
        }
        .results-container {
            flex: 1;
            overflow: auto;
            padding: 15px;
        }
        .results-info {
            margin-bottom: 10px;
            font-size: 13px;
            color: #888;
        }
        .results-info .success {
            color: #4ec9b0;
        }
        .results-info .error {
            color: #f14c4c;
        }
        table {
            width: 100%;
            border-collapse: collapse;
            font-size: 13px;
        }
        th, td {
            text-align: left;
            padding: 8px 12px;
            border: 1px solid #333;
        }
        th {
            background: #252526;
            position: sticky;
            top: 0;
        }
        tr:nth-child(even) {
            background: #252526;
        }
        tr:hover {
            background: #2d2d2d;
        }
        .explain-output {
            background: #252526;
            padding: 15px;
            border-radius: 4px;
            font-family: monospace;
            white-space: pre-wrap;
            margin-bottom: 15px;
        }
        .history-item {
            padding: 8px 10px;
            border-radius: 4px;
            margin-bottom: 4px;
            cursor: pointer;
            font-size: 12px;
            overflow: hidden;
            text-overflow: ellipsis;
            white-space: nowrap;
        }
        .history-item:hover {
            background: #2d2d2d;
        }
        .history-item .duration {
            color: #888;
            float: right;
        }
        .loading {
            display: inline-block;
            width: 16px;
            height: 16px;
            border: 2px solid #333;
            border-top-color: #569cd6;
            border-radius: 50%;
            animation: spin 1s linear infinite;
            margin-right: 8px;
        }
        @keyframes spin {
            to { transform: rotate(360deg); }
        }
        .column-list {
            font-size: 12px;
            color: #888;
            margin-left: 10px;
        }
    </style>
</head>
<body>
    <header>
        <div style="display: flex; align-items: center;">
            <span class="logo">🎮 Genus Playground</span>
            <span class="badge">{{.Driver}}</span>
            {{if .ReadOnly}}<span class="badge readonly">Read-Only</span>{{end}}
        </div>
        <div>
            <button class="secondary" onclick="exportCSV()">Export CSV</button>
            <button class="secondary" onclick="exportJSON()">Export JSON</button>
        </div>
    </header>

    <main>
        <div class="sidebar">
            <div class="sidebar-section">
                <div class="sidebar-title">Tables</div>
                <div id="tables-list"></div>
            </div>
            <div class="sidebar-section">
                <div class="sidebar-title">History</div>
                <div id="history-list"></div>
            </div>
        </div>

        <div class="editor-container">
            <div class="query-editor">
                <textarea id="query-input" placeholder="Enter your SQL query here...

Examples:
  SELECT * FROM users LIMIT 10;
  SELECT COUNT(*) FROM orders WHERE status = 'completed';

Shortcuts:
  Ctrl+Enter - Execute query
  Ctrl+Shift+Enter - Execute with EXPLAIN"></textarea>
            </div>

            <div class="toolbar">
                <button onclick="executeQuery()" id="run-btn">▶ Run Query</button>
                <button class="secondary" onclick="executeQuery(true)">📊 Explain</button>
                <button class="secondary" onclick="formatQuery()">Format</button>
                <button class="secondary" onclick="clearResults()">Clear</button>
            </div>

            <div class="results-container">
                <div class="results-info" id="results-info"></div>
                <div id="explain-output" class="explain-output" style="display: none;"></div>
                <div id="results-table"></div>
            </div>
        </div>
    </main>

    <script>
        let lastQuery = '';

        // Load tables on startup
        loadTables();
        loadHistory();

        // Keyboard shortcuts
        document.getElementById('query-input').addEventListener('keydown', function(e) {
            if (e.ctrlKey && e.key === 'Enter') {
                e.preventDefault();
                executeQuery(e.shiftKey);
            }
        });

        async function loadTables() {
            try {
                const response = await fetch('/api/schema');
                const schema = await response.json();
                const list = document.getElementById('tables-list');
                list.innerHTML = schema.map(table =>
                    '<div class="table-item" onclick="insertTableName(\'' + table.name + '\')">' +
                    '📋 ' + table.name +
                    '<div class="column-list">' + table.columns.map(c => c.name).join(', ') + '</div>' +
                    '</div>'
                ).join('');
            } catch (e) {
                console.error('Failed to load tables:', e);
            }
        }

        async function loadHistory() {
            try {
                const response = await fetch('/api/history');
                const history = await response.json();
                const list = document.getElementById('history-list');
                list.innerHTML = history.reverse().slice(0, 20).map(h =>
                    '<div class="history-item" onclick="loadQuery(\'' + escapeHtml(h.query) + '\')">' +
                    h.query.substring(0, 40) + (h.query.length > 40 ? '...' : '') +
                    '<span class="duration">' + formatDuration(h.duration) + '</span>' +
                    '</div>'
                ).join('');
            } catch (e) {
                console.error('Failed to load history:', e);
            }
        }

        function escapeHtml(text) {
            return text.replace(/'/g, "\\'").replace(/"/g, '\\"');
        }

        function formatDuration(ns) {
            const ms = ns / 1000000;
            if (ms < 1) return '<1ms';
            if (ms < 1000) return Math.round(ms) + 'ms';
            return (ms / 1000).toFixed(2) + 's';
        }

        function insertTableName(name) {
            const input = document.getElementById('query-input');
            const pos = input.selectionStart;
            const text = input.value;
            input.value = text.substring(0, pos) + name + text.substring(pos);
            input.focus();
        }

        function loadQuery(query) {
            document.getElementById('query-input').value = query;
        }

        async function executeQuery(explain = false) {
            const query = document.getElementById('query-input').value.trim();
            if (!query) return;

            lastQuery = query;

            const btn = document.getElementById('run-btn');
            btn.disabled = true;
            btn.innerHTML = '<span class="loading"></span>Running...';

            try {
                const response = await fetch('/api/query', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ query, explain })
                });

                const data = await response.json();
                displayResults(data);
                loadHistory();
            } catch (e) {
                document.getElementById('results-info').innerHTML =
                    '<span class="error">Error: ' + e.message + '</span>';
            } finally {
                btn.disabled = false;
                btn.innerHTML = '▶ Run Query';
            }
        }

        function displayResults(data) {
            const info = document.getElementById('results-info');
            const table = document.getElementById('results-table');
            const explain = document.getElementById('explain-output');

            if (data.error) {
                info.innerHTML = '<span class="error">Error: ' + data.error + '</span>';
                table.innerHTML = '';
                explain.style.display = 'none';
                return;
            }

            info.innerHTML = '<span class="success">' + data.row_count + ' rows returned in ' + data.duration + '</span>';

            if (data.explain) {
                explain.style.display = 'block';
                explain.textContent = data.explain;
            } else {
                explain.style.display = 'none';
            }

            if (data.columns && data.columns.length > 0) {
                let html = '<table><thead><tr>';
                data.columns.forEach(col => {
                    html += '<th>' + col + '</th>';
                });
                html += '</tr></thead><tbody>';

                (data.rows || []).forEach(row => {
                    html += '<tr>';
                    data.columns.forEach(col => {
                        let val = row[col];
                        if (val === null) val = '<span style="color:#888">NULL</span>';
                        html += '<td>' + val + '</td>';
                    });
                    html += '</tr>';
                });

                html += '</tbody></table>';
                table.innerHTML = html;
            } else {
                table.innerHTML = '<p style="color:#888">No results</p>';
            }
        }

        function clearResults() {
            document.getElementById('results-info').innerHTML = '';
            document.getElementById('results-table').innerHTML = '';
            document.getElementById('explain-output').style.display = 'none';
        }

        function formatQuery() {
            const input = document.getElementById('query-input');
            let query = input.value;

            // Simple formatting
            const keywords = ['SELECT', 'FROM', 'WHERE', 'AND', 'OR', 'ORDER BY', 'GROUP BY',
                            'HAVING', 'LIMIT', 'OFFSET', 'JOIN', 'LEFT JOIN', 'RIGHT JOIN',
                            'INNER JOIN', 'ON', 'INSERT INTO', 'VALUES', 'UPDATE', 'SET', 'DELETE'];

            keywords.forEach(kw => {
                const regex = new RegExp('\\b' + kw + '\\b', 'gi');
                query = query.replace(regex, '\n' + kw);
            });

            input.value = query.trim();
        }

        function exportCSV() {
            if (!lastQuery) {
                alert('Run a query first');
                return;
            }
            window.open('/api/export?format=csv&query=' + encodeURIComponent(lastQuery));
        }

        function exportJSON() {
            if (!lastQuery) {
                alert('Run a query first');
                return;
            }
            window.open('/api/export?format=json&query=' + encodeURIComponent(lastQuery));
        }
    </script>
</body>
</html>`
