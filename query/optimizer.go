package query

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-genus/genus/core"
)

// IndexSuggestion representa uma sugestão de índice.
type IndexSuggestion struct {
	Table       string   `json:"table"`
	Columns     []string `json:"columns"`
	Type        string   `json:"type"` // btree, hash, gin, gist
	Reason      string   `json:"reason"`
	Impact      string   `json:"impact"` // high, medium, low
	CreateSQL   string   `json:"create_sql"`
	EstimatedImprovement float64 `json:"estimated_improvement"` // percentage
}

// QueryAnalysis representa a análise de uma query.
type QueryAnalysis struct {
	SQL               string             `json:"sql"`
	EstimatedCost     float64            `json:"estimated_cost"`
	EstimatedRows     int64              `json:"estimated_rows"`
	SeqScans          []string           `json:"seq_scans"`
	IndexScans        []string           `json:"index_scans"`
	MissingIndexes    []IndexSuggestion  `json:"missing_indexes"`
	Warnings          []string           `json:"warnings"`
	Recommendations   []string           `json:"recommendations"`
	ExecutionPlan     string             `json:"execution_plan"`
}

// QueryOptimizer analisa e otimiza queries.
type QueryOptimizer struct {
	executor core.Executor
	dialect  core.Dialect
}

// NewQueryOptimizer cria um novo otimizador de queries.
func NewQueryOptimizer(executor core.Executor, dialect core.Dialect) *QueryOptimizer {
	return &QueryOptimizer{
		executor: executor,
		dialect:  dialect,
	}
}

// Analyze analisa uma query e retorna recomendações.
func (qo *QueryOptimizer) Analyze(ctx context.Context, sql string, args ...interface{}) (*QueryAnalysis, error) {
	analysis := &QueryAnalysis{
		SQL: sql,
	}

	// Executa EXPLAIN
	explainSQL := "EXPLAIN (FORMAT JSON) " + sql
	if qo.dialect.Placeholder(1) == "?" {
		// MySQL
		explainSQL = "EXPLAIN FORMAT=JSON " + sql
	}

	rows, err := qo.executor.QueryContext(ctx, explainSQL, args...)
	if err != nil {
		// Fallback para EXPLAIN simples
		explainSQL = "EXPLAIN " + sql
		rows, err = qo.executor.QueryContext(ctx, explainSQL, args...)
		if err != nil {
			return nil, err
		}
	}
	defer rows.Close()

	var planLines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			// Tenta scan de múltiplas colunas (MySQL)
			var cols []interface{}
			for i := 0; i < 12; i++ {
				var col interface{}
				cols = append(cols, &col)
			}
			rows.Scan(cols...)
			continue
		}
		planLines = append(planLines, line)
	}

	analysis.ExecutionPlan = strings.Join(planLines, "\n")

	// Analisa o plano
	qo.analyzePlan(analysis)

	// Gera sugestões de índices
	qo.suggestIndexes(analysis, sql)

	// Adiciona recomendações gerais
	qo.addRecommendations(analysis, sql)

	return analysis, nil
}

// analyzePlan extrai informações do plano de execução.
func (qo *QueryOptimizer) analyzePlan(analysis *QueryAnalysis) {
	plan := strings.ToLower(analysis.ExecutionPlan)

	// Detecta Seq Scans
	seqScanRegex := regexp.MustCompile(`seq scan on (\w+)`)
	matches := seqScanRegex.FindAllStringSubmatch(plan, -1)
	for _, match := range matches {
		if len(match) > 1 {
			analysis.SeqScans = append(analysis.SeqScans, match[1])
		}
	}

	// Detecta Index Scans
	indexScanRegex := regexp.MustCompile(`index scan using (\w+) on (\w+)`)
	matches = indexScanRegex.FindAllStringSubmatch(plan, -1)
	for _, match := range matches {
		if len(match) > 2 {
			analysis.IndexScans = append(analysis.IndexScans, match[2])
		}
	}

	// Detecta custo estimado
	costRegex := regexp.MustCompile(`cost=[\d.]+\.\.(\d+\.?\d*)`)
	if match := costRegex.FindStringSubmatch(plan); len(match) > 1 {
		fmt.Sscanf(match[1], "%f", &analysis.EstimatedCost)
	}

	// Detecta rows estimadas
	rowsRegex := regexp.MustCompile(`rows=(\d+)`)
	if match := rowsRegex.FindStringSubmatch(plan); len(match) > 1 {
		fmt.Sscanf(match[1], "%d", &analysis.EstimatedRows)
	}

	// Warnings para Seq Scans em tabelas grandes
	if len(analysis.SeqScans) > 0 {
		analysis.Warnings = append(analysis.Warnings,
			fmt.Sprintf("Sequential scan detected on tables: %s. Consider adding indexes.",
				strings.Join(analysis.SeqScans, ", ")))
	}
}

