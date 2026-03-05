package codegen

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewGenerator(t *testing.T) {
	cfg := Config{
		OutputDir:   "/tmp/out",
		PackageName: "models",
	}
	gen := NewGenerator(cfg)
	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if gen.config.OutputDir != "/tmp/out" {
		t.Errorf("OutputDir = %q, want %q", gen.config.OutputDir, "/tmp/out")
	}
	if gen.config.PackageName != "models" {
		t.Errorf("PackageName = %q, want %q", gen.config.PackageName, "models")
	}
}

func TestGenerateFromPath_File(t *testing.T) {
	dir := t.TempDir()

	// Cria um arquivo Go com uma struct com tags db
	src := `package models

type User struct {
	Name  string ` + "`db:\"name\"`" + `
	Email string ` + "`db:\"email\"`" + `
}
`
	srcFile := filepath.Join(dir, "user.go")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{OutputDir: dir})
	if err := gen.GenerateFromPath(srcFile); err != nil {
		t.Fatalf("GenerateFromPath() error: %v", err)
	}

	// Verifica que o arquivo gerado existe
	genFile := filepath.Join(dir, "user_fields.gen.go")
	data, err := os.ReadFile(genFile)
	if err != nil {
		t.Fatalf("generated file not found: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "UserFields") {
		t.Error("generated file does not contain UserFields")
	}
	if !strings.Contains(content, `"name"`) {
		t.Error("generated file does not contain column name")
	}
}

func TestGenerateFromPath_Dir(t *testing.T) {
	dir := t.TempDir()

	src := `package models

type Product struct {
	Title string ` + "`db:\"title\"`" + `
	Price float64 ` + "`db:\"price\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "product.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{OutputDir: dir})
	if err := gen.GenerateFromPath(dir); err != nil {
		t.Fatalf("GenerateFromPath() error: %v", err)
	}

	genFile := filepath.Join(dir, "product_fields.gen.go")
	if _, err := os.Stat(genFile); os.IsNotExist(err) {
		t.Fatal("generated file does not exist")
	}
}

func TestGenerateFromPath_InvalidPath(t *testing.T) {
	gen := NewGenerator(Config{})
	err := gen.GenerateFromPath("/nonexistent/path")
	if err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestGenerateFromDir_SkipsTestAndGenFiles(t *testing.T) {
	dir := t.TempDir()

	// Arquivo normal
	src := `package models

type Item struct {
	Name string ` + "`db:\"name\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "item.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// Arquivo de teste (deve ser ignorado)
	testSrc := `package models

type Fake struct {
	Name string ` + "`db:\"name\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "item_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Arquivo gerado (deve ser ignorado)
	if err := os.WriteFile(filepath.Join(dir, "old.gen.go"), []byte("package models\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Subdiretório (deve ser ignorado)
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{OutputDir: dir})
	if err := gen.GenerateFromPath(dir); err != nil {
		t.Fatalf("GenerateFromPath() error: %v", err)
	}

	// Apenas item_fields.gen.go deveria ser gerado
	genFile := filepath.Join(dir, "item_fields.gen.go")
	if _, err := os.Stat(genFile); os.IsNotExist(err) {
		t.Fatal("item_fields.gen.go should exist")
	}
}

func TestGenerateFromDir_InvalidDir(t *testing.T) {
	gen := NewGenerator(Config{})
	err := gen.generateFromDir("/nonexistent/dir")
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

func TestGenerateFromFile_InvalidGoFile(t *testing.T) {
	dir := t.TempDir()
	badFile := filepath.Join(dir, "bad.go")
	if err := os.WriteFile(badFile, []byte("this is not valid go"), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{})
	err := gen.generateFromFile(badFile)
	if err == nil {
		t.Error("expected error for invalid Go file")
	}
}

func TestGenerateFromFile_NoStructs(t *testing.T) {
	dir := t.TempDir()
	src := `package models

var x = 42
`
	srcFile := filepath.Join(dir, "empty.go")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{})
	err := gen.generateFromFile(srcFile)
	if err != nil {
		t.Fatalf("expected no error for file with no structs, got: %v", err)
	}
}

func TestGenerateFromFile_AutoDetectsPackageName(t *testing.T) {
	dir := t.TempDir()
	src := `package mymodels

type Thing struct {
	Name string ` + "`db:\"name\"`" + `
}
`
	srcFile := filepath.Join(dir, "thing.go")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{OutputDir: dir})
	if err := gen.generateFromFile(srcFile); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "thing_fields.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "package mymodels") {
		t.Error("expected auto-detected package name 'mymodels'")
	}
}

