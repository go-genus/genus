package main

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// runREPL executa o REPL interativo.
func runREPL() error {
	args := os.Args[2:]

	// Parse flags
	var (
		dsn        = os.Getenv("DATABASE_URL")
		driver     = "postgres"
		format     = "table"
		showTiming = true
		limit      = 100
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--driver":
			if i+1 < len(args) {
				driver = args[i+1]
				i++
			}
		case "--format":
			if i+1 < len(args) {
				format = args[i+1]
				i++
			}
		case "--no-timing":
			showTiming = false
		case "--limit":
			if i+1 < len(args) {
				limit, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "-h", "--help":
			printREPLUsage()
			return nil
		}
	}

	if dsn == "" {
		return fmt.Errorf("database connection string required (--dsn or DATABASE_URL)")
	}

	// Conecta ao banco
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer db.Close()

	// Testa conexão
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	repl := &REPLContext{
		db:           db,
		driver:       driver,
		format:       format,
		showTiming:   showTiming,
		defaultLimit: limit,
		history:      make([]string, 0),
	}

	// Carrega schema para autocomplete
	if err := repl.loadSchema(); err != nil {
		fmt.Printf("Warning: failed to load schema: %v\n", err)
	}

	// Banner
	printBanner()

	// Loop principal
	scanner := bufio.NewScanner(os.Stdin)
	var multiLine strings.Builder

	for {
		if multiLine.Len() == 0 {
			fmt.Print("genus> ")
		} else {
			fmt.Print("    -> ")
		}

		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())

		// Comandos especiais
		if multiLine.Len() == 0 && strings.HasPrefix(line, "\\") {
			repl.handleCommand(line)
			continue
		}

		// Acumula linhas para queries multi-linha
		multiLine.WriteString(line)
		multiLine.WriteString(" ")

		// Verifica se a query está completa (termina com ;)
		if strings.HasSuffix(line, ";") {
			query := strings.TrimSuffix(strings.TrimSpace(multiLine.String()), ";")
			multiLine.Reset()

			if query != "" {
				repl.executeQuery(query)
			}
		}
	}

	return nil
}

func printREPLUsage() {
	fmt.Println(`Interactive query builder REPL

Usage:
  genus repl [flags]

Flags:
  --dsn <string>       Database connection string (or use DATABASE_URL env)
  --driver <string>    Database driver: postgres, mysql, sqlite3 (default: postgres)
  --format <string>    Output format: table, json, csv (default: table)
  --no-timing          Disable query timing display
  --limit <int>        Default result limit (default: 100)
  -h, --help           Show this help message

Examples:
  genus repl --dsn "postgres://user:pass@localhost/db"
  DATABASE_URL="postgres://..." genus repl
  genus repl --driver mysql --dsn "user:pass@tcp(localhost)/db"

Commands inside REPL:
  \h, \help     Show help
  \dt           List tables
  \d TABLE      Describe table
  \e QUERY      Explain query
  \timing       Toggle timing display
  \format FMT   Set output format
  \begin        Start transaction
  \commit       Commit transaction
  \rollback     Rollback transaction
  \history      Show query history
  \clear        Clear screen
  \q, \quit     Exit`)
}

// REPLContext mantém o estado do REPL.
type REPLContext struct {
	db           *sql.DB
	driver       string
	format       string
	showTiming   bool
	defaultLimit int
	history      []string
	historyIndex int
	inTx         bool
	tx           *sql.Tx
	schema       *SchemaInfo
}

// SchemaInfo armazena informações do schema para autocomplete.
type SchemaInfo struct {
	Tables  map[string]*TableSchema
	Columns map[string][]string
}

// TableSchema informações de uma tabela.
type TableSchema struct {
	Name    string
	Columns []ColumnSchema
}

// ColumnSchema informações de uma coluna.
type ColumnSchema struct {
	Name     string
	Type     string
	Nullable bool
}

