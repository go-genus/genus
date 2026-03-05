package query

import (
	"encoding/json"
	"strings"
	"testing"
)

// --- JSON[T] ---

func TestNewJSON(t *testing.T) {
	j := NewJSON(map[string]string{"key": "value"})
	if !j.Valid {
		t.Error("NewJSON should set Valid = true")
	}
}

func TestNullJSON(t *testing.T) {
	j := NullJSON[string]()
	if j.Valid {
		t.Error("NullJSON should set Valid = false")
	}
}

func TestJSON_Scan(t *testing.T) {
	var j JSON[map[string]string]

	// Scan from []byte
	err := j.Scan([]byte(`{"key":"value"}`))
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if !j.Valid {
		t.Error("Valid should be true after scan")
	}
	if j.Data["key"] != "value" {
		t.Errorf("Data = %v, want key=value", j.Data)
	}

	// Scan from string
	var j2 JSON[map[string]string]
	err = j2.Scan(`{"a":"b"}`)
	if err != nil {
		t.Fatalf("Scan from string error: %v", err)
	}
	if j2.Data["a"] != "b" {
		t.Errorf("Data = %v, want a=b", j2.Data)
	}

	// Scan nil
	var j3 JSON[string]
	err = j3.Scan(nil)
	if err != nil {
		t.Fatalf("Scan nil error: %v", err)
	}
	if j3.Valid {
		t.Error("Valid should be false after scan nil")
	}

	// Scan unsupported type
	var j4 JSON[string]
	err = j4.Scan(42)
	if err == nil {
		t.Error("Scan unsupported type should return error")
	}
}

func TestJSON_Value(t *testing.T) {
	j := NewJSON(map[string]string{"key": "value"})
	v, err := j.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}
	if v == nil {
		t.Error("Value should not be nil")
	}

	// Null JSON
	j2 := NullJSON[string]()
	v2, err := j2.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}
	if v2 != nil {
		t.Error("Value should be nil for invalid JSON")
	}
}

func TestJSON_MarshalJSON(t *testing.T) {
	j := NewJSON("hello")
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}
	if string(data) != `"hello"` {
		t.Errorf("MarshalJSON = %q, want '\"hello\"'", string(data))
	}

	// Null
	j2 := NullJSON[string]()
	data2, err := json.Marshal(j2)
	if err != nil {
		t.Fatalf("MarshalJSON error: %v", err)
	}
	if string(data2) != "null" {
		t.Errorf("MarshalJSON null = %q, want 'null'", string(data2))
	}
}

func TestJSON_UnmarshalJSON(t *testing.T) {
	var j JSON[string]
	err := json.Unmarshal([]byte(`"hello"`), &j)
	if err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}
	if !j.Valid || j.Data != "hello" {
		t.Errorf("UnmarshalJSON = %v/%q, want true/hello", j.Valid, j.Data)
	}

	// null
	var j2 JSON[string]
	err = json.Unmarshal([]byte("null"), &j2)
	if err != nil {
		t.Fatalf("UnmarshalJSON null error: %v", err)
	}
	if j2.Valid {
		t.Error("UnmarshalJSON null should set Valid = false")
	}
}

// --- JSONField / JSONPath ---

func TestJSONField_Path(t *testing.T) {
	f := NewJSONField("metadata")
	path := f.Path("user", "name")
	if path.Column != "metadata" {
		t.Errorf("Column = %q, want 'metadata'", path.Column)
	}
	if len(path.Path) != 2 {
		t.Errorf("Path len = %d, want 2", len(path.Path))
	}
}

func TestJSONPath_Eq(t *testing.T) {
	f := NewJSONField("metadata")
	cond := f.Path("user", "name").Eq("John")
	if cond.Operator != OpEq {
		t.Errorf("Operator = %v, want OpEq", cond.Operator)
	}
	if !strings.Contains(cond.Field, "->>") {
		t.Errorf("Field should use ->> for text extraction, got %q", cond.Field)
	}
}

func TestJSONPath_EqText(t *testing.T) {
	f := NewJSONField("data")
	cond := f.Path("status").EqText("active")
	if cond.Operator != OpEq {
		t.Errorf("Operator = %v, want OpEq", cond.Operator)
	}
	if cond.Value != "active" {
		t.Errorf("Value = %v, want 'active'", cond.Value)
	}
}

func TestJSONPath_ToSQL(t *testing.T) {
	path := JSONPath{Column: "data", Path: []string{"user", "name"}}
	sql := path.toSQL()
	if sql != "data->'user'->'name'" {
		t.Errorf("toSQL() = %q, want 'data->'user'->'name''", sql)
	}
}

