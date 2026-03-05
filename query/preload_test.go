package query

import (
	"reflect"
	"testing"
)

func TestParsePreloadPath_Simple(t *testing.T) {
	spec := parsePreloadPath("Posts")
	if spec == nil {
		t.Fatal("parsePreloadPath returned nil")
	}
	if spec.Relation != "Posts" {
		t.Errorf("Relation = %q, want 'Posts'", spec.Relation)
	}
	if len(spec.Nested) != 0 {
		t.Errorf("Nested len = %d, want 0", len(spec.Nested))
	}
}

func TestParsePreloadPath_Nested(t *testing.T) {
	spec := parsePreloadPath("Posts.Comments")
	if spec == nil {
		t.Fatal("parsePreloadPath returned nil")
	}
	if spec.Relation != "Posts" {
		t.Errorf("Relation = %q, want 'Posts'", spec.Relation)
	}
	if len(spec.Nested) != 1 {
		t.Fatalf("Nested len = %d, want 1", len(spec.Nested))
	}
	if spec.Nested[0].Relation != "Comments" {
		t.Errorf("Nested[0].Relation = %q, want 'Comments'", spec.Nested[0].Relation)
	}
}

func TestParsePreloadPath_DeepNested(t *testing.T) {
	spec := parsePreloadPath("Posts.Comments.Author")
	if spec.Relation != "Posts" {
		t.Errorf("Relation = %q, want 'Posts'", spec.Relation)
	}
	if len(spec.Nested) != 1 {
		t.Fatal("should have 1 nested")
	}
	if spec.Nested[0].Relation != "Comments" {
		t.Errorf("Nested Relation = %q, want 'Comments'", spec.Nested[0].Relation)
	}
	if len(spec.Nested[0].Nested) != 1 {
		t.Fatal("should have deep nested")
	}
	if spec.Nested[0].Nested[0].Relation != "Author" {
		t.Errorf("Deep nested = %q, want 'Author'", spec.Nested[0].Nested[0].Relation)
	}
}

func TestGetTableNameFromType(t *testing.T) {
	type UserProfile struct{}
	typ := reflect.TypeOf(UserProfile{})
	name := getTableNameFromType(typ)
	if name != "user_profile" {
		t.Errorf("getTableNameFromType = %q, want 'user_profile'", name)
	}
}

func TestGetTableNameFromType_Slice(t *testing.T) {
	type Post struct{}
	typ := reflect.TypeOf([]Post{})
	name := getTableNameFromType(typ)
	if name != "post" {
		t.Errorf("getTableNameFromType slice = %q, want 'post'", name)
	}
}

func TestGetTableNameFromType_Pointer(t *testing.T) {
	type Order struct{}
	var p *Order
	typ := reflect.TypeOf(p)
	name := getTableNameFromType(typ)
	if name != "order" {
		t.Errorf("getTableNameFromType ptr = %q, want 'order'", name)
	}
}

func TestGetTableNameFromType_SlicePtr(t *testing.T) {
	type Tag struct{}
	var p *[]Tag
	typ := reflect.TypeOf(p)
	name := getTableNameFromType(typ)
	if name != "tag" {
		t.Errorf("getTableNameFromType slice ptr = %q, want 'tag'", name)
	}
}

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"user_id", "UserId"},
		{"created_at", "CreatedAt"},
		{"name", "Name"},
		{"id", "Id"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toPascalCase(tt.input)
			if result != tt.expected {
				t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToSnakeCasePlural(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Post", "post"},
		{"UserProfile", "user_profile"},
		{"Comment", "comment"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toSnakeCasePlural(tt.input)
			if result != tt.expected {
				t.Errorf("toSnakeCasePlural(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestBuildFieldMapForStruct(t *testing.T) {
	type TestStruct struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	fm := buildFieldMapForStruct(reflect.TypeOf(TestStruct{}))
	if _, ok := fm["id"]; !ok {
		t.Error("should contain 'id'")
	}
	if _, ok := fm["name"]; !ok {
		t.Error("should contain 'name'")
	}
}