func printBanner() {
	fmt.Println(`
╔═══════════════════════════════════════════════════════════╗
║              Genus Interactive Query Builder               ║
╠═══════════════════════════════════════════════════════════╣
║  Commands:                                                 ║
║    \h, \help     Show help                                ║
║    \dt           List tables                              ║
║    \d TABLE      Describe table                           ║
║    \e QUERY      Explain query                            ║
║    \timing       Toggle timing display                    ║
║    \format FMT   Set output format (table/json/csv)       ║
║    \begin        Start transaction                        ║
║    \commit       Commit transaction                       ║
║    \rollback     Rollback transaction                     ║
║    \history      Show query history                       ║
║    \clear        Clear screen                             ║
║    \q, \quit     Exit                                     ║
╚═══════════════════════════════════════════════════════════╝`)
}

func (r *REPLContext) handleCommand(cmd string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return
	}

	switch parts[0] {
	case "\\h", "\\help":
		printBanner()

	case "\\q", "\\quit", "\\exit":
		if r.inTx {
			fmt.Println("Warning: Rolling back uncommitted transaction")
			r.tx.Rollback()
		}
		fmt.Println("Bye!")
		os.Exit(0)

	case "\\dt":
		r.listTables()

	case "\\d":
		if len(parts) < 2 {
			fmt.Println("Usage: \\d TABLE_NAME")
			return
		}
		r.describeTable(parts[1])

	case "\\e":
		if len(parts) < 2 {
			fmt.Println("Usage: \\e QUERY")
			return
		}
		query := strings.Join(parts[1:], " ")
		r.explainQuery(query)

	case "\\timing":
		r.showTiming = !r.showTiming
		fmt.Printf("Timing is %s\n", map[bool]string{true: "on", false: "off"}[r.showTiming])

	case "\\format":
		if len(parts) < 2 {
			fmt.Printf("Current format: %s\n", r.format)
			return
		}
		switch parts[1] {
		case "table", "json", "csv":
			r.format = parts[1]
			fmt.Printf("Output format set to: %s\n", r.format)
		default:
			fmt.Println("Invalid format. Use: table, json, csv")
		}

	case "\\begin":
		r.beginTransaction()

	case "\\commit":
		r.commitTransaction()

	case "\\rollback":
		r.rollbackTransaction()

	case "\\history":
		r.showHistory()

	case "\\clear":
		fmt.Print("\033[H\033[2J")

	case "\\schema":
		r.showSchemaInfo()

	default:
		fmt.Printf("Unknown command: %s\n", parts[0])
	}
}

func (r *REPLContext) loadSchema() error {
	r.schema = &SchemaInfo{
		Tables:  make(map[string]*TableSchema),
		Columns: make(map[string][]string),
	}

	ctx := context.Background()
	var query string

	switch r.driver {
	case "postgres":
		query = `
			SELECT table_name, column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_schema = 'public'
			ORDER BY table_name, ordinal_position`
	case "mysql":
		query = `
			SELECT table_name, column_name, data_type, is_nullable
			FROM information_schema.columns
			WHERE table_schema = DATABASE()
			ORDER BY table_name, ordinal_position`
	case "sqlite3":
		return r.loadSQLiteSchema()
	default:
		return fmt.Errorf("unsupported driver: %s", r.driver)
	}

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var tableName, columnName, dataType, isNullable string
		if err := rows.Scan(&tableName, &columnName, &dataType, &isNullable); err != nil {
			continue
		}

		if _, ok := r.schema.Tables[tableName]; !ok {
			r.schema.Tables[tableName] = &TableSchema{
				Name:    tableName,
				Columns: make([]ColumnSchema, 0),
			}
		}

		r.schema.Tables[tableName].Columns = append(r.schema.Tables[tableName].Columns, ColumnSchema{
			Name:     columnName,
			Type:     dataType,
			Nullable: isNullable == "YES",
		})

		r.schema.Columns[tableName] = append(r.schema.Columns[tableName], columnName)
	}

	return nil
}

