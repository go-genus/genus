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

func TestGenerateScannersForDir(t *testing.T) {
	dir := t.TempDir()

	src := `package models

type User struct {
	Name  string ` + "`db:\"name\"`" + `
	Email string ` + "`db:\"email\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "user.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateScannersForDir(dir); err != nil {
		t.Fatalf("GenerateScannersForDir() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "scanners.gen.go"))
	if err != nil {
		t.Fatal("scanners.gen.go should exist")
	}

	content := string(data)
	if !strings.Contains(content, "ScanUser") {
		t.Error("generated file should contain ScanUser")
	}
	if !strings.Contains(content, "ScanUsers") {
		t.Error("generated file should contain ScanUsers")
	}
	if !strings.Contains(content, "UserColumns") {
		t.Error("generated file should contain UserColumns")
	}
}

func TestGenerateScannersForDir_InvalidDir(t *testing.T) {
	err := GenerateScannersForDir("/nonexistent/path")
	if err == nil {
		t.Error("expected error for invalid directory")
	}
}

func TestGenerateScannersForDir_NoModels(t *testing.T) {
	dir := t.TempDir()

	src := `package models

var x = 42
`
	if err := os.WriteFile(filepath.Join(dir, "empty.go"), []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	err := GenerateScannersForDir(dir)
	if err == nil {
		t.Error("expected error when no models found")
	}
	if !strings.Contains(err.Error(), "no models found") {
		t.Errorf("error = %q, should contain 'no models found'", err.Error())
	}
}

func TestGenerateScannersForDir_SkipsTestAndGenFiles(t *testing.T) {
	dir := t.TempDir()

	// Arquivo normal com model
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

type FakeModel struct {
	Fake string ` + "`db:\"fake\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "item_test.go"), []byte(testSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// Arquivo gerado (deve ser ignorado)
	if err := os.WriteFile(filepath.Join(dir, "old.gen.go"), []byte("package models\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateScannersForDir(dir); err != nil {
		t.Fatalf("GenerateScannersForDir() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "scanners.gen.go"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if strings.Contains(content, "FakeModel") {
		t.Error("generated file should not contain FakeModel from test file")
	}
}

func TestScannerGenerator_Generate(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "scanners.gen.go")

	gen := &ScannerGenerator{
		PackageName: "models",
		Models: []ModelInfo{
			{
				Name:      "User",
				TableName: "user",
				Fields: []ScanField{
					{Name: "Name", Column: "name", Type: "string", Path: "Name"},
					{Name: "Email", Column: "email", Type: "string", Path: "Email"},
				},
			},
		},
	}

	if err := gen.Generate(outputPath); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "package models") {
		t.Error("missing package declaration")
	}
	if !strings.Contains(content, "ScanUser") {
		t.Error("missing ScanUser function")
	}
	if !strings.Contains(content, "ScanUsersWithCap") {
		t.Error("missing ScanUsersWithCap function")
	}
	if !strings.Contains(content, "UserColumnsString") {
		t.Error("missing UserColumnsString function")
	}
}

func TestScannerGenerator_Generate_InvalidPath(t *testing.T) {
	gen := &ScannerGenerator{
		PackageName: "models",
		Models: []ModelInfo{
			{
				Name:      "User",
				TableName: "user",
				Fields: []ScanField{
					{Name: "Name", Column: "name", Type: "string", Path: "Name"},
				},
			},
		},
	}

	err := gen.Generate("/nonexistent/dir/scanners.gen.go")
	if err == nil {
		t.Error("expected error for invalid output path")
	}
}

func TestExtractModels(t *testing.T) {
	src := `package models

type User struct {
	Name  string ` + "`db:\"name\"`" + `
	Email string ` + "`db:\"email\"`" + `
}

type Empty struct {
	unexported string
}

type NotAStruct int
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	models := extractModels(node)
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if models[0].Name != "User" {
		t.Errorf("Name = %q, want %q", models[0].Name, "User")
	}
	if len(models[0].Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(models[0].Fields))
	}
}

func TestExtractModels_WithCoreModel(t *testing.T) {
	src := `package models

type User struct {
	core.Model
	Name string ` + "`db:\"name\"`" + `
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	models := extractModels(node)
	if len(models) != 1 {
		t.Fatalf("expected 1 model, got %d", len(models))
	}
	if !models[0].HasModel {
		t.Error("expected HasModel to be true")
	}

	// core.Model adiciona 3 campos (ID, CreatedAt, UpdatedAt) + Name
	if len(models[0].Fields) != 4 {
		t.Errorf("expected 4 fields (3 from core.Model + Name), got %d", len(models[0].Fields))
	}
}

func TestParseStruct(t *testing.T) {
	src := `package models

type User struct {
	Name    string ` + "`db:\"name\"`" + `
	Age     int    ` + "`db:\"age\"`" + `
	Skipped string ` + "`db:\"-\"`" + `
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var structType *ast.StructType
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				structType = st
			}
		}
		return true
	})

	model := parseStruct("User", structType)
	if model == nil {
		t.Fatal("parseStruct returned nil")
	}
	if model.Name != "User" {
		t.Errorf("Name = %q, want %q", model.Name, "User")
	}
	if model.TableName != "user" {
		t.Errorf("TableName = %q, want %q", model.TableName, "user")
	}
	// Name + Age (Skipped has "-" tag, so column is "-" and gets skipped)
	if len(model.Fields) != 2 {
		t.Errorf("expected 2 fields, got %d", len(model.Fields))
	}
}

func TestParseStruct_NoDBTags(t *testing.T) {
	src := `package models

type Plain struct {
	Name string
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var structType *ast.StructType
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				structType = st
			}
		}
		return true
	})

	// Exported field without db tag still gets a column via snake_case
	model := parseStruct("Plain", structType)
	// The model has fields but no db tags, and doesn't embed core.Model
	// parseStruct checks hasDBTags: fields without explicit db tag get column via toSnakeCase
	// Since column is not empty (it's "name"), hasDBTags will be true
	if model == nil {
		t.Fatal("parseStruct returned nil for struct with exported field")
	}
}

func TestParseStruct_OnlyUnexported(t *testing.T) {
	src := `package models

type Internal struct {
	name string
	age  int
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var structType *ast.StructType
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				structType = st
			}
		}
		return true
	})

	model := parseStruct("Internal", structType)
	if model != nil {
		t.Error("expected nil for struct with only unexported fields")
	}
}

func TestParseField_EmbeddedCoreModel(t *testing.T) {
	src := `package models

type User struct {
	core.Model
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var fields []ScanField
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				for _, f := range st.Fields.List {
					fields = append(fields, parseField(f, "")...)
				}
			}
		}
		return true
	})

	if len(fields) != 3 {
		t.Fatalf("expected 3 embedded fields from core.Model, got %d", len(fields))
	}

	expected := map[string]string{
		"ID":        "id",
		"CreatedAt": "created_at",
		"UpdatedAt": "updated_at",
	}
	for _, f := range fields {
		if !f.IsEmbedded {
			t.Errorf("field %q should be embedded", f.Name)
		}
		if col, ok := expected[f.Name]; ok {
			if f.Column != col {
				t.Errorf("field %q column = %q, want %q", f.Name, f.Column, col)
			}
		} else {
			t.Errorf("unexpected field %q", f.Name)
		}
	}
}

