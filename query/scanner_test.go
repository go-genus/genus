package query

import (
	"reflect"
	"testing"
)

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ID", "i_d"},
		{"Name", "name"},
		{"UserName", "user_name"},
		{"CreatedAt", "created_at"},
		{"HTTPStatus", "h_t_t_p_status"},
		{"simple", "simple"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := toSnakeCase(tt.input)
			if got != tt.expected {
				t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestBuildFieldMap(t *testing.T) {
	type TestStruct struct {
		ID    int64  `db:"id"`
		Name  string `db:"name"`
		Email string `db:"email"`
		Skip  string `db:"-"`
	}

	fm := buildFieldMap(reflect.TypeOf(TestStruct{}))

	if _, ok := fm["id"]; !ok {
		t.Error("field map should contain 'id'")
	}
	if _, ok := fm["name"]; !ok {
		t.Error("field map should contain 'name'")
	}
	if _, ok := fm["email"]; !ok {
		t.Error("field map should contain 'email'")
	}
	if _, ok := fm["-"]; ok {
		t.Error("field map should not contain skipped field")
	}
}

func TestBuildFieldMap_DefaultSnakeCase(t *testing.T) {
	type TestStruct struct {
		FirstName string
		LastName  string
	}

	fm := buildFieldMap(reflect.TypeOf(TestStruct{}))

	if _, ok := fm["first_name"]; !ok {
		t.Error("field map should contain 'first_name' (default snake_case)")
	}
	if _, ok := fm["last_name"]; !ok {
		t.Error("field map should contain 'last_name' (default snake_case)")
	}
}

func TestBuildFieldMap_Embedded(t *testing.T) {
	type Base struct {
		ID int64 `db:"id"`
	}
	type TestStruct struct {
		Base
		Name string `db:"name"`
	}

	fm := buildFieldMap(reflect.TypeOf(TestStruct{}))

	if _, ok := fm["id"]; !ok {
		t.Error("field map should contain 'id' from embedded struct")
	}
	if _, ok := fm["name"]; !ok {
		t.Error("field map should contain 'name'")
	}
}

func TestGetFieldByPath(t *testing.T) {
	type Base struct {
		ID int64
	}
	type TestStruct struct {
		Base
		Name string
	}

	val := reflect.ValueOf(TestStruct{Base: Base{ID: 42}, Name: "test"})

	// Direct field
	field := getFieldByPath(val, fieldPath{1})
	if !field.IsValid() || field.String() != "test" {
		t.Errorf("getFieldByPath for Name = %v, want 'test'", field)
	}

	// Embedded field
	field = getFieldByPath(val, fieldPath{0, 0})
	if !field.IsValid() || field.Int() != 42 {
		t.Errorf("getFieldByPath for embedded ID = %v, want 42", field)
	}

	// Out of bounds
	field = getFieldByPath(val, fieldPath{99})
	if field.IsValid() {
		t.Error("getFieldByPath with invalid index should return invalid Value")
	}
}

func TestGetOrBuildFieldMap_Cache(t *testing.T) {
	ClearFieldMapCache()

	type CacheTest struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	typ := reflect.TypeOf(CacheTest{})

	// First call builds
	fm1 := getOrBuildFieldMap(typ)
	if fm1 == nil {
		t.Fatal("getOrBuildFieldMap returned nil")
	}

	// Second call returns cached
	fm2 := getOrBuildFieldMap(typ)
	if fm2 == nil {
		t.Fatal("cached getOrBuildFieldMap returned nil")
	}

	if len(fm1) != len(fm2) {
		t.Error("cached map should have same length")
	}
}

func TestGetCachedFieldMap(t *testing.T) {
	ClearFieldMapCache()

	type ExportTest struct {
		ID int64 `db:"id"`
	}

	fm := GetCachedFieldMap(reflect.TypeOf(ExportTest{}))
	if _, ok := fm["id"]; !ok {
		t.Error("GetCachedFieldMap should return field map with 'id'")
	}
}

func TestGetFieldIndices(t *testing.T) {
	type TestStruct struct {
		ID   int64
		Name string
	}

	s := TestStruct{ID: 1, Name: "test"}
	indices, err := GetFieldIndices(&s)
	if err != nil {
		t.Fatalf("GetFieldIndices error: %v", err)
	}
	if len(indices) != 2 {
		t.Errorf("GetFieldIndices len = %d, want 2", len(indices))
	}
}

func TestGetFieldIndices_NotPointer(t *testing.T) {
	type TestStruct struct{ ID int64 }
	s := TestStruct{}
	_, err := GetFieldIndices(s)
	if err == nil {
		t.Error("expected error for non-pointer")
	}
}

func TestGetFieldIndices_NotStruct(t *testing.T) {
	val := 42
	_, err := GetFieldIndices(&val)
	if err == nil {
		t.Error("expected error for non-struct pointer")
	}
}

func TestClearFieldMapCache(t *testing.T) {
	type X struct{ ID int64 `db:"id"` }
	getOrBuildFieldMap(reflect.TypeOf(X{}))
	ClearFieldMapCache()
	// Should not panic and cache should be empty
	// Next call should rebuild
	fm := getOrBuildFieldMap(reflect.TypeOf(X{}))
	if _, ok := fm["id"]; !ok {
		t.Error("should rebuild field map after cache clear")
	}
}