func (r *REPLContext) loadSQLiteSchema() error {
	ctx := context.Background()

	// Lista tabelas
	rows, err := r.db.QueryContext(ctx, "SELECT name FROM sqlite_master WHERE type='table'")
	if err != nil {
		return err
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

	// Para cada tabela, obtém colunas
	for _, table := range tables {
		r.schema.Tables[table] = &TableSchema{
			Name:    table,
			Columns: make([]ColumnSchema, 0),
		}

		colRows, err := r.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
		if err != nil {
			continue
		}

		for colRows.Next() {
			var cid int
			var name, colType string
			var notNull, pk int
			var dfltValue interface{}

			if err := colRows.Scan(&cid, &name, &colType, &notNull, &dfltValue, &pk); err != nil {
				continue
			}

			r.schema.Tables[table].Columns = append(r.schema.Tables[table].Columns, ColumnSchema{
				Name:     name,
				Type:     colType,
				Nullable: notNull == 0,
			})

			r.schema.Columns[table] = append(r.schema.Columns[table], name)
		}
		colRows.Close()
	}

	return nil
}

func (r *REPLContext) listTables() {
	if r.schema == nil {
		fmt.Println("Schema not loaded")
		return
	}

	fmt.Println("\nTables:")
	fmt.Println(strings.Repeat("-", 40))
	for name, table := range r.schema.Tables {
		fmt.Printf("  %-30s (%d columns)\n", name, len(table.Columns))
	}
	fmt.Println()
}

func (r *REPLContext) describeTable(tableName string) {
	if r.schema == nil {
		fmt.Println("Schema not loaded")
		return
	}

	table, ok := r.schema.Tables[tableName]
	if !ok {
		fmt.Printf("Table '%s' not found\n", tableName)
		return
	}

	fmt.Printf("\nTable: %s\n", table.Name)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("%-20s %-20s %-10s\n", "Column", "Type", "Nullable")
	fmt.Println(strings.Repeat("-", 60))

	for _, col := range table.Columns {
		nullable := "NO"
		if col.Nullable {
			nullable = "YES"
		}
		fmt.Printf("%-20s %-20s %-10s\n", col.Name, col.Type, nullable)
	}
	fmt.Println()
}

func (r *REPLContext) showSchemaInfo() {
	if r.schema == nil {
		fmt.Println("Schema not loaded")
		return
	}

	for _, table := range r.schema.Tables {
		fmt.Printf("\n%s:\n", table.Name)
		for _, col := range table.Columns {
			fmt.Printf("  - %s (%s)\n", col.Name, col.Type)
		}
	}
}

func (r *REPLContext) executeQuery(query string) {
	// Adiciona ao histórico
	r.history = append(r.history, query)

	ctx := context.Background()
	start := time.Now()

	// Detecta tipo de query
	upperQuery := strings.ToUpper(strings.TrimSpace(query))
	isSelect := strings.HasPrefix(upperQuery, "SELECT") ||
		strings.HasPrefix(upperQuery, "WITH") ||
		strings.HasPrefix(upperQuery, "SHOW") ||
		strings.HasPrefix(upperQuery, "DESCRIBE") ||
		strings.HasPrefix(upperQuery, "EXPLAIN")

	var err error
	if isSelect {
		err = r.executeSelectQuery(ctx, query)
	} else {
		err = r.executeExecQuery(ctx, query)
	}

	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	if r.showTiming {
		fmt.Printf("Time: %v\n", elapsed)
	}
}

func (r *REPLContext) executeSelectQuery(ctx context.Context, query string) error {
	var rows *sql.Rows
	var err error

	if r.inTx && r.tx != nil {
		rows, err = r.tx.QueryContext(ctx, query)
	} else {
		rows, err = r.db.QueryContext(ctx, query)
	}

	if err != nil {
		return err
	}
	defer rows.Close()

	// Obtém colunas
	columns, err := rows.Columns()
	if err != nil {
		return err
	}

	// Lê resultados
	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return err
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
		results = append(results, row)
	}

	// Formata saída
	switch r.format {
	case "json":
		r.printJSON(results)
	case "csv":
		r.printCSV(columns, results)
	default:
		r.printTable(columns, results)
	}

	fmt.Printf("\n(%d rows)\n", len(results))
	return nil
}

