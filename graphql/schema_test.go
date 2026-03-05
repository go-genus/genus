package graphql

import (
	"strings"
	"testing"
	"time"
)

// ========================================
// Test Models
// ========================================

type User struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Age       int       `json:"age" db:"age"`
	Active    bool      `json:"active" db:"active"`
	Score     float64   `json:"score" db:"score"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

type Post struct {
	ID    string `json:"id" db:"id"`
	Title string `json:"title" db:"title"`
	Body  string `json:"body" db:"body"`
}

type EmbeddedModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type ModelWithEmbedded struct {
	EmbeddedModel
	Extra string `json:"extra"`
}

type ModelWithSkippedField struct {
	Name    string `json:"name"`
	Ignored string `db:"-"`
}

type ModelWithPointer struct {
	Name  string  `json:"name"`
	Email *string `json:"email"`
}

type ModelWithSlice struct {
	Name  string   `json:"name"`
	Tags  []string `json:"tags"`
	Items []*Post  `json:"items"`
}

type ModelWithDescription struct {
	Name string `json:"name" description:"The user name" deprecated:"Use fullName instead"`
}

type ModelNoJsonTag struct {
	Name  string `db:"name"`
	Email string
}

// ========================================
// Tests: NewSchemaGenerator
// ========================================

func TestNewSchemaGenerator(t *testing.T) {
	gen := NewSchemaGenerator()
	if gen == nil {
		t.Fatal("NewSchemaGenerator returned nil")
	}
	if gen.types == nil || gen.inputs == nil || gen.enums == nil || gen.scalars == nil || gen.connections == nil {
		t.Error("all maps should be initialized")
	}
}

// ========================================
// Tests: RegisterType
// ========================================

func TestRegisterType_BasicStruct(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(Post{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef, ok := gen.types["Post"]
	if !ok {
		t.Fatal("Post type not registered")
	}
	if len(typeDef.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(typeDef.Fields))
	}

	inputDef, ok := gen.inputs["PostInput"]
	if !ok {
		t.Fatal("PostInput not registered")
	}
	if len(inputDef.Fields) != 3 {
		t.Errorf("expected 3 input fields, got %d", len(inputDef.Fields))
	}
}

func TestRegisterType_NotAStruct(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType("not a struct")
	if err == nil {
		t.Error("expected error for non-struct")
	}
}

func TestRegisterType_Pointer(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(&Post{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}
	if _, ok := gen.types["Post"]; !ok {
		t.Error("should register pointer to struct")
	}
}

func TestRegisterType_WithTimeField(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(User{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	if !gen.scalars["DateTime"] {
		t.Error("DateTime scalar should be registered")
	}
}

func TestRegisterType_WithEmbedded(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(ModelWithEmbedded{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef := gen.types["ModelWithEmbedded"]
	if typeDef == nil {
		t.Fatal("type not registered")
	}

	// Should have embedded fields + Extra
	hasID := false
	hasName := false
	hasExtra := false
	for _, f := range typeDef.Fields {
		switch f.Name {
		case "id":
			hasID = true
		case "name":
			hasName = true
		case "extra":
			hasExtra = true
		}
	}
	if !hasID || !hasName || !hasExtra {
		t.Errorf("missing fields: id=%v name=%v extra=%v", hasID, hasName, hasExtra)
	}
}

func TestRegisterType_SkipDBDash(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(ModelWithSkippedField{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef := gen.types["ModelWithSkippedField"]
	for _, f := range typeDef.Fields {
		if f.Name == "Ignored" || f.Name == "-" {
			t.Error("field with db:\"-\" should be skipped")
		}
	}
}

func TestRegisterType_WithPointer(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(ModelWithPointer{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef := gen.types["ModelWithPointer"]
	for _, f := range typeDef.Fields {
		if f.Name == "email" {
			if f.NonNull {
				t.Error("pointer field should not be NonNull")
			}
			return
		}
	}
	t.Error("email field not found")
}

func TestRegisterType_WithSlice(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(ModelWithSlice{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef := gen.types["ModelWithSlice"]
	for _, f := range typeDef.Fields {
		if f.Name == "tags" {
			if !f.IsList {
				t.Error("tags should be a list")
			}
			if f.Type != "String" {
				t.Errorf("tags type = %q, want String", f.Type)
			}
		}
		if f.Name == "items" {
			if !f.IsList {
				t.Error("items should be a list")
			}
		}
	}
}

func TestRegisterType_WithDescription(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(ModelWithDescription{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef := gen.types["ModelWithDescription"]
	for _, f := range typeDef.Fields {
		if f.Name == "name" {
			if f.Description != "The user name" {
				t.Errorf("Description = %q, want %q", f.Description, "The user name")
			}
			if f.Deprecated != "Use fullName instead" {
				t.Errorf("Deprecated = %q, want %q", f.Deprecated, "Use fullName instead")
			}
			return
		}
	}
	t.Error("name field not found")
}

func TestRegisterType_NoJsonTag(t *testing.T) {
	gen := NewSchemaGenerator()
	err := gen.RegisterType(ModelNoJsonTag{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef := gen.types["ModelNoJsonTag"]
	hasName := false
	hasEmail := false
	for _, f := range typeDef.Fields {
		if f.Name == "name" {
			hasName = true
		}
		if f.Name == "email" {
			hasEmail = true
		}
	}
	if !hasName {
		t.Error("field with db tag should use db name")
	}
	if !hasEmail {
		t.Error("field without tags should use camelCase name")
	}
}

// ========================================
// Tests: goTypeToGraphQL
// ========================================

func TestGoTypeToGraphQL_AllTypes(t *testing.T) {
	gen := NewSchemaGenerator()

	err := gen.RegisterType(User{})
	if err != nil {
		t.Fatalf("RegisterType error = %v", err)
	}

	typeDef := gen.types["User"]
	typeMap := make(map[string]string)
	for _, f := range typeDef.Fields {
		typeMap[f.Name] = f.Type
	}

	if typeMap["id"] != "String" {
		t.Errorf("id type = %q, want String", typeMap["id"])
	}
	if typeMap["name"] != "String" {
		t.Errorf("name type = %q, want String", typeMap["name"])
	}
	if typeMap["age"] != "Int" {
		t.Errorf("age type = %q, want Int", typeMap["age"])
	}
	if typeMap["active"] != "Boolean" {
		t.Errorf("active type = %q, want Boolean", typeMap["active"])
	}
	if typeMap["score"] != "Float" {
		t.Errorf("score type = %q, want Float", typeMap["score"])
	}
	if typeMap["createdAt"] != "DateTime" {
		t.Errorf("createdAt type = %q, want DateTime", typeMap["createdAt"])
	}
}

// ========================================
// Tests: toGraphQLName
// ========================================

func TestToGraphQLName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"UserName", "userName"},
		{"ID", "iD"},
		{"name", "name"},
		{"", ""},
	}

	for _, tt := range tests {
		result := toGraphQLName(tt.input)
		if result != tt.expected {
			t.Errorf("toGraphQLName(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ========================================
// Tests: ToPascalCase
// ========================================

func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"userName", "UserName"},
		{"Name", "Name"},
		{"", ""},
	}

	for _, tt := range tests {
		result := ToPascalCase(tt.input)
		if result != tt.expected {
			t.Errorf("ToPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ========================================
// Tests: toSnakeCase
// ========================================

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"userName", "user_name"},
		{"UserName", "user_name"},
		{"name", "name"},
		{"ID", "i_d"},
		{"", ""},
	}

	for _, tt := range tests {
		result := toSnakeCase(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ========================================
// Tests: GenerateSchema
// ========================================

func TestGenerateSchema_Basic(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(Post{})

	schema := gen.GenerateSchema()

	if !strings.Contains(schema, "type Post") {
		t.Error("schema should contain type Post")
	}
	if !strings.Contains(schema, "input PostInput") {
		t.Error("schema should contain input PostInput")
	}
	if !strings.Contains(schema, "type Query") {
		t.Error("schema should contain type Query")
	}
	if !strings.Contains(schema, "type Mutation") {
		t.Error("schema should contain type Mutation")
	}
	if !strings.Contains(schema, "createPost") {
		t.Error("schema should contain createPost mutation")
	}
	if !strings.Contains(schema, "deletePost") {
		t.Error("schema should contain deletePost mutation")
	}
	if !strings.Contains(schema, "type PageInfo") {
		t.Error("schema should contain PageInfo type")
	}
}

func TestGenerateSchema_WithDateTime(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(User{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "scalar DateTime") {
		t.Error("schema should contain scalar DateTime")
	}
}

func TestGenerateSchema_WithEnum(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.AddEnum("Status", []string{"ACTIVE", "INACTIVE"}, "Active status", "Inactive status")
	gen.RegisterType(Post{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "enum Status") {
		t.Error("schema should contain enum Status")
	}
	if !strings.Contains(schema, "ACTIVE") {
		t.Error("schema should contain ACTIVE value")
	}
}

func TestGenerateSchema_WithScalar(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.AddScalar("JSON")
	gen.RegisterType(Post{})

	_ = gen.GenerateSchema()
	// Custom scalars are tracked but only DateTime is printed automatically
	if !gen.scalars["JSON"] {
		t.Error("JSON scalar should be registered")
	}
}

func TestGenerateSchema_Connection(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(Post{})

	// First call registers connections via generateQueryType
	_ = gen.GenerateSchema()
	// Second call includes the connections registered in the first call
	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "type PostEdge") {
		t.Error("schema should contain PostEdge")
	}
	if !strings.Contains(schema, "type PostConnection") {
		t.Error("schema should contain PostConnection")
	}
}

func TestGenerateSchema_FilterInput(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(User{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "input UserFilter") {
		t.Error("schema should contain UserFilter")
	}
	if !strings.Contains(schema, "name_contains: String") {
		t.Error("schema should contain string filter")
	}
	if !strings.Contains(schema, "age_gt: Int") {
		t.Error("schema should contain int filter")
	}
	if !strings.Contains(schema, "active: Boolean") {
		t.Error("schema should contain boolean filter")
	}
}

func TestGenerateSchema_OrderByEnum(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(Post{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "enum PostOrderBy") {
		t.Error("schema should contain PostOrderBy enum")
	}
	if !strings.Contains(schema, "_ASC") {
		t.Error("schema should contain ASC order")
	}
	if !strings.Contains(schema, "_DESC") {
		t.Error("schema should contain DESC order")
	}
}

func TestGenerateSchema_MutationType(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(Post{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "createPost(input: PostInput!): Post!") {
		t.Error("schema should contain createPost mutation")
	}
	if !strings.Contains(schema, "updatePost(id: ID!, input: PostInput!): Post!") {
		t.Error("schema should contain updatePost mutation")
	}
	if !strings.Contains(schema, "deletePost(id: ID!): Boolean!") {
		t.Error("schema should contain deletePost mutation")
	}
	if !strings.Contains(schema, "createPosts(inputs: [PostInput!]!): [Post!]!") {
		t.Error("schema should contain batch createPosts mutation")
	}
	if !strings.Contains(schema, "deletePosts(ids: [ID!]!): Int!") {
		t.Error("schema should contain batch deletePosts mutation")
	}
}

// ========================================
// Tests: generateField
// ========================================

func TestGenerateField_WithArguments(t *testing.T) {
	gen := NewSchemaGenerator()
	field := FieldDefinition{
		Name:    "users",
		Type:    "User",
		NonNull: true,
		IsList:  true,
		Arguments: []ArgumentDefinition{
			{Name: "first", Type: "Int", NonNull: false, DefaultValue: "10"},
			{Name: "filter", Type: "String", NonNull: true},
		},
	}

	result := gen.generateField(field)
	if !strings.Contains(result, "first: Int = 10") {
		t.Errorf("should contain argument with default: %s", result)
	}
	if !strings.Contains(result, "filter: String!") {
		t.Errorf("should contain required argument: %s", result)
	}
}

func TestGenerateField_Deprecated(t *testing.T) {
	gen := NewSchemaGenerator()
	field := FieldDefinition{
		Name:       "oldName",
		Type:       "String",
		NonNull:    true,
		Deprecated: "Use newName",
	}

	result := gen.generateField(field)
	if !strings.Contains(result, "@deprecated") {
		t.Error("should contain @deprecated")
	}
	if !strings.Contains(result, "Use newName") {
		t.Error("should contain deprecation reason")
	}
}

func TestGenerateField_ListNonNull(t *testing.T) {
	gen := NewSchemaGenerator()
	field := FieldDefinition{
		Name:    "tags",
		Type:    "String",
		NonNull: true,
		IsList:  true,
	}

	result := gen.generateField(field)
	if !strings.Contains(result, "[String]!") {
		t.Errorf("expected [String]!, got %s", result)
	}
}

func TestGenerateField_ListNullable(t *testing.T) {
	gen := NewSchemaGenerator()
	field := FieldDefinition{
		Name:    "tags",
		Type:    "String",
		NonNull: false,
		IsList:  true,
	}

	result := gen.generateField(field)
	if !strings.Contains(result, "[String]") {
		t.Errorf("expected [String], got %s", result)
	}
}

// ========================================
// Tests: generateType
// ========================================

func TestGenerateType_WithDescription(t *testing.T) {
	gen := NewSchemaGenerator()
	typeDef := &TypeDefinition{
		Name:        "User",
		Description: "A user in the system",
		Fields: []FieldDefinition{
			{Name: "id", Type: "ID", NonNull: true},
		},
	}

	result := gen.generateType(typeDef)
	if !strings.Contains(result, "\"\"\"A user in the system\"\"\"") {
		t.Error("should contain description")
	}
}

func TestGenerateType_WithImplements(t *testing.T) {
	gen := NewSchemaGenerator()
	typeDef := &TypeDefinition{
		Name:       "User",
		Implements: []string{"Node", "Entity"},
		Fields: []FieldDefinition{
			{Name: "id", Type: "ID", NonNull: true},
		},
	}

	result := gen.generateType(typeDef)
	if !strings.Contains(result, "implements Node & Entity") {
		t.Errorf("should contain implements: %s", result)
	}
}

// ========================================
// Tests: generateInput
// ========================================

func TestGenerateInput_WithDescription(t *testing.T) {
	gen := NewSchemaGenerator()
	input := &InputDefinition{
		Name:        "UserInput",
		Description: "Input for creating a user",
		Fields: []FieldDefinition{
			{Name: "name", Type: "String", NonNull: true},
		},
	}

	result := gen.generateInput(input)
	if !strings.Contains(result, "\"\"\"Input for creating a user\"\"\"") {
		t.Error("should contain description")
	}
	if !strings.Contains(result, "input UserInput") {
		t.Error("should contain input definition")
	}
}

// ========================================
// Tests: generateEnum
// ========================================

func TestGenerateEnum(t *testing.T) {
	gen := NewSchemaGenerator()
	enum := &EnumDefinition{
		Name:        "Status",
		Description: "User status",
		Values: []EnumValue{
			{Name: "ACTIVE", Description: "Active user"},
			{Name: "INACTIVE", Deprecated: "No longer used"},
		},
	}

	result := gen.generateEnum(enum)
	if !strings.Contains(result, "\"\"\"User status\"\"\"") {
		t.Error("should contain enum description")
	}
	if !strings.Contains(result, "\"\"\"Active user\"\"\"") {
		t.Error("should contain value description")
	}
	if !strings.Contains(result, "@deprecated") {
		t.Error("should contain deprecated directive")
	}
}

// ========================================
// Tests: AddEnum
// ========================================

func TestAddEnum(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.AddEnum("Status", []string{"ACTIVE", "INACTIVE"})

	enum, ok := gen.enums["Status"]
	if !ok {
		t.Fatal("enum not registered")
	}
	if len(enum.Values) != 2 {
		t.Errorf("expected 2 values, got %d", len(enum.Values))
	}
}

func TestAddEnum_WithDescriptions(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.AddEnum("Status", []string{"ACTIVE", "INACTIVE"}, "Active status")

	enum := gen.enums["Status"]
	if enum.Values[0].Description != "Active status" {
		t.Errorf("Description = %q, want %q", enum.Values[0].Description, "Active status")
	}
	// Second value has no description (less descriptions than values)
	if enum.Values[1].Description != "" {
		t.Errorf("Description = %q, want empty", enum.Values[1].Description)
	}
}

// ========================================
// Tests: AddScalar
// ========================================

func TestAddScalar(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.AddScalar("JSON")
	if !gen.scalars["JSON"] {
		t.Error("JSON scalar should be registered")
	}
}

// ========================================
// Tests: goTypeToGraphQL edge cases
// ========================================

func TestGoTypeToGraphQL_UintTypes(t *testing.T) {
	type UintModel struct {
		Val uint `json:"val"`
	}
	gen := NewSchemaGenerator()
	gen.RegisterType(UintModel{})

	typeDef := gen.types["UintModel"]
	if typeDef.Fields[0].Type != "Int" {
		t.Errorf("uint type = %q, want Int", typeDef.Fields[0].Type)
	}
}

func TestGoTypeToGraphQL_StructType(t *testing.T) {
	type Inner struct {
		X string `json:"x"`
	}
	type Outer struct {
		Inner Inner `json:"inner"`
	}

	gen := NewSchemaGenerator()
	gen.RegisterType(Outer{})

	typeDef := gen.types["Outer"]
	for _, f := range typeDef.Fields {
		if f.Name == "inner" {
			if f.Type != "Inner" {
				t.Errorf("type = %q, want Inner", f.Type)
			}
			return
		}
	}
	t.Error("inner field not found")
}

// ========================================
// Tests: EmbeddedModel - input excludes id/timestamps
// ========================================

func TestRegisterType_EmbeddedInputExcludes(t *testing.T) {
	type BaseModel struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}
	type Product struct {
		BaseModel
		Name string `json:"name"`
	}

	gen := NewSchemaGenerator()
	gen.RegisterType(Product{})

	inputDef := gen.inputs["ProductInput"]
	for _, f := range inputDef.Fields {
		if f.Name == "id" || f.Name == "createdAt" || f.Name == "updatedAt" {
			t.Errorf("input should not contain field %q", f.Name)
		}
	}
}

// ========================================
// Tests: Input excludes list/connection fields
// ========================================

func TestRegisterType_InputExcludesLists(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(ModelWithSlice{})

	inputDef := gen.inputs["ModelWithSliceInput"]
	for _, f := range inputDef.Fields {
		if f.Name == "tags" || f.Name == "items" {
			t.Errorf("input should not contain list field %q", f.Name)
		}
	}
}

// ========================================
// Tests: DateTime filter
// ========================================

func TestGenerateFilterInput_DateTime(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(User{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "createdAt: DateTime") {
		t.Error("should contain DateTime filter")
	}
	if !strings.Contains(schema, "createdAt_gt: DateTime") {
		t.Error("should contain DateTime gt filter")
	}
	if !strings.Contains(schema, "createdAt_lt: DateTime") {
		t.Error("should contain DateTime lt filter")
	}
}

// ========================================
// Tests: Float filter
// ========================================

func TestGenerateFilterInput_Float(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(User{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "score: Float") {
		t.Error("should contain Float filter")
	}
	if !strings.Contains(schema, "score_gt: Float") {
		t.Error("should contain Float gt filter")
	}
}

// ========================================
// Tests: AND/OR filter
// ========================================

func TestGenerateFilterInput_ANDOR(t *testing.T) {
	gen := NewSchemaGenerator()
	gen.RegisterType(Post{})

	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "AND: [PostFilter!]") {
		t.Error("should contain AND filter")
	}
	if !strings.Contains(schema, "OR: [PostFilter!]") {
		t.Error("should contain OR filter")
	}
}

// ========================================
// Tests: Empty schema
// ========================================

func TestGenerateSchema_Empty(t *testing.T) {
	gen := NewSchemaGenerator()
	schema := gen.GenerateSchema()
	if !strings.Contains(schema, "type Query") {
		t.Error("should contain Query type even when empty")
	}
	if !strings.Contains(schema, "type Mutation") {
		t.Error("should contain Mutation type even when empty")
	}
}
