package query

import (
	"strings"
	"testing"
)

func TestDefaultFullTextConfig(t *testing.T) {
	config := DefaultFullTextConfig()
	if config.Language != "english" {
		t.Errorf("Language = %q, want 'english'", config.Language)
	}
}

func TestTextSearchVector_Match(t *testing.T) {
	v := TextSearchVector("search_vector")
	cond := v.Match("hello world", "english")
	if !strings.Contains(cond.SQL, "@@") {
		t.Errorf("Match SQL = %q, should contain @@", cond.SQL)
	}
	if !strings.Contains(cond.SQL, "plainto_tsquery") {
		t.Errorf("Match SQL = %q, should contain plainto_tsquery", cond.SQL)
	}
	if len(cond.Args) != 1 {
		t.Errorf("Match args len = %d, want 1", len(cond.Args))
	}
}

func TestTextSearchVector_MatchPhrase(t *testing.T) {
	v := TextSearchVector("search_vector")
	cond := v.MatchPhrase("exact phrase", "english")
	if !strings.Contains(cond.SQL, "phraseto_tsquery") {
		t.Errorf("MatchPhrase SQL = %q, should contain phraseto_tsquery", cond.SQL)
	}
}

func TestTextSearchVector_MatchPrefix(t *testing.T) {
	v := TextSearchVector("search_vector")
	cond := v.MatchPrefix("hel", "english")
	if !strings.Contains(cond.SQL, "to_tsquery") {
		t.Errorf("MatchPrefix SQL = %q, should contain to_tsquery", cond.SQL)
	}
	if !strings.Contains(cond.SQL, ":*") {
		t.Errorf("MatchPrefix SQL = %q, should contain :*", cond.SQL)
	}
}

func TestSimpleSearch(t *testing.T) {
	cond := SimpleSearch("name", "john")
	if !strings.Contains(cond.SQL, "to_tsvector") {
		t.Errorf("SimpleSearch SQL = %q, should contain to_tsvector", cond.SQL)
	}
	if cond.Args[0] != "john" {
		t.Errorf("SimpleSearch arg = %v, want 'john'", cond.Args[0])
	}
}

func TestSimpleSearchWithLang(t *testing.T) {
	cond := SimpleSearchWithLang("name", "joao", "portuguese")
	if !strings.Contains(cond.SQL, "portuguese") {
		t.Errorf("SQL = %q, should contain 'portuguese'", cond.SQL)
	}
}

func TestMultiColumnSearch(t *testing.T) {
	cond := MultiColumnSearch([]string{"title", "body"}, "test", "english")
	if !strings.Contains(cond.SQL, "coalesce") {
		t.Errorf("SQL = %q, should contain coalesce", cond.SQL)
	}
	if !strings.Contains(cond.SQL, "||") {
		t.Errorf("SQL = %q, should contain || for concatenation", cond.SQL)
	}
}

func TestSimpleSearchMySQL(t *testing.T) {
	cond := SimpleSearchMySQL([]string{"name", "bio"}, "john")
	if !strings.Contains(cond.SQL, "MATCH(") {
		t.Errorf("SQL = %q, should contain MATCH", cond.SQL)
	}
	if !strings.Contains(cond.SQL, "NATURAL LANGUAGE") {
		t.Errorf("SQL = %q, should contain NATURAL LANGUAGE", cond.SQL)
	}
}

func TestBooleanSearchMySQL(t *testing.T) {
	cond := BooleanSearchMySQL([]string{"name"}, "+john -doe")
	if !strings.Contains(cond.SQL, "BOOLEAN MODE") {
		t.Errorf("SQL = %q, should contain BOOLEAN MODE", cond.SQL)
	}
}

func TestQueryExpansionSearchMySQL(t *testing.T) {
	cond := QueryExpansionSearchMySQL([]string{"name"}, "john")
	if !strings.Contains(cond.SQL, "QUERY EXPANSION") {
		t.Errorf("SQL = %q, should contain QUERY EXPANSION", cond.SQL)
	}
}

func TestWeightedSearch(t *testing.T) {
	weights := map[string]string{
		"title": "A",
		"body":  "B",
	}
	cond := WeightedSearch(weights, "test", "english")
	if !strings.Contains(cond.SQL, "setweight") {
		t.Errorf("SQL = %q, should contain setweight", cond.SQL)
	}
	if !strings.Contains(cond.SQL, "@@") {
		t.Errorf("SQL = %q, should contain @@", cond.SQL)
	}
}

func TestRankSearch(t *testing.T) {
	result := RankSearch([]string{"title", "body"}, "test", "english", "rank")
	if !strings.Contains(result, "ts_rank") {
		t.Errorf("RankSearch = %q, should contain ts_rank", result)
	}
	if !strings.Contains(result, "AS rank") {
		t.Errorf("RankSearch = %q, should contain AS rank", result)
	}
}

func TestHeadlineSearch(t *testing.T) {
	result := HeadlineSearch("body", "test", "english")
	if !strings.Contains(result, "ts_headline") {
		t.Errorf("HeadlineSearch = %q, should contain ts_headline", result)
	}
	if !strings.Contains(result, "<b>") {
		t.Errorf("HeadlineSearch = %q, should contain <b> tag", result)
	}
}

func TestLikeSearch(t *testing.T) {
	cond := LikeSearch("name", "john")
	if !strings.Contains(cond.SQL, "LOWER") {
		t.Errorf("SQL = %q, should contain LOWER", cond.SQL)
	}
	if !strings.Contains(cond.SQL, "LIKE") {
		t.Errorf("SQL = %q, should contain LIKE", cond.SQL)
	}
	if cond.Args[0] != "%john%" {
		t.Errorf("arg = %v, want '%%john%%'", cond.Args[0])
	}
}

func TestILikeSearch(t *testing.T) {
	cond := ILikeSearch("name", "john")
	if !strings.Contains(cond.SQL, "ILIKE") {
		t.Errorf("SQL = %q, should contain ILIKE", cond.SQL)
	}
	if cond.Args[0] != "%john%" {
		t.Errorf("arg = %v, want '%%john%%'", cond.Args[0])
	}
}