// suggestIndexes sugere índices baseado na query.
func (qo *QueryOptimizer) suggestIndexes(analysis *QueryAnalysis, sql string) {
	sql = strings.ToLower(sql)

	// Extrai tabela do FROM
	tableRegex := regexp.MustCompile(`from\s+["']?(\w+)["']?`)
	tableMatch := tableRegex.FindStringSubmatch(sql)
	var tableName string
	if len(tableMatch) > 1 {
		tableName = tableMatch[1]
	}

	if tableName == "" {
		return
	}

	// Extrai colunas do WHERE
	whereRegex := regexp.MustCompile(`where\s+(.+?)(?:order|group|limit|$)`)
	whereMatch := whereRegex.FindStringSubmatch(sql)
	if len(whereMatch) > 1 {
		whereClause := whereMatch[1]

		// Extrai colunas usadas em condições
		colRegex := regexp.MustCompile(`["']?(\w+)["']?\s*(?:=|>|<|>=|<=|<>|!=|like|in|between)`)
		colMatches := colRegex.FindAllStringSubmatch(whereClause, -1)

		var columns []string
		seen := make(map[string]bool)
		for _, match := range colMatches {
			if len(match) > 1 && !seen[match[1]] {
				// Ignora valores numéricos e keywords
				if !isNumeric(match[1]) && !isKeyword(match[1]) {
					columns = append(columns, match[1])
					seen[match[1]] = true
				}
			}
		}

		if len(columns) > 0 {
			// Verifica se já existe índice (para tabelas com seq scan)
			for _, seqTable := range analysis.SeqScans {
				if seqTable == tableName {
					suggestion := IndexSuggestion{
						Table:   tableName,
						Columns: columns,
						Type:    "btree",
						Reason:  "Sequential scan detected with WHERE clause filters",
						Impact:  "high",
						EstimatedImprovement: 50.0,
					}

					// Gera SQL de criação
					indexName := fmt.Sprintf("idx_%s_%s", tableName, strings.Join(columns, "_"))
					quotedCols := make([]string, len(columns))
					for i, col := range columns {
						quotedCols[i] = qo.dialect.QuoteIdentifier(col)
					}
					suggestion.CreateSQL = fmt.Sprintf("CREATE INDEX %s ON %s (%s)",
						qo.dialect.QuoteIdentifier(indexName),
						qo.dialect.QuoteIdentifier(tableName),
						strings.Join(quotedCols, ", "))

					analysis.MissingIndexes = append(analysis.MissingIndexes, suggestion)
				}
			}
		}
	}

	// Sugere índice para ORDER BY
	orderRegex := regexp.MustCompile(`order\s+by\s+["']?(\w+)["']?`)
	orderMatch := orderRegex.FindStringSubmatch(sql)
	if len(orderMatch) > 1 {
		orderCol := orderMatch[1]

		// Verifica se a tabela tem seq scan
		for _, seqTable := range analysis.SeqScans {
			if seqTable == tableName {
				suggestion := IndexSuggestion{
					Table:   tableName,
					Columns: []string{orderCol},
					Type:    "btree",
					Reason:  "ORDER BY without index causes sorting in memory",
					Impact:  "medium",
					EstimatedImprovement: 30.0,
				}

				indexName := fmt.Sprintf("idx_%s_%s", tableName, orderCol)
				suggestion.CreateSQL = fmt.Sprintf("CREATE INDEX %s ON %s (%s)",
					qo.dialect.QuoteIdentifier(indexName),
					qo.dialect.QuoteIdentifier(tableName),
					qo.dialect.QuoteIdentifier(orderCol))

				analysis.MissingIndexes = append(analysis.MissingIndexes, suggestion)
			}
		}
	}
}

