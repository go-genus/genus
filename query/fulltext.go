package query

import (
	"fmt"
	"strings"
)

// FullTextConfig configura busca full-text.
type FullTextConfig struct {
	// Language define o idioma para stemming (PostgreSQL: 'english', 'portuguese', etc).
	Language string

	// Columns são as colunas para busca.
	Columns []string

	// Weights são os pesos das colunas (A, B, C, D para PostgreSQL).
	Weights map[string]string
}

// DefaultFullTextConfig retorna configuração padrão.
func DefaultFullTextConfig() FullTextConfig {
	return FullTextConfig{
		Language: "english",
	}
}

// TextSearchVector representa um vetor de busca full-text.
type TextSearchVector string

// Match cria uma condição de match para busca full-text (PostgreSQL).
func (v TextSearchVector) Match(query string, language string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("%s @@ plainto_tsquery('%s', ?)", string(v), language),
		Args: []interface{}{query},
	}
}

// MatchPhrase cria uma condição de match por frase (PostgreSQL).
func (v TextSearchVector) MatchPhrase(phrase string, language string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("%s @@ phraseto_tsquery('%s', ?)", string(v), language),
		Args: []interface{}{phrase},
	}
}

// MatchPrefix cria uma condição de match por prefixo (PostgreSQL).
func (v TextSearchVector) MatchPrefix(prefix string, language string) RawCondition {
	// Adiciona :* para busca por prefixo
	return RawCondition{
		SQL:  fmt.Sprintf("%s @@ to_tsquery('%s', ? || ':*')", string(v), language),
		Args: []interface{}{prefix},
	}
}

// SimpleSearch cria uma busca full-text simples em uma coluna (PostgreSQL).
//
// Exemplo:
//
//	users, _ := genus.Table[User](db).
//	    WhereRaw(query.SimpleSearch("name", "john")).
//	    Find(ctx)
func SimpleSearch(column, term string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("to_tsvector('english', %s) @@ plainto_tsquery('english', ?)", column),
		Args: []interface{}{term},
	}
}

// SimpleSearchWithLang cria uma busca full-text simples com idioma especificado.
func SimpleSearchWithLang(column, term, language string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("to_tsvector('%s', %s) @@ plainto_tsquery('%s', ?)", language, column, language),
		Args: []interface{}{term},
	}
}

// MultiColumnSearch cria uma busca full-text em múltiplas colunas (PostgreSQL).
func MultiColumnSearch(columns []string, term, language string) RawCondition {
	var parts []string
	for _, col := range columns {
		parts = append(parts, fmt.Sprintf("coalesce(%s, '')", col))
	}
	concat := strings.Join(parts, " || ' ' || ")
	return RawCondition{
		SQL:  fmt.Sprintf("to_tsvector('%s', %s) @@ plainto_tsquery('%s', ?)", language, concat, language),
		Args: []interface{}{term},
	}
}

// SimpleSearchMySQL cria uma busca full-text simples no MySQL.
//
// Exemplo:
//
//	users, _ := genus.Table[User](db).
//	    WhereRaw(query.SimpleSearchMySQL([]string{"name", "bio"}, "john")).
//	    Find(ctx)
func SimpleSearchMySQL(columns []string, term string) RawCondition {
	cols := strings.Join(columns, ", ")
	return RawCondition{
		SQL:  fmt.Sprintf("MATCH(%s) AGAINST(? IN NATURAL LANGUAGE MODE)", cols),
		Args: []interface{}{term},
	}
}

// BooleanSearchMySQL cria uma busca full-text com operadores booleanos no MySQL.
// Operadores: + (obrigatório), - (excluir), * (wildcard), "" (frase exata).
//
// Exemplo:
//
//	// Busca por "john" mas não "doe"
//	query.BooleanSearchMySQL([]string{"name"}, "+john -doe")
func BooleanSearchMySQL(columns []string, term string) RawCondition {
	cols := strings.Join(columns, ", ")
	return RawCondition{
		SQL:  fmt.Sprintf("MATCH(%s) AGAINST(? IN BOOLEAN MODE)", cols),
		Args: []interface{}{term},
	}
}

// QueryExpansionSearchMySQL cria uma busca com expansão de query no MySQL.
// O banco adiciona automaticamente termos relacionados à busca.
func QueryExpansionSearchMySQL(columns []string, term string) RawCondition {
	cols := strings.Join(columns, ", ")
	return RawCondition{
		SQL:  fmt.Sprintf("MATCH(%s) AGAINST(? WITH QUERY EXPANSION)", cols),
		Args: []interface{}{term},
	}
}

// WeightedSearch cria uma busca full-text com pesos (PostgreSQL).
// Pesos válidos: 'A' (1.0), 'B' (0.4), 'C' (0.2), 'D' (0.1).
//
// Exemplo:
//
//	query.WeightedSearch(map[string]string{
//	    "title": "A",  // peso maior
//	    "body":  "B",  // peso menor
//	}, "search term", "english")
func WeightedSearch(columnWeights map[string]string, term, language string) RawCondition {
	var parts []string
	for col, weight := range columnWeights {
		parts = append(parts, fmt.Sprintf(
			"setweight(to_tsvector('%s', coalesce(%s, '')), '%s')",
			language, col, weight,
		))
	}
	tsvector := strings.Join(parts, " || ")
	return RawCondition{
		SQL:  fmt.Sprintf("(%s) @@ plainto_tsquery('%s', ?)", tsvector, language),
		Args: []interface{}{term},
	}
}

// RankSearch retorna SQL para ordenar por relevância (PostgreSQL).
// Use com Select para adicionar a coluna de rank.
func RankSearch(columns []string, term, language, alias string) string {
	var parts []string
	for _, col := range columns {
		parts = append(parts, fmt.Sprintf("to_tsvector('%s', coalesce(%s, ''))", language, col))
	}
	tsvector := strings.Join(parts, " || ")
	return fmt.Sprintf("ts_rank((%s), plainto_tsquery('%s', '%s')) AS %s",
		tsvector, language, term, alias)
}

// HeadlineSearch retorna SQL para destacar termos encontrados (PostgreSQL).
// Adiciona tags <b></b> ao redor dos termos encontrados.
func HeadlineSearch(column, term, language string) string {
	return fmt.Sprintf(
		"ts_headline('%s', %s, plainto_tsquery('%s', '%s'), 'StartSel=<b>, StopSel=</b>')",
		language, column, language, term,
	)
}

// LikeSearch cria uma busca LIKE simples (case-insensitive).
// Útil para buscas simples quando full-text não está disponível.
func LikeSearch(column, term string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("LOWER(%s) LIKE LOWER(?)", column),
		Args: []interface{}{"%" + term + "%"},
	}
}

// ILikeSearch cria uma busca ILIKE (PostgreSQL only, case-insensitive).
func ILikeSearch(column, term string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("%s ILIKE ?", column),
		Args: []interface{}{"%" + term + "%"},
	}
}