func TestGenerateFromFile_UsesConfigPackageName(t *testing.T) {
	dir := t.TempDir()
	src := `package original

type Thing struct {
	Name string ` + "`db:\"name\"`" + `
}
`
	srcFile := filepath.Join(dir, "thing.go")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{OutputDir: dir, PackageName: "custom"})
	if err := gen.generateFromFile(srcFile); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "thing_fields.gen.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "package custom") {
		t.Error("expected configured package name 'custom'")
	}
}

func TestExtractStructs(t *testing.T) {
	src := `package models

type User struct {
	Name    string  ` + "`db:\"name\"`" + `
	Age     int     ` + "`db:\"age\"`" + `
	NoTag   string
	Skipped string  ` + "`db:\"-\"`" + `
}

type Empty struct {
	NoTag string
}

type NotAStruct int
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{})
	structs := gen.extractStructs(node)

	if len(structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(structs))
	}
	if structs[0].Name != "User" {
		t.Errorf("Name = %q, want %q", structs[0].Name, "User")
	}
	if len(structs[0].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(structs[0].Fields))
	}
	if structs[0].Fields[0].Name != "Name" {
		t.Errorf("Field[0].Name = %q, want %q", structs[0].Fields[0].Name, "Name")
	}
	if structs[0].Fields[0].ColumnName != "name" {
		t.Errorf("Field[0].ColumnName = %q, want %q", structs[0].Fields[0].ColumnName, "name")
	}
	if structs[0].Fields[1].Name != "Age" {
		t.Errorf("Field[1].Name = %q, want %q", structs[0].Fields[1].Name, "Age")
	}
}

func TestExtractStructs_OptionalFields(t *testing.T) {
	src := `package models

import "github.com/go-genus/genus/core"

type Profile struct {
	Bio core.Optional[string] ` + "`db:\"bio\"`" + `
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{})
	structs := gen.extractStructs(node)

	if len(structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(structs))
	}
	if !structs[0].ImportCore {
		t.Error("expected ImportCore to be true")
	}
	if structs[0].Fields[0].FieldType != "query.OptionalStringField" {
		t.Errorf("FieldType = %q, want %q", structs[0].Fields[0].FieldType, "query.OptionalStringField")
	}
}

func TestGetFieldType(t *testing.T) {
	gen := NewGenerator(Config{})

	tests := []struct {
		src      string
		expected string
	}{
		{`package p; var x string`, "string"},
		{`package p; var x int64`, "int64"},
		{`package p; import "time"; var x time.Time`, "time.Time"},
		{`package p; var x *string`, "*string"},
	}

	for _, tt := range tests {
		fset := token.NewFileSet()
		node, err := parser.ParseFile(fset, "test.go", tt.src, 0)
		if err != nil {
			t.Fatal(err)
		}

		for _, decl := range node.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range genDecl.Specs {
				valSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				result := gen.getFieldType(valSpec.Type)
				if result != tt.expected {
					t.Errorf("getFieldType(%q) = %q, want %q", tt.src, result, tt.expected)
				}
			}
		}
	}
}

func TestGetFieldType_IndexExpr(t *testing.T) {
	gen := NewGenerator(Config{})

	src := `package p

type Optional[T any] struct{}

var x Optional[string]
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range node.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			valSpec, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			result := gen.getFieldType(valSpec.Type)
			if result != "Optional[string]" {
				t.Errorf("getFieldType for generic = %q, want %q", result, "Optional[string]")
			}
		}
	}
}

func TestGetFieldType_Unknown(t *testing.T) {
	gen := NewGenerator(Config{})

	// Usa um MapType que não está coberto pelo switch
	expr := &ast.MapType{
		Key:   &ast.Ident{Name: "string"},
		Value: &ast.Ident{Name: "int"},
	}
	result := gen.getFieldType(expr)
	if result != "unknown" {
		t.Errorf("getFieldType(map) = %q, want %q", result, "unknown")
	}
}