func (r *REPLContext) executeExecQuery(ctx context.Context, query string) error {
	var result sql.Result
	var err error

	if r.inTx && r.tx != nil {
		result, err = r.tx.ExecContext(ctx, query)
	} else {
		result, err = r.db.ExecContext(ctx, query)
	}

	if err != nil {
		return err
	}

	affected, _ := result.RowsAffected()
	lastID, _ := result.LastInsertId()

	fmt.Printf("Query OK, %d row(s) affected", affected)
	if lastID > 0 {
		fmt.Printf(", last insert ID: %d", lastID)
	}
	fmt.Println()

	// Recarrega schema se foi DDL
	upperQuery := strings.ToUpper(query)
	if strings.Contains(upperQuery, "CREATE") ||
		strings.Contains(upperQuery, "ALTER") ||
		strings.Contains(upperQuery, "DROP") {
		r.loadSchema()
	}

	return nil
}

func (r *REPLContext) printTable(columns []string, results []map[string]interface{}) {
	if len(results) == 0 {
		fmt.Println("(empty result set)")
		return
	}

	// Calcula largura das colunas
	widths := make([]int, len(columns))
	for i, col := range columns {
		widths[i] = len(col)
	}

	for _, row := range results {
		for i, col := range columns {
			val := fmt.Sprintf("%v", row[col])
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
			// Limita largura máxima
			if widths[i] > 50 {
				widths[i] = 50
			}
		}
	}

	// Imprime cabeçalho
	fmt.Println()
	printTableSeparator(widths)
	printTableRow(columns, widths)
	printTableSeparator(widths)

	// Imprime dados
	for _, row := range results {
		values := make([]string, len(columns))
		for i, col := range columns {
			val := fmt.Sprintf("%v", row[col])
			if row[col] == nil {
				val = "NULL"
			}
			// Trunca se muito longo
			if len(val) > 50 {
				val = val[:47] + "..."
			}
			values[i] = val
		}
		printTableRow(values, widths)
	}

	printTableSeparator(widths)
}

func printTableSeparator(widths []int) {
	fmt.Print("+")
	for _, w := range widths {
		fmt.Print(strings.Repeat("-", w+2))
		fmt.Print("+")
	}
	fmt.Println()
}

func printTableRow(values []string, widths []int) {
	fmt.Print("|")
	for i, val := range values {
		fmt.Printf(" %-*s |", widths[i], val)
	}
	fmt.Println()
}

func (r *REPLContext) printJSON(results []map[string]interface{}) {
	output, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(output))
}

func (r *REPLContext) printCSV(columns []string, results []map[string]interface{}) {
	// Cabeçalho
	fmt.Println(strings.Join(columns, ","))

	// Dados
	for _, row := range results {
		values := make([]string, len(columns))
		for i, col := range columns {
			val := fmt.Sprintf("%v", row[col])
			// Escapa valores com vírgula ou aspas
			if strings.Contains(val, ",") || strings.Contains(val, "\"") {
				val = "\"" + strings.ReplaceAll(val, "\"", "\"\"") + "\""
			}
			values[i] = val
		}
		fmt.Println(strings.Join(values, ","))
	}
}

func (r *REPLContext) explainQuery(query string) {
	ctx := context.Background()

	var explainQuery string
	switch r.driver {
	case "postgres":
		explainQuery = "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) " + query
	case "mysql":
		explainQuery = "EXPLAIN ANALYZE " + query
	default:
		explainQuery = "EXPLAIN " + query
	}

	rows, err := r.db.QueryContext(ctx, explainQuery)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer rows.Close()

	fmt.Println("\nQuery Plan:")
	fmt.Println(strings.Repeat("-", 80))

	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			// Tenta scan de múltiplas colunas (MySQL)
			cols, _ := rows.Columns()
			values := make([]interface{}, len(cols))
			valuePtrs := make([]interface{}, len(cols))
			for i := range values {
				valuePtrs[i] = &values[i]
			}
			if err := rows.Scan(valuePtrs...); err == nil {
				for i, col := range cols {
					fmt.Printf("%s: %v\n", col, values[i])
				}
			}
			continue
		}
		fmt.Println(line)
	}
	fmt.Println()
}

