package core

import (
	"reflect"
	"testing"
)

func TestRelationType_Constants(t *testing.T) {
	if HasMany != "has_many" {
		t.Errorf("HasMany = %q", HasMany)
	}
	if BelongsTo != "belongs_to" {
		t.Errorf("BelongsTo = %q", BelongsTo)
	}
	if ManyToMany != "many_to_many" {
		t.Errorf("ManyToMany = %q", ManyToMany)
	}
	if Polymorphic != "polymorphic" {
		t.Errorf("Polymorphic = %q", Polymorphic)
	}
}

// Test model with relationships
type UserWithRels struct {
	Model
	Name  string         `db:"name"`
	Posts []PostWithRels `relation:"has_many,foreign_key=user_id"`
}

type PostWithRels struct {
	Model
	Title  string `db:"title"`
	UserID int64  `db:"user_id"`
}

func TestRegisterModel_HasMany(t *testing.T) {
	// Reset registry
	registryMu.Lock()
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu.Unlock()

	err := RegisterModel(&UserWithRels{})
	if err != nil {
		t.Fatalf("RegisterModel error = %v", err)
	}

	rels := GetRelationships(reflect.TypeOf(UserWithRels{}))
	if rels == nil {
		t.Fatal("GetRelationships returned nil")
	}

	postRel, ok := rels.Relationships["Posts"]
	if !ok {
		t.Fatal("Posts relationship not found")
	}
	if postRel.Type != HasMany {
		t.Errorf("Type = %q, want has_many", postRel.Type)
	}
	if postRel.ForeignKey != "user_id" {
		t.Errorf("ForeignKey = %q, want user_id", postRel.ForeignKey)
	}
	if postRel.References != "id" {
		t.Errorf("References = %q, want id (default)", postRel.References)
	}
}

func TestRegisterModel_Pointer(t *testing.T) {
	registryMu.Lock()
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu.Unlock()

	err := RegisterModel(&UserWithRels{})
	if err != nil {
		t.Fatalf("RegisterModel error = %v", err)
	}

	// Get with pointer type
	rels := GetRelationships(reflect.TypeOf(&UserWithRels{}))
	if rels == nil {
		t.Fatal("should resolve pointer type")
	}
}

func TestGetRelationships_NotRegistered(t *testing.T) {
	registryMu.Lock()
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu.Unlock()

	rels := GetRelationships(reflect.TypeOf(TestModel{}))
	if rels != nil {
		t.Error("should return nil for unregistered model")
	}
}

// Test model with ManyToMany
type TaggableModel struct {
	Model
	Name string     `db:"name"`
	Tags []TagModel `relation:"many_to_many,join_table=model_tags,foreign_key=model_id,association_foreign_key=tag_id"`
}

type TagModel struct {
	Model
	Name string `db:"name"`
}

func TestRegisterModel_ManyToMany(t *testing.T) {
	registryMu.Lock()
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu.Unlock()

	err := RegisterModel(&TaggableModel{})
	if err != nil {
		t.Fatalf("RegisterModel error = %v", err)
	}

	rels := GetRelationships(reflect.TypeOf(TaggableModel{}))
	tagRel := rels.Relationships["Tags"]
	if tagRel == nil {
		t.Fatal("Tags relationship not found")
	}
	if tagRel.Type != ManyToMany {
		t.Errorf("Type = %q, want many_to_many", tagRel.Type)
	}
	if tagRel.JoinTable != "model_tags" {
		t.Errorf("JoinTable = %q, want model_tags", tagRel.JoinTable)
	}
	if tagRel.ForeignKey != "model_id" {
		t.Errorf("ForeignKey = %q, want model_id", tagRel.ForeignKey)
	}
	if tagRel.AssociationForeignKey != "tag_id" {
		t.Errorf("AssociationForeignKey = %q, want tag_id", tagRel.AssociationForeignKey)
	}
}

// Test model with Polymorphic
type CommentModel struct {
	Model
	Body            string `db:"body"`
	CommentableType string `db:"commentable_type" relation:"polymorphic,polymorphic=commentable"`
}