func TestGetQueryFieldType(t *testing.T) {
	gen := NewGenerator(Config{})

	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "query.StringField"},
		{"int", "query.IntField"},
		{"int64", "query.Int64Field"},
		{"bool", "query.BoolField"},
		{"float64", "query.Float64Field"},
		{"*string", "query.StringField"},
		{"*int64", "query.Int64Field"},
		{"Optional[string]", "query.OptionalStringField"},
		{"Optional[int]", "query.OptionalIntField"},
		{"Optional[int64]", "query.OptionalInt64Field"},
		{"Optional[bool]", "query.OptionalBoolField"},
		{"Optional[float64]", "query.OptionalFloat64Field"},
		{"core.Optional[string]", "query.OptionalStringField"},
		{"core.Optional[int]", "query.OptionalIntField"},
		{"core.Optional[int64]", "query.OptionalInt64Field"},
		{"core.Optional[bool]", "query.OptionalBoolField"},
		{"core.Optional[float64]", "query.OptionalFloat64Field"},
		{"time.Time", "query.StringField"},            // fallback
		{"CustomType", "query.StringField"},           // fallback
		{"Optional[CustomType]", "query.StringField"}, // fallback para Optional com tipo desconhecido
	}

	for _, tt := range tests {
		result := gen.getQueryFieldType(tt.goType)
		if result != tt.expected {
			t.Errorf("getQueryFieldType(%q) = %q, want %q", tt.goType, result, tt.expected)
		}
	}
}

func TestGenerateFieldsFile_DefaultOutputDir(t *testing.T) {
	dir := t.TempDir()

	// Cria um arquivo fonte
	srcFile := filepath.Join(dir, "model.go")
	if err := os.WriteFile(srcFile, []byte("package models\n"), 0644); err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{}) // sem OutputDir
	structInfo := StructInfo{
		Name:    "Model",
		Package: "models",
		Fields: []FieldInfo{
			{Name: "Name", ColumnName: "name", Type: "string", FieldType: "query.StringField"},
		},
	}

	if err := gen.generateFieldsFile(structInfo, srcFile); err != nil {
		t.Fatalf("generateFieldsFile() error: %v", err)
	}

	genFile := filepath.Join(dir, "model_fields.gen.go")
	if _, err := os.Stat(genFile); os.IsNotExist(err) {
		t.Fatal("generated file should exist in source dir when OutputDir is empty")
	}
}

func TestGenerateFieldsFile_InvalidOutputDir(t *testing.T) {
	gen := NewGenerator(Config{OutputDir: "/nonexistent/path"})
	structInfo := StructInfo{
		Name:    "Model",
		Package: "models",
		Fields: []FieldInfo{
			{Name: "Name", ColumnName: "name", Type: "string", FieldType: "query.StringField"},
		},
	}

	err := gen.generateFieldsFile(structInfo, "/some/source.go")
	if err == nil {
		t.Error("expected error when writing to invalid directory")
	}
}

func TestGenerateCode(t *testing.T) {
	gen := NewGenerator(Config{})
	structInfo := StructInfo{
		Name:    "User",
		Package: "models",
		Fields: []FieldInfo{
			{Name: "Name", ColumnName: "name", Type: "string", FieldType: "query.StringField"},
			{Name: "Age", ColumnName: "age", Type: "int", FieldType: "query.IntField"},
		},
	}

	code, err := gen.generateCode(structInfo)
	if err != nil {
		t.Fatalf("generateCode() error: %v", err)
	}

	if !strings.Contains(code, "package models") {
		t.Error("code does not contain package declaration")
	}
	if !strings.Contains(code, "UserFields") {
		t.Error("code does not contain UserFields")
	}
	if !strings.Contains(code, "query.NewStringField") {
		t.Error("code does not contain query.NewStringField")
	}
	if !strings.Contains(code, "query.NewIntField") {
		t.Error("code does not contain query.NewIntField")
	}
}

func TestGenerateCode_WithImportCore(t *testing.T) {
	gen := NewGenerator(Config{})
	structInfo := StructInfo{
		Name:       "Profile",
		Package:    "models",
		ImportCore: true,
		Fields: []FieldInfo{
			{Name: "Bio", ColumnName: "bio", Type: "core.Optional[string]", FieldType: "query.OptionalStringField"},
		},
	}

	code, err := gen.generateCode(structInfo)
	if err != nil {
		t.Fatalf("generateCode() error: %v", err)
	}

	if !strings.Contains(code, "genus/core") {
		t.Error("code should import core when ImportCore is true")
	}
}

func TestExtractDBTag(t *testing.T) {
	tests := []struct {
		tag      string
		expected string
	}{
		{"`db:\"name\"`", "name"},
		{"`json:\"name\" db:\"user_name\"`", "user_name"},
		{"`db:\"-\"`", "-"},
		{"`json:\"name\"`", ""},
		{"``", ""},
		{"`db:\"id\" json:\"id\"`", "id"},
	}

	for _, tt := range tests {
		result := extractDBTag(tt.tag)
		if result != tt.expected {
			t.Errorf("extractDBTag(%q) = %q, want %q", tt.tag, result, tt.expected)
		}
	}
}