// addRecommendations adiciona recomendações gerais.
func (qo *QueryOptimizer) addRecommendations(analysis *QueryAnalysis, sql string) {
	sql = strings.ToLower(sql)

	// SELECT *
	if strings.Contains(sql, "select *") {
		analysis.Recommendations = append(analysis.Recommendations,
			"Avoid SELECT * - specify only needed columns to reduce I/O and memory usage")
	}

	// LIKE com wildcard no início
	if regexp.MustCompile(`like\s+['"]%`).MatchString(sql) {
		analysis.Recommendations = append(analysis.Recommendations,
			"LIKE with leading wildcard (%...) cannot use indexes. Consider full-text search instead")
	}

	// OR conditions
	if strings.Count(sql, " or ") > 2 {
		analysis.Recommendations = append(analysis.Recommendations,
			"Multiple OR conditions may prevent index usage. Consider using IN() or UNION")
	}

	// Subquery in WHERE
	if strings.Contains(sql, "where") && strings.Contains(sql, "select") {
		analysis.Recommendations = append(analysis.Recommendations,
			"Subquery in WHERE clause detected. Consider using JOIN for better performance")
	}

	// NOT IN
	if strings.Contains(sql, "not in") {
		analysis.Recommendations = append(analysis.Recommendations,
			"NOT IN can be slow with large datasets. Consider using NOT EXISTS or LEFT JOIN")
	}

	// DISTINCT
	if strings.Contains(sql, "distinct") {
		analysis.Recommendations = append(analysis.Recommendations,
			"DISTINCT requires sorting/hashing. Ensure it's necessary or use GROUP BY with indexes")
	}

	// Custo alto
	if analysis.EstimatedCost > 10000 {
		analysis.Recommendations = append(analysis.Recommendations,
			fmt.Sprintf("High estimated cost (%.0f). Review indexes and query structure", analysis.EstimatedCost))
	}
}

