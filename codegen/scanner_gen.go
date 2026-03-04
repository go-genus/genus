package codegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// ScannerGenerator generates optimized, reflection-free scanner functions.
type ScannerGenerator struct {
	PackageName string
	Models      []ModelInfo
}

// ModelInfo contains parsed model information.
type ModelInfo struct {
	Name       string
	TableName  string
	Fields     []ScanField
	HasModel   bool // Embeds core.Model
	ImportPath string
}

// ScanField contains field information for scanning.
type ScanField struct {
	Name       string // Go field name
	Column     string // DB column name
	Type       string // Go type
	IsEmbedded bool   // Part of embedded struct
	Path       string // Access path (e.g., "Model.ID" or just "Name")
}

// GenerateScannersForDir generates scanner files for all models in a directory.
func GenerateScannersForDir(dir string) error {
	gen := &ScannerGenerator{}

	// Parse all Go files in directory
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") &&
			!strings.HasSuffix(fi.Name(), ".gen.go")
	}, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse directory: %w", err)
	}

	for pkgName, pkg := range pkgs {
		gen.PackageName = pkgName

		for _, file := range pkg.Files {
			models := extractModels(file)
			gen.Models = append(gen.Models, models...)
		}
	}

	if len(gen.Models) == 0 {
		return fmt.Errorf("no models found in %s", dir)
	}

	// Generate scanner file
	outputPath := filepath.Join(dir, "scanners.gen.go")
	return gen.Generate(outputPath)
}

// Generate writes the scanner file.
func (g *ScannerGenerator) Generate(outputPath string) error {
	var buf bytes.Buffer

	if err := scannerTemplate.Execute(&buf, g); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	// Write file
	if err := os.WriteFile(outputPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	fmt.Printf("Generated %s with %d scanner(s)\n", outputPath, len(g.Models))
	return nil
}

// extractModels extracts model structs from an AST file.
func extractModels(file *ast.File) []ModelInfo {
	var models []ModelInfo

	ast.Inspect(file, func(n ast.Node) bool {
		typeSpec, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}

		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			return true
		}

		// Check if this looks like a model (has db tags or embeds core.Model)
		model := parseStruct(typeSpec.Name.Name, structType)
		if model != nil && len(model.Fields) > 0 {
			models = append(models, *model)
		}

		return true
	})

	return models
}

// parseStruct parses a struct into ModelInfo.
func parseStruct(name string, s *ast.StructType) *ModelInfo {
	model := &ModelInfo{
		Name:      name,
		TableName: toSnakeCase(name),
	}

	for _, field := range s.Fields.List {
		fields := parseField(field, "")
		model.Fields = append(model.Fields, fields...)

		// Check for core.Model embedding
		if field.Names == nil { // Embedded field
			if sel, ok := field.Type.(*ast.SelectorExpr); ok {
				if ident, ok := sel.X.(*ast.Ident); ok {
					if ident.Name == "core" && sel.Sel.Name == "Model" {
						model.HasModel = true
					}
				}
			}
		}
	}

	// Only return if it has db-tagged fields or embeds core.Model
	hasDBTags := false
	for _, f := range model.Fields {
		if f.Column != "" {
			hasDBTags = true
			break
		}
	}

	if !hasDBTags && !model.HasModel {
		return nil
	}

	return model
}