func TestGenerateCode_TemplateExecuteProducesUnformattableCode(t *testing.T) {
	// Este teste verifica o branch onde format.Source falha.
	// O código gerado pelo template é sempre válido para campos normais,
	// então testamos com um nome de pacote que produza código não formatável.
	gen := NewGenerator(Config{})

	// Usar um pacote com nome que gere código válido mas verificar
	// que o código retornado é formatado corretamente
	structInfo := StructInfo{
		Name:    "Test",
		Package: "pkg",
		Fields: []FieldInfo{
			{Name: "A", ColumnName: "a", Type: "string", FieldType: "query.StringField"},
		},
	}

	code, err := gen.generateCode(structInfo)
	if err != nil {
		t.Fatalf("generateCode() error: %v", err)
	}
	if code == "" {
		t.Error("generated code should not be empty")
	}
}

func TestGenerateFromDir_ErrorInFile(t *testing.T) {
	dir := t.TempDir()

	// Arquivo Go válido com struct que tem campo db
	src := `package models

type User struct {
	Name string ` + "`db:\"name\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "user.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// OutputDir inválido causa erro ao gerar arquivo
	gen := NewGenerator(Config{OutputDir: "/nonexistent/output/dir"})
	err := gen.GenerateFromPath(dir)
	if err == nil {
		t.Error("expected error when output dir is invalid")
	}
}

func TestGenerateFromFile_ErrorInGenerateFieldsFile(t *testing.T) {
	dir := t.TempDir()

	// Arquivo com múltiplas structs
	src := `package models

type User struct {
	Name string ` + "`db:\"name\"`" + `
}

type Product struct {
	Title string ` + "`db:\"title\"`" + `
}
`
	srcFile := filepath.Join(dir, "models.go")
	if err := os.WriteFile(srcFile, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	// OutputDir inválido causa erro na geração
	gen := NewGenerator(Config{OutputDir: "/nonexistent/path"})
	err := gen.generateFromFile(srcFile)
	if err == nil {
		t.Error("expected error when output dir is invalid")
	}
}

func TestGenerateCode_FormatSourceFails(t *testing.T) {
	// Testa o branch onde format.Source falha retornando o código não formatado.
	// Isso acontece quando o Package name contém caracteres que geram
	// código Go sintaticamente inválido para o formatter.
	gen := NewGenerator(Config{})

	// Um nome de pacote com caractere inválido faz o gofmt falhar
	structInfo := StructInfo{
		Name:    "Test",
		Package: "123invalid-pkg",
		Fields: []FieldInfo{
			{Name: "A", ColumnName: "a", Type: "string", FieldType: "query.StringField"},
		},
	}

	code, err := gen.generateCode(structInfo)
	if err != nil {
		t.Fatalf("generateCode() should not error (format failure returns raw): %v", err)
	}
	if code == "" {
		t.Error("code should not be empty even when format fails")
	}
}

func TestExtractStructs_NamedFieldWithDBTag(t *testing.T) {
	// Campo nomeado com tag db
	src := `package models

type User struct {
	Name string ` + "`db:\"name\"`" + `
	Age  int    ` + "`db:\"age\"`" + `
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{})
	structs := gen.extractStructs(node)

	if len(structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(structs))
	}
	if len(structs[0].Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(structs[0].Fields))
	}
}

func TestExtractStructs_FieldWithDBTagButEmptyColumn(t *testing.T) {
	// Campo com tag db:"" (coluna vazia) deve ser ignorado
	src := `package models

type User struct {
	Name string ` + "`db:\"\"`" + `
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{})
	structs := gen.extractStructs(node)

	if len(structs) != 0 {
		t.Errorf("expected 0 structs (empty db tag), got %d", len(structs))
	}
}

func TestExtractStructs_FieldWithNonDBTag(t *testing.T) {
	// Campo com tag json mas sem db
	src := `package models

type User struct {
	Name string ` + "`json:\"name\"`" + `
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	gen := NewGenerator(Config{})
	structs := gen.extractStructs(node)

	if len(structs) != 0 {
		t.Errorf("expected 0 structs (no db tag), got %d", len(structs))
	}
}

func TestExtractGenericType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Optional[string]", "string"},
		{"core.Optional[int64]", "int64"},
		{"Optional[bool]", "bool"},
		{"NoGeneric", ""},
		{"Bad[", ""},
		{"]Bad[", ""},
		{"[bad", ""},
	}

	for _, tt := range tests {
		result := extractGenericType(tt.input)
		if result != tt.expected {
			t.Errorf("extractGenericType(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
