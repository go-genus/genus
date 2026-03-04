package query

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// fieldPath representa o caminho para um campo através de embedded structs
type fieldPath []int

// fieldMapCache caches field maps per type to avoid rebuilding on every scan.
var fieldMapCache sync.Map // map[reflect.Type]map[string]fieldPath

// scanStruct faz o scan de uma row para uma struct.
// Usa cache de field maps para evitar reflection repetida.
func scanStruct(rows *sql.Rows, dest interface{}) error {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return fmt.Errorf("dest must be a pointer")
	}

	destValue = destValue.Elem()
	if destValue.Kind() != reflect.Struct {
		return fmt.Errorf("dest must be a pointer to struct")
	}

	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// Get or build cached field map
	fieldMap := getOrBuildFieldMap(destValue.Type())

	// Create scan values slice
	numCols := len(columns)
	scanValues := make([]interface{}, numCols)

	// Use a single placeholder for unmapped columns
	var placeholder interface{}

	for i, colName := range columns {
		if path, ok := fieldMap[colName]; ok {
			field := getFieldByPath(destValue, path)
			if field.IsValid() && field.CanAddr() {
				scanValues[i] = field.Addr().Interface()
			} else {
				scanValues[i] = &placeholder
			}
		} else {
			scanValues[i] = &placeholder
		}
	}

	return rows.Scan(scanValues...)
}

// scanStructFast is an optimized version that reuses scan value slices.
// Call with pre-allocated scanValues slice for better performance in loops.
func scanStructFast(rows *sql.Rows, dest interface{}, columns []string, fieldMap map[string]fieldPath, scanValues []interface{}) error {
	destValue := reflect.ValueOf(dest).Elem()

	// Use a single placeholder for unmapped columns
	var placeholder interface{}

	for i, colName := range columns {
		if path, ok := fieldMap[colName]; ok {
			field := getFieldByPath(destValue, path)
			if field.IsValid() && field.CanAddr() {
				scanValues[i] = field.Addr().Interface()
			} else {
				scanValues[i] = &placeholder
			}
		} else {
			scanValues[i] = &placeholder
		}
	}

	return rows.Scan(scanValues...)
}

// buildFieldMap builds a field map for a type (wrapper for compatibility).
func buildFieldMap(typ reflect.Type) map[string]fieldPath {
	return getOrBuildFieldMap(typ)
}

// getOrBuildFieldMap returns a cached field map or builds and caches one.
func getOrBuildFieldMap(typ reflect.Type) map[string]fieldPath {
	if cached, ok := fieldMapCache.Load(typ); ok {
		return cached.(map[string]fieldPath)
	}

	fieldMap := buildFieldMapWithPrefix(typ, nil)
	fieldMapCache.Store(typ, fieldMap)
	return fieldMap
}

// GetCachedFieldMap returns the cached field map for a type.
// Exported for use in builder.go.
func GetCachedFieldMap(typ reflect.Type) map[string]fieldPath {
	return getOrBuildFieldMap(typ)
}

// getFieldByPath navega até um campo usando um caminho de índices
func getFieldByPath(value reflect.Value, path fieldPath) reflect.Value {
	current := value
	for _, index := range path {
		if current.Kind() == reflect.Ptr {
			current = current.Elem()
		}
		if index >= current.NumField() {
			return reflect.Value{}
		}
		current = current.Field(index)
	}
	return current
}

// buildFieldMapWithPrefix constrói um mapa de nome de coluna -> caminho do campo com suporte para embedded structs.
func buildFieldMapWithPrefix(typ reflect.Type, parentPath fieldPath) map[string]fieldPath {
	fieldMap := make(map[string]fieldPath)

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		currentPath := make(fieldPath, len(parentPath)+1)
		copy(currentPath, parentPath)
		currentPath[len(parentPath)] = i

		// Se o campo é um embedded struct, processa recursivamente
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			embeddedMap := buildFieldMapWithPrefix(field.Type, currentPath)

			// Mescla o mapa do embedded struct com o mapa atual
			for k, v := range embeddedMap {
				fieldMap[k] = v
			}
			continue
		}

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Obtém o nome da coluna da tag `db`
		colName := field.Tag.Get("db")
		if colName == "-" {
			continue
		}
		if colName == "" {
			// Use snake_case of field name as default
			colName = toSnakeCase(field.Name)
		}

		fieldMap[colName] = currentPath
	}

	return fieldMap
}

// toSnakeCase converte CamelCase para snake_case.
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

// GetFieldIndices retorna os índices dos campos de uma struct para scanning.
// Usado internamente pelo scanner.
func GetFieldIndices(dest interface{}) ([]interface{}, error) {
	destValue := reflect.ValueOf(dest)
	if destValue.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("dest must be a pointer")
	}

	destValue = destValue.Elem()
	if destValue.Kind() != reflect.Struct {
		return nil, fmt.Errorf("dest must be a pointer to struct")
	}

	var indices []interface{}
	for i := 0; i < destValue.NumField(); i++ {
		field := destValue.Field(i)
		if field.CanAddr() {
			indices = append(indices, field.Addr().Interface())
		}
	}

	return indices, nil
}

// ClearFieldMapCache clears the field map cache. Useful for testing.
func ClearFieldMapCache() {
	fieldMapCache = sync.Map{}
}