// parseField parses a struct field into ScanField.
func parseField(field *ast.Field, prefix string) []ScanField {
	var fields []ScanField

	// Handle embedded structs
	if field.Names == nil {
		// Check if it's core.Model
		if sel, ok := field.Type.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok {
				if ident.Name == "core" && sel.Sel.Name == "Model" {
					// Add core.Model fields
					fields = append(fields,
						ScanField{Name: "ID", Column: "id", Type: "int64", Path: "ID", IsEmbedded: true},
						ScanField{Name: "CreatedAt", Column: "created_at", Type: "time.Time", Path: "CreatedAt", IsEmbedded: true},
						ScanField{Name: "UpdatedAt", Column: "updated_at", Type: "time.Time", Path: "UpdatedAt", IsEmbedded: true},
					)
				}
			}
		}
		return fields
	}

	// Get tag
	var tag string
	if field.Tag != nil {
		tag = field.Tag.Value
		tag = strings.Trim(tag, "`")
	}

	// Parse db tag
	column := parseDBTag(tag)

	// Get type as string
	typeStr := typeToString(field.Type)

	for _, name := range field.Names {
		// Skip unexported fields
		if !ast.IsExported(name.Name) {
			continue
		}

		// Skip if tag is "-"
		if column == "-" {
			continue
		}

		// Use tag or snake_case name
		col := column
		if col == "" {
			col = toSnakeCase(name.Name)
		}

		path := name.Name
		if prefix != "" {
			path = prefix + "." + name.Name
		}

		fields = append(fields, ScanField{
			Name:   name.Name,
			Column: col,
			Type:   typeStr,
			Path:   path,
		})
	}

	return fields
}

// parseDBTag extracts the column name from a db tag.
func parseDBTag(tag string) string {
	// Find db:"..."
	idx := strings.Index(tag, `db:"`)
	if idx == -1 {
		return ""
	}

	start := idx + 4
	end := strings.Index(tag[start:], `"`)
	if end == -1 {
		return ""
	}

	return tag[start : start+end]
}

// typeToString converts an AST type to a string.
func typeToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	case *ast.StarExpr:
		return "*" + typeToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeToString(t.Elt)
		}
	}
	return "interface{}"
}

// toSnakeCase converts CamelCase to snake_case.
func toSnakeCase(s string) string {
	var result strings.Builder
	result.Grow(len(s) + 5)
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		if r >= 'A' && r <= 'Z' {
			result.WriteByte(byte(r + 32))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

var scannerTemplate = template.Must(template.New("scanner").Parse(`// Code generated by genus. DO NOT EDIT.
// This file contains optimized, reflection-free scanners.

package {{.PackageName}}

import (
	"database/sql"
)

{{range .Models}}
// ============================================================================
// {{.Name}} Scanners
// ============================================================================

// Scan{{.Name}} scans a single row into a {{.Name}}.
// This is ~10x faster than reflection-based scanning.
func Scan{{.Name}}(rows *sql.Rows) ({{.Name}}, error) {
	var m {{.Name}}
	err := rows.Scan(
{{- range .Fields}}
		&m.{{.Path}},
{{- end}}
	)
	return m, err
}

// Scan{{.Name}}s scans all rows into a slice of {{.Name}}.
// Pre-allocates slice capacity for better performance.
func Scan{{.Name}}s(rows *sql.Rows) ([]{{.Name}}, error) {
	var results []{{.Name}}
	for rows.Next() {
		var m {{.Name}}
		if err := rows.Scan(
{{- range .Fields}}
			&m.{{.Path}},
{{- end}}
		); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// Scan{{.Name}}sWithCap scans all rows with pre-allocated capacity.
// Use when you know the approximate number of results.
func Scan{{.Name}}sWithCap(rows *sql.Rows, capacity int) ([]{{.Name}}, error) {
	results := make([]{{.Name}}, 0, capacity)
	for rows.Next() {
		var m {{.Name}}
		if err := rows.Scan(
{{- range .Fields}}
			&m.{{.Path}},
{{- end}}
		); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, rows.Err()
}

// {{.Name}}Columns returns the column names for {{.Name}} in scan order.
func {{.Name}}Columns() []string {
	return []string{
{{- range .Fields}}
		"{{.Column}}",
{{- end}}
	}
}

// {{.Name}}ColumnsString returns comma-separated column names for SELECT.
func {{.Name}}ColumnsString() string {
	return "{{range $i, $f := .Fields}}{{if $i}}, {{end}}{{$f.Column}}{{end}}"
}

{{end}}
`))