func TestRegisterModel_Polymorphic(t *testing.T) {
	registryMu.Lock()
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu.Unlock()

	err := RegisterModel(&CommentModel{})
	if err != nil {
		t.Fatalf("RegisterModel error = %v", err)
	}

	rels := GetRelationships(reflect.TypeOf(CommentModel{}))
	rel := rels.Relationships["CommentableType"]
	if rel == nil {
		t.Fatal("CommentableType relationship not found")
	}
	if rel.Type != Polymorphic {
		t.Errorf("Type = %q, want polymorphic", rel.Type)
	}
	if rel.Polymorphic != "commentable" {
		t.Errorf("Polymorphic = %q, want commentable", rel.Polymorphic)
	}
	if rel.PolymorphicType != "commentable_type" {
		t.Errorf("PolymorphicType = %q, want commentable_type", rel.PolymorphicType)
	}
	if rel.PolymorphicID != "commentable_id" {
		t.Errorf("PolymorphicID = %q, want commentable_id", rel.PolymorphicID)
	}
}

func TestParseRelationTag_EmptyTag(t *testing.T) {
	field := reflect.StructField{Name: "Test"}
	_, err := parseRelationTag(field, "")
	if err == nil {
		t.Error("should return error for empty tag")
	}
}

func TestParseRelationTag_UnknownType(t *testing.T) {
	field := reflect.StructField{Name: "Test"}
	_, err := parseRelationTag(field, "unknown_type")
	if err == nil {
		t.Error("should return error for unknown relation type")
	}
}

func TestParseRelationTag_HasMany_MissingFK(t *testing.T) {
	field := reflect.StructField{Name: "Posts"}
	_, err := parseRelationTag(field, "has_many")
	if err == nil {
		t.Error("should return error when foreign_key is missing for has_many")
	}
}

func TestParseRelationTag_BelongsTo_MissingFK(t *testing.T) {
	field := reflect.StructField{Name: "User"}
	_, err := parseRelationTag(field, "belongs_to")
	if err == nil {
		t.Error("should return error when foreign_key is missing for belongs_to")
	}
}

func TestParseRelationTag_ManyToMany_MissingJoinTable(t *testing.T) {
	field := reflect.StructField{Name: "Tags"}
	_, err := parseRelationTag(field, "many_to_many,foreign_key=model_id,association_foreign_key=tag_id")
	if err == nil {
		t.Error("should return error when join_table is missing")
	}
}

func TestParseRelationTag_ManyToMany_MissingFKs(t *testing.T) {
	field := reflect.StructField{Name: "Tags"}
	_, err := parseRelationTag(field, "many_to_many,join_table=model_tags")
	if err == nil {
		t.Error("should return error when FKs are missing")
	}
}

func TestParseRelationTag_Polymorphic_MissingName(t *testing.T) {
	field := reflect.StructField{Name: "Commentable"}
	_, err := parseRelationTag(field, "polymorphic")
	if err == nil {
		t.Error("should return error when polymorphic name is missing")
	}
}

func TestParseRelationTag_HasMany_WithReferences(t *testing.T) {
	field := reflect.StructField{Name: "Posts", Type: reflect.TypeOf([]int{})}
	meta, err := parseRelationTag(field, "has_many,foreign_key=user_id,references=custom_id")
	if err != nil {
		t.Fatalf("parseRelationTag error = %v", err)
	}
	if meta.References != "custom_id" {
		t.Errorf("References = %q, want custom_id", meta.References)
	}
}

func TestParseRelationTag_HasMany_DefaultReferences(t *testing.T) {
	field := reflect.StructField{Name: "Posts", Type: reflect.TypeOf([]int{})}
	meta, err := parseRelationTag(field, "has_many,foreign_key=user_id")
	if err != nil {
		t.Fatalf("parseRelationTag error = %v", err)
	}
	if meta.References != "id" {
		t.Errorf("References = %q, want id", meta.References)
	}
}