func TestParseField_EmbeddedNonCoreModel(t *testing.T) {
	src := `package models

type User struct {
	SomeOther
}

type SomeOther struct {}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var fields []ScanField
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if ts.Name.Name == "User" {
				if st, ok := ts.Type.(*ast.StructType); ok {
					for _, f := range st.Fields.List {
						fields = append(fields, parseField(f, "")...)
					}
				}
			}
		}
		return true
	})

	// Embedded Ident (not SelectorExpr) should return empty
	if len(fields) != 0 {
		t.Errorf("expected 0 fields for non-core.Model embedded, got %d", len(fields))
	}
}

func TestParseField_WithPrefix(t *testing.T) {
	src := `package models

type User struct {
	Name string ` + "`db:\"name\"`" + `
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var fields []ScanField
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				for _, f := range st.Fields.List {
					fields = append(fields, parseField(f, "Parent")...)
				}
			}
		}
		return true
	})

	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	if fields[0].Path != "Parent.Name" {
		t.Errorf("Path = %q, want %q", fields[0].Path, "Parent.Name")
	}
}

func TestParseField_NoTag(t *testing.T) {
	src := `package models

type User struct {
	Name string
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var fields []ScanField
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				for _, f := range st.Fields.List {
					fields = append(fields, parseField(f, "")...)
				}
			}
		}
		return true
	})

	if len(fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(fields))
	}
	// Sem tag, deve usar snake_case do nome
	if fields[0].Column != "name" {
		t.Errorf("Column = %q, want %q", fields[0].Column, "name")
	}
}

func TestParseDBTag(t *testing.T) {
	tests := []struct {
		tag      string
		expected string
	}{
		{`db:"name"`, "name"},
		{`json:"j" db:"user_name"`, "user_name"},
		{`db:"-"`, "-"},
		{`json:"j"`, ""},
		{``, ""},
		{`db:"id" json:"id"`, "id"},
	}

	for _, tt := range tests {
		result := parseDBTag(tt.tag)
		if result != tt.expected {
			t.Errorf("parseDBTag(%q) = %q, want %q", tt.tag, result, tt.expected)
		}
	}
}

func TestParseDBTag_MalformedTag(t *testing.T) {
	// Tag com db:" mas sem fechar aspas
	result := parseDBTag(`db:"unterminated`)
	if result != "" {
		t.Errorf("parseDBTag with unterminated quote = %q, want empty", result)
	}
}