func TestJSONPath_ToSQLText(t *testing.T) {
	path := JSONPath{Column: "data", Path: []string{"user", "name"}}
	sql := path.toSQLText()
	if sql != "data->'user'->>'name'" {
		t.Errorf("toSQLText() = %q, want 'data->'user'->>'name''", sql)
	}

	// Empty path
	path2 := JSONPath{Column: "data", Path: nil}
	sql2 := path2.toSQLText()
	if sql2 != "data" {
		t.Errorf("toSQLText() empty = %q, want 'data'", sql2)
	}

	// Single path
	path3 := JSONPath{Column: "data", Path: []string{"key"}}
	sql3 := path3.toSQLText()
	if sql3 != "data->>'key'" {
		t.Errorf("toSQLText() single = %q, want 'data->>'key''", sql3)
	}
}

func TestJSONField_Contains(t *testing.T) {
	f := NewJSONField("data")
	cond := f.Contains(map[string]string{"key": "val"})
	if !strings.Contains(cond.SQL, "@>") {
		t.Errorf("Contains SQL = %q, should contain @>", cond.SQL)
	}
}

func TestJSONField_ContainedBy(t *testing.T) {
	f := NewJSONField("data")
	cond := f.ContainedBy(map[string]string{"key": "val"})
	if !strings.Contains(cond.SQL, "<@") {
		t.Errorf("ContainedBy SQL = %q, should contain <@", cond.SQL)
	}
}

func TestJSONField_HasKey(t *testing.T) {
	f := NewJSONField("data")
	cond := f.HasKey("name")
	if !strings.Contains(cond.SQL, "?") {
		t.Errorf("HasKey SQL = %q, should contain ?", cond.SQL)
	}
}

func TestJSONField_HasAnyKey(t *testing.T) {
	f := NewJSONField("data")
	cond := f.HasAnyKey("a", "b")
	if !strings.Contains(cond.SQL, "?|") {
		t.Errorf("HasAnyKey SQL = %q, should contain ?|", cond.SQL)
	}
}

func TestJSONField_HasAllKeys(t *testing.T) {
	f := NewJSONField("data")
	cond := f.HasAllKeys("a", "b")
	if !strings.Contains(cond.SQL, "?&") {
		t.Errorf("HasAllKeys SQL = %q, should contain ?&", cond.SQL)
	}
}

// --- JSONRaw ---

func TestJSONRaw_Scan(t *testing.T) {
	var j JSONRaw

	// From []byte
	err := j.Scan([]byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("Scan error: %v", err)
	}
	if string(j) != `{"a":1}` {
		t.Errorf("Scan = %q, want '{\"a\":1}'", string(j))
	}

	// From string
	err = j.Scan(`{"b":2}`)
	if err != nil {
		t.Fatalf("Scan string error: %v", err)
	}
	if string(j) != `{"b":2}` {
		t.Errorf("Scan = %q", string(j))
	}

	// From nil
	err = j.Scan(nil)
	if err != nil {
		t.Fatalf("Scan nil error: %v", err)
	}
	if string(j) != "" {
		t.Errorf("Scan nil = %q, want empty", string(j))
	}

	// Unsupported
	err = j.Scan(42)
	if err == nil {
		t.Error("Scan unsupported should return error")
	}
}

func TestJSONRaw_Value(t *testing.T) {
	j := JSONRaw(`{"a":1}`)
	v, err := j.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}
	if v != `{"a":1}` {
		t.Errorf("Value = %v, want '{\"a\":1}'", v)
	}

	// Empty
	j2 := JSONRaw("")
	v2, err := j2.Value()
	if err != nil {
		t.Fatalf("Value error: %v", err)
	}
	if v2 != nil {
		t.Errorf("Value empty = %v, want nil", v2)
	}
}

func TestJSONRaw_Unmarshal(t *testing.T) {
	j := JSONRaw(`{"key":"value"}`)
	var result map[string]string
	err := j.Unmarshal(&result)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if result["key"] != "value" {
		t.Errorf("Unmarshal = %v", result)
	}
}

func TestJSONRaw_String(t *testing.T) {
	j := JSONRaw(`{"a":1}`)
	if j.String() != `{"a":1}` {
		t.Errorf("String = %q", j.String())
	}
}

func TestJSONRaw_IsNull(t *testing.T) {
	tests := []struct {
		input    JSONRaw
		expected bool
	}{
		{"", true},
		{"null", true},
		{`{"a":1}`, false},
		{"hello", false},
	}
	for _, tt := range tests {
		if tt.input.IsNull() != tt.expected {
			t.Errorf("IsNull(%q) = %v, want %v", tt.input, tt.input.IsNull(), tt.expected)
		}
	}
}
