package core

import (
	"reflect"
	"strings"
	"sync"
)

// TypeMetadata holds cached reflection information for a struct type.
// This avoids repeated reflect.TypeOf() and field lookups.
type TypeMetadata struct {
	Type       reflect.Type
	TableName  string
	Columns    []string          // Column names in order
	FieldIndex map[string][]int  // Column name -> struct field index path
	HasID      bool
	IDIndex    []int // Index path to ID field
	HasCreated bool
	CreatedIdx []int
	HasUpdated bool
	UpdatedIdx []int
}

var (
	metadataCache sync.Map // map[reflect.Type]*TypeMetadata
)

// GetTypeMetadata returns cached metadata for a type, or builds and caches it.
func GetTypeMetadata(t reflect.Type) *TypeMetadata {
	// Dereference pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check cache first
	if cached, ok := metadataCache.Load(t); ok {
		return cached.(*TypeMetadata)
	}

	// Build metadata
	meta := buildTypeMetadata(t)

	// Store in cache (LoadOrStore handles race conditions)
	actual, _ := metadataCache.LoadOrStore(t, meta)
	return actual.(*TypeMetadata)
}

// GetTypeMetadataFromValue returns cached metadata from a reflect.Value.
func GetTypeMetadataFromValue(v reflect.Value) *TypeMetadata {
	t := v.Type()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return GetTypeMetadata(t)
}

// buildTypeMetadata builds metadata by inspecting struct fields.
func buildTypeMetadata(t reflect.Type) *TypeMetadata {
	meta := &TypeMetadata{
		Type:       t,
		FieldIndex: make(map[string][]int),
	}

	// Get table name
	meta.TableName = toSnakeCase(t.Name())

	// Build field index map
	buildFieldIndex(t, nil, meta)

	return meta
}

// buildFieldIndex recursively builds field index for embedded structs.
func buildFieldIndex(t reflect.Type, indexPath []int, meta *TypeMetadata) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		currentPath := append(append([]int{}, indexPath...), i)

		// Handle embedded structs (like core.Model)
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			buildFieldIndex(field.Type, currentPath, meta)
			continue
		}

		// Skip unexported fields
		if field.PkgPath != "" {
			continue
		}

		// Get column name from tag or field name
		colName := field.Tag.Get("db")
		if colName == "" || colName == "-" {
			colName = toSnakeCase(field.Name)
		}

		// Skip fields marked as "-"
		if colName == "-" {
			continue
		}

		meta.Columns = append(meta.Columns, colName)
		meta.FieldIndex[colName] = currentPath

		// Track special fields
		switch strings.ToLower(field.Name) {
		case "id":
			meta.HasID = true
			meta.IDIndex = currentPath
		case "createdat":
			meta.HasCreated = true
			meta.CreatedIdx = currentPath
		case "updatedat":
			meta.HasUpdated = true
			meta.UpdatedIdx = currentPath
		}
	}
}

// GetFieldValue gets a field value using cached index path.
func (m *TypeMetadata) GetFieldValue(v reflect.Value, column string) reflect.Value {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if idx, ok := m.FieldIndex[column]; ok {
		return v.FieldByIndex(idx)
	}
	return reflect.Value{}
}

// SetFieldValue sets a field value using cached index path.
func (m *TypeMetadata) SetFieldValue(v reflect.Value, column string, val interface{}) bool {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if idx, ok := m.FieldIndex[column]; ok {
		field := v.FieldByIndex(idx)
		if field.CanSet() {
			field.Set(reflect.ValueOf(val))
			return true
		}
	}
	return false
}

// GetID returns the ID field value.
func (m *TypeMetadata) GetID(v reflect.Value) int64 {
	if !m.HasID {
		return 0
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	field := v.FieldByIndex(m.IDIndex)
	return field.Int()
}

// SetID sets the ID field value.
func (m *TypeMetadata) SetID(v reflect.Value, id int64) {
	if !m.HasID {
		return
	}
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	field := v.FieldByIndex(m.IDIndex)
	if field.CanSet() {
		field.SetInt(id)
	}
}

// ClearMetadataCache clears the type metadata cache.
// Useful for testing.
func ClearMetadataCache() {
	metadataCache = sync.Map{}
}