// isNumeric verifica se a string é numérica.
func isNumeric(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

// isKeyword verifica se é uma keyword SQL.
func isKeyword(s string) bool {
	keywords := map[string]bool{
		"and": true, "or": true, "not": true, "null": true,
		"true": true, "false": true, "is": true,
	}
	return keywords[strings.ToLower(s)]
}

// OptimizeBuilder adiciona análise ao Builder.
func (b *Builder[T]) Optimize(ctx context.Context) (*QueryAnalysis, error) {
	sql, args := b.buildDryRunSelectQuery()
	optimizer := NewQueryOptimizer(b.executor, b.dialect)
	return optimizer.Analyze(ctx, sql, args...)
}

// AutoIndex cria índices sugeridos automaticamente.
func (qo *QueryOptimizer) AutoIndex(ctx context.Context, analysis *QueryAnalysis, dryRun bool) ([]string, error) {
	var executed []string

	for _, suggestion := range analysis.MissingIndexes {
		if dryRun {
			executed = append(executed, suggestion.CreateSQL)
			continue
		}

		if _, err := qo.executor.ExecContext(ctx, suggestion.CreateSQL); err != nil {
			// Índice pode já existir
			if !strings.Contains(err.Error(), "already exists") {
				return executed, err
			}
		}
		executed = append(executed, suggestion.CreateSQL)
	}

	return executed, nil
}

// TableStats representa estatísticas de uma tabela.
type TableStats struct {
	TableName    string  `json:"table_name"`
	RowCount     int64   `json:"row_count"`
	TotalSize    string  `json:"total_size"`
	IndexSize    string  `json:"index_size"`
	SeqScans     int64   `json:"seq_scans"`
	IndexScans   int64   `json:"index_scans"`
	DeadTuples   int64   `json:"dead_tuples"`
	LastVacuum   string  `json:"last_vacuum"`
	LastAnalyze  string  `json:"last_analyze"`
}

// GetTableStats obtém estatísticas de uma tabela (PostgreSQL).
func (qo *QueryOptimizer) GetTableStats(ctx context.Context, tableName string) (*TableStats, error) {
	if qo.dialect.Placeholder(1) == "?" {
		return qo.getTableStatsMySQL(ctx, tableName)
	}

	query := `
		SELECT
			relname as table_name,
			n_live_tup as row_count,
			pg_size_pretty(pg_total_relation_size(relid)) as total_size,
			pg_size_pretty(pg_indexes_size(relid)) as index_size,
			seq_scan as seq_scans,
			idx_scan as index_scans,
			n_dead_tup as dead_tuples,
			COALESCE(last_vacuum::text, 'never') as last_vacuum,
			COALESCE(last_analyze::text, 'never') as last_analyze
		FROM pg_stat_user_tables
		WHERE relname = $1
	`

	var stats TableStats
	row := qo.executor.QueryRowContext(ctx, query, tableName)
	err := row.Scan(
		&stats.TableName,
		&stats.RowCount,
		&stats.TotalSize,
		&stats.IndexSize,
		&stats.SeqScans,
		&stats.IndexScans,
		&stats.DeadTuples,
		&stats.LastVacuum,
		&stats.LastAnalyze,
	)

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// getTableStatsMySQL obtém estatísticas para MySQL.
func (qo *QueryOptimizer) getTableStatsMySQL(ctx context.Context, tableName string) (*TableStats, error) {
	query := `
		SELECT
			TABLE_NAME,
			TABLE_ROWS,
			CONCAT(ROUND(DATA_LENGTH / 1024 / 1024, 2), ' MB') as total_size,
			CONCAT(ROUND(INDEX_LENGTH / 1024 / 1024, 2), ' MB') as index_size
		FROM information_schema.TABLES
		WHERE TABLE_NAME = ?
	`

	var stats TableStats
	var dataSize, indexSize string
	row := qo.executor.QueryRowContext(ctx, query, tableName)
	err := row.Scan(&stats.TableName, &stats.RowCount, &dataSize, &indexSize)

	if err != nil {
		return nil, err
	}

	stats.TotalSize = dataSize
	stats.IndexSize = indexSize

	return &stats, nil
}

// ListUnusedIndexes lista índices não utilizados (PostgreSQL).
func (qo *QueryOptimizer) ListUnusedIndexes(ctx context.Context) ([]string, error) {
	if qo.dialect.Placeholder(1) == "?" {
		return nil, fmt.Errorf("unused index detection requires PostgreSQL")
	}

	query := `
		SELECT indexrelname
		FROM pg_stat_user_indexes
		WHERE idx_scan = 0
		AND indexrelname NOT LIKE '%_pkey'
		ORDER BY pg_relation_size(indexrelid) DESC
	`

	rows, err := qo.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indexes []string
	for rows.Next() {
		var indexName string
		if err := rows.Scan(&indexName); err != nil {
			return nil, err
		}
		indexes = append(indexes, indexName)
	}

	return indexes, rows.Err()
}

// ListDuplicateIndexes lista índices duplicados (PostgreSQL).
func (qo *QueryOptimizer) ListDuplicateIndexes(ctx context.Context) ([]string, error) {
	if qo.dialect.Placeholder(1) == "?" {
		return nil, fmt.Errorf("duplicate index detection requires PostgreSQL")
	}

	query := `
		SELECT pg_size_pretty(sum(pg_relation_size(idx))::bigint) as size,
			   (array_agg(idx))[1] as idx1, (array_agg(idx))[2] as idx2,
			   (array_agg(idx))[3] as idx3, (array_agg(idx))[4] as idx4
		FROM (
			SELECT indexrelid::regclass as idx, (indrelid::text ||E'\n'|| indclass::text ||E'\n'|| indkey::text ||E'\n'||
				   coalesce(indexprs::text,'')||E'\n' || coalesce(indpred::text,'')) as key
			FROM pg_index
		) sub
		GROUP BY key
		HAVING count(*) > 1
		ORDER BY sum(pg_relation_size(idx)) DESC
	`

	rows, err := qo.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var duplicates []string
	for rows.Next() {
		var size, idx1, idx2, idx3, idx4 *string
		if err := rows.Scan(&size, &idx1, &idx2, &idx3, &idx4); err != nil {
			continue
		}
		if idx1 != nil && idx2 != nil {
			duplicates = append(duplicates, fmt.Sprintf("%s (duplicate of %s)", *idx2, *idx1))
		}
	}

	return duplicates, rows.Err()
}