func TestTypeToString(t *testing.T) {
	tests := []struct {
		name     string
		expr     ast.Expr
		expected string
	}{
		{
			"ident",
			&ast.Ident{Name: "string"},
			"string",
		},
		{
			"selector",
			&ast.SelectorExpr{
				X:   &ast.Ident{Name: "time"},
				Sel: &ast.Ident{Name: "Time"},
			},
			"time.Time",
		},
		{
			"star",
			&ast.StarExpr{X: &ast.Ident{Name: "string"}},
			"*string",
		},
		{
			"slice",
			&ast.ArrayType{Elt: &ast.Ident{Name: "byte"}},
			"[]byte",
		},
		{
			"unknown",
			&ast.MapType{
				Key:   &ast.Ident{Name: "string"},
				Value: &ast.Ident{Name: "int"},
			},
			"interface{}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := typeToString(tt.expr)
			if result != tt.expected {
				t.Errorf("typeToString() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestTypeToString_SelectorWithNonIdent(t *testing.T) {
	// SelectorExpr onde X não é *ast.Ident
	expr := &ast.SelectorExpr{
		X:   &ast.StarExpr{X: &ast.Ident{Name: "pkg"}},
		Sel: &ast.Ident{Name: "Type"},
	}
	result := typeToString(expr)
	if result != "interface{}" {
		t.Errorf("typeToString(non-ident selector) = %q, want %q", result, "interface{}")
	}
}

func TestTypeToString_ArrayWithLength(t *testing.T) {
	// Array com comprimento definido (não slice)
	expr := &ast.ArrayType{
		Len: &ast.BasicLit{Value: "5"},
		Elt: &ast.Ident{Name: "int"},
	}
	result := typeToString(expr)
	if result != "interface{}" {
		t.Errorf("typeToString(fixed array) = %q, want %q", result, "interface{}")
	}
}

func TestToSnakeCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User", "user"},
		{"UserName", "user_name"},
		{"ID", "i_d"},
		{"CreatedAt", "created_at"},
		{"HTMLParser", "h_t_m_l_parser"},
		{"simple", "simple"},
		{"A", "a"},
		{"", ""},
	}

	for _, tt := range tests {
		result := toSnakeCase(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCase(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestGenerateScannersForDir_MultipleModels(t *testing.T) {
	dir := t.TempDir()

	src1 := `package models

type User struct {
	Name  string ` + "`db:\"name\"`" + `
}
`
	src2 := `package models

type Product struct {
	Title string ` + "`db:\"title\"`" + `
	Price float64 ` + "`db:\"price\"`" + `
}
`
	if err := os.WriteFile(filepath.Join(dir, "user.go"), []byte(src1), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "product.go"), []byte(src2), 0644); err != nil {
		t.Fatal(err)
	}

	if err := GenerateScannersForDir(dir); err != nil {
		t.Fatalf("GenerateScannersForDir() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "scanners.gen.go"))
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "ScanUser") {
		t.Error("missing ScanUser")
	}
	if !strings.Contains(content, "ScanProduct") {
		t.Error("missing ScanProduct")
	}
}

func TestScannerGenerator_Generate_MultipleModels(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "scanners.gen.go")

	gen := &ScannerGenerator{
		PackageName: "models",
		Models: []ModelInfo{
			{
				Name:      "User",
				TableName: "user",
				Fields: []ScanField{
					{Name: "Name", Column: "name", Type: "string", Path: "Name"},
				},
			},
			{
				Name:      "Product",
				TableName: "product",
				Fields: []ScanField{
					{Name: "Title", Column: "title", Type: "string", Path: "Title"},
					{Name: "Price", Column: "price", Type: "float64", Path: "Price"},
				},
			},
		},
	}

	if err := gen.Generate(outputPath); err != nil {
		t.Fatalf("Generate() error: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	content := string(data)
	if !strings.Contains(content, "ScanUser") {
		t.Error("missing ScanUser")
	}
	if !strings.Contains(content, "ScanProduct") {
		t.Error("missing ScanProduct")
	}
	if !strings.Contains(content, "ProductColumnsString") {
		t.Error("missing ProductColumnsString")
	}
}

func TestParseField_EmbeddedSelectorNonCoreModel(t *testing.T) {
	// Embedded field com SelectorExpr mas não é core.Model
	src := `package models

type User struct {
	other.Thing
}
`
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}

	var fields []ScanField
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok {
			if st, ok := ts.Type.(*ast.StructType); ok {
				for _, f := range st.Fields.List {
					fields = append(fields, parseField(f, "")...)
				}
			}
		}
		return true
	})

	if len(fields) != 0 {
		t.Errorf("expected 0 fields for embedded non-core.Model selector, got %d", len(fields))
	}
}