func (r *REPLContext) beginTransaction() {
	if r.inTx {
		fmt.Println("Already in a transaction")
		return
	}

	tx, err := r.db.Begin()
	if err != nil {
		fmt.Printf("Error starting transaction: %v\n", err)
		return
	}

	r.tx = tx
	r.inTx = true
	fmt.Println("Transaction started")
}

func (r *REPLContext) commitTransaction() {
	if !r.inTx {
		fmt.Println("Not in a transaction")
		return
	}

	if err := r.tx.Commit(); err != nil {
		fmt.Printf("Error committing transaction: %v\n", err)
		return
	}

	r.tx = nil
	r.inTx = false
	fmt.Println("Transaction committed")
}

func (r *REPLContext) rollbackTransaction() {
	if !r.inTx {
		fmt.Println("Not in a transaction")
		return
	}

	if err := r.tx.Rollback(); err != nil {
		fmt.Printf("Error rolling back transaction: %v\n", err)
		return
	}

	r.tx = nil
	r.inTx = false
	fmt.Println("Transaction rolled back")
}

func (r *REPLContext) showHistory() {
	fmt.Println("\nQuery History:")
	fmt.Println(strings.Repeat("-", 60))
	for i, query := range r.history {
		// Trunca queries longas
		display := query
		if len(display) > 60 {
			display = display[:57] + "..."
		}
		fmt.Printf("%3d: %s\n", i+1, display)
	}
	fmt.Println()
}

// SuggestCompletion sugere completions para o texto atual.
func (r *REPLContext) SuggestCompletion(text string) []string {
	var suggestions []string

	// Extrai última palavra
	words := strings.Fields(text)
	if len(words) == 0 {
		return suggestions
	}

	lastWord := strings.ToLower(words[len(words)-1])

	// Após FROM ou JOIN, sugere tabelas
	if len(words) >= 2 {
		prevWord := strings.ToUpper(words[len(words)-2])
		if prevWord == "FROM" || prevWord == "JOIN" || prevWord == "INTO" || prevWord == "UPDATE" {
			for tableName := range r.schema.Tables {
				if strings.HasPrefix(strings.ToLower(tableName), lastWord) {
					suggestions = append(suggestions, tableName)
				}
			}
			return suggestions
		}
	}

	// Detecta tabela no contexto para sugerir colunas
	tablePattern := regexp.MustCompile(`(?i)(?:FROM|JOIN|UPDATE)\s+(\w+)`)
	matches := tablePattern.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			tableName := match[1]
			if cols, ok := r.schema.Columns[tableName]; ok {
				for _, col := range cols {
					if strings.HasPrefix(strings.ToLower(col), lastWord) {
						suggestions = append(suggestions, col)
					}
				}
			}
		}
	}

	// Keywords SQL
	keywords := []string{
		"SELECT", "FROM", "WHERE", "AND", "OR", "NOT", "IN", "LIKE",
		"ORDER", "BY", "ASC", "DESC", "LIMIT", "OFFSET", "GROUP",
		"HAVING", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER", "ON",
		"INSERT", "INTO", "VALUES", "UPDATE", "SET", "DELETE",
		"CREATE", "TABLE", "INDEX", "DROP", "ALTER", "ADD", "COLUMN",
		"NULL", "NOT NULL", "PRIMARY", "KEY", "FOREIGN", "REFERENCES",
		"COUNT", "SUM", "AVG", "MAX", "MIN", "DISTINCT", "AS",
	}

	for _, kw := range keywords {
		if strings.HasPrefix(strings.ToUpper(kw), strings.ToUpper(lastWord)) {
			suggestions = append(suggestions, kw)
		}
	}

	return suggestions
}