func TestParseRelationTag_BelongsTo_WithPolymorphic(t *testing.T) {
	field := reflect.StructField{Name: "Owner", Type: reflect.TypeOf(0)}
	meta, err := parseRelationTag(field, "belongs_to,polymorphic=commentable")
	if err != nil {
		t.Fatalf("parseRelationTag error = %v", err)
	}
	if meta.Polymorphic != "commentable" {
		t.Errorf("Polymorphic = %q, want commentable", meta.Polymorphic)
	}
	if meta.PolymorphicType != "commentable_type" {
		t.Errorf("PolymorphicType = %q, want commentable_type", meta.PolymorphicType)
	}
	if meta.PolymorphicID != "commentable_id" {
		t.Errorf("PolymorphicID = %q, want commentable_id", meta.PolymorphicID)
	}
}

func TestParseRelationTag_Polymorphic_CustomTypeAndID(t *testing.T) {
	field := reflect.StructField{Name: "Owner", Type: reflect.TypeOf(0)}
	meta, err := parseRelationTag(field, "polymorphic,polymorphic=commentable,polymorphic_type=custom_type,polymorphic_id=custom_id")
	if err != nil {
		t.Fatalf("parseRelationTag error = %v", err)
	}
	if meta.PolymorphicType != "custom_type" {
		t.Errorf("PolymorphicType = %q, want custom_type", meta.PolymorphicType)
	}
	if meta.PolymorphicID != "custom_id" {
		t.Errorf("PolymorphicID = %q, want custom_id", meta.PolymorphicID)
	}
}

func TestRegisterModel_Error(t *testing.T) {
	registryMu.Lock()
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu.Unlock()

	type BadModel struct {
		Model
		// Invalid relation tag
		Bad string `relation:"unknown_type"`
	}

	err := RegisterModel(&BadModel{})
	if err == nil {
		t.Error("should return error for invalid relation tag")
	}
}

func TestRegisterModel_NoRelations(t *testing.T) {
	registryMu.Lock()
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu.Unlock()

	type SimpleModel struct {
		Model
		Name string `db:"name"`
	}

	err := RegisterModel(&SimpleModel{})
	if err != nil {
		t.Fatalf("RegisterModel error = %v", err)
	}

	rels := GetRelationships(reflect.TypeOf(SimpleModel{}))
	if rels == nil {
		t.Fatal("should return relationships even if empty")
	}
	if len(rels.Relationships) != 0 {
		t.Errorf("expected 0 relationships, got %d", len(rels.Relationships))
	}
}

func TestSplitTag(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"has_many,foreign_key=user_id", []string{"has_many", "foreign_key=user_id"}},
		{"has_many", []string{"has_many"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{"", []string{}},
		{",,,", []string{}},
	}

	for _, tt := range tests {
		got := splitTag(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitTag(%q): len = %d, want %d; got %v", tt.input, len(got), len(tt.expected), got)
			continue
		}
		for i, v := range got {
			if v != tt.expected[i] {
				t.Errorf("splitTag(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestParseKeyValue(t *testing.T) {
	tests := []struct {
		input     string
		wantKey   string
		wantValue string
	}{
		{"foreign_key=user_id", "foreign_key", "user_id"},
		{"key=value", "key", "value"},
		{"novalue", "novalue", ""},
		{"key=", "key", ""},
		{"=value", "", "value"},
	}

	for _, tt := range tests {
		key, value := parseKeyValue(tt.input)
		if key != tt.wantKey || value != tt.wantValue {
			t.Errorf("parseKeyValue(%q) = (%q, %q), want (%q, %q)", tt.input, key, value, tt.wantKey, tt.wantValue)
		}
	}
}

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"  hello  ", "hello"},
		{"hello", "hello"},
		{"\thello\t", "hello"},
		{"  ", ""},
		{"", ""},
	}

	for _, tt := range tests {
		got := trimSpace(tt.input)
		if got != tt.expected {
			t.Errorf("trimSpace(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
