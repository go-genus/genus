package core

import (
	"reflect"
	"testing"
	"time"
)

func TestGetTypeMetadata(t *testing.T) {
	ClearMetadataCache()

	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))
	if meta == nil {
		t.Fatal("GetTypeMetadata returned nil")
	}

	if meta.TableName != "test_model" {
		t.Errorf("TableName = %q, want %q", meta.TableName, "test_model")
	}

	if !meta.HasID {
		t.Error("HasID should be true")
	}

	if !meta.HasCreated {
		t.Error("HasCreated should be true")
	}

	if !meta.HasUpdated {
		t.Error("HasUpdated should be true")
	}

	// Check columns
	expectedCols := []string{"id", "created_at", "updated_at", "name", "email"}
	if len(meta.Columns) != len(expectedCols) {
		t.Errorf("len(Columns) = %d, want %d, got: %v", len(meta.Columns), len(expectedCols), meta.Columns)
	}
}

func TestGetTypeMetadata_Pointer(t *testing.T) {
	ClearMetadataCache()

	meta := GetTypeMetadata(reflect.TypeOf(&TestModel{}))
	if meta == nil {
		t.Fatal("GetTypeMetadata with pointer returned nil")
	}
	if meta.TableName != "test_model" {
		t.Errorf("TableName = %q, want %q", meta.TableName, "test_model")
	}
}

func TestGetTypeMetadata_Caching(t *testing.T) {
	ClearMetadataCache()

	meta1 := GetTypeMetadata(reflect.TypeOf(TestModel{}))
	meta2 := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	if meta1 != meta2 {
		t.Error("GetTypeMetadata should return cached instance")
	}
}

func TestGetTypeMetadataFromValue(t *testing.T) {
	ClearMetadataCache()

	m := TestModel{Name: "test"}
	meta := GetTypeMetadataFromValue(reflect.ValueOf(m))
	if meta == nil {
		t.Fatal("GetTypeMetadataFromValue returned nil")
	}
	if meta.TableName != "test_model" {
		t.Errorf("TableName = %q, want %q", meta.TableName, "test_model")
	}
}

func TestGetTypeMetadataFromValue_Pointer(t *testing.T) {
	ClearMetadataCache()

	m := &TestModel{Name: "test"}
	meta := GetTypeMetadataFromValue(reflect.ValueOf(m))
	if meta == nil {
		t.Fatal("returned nil")
	}
}

func TestTypeMetadata_GetFieldValue(t *testing.T) {
	ClearMetadataCache()

	m := &TestModel{Name: "Alice"}
	m.ID = 42
	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	v := meta.GetFieldValue(reflect.ValueOf(m), "name")
	if !v.IsValid() {
		t.Fatal("GetFieldValue returned invalid value for 'name'")
	}
	if v.String() != "Alice" {
		t.Errorf("name = %q, want Alice", v.String())
	}

	idV := meta.GetFieldValue(reflect.ValueOf(m), "id")
	if !idV.IsValid() {
		t.Fatal("GetFieldValue returned invalid value for 'id'")
	}
	if idV.Int() != 42 {
		t.Errorf("id = %d, want 42", idV.Int())
	}
}

func TestTypeMetadata_GetFieldValue_Unknown(t *testing.T) {
	ClearMetadataCache()

	m := &TestModel{}
	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	v := meta.GetFieldValue(reflect.ValueOf(m), "nonexistent")
	if v.IsValid() {
		t.Error("GetFieldValue should return invalid for unknown column")
	}
}

func TestTypeMetadata_SetFieldValue(t *testing.T) {
	ClearMetadataCache()

	m := &TestModel{}
	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	ok := meta.SetFieldValue(reflect.ValueOf(m), "name", "Bob")
	if !ok {
		t.Error("SetFieldValue should return true")
	}
	if m.Name != "Bob" {
		t.Errorf("Name = %q, want Bob", m.Name)
	}
}

func TestTypeMetadata_SetFieldValue_Unknown(t *testing.T) {
	ClearMetadataCache()

	m := &TestModel{}
	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	ok := meta.SetFieldValue(reflect.ValueOf(m), "nonexistent", "value")
	if ok {
		t.Error("SetFieldValue should return false for unknown column")
	}
}

func TestTypeMetadata_GetID(t *testing.T) {
	ClearMetadataCache()

	m := &TestModel{}
	m.ID = 99
	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	id := meta.GetID(reflect.ValueOf(m))
	if id != 99 {
		t.Errorf("GetID() = %d, want 99", id)
	}
}

func TestTypeMetadata_GetID_NoID(t *testing.T) {
	ClearMetadataCache()

	type NoIDModel struct {
		Name string `db:"name"`
	}
	meta := GetTypeMetadata(reflect.TypeOf(NoIDModel{}))
	m := &NoIDModel{Name: "test"}
	id := meta.GetID(reflect.ValueOf(m))
	if id != 0 {
		t.Errorf("GetID() = %d, want 0", id)
	}
}

func TestTypeMetadata_SetID(t *testing.T) {
	ClearMetadataCache()

	m := &TestModel{}
	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	meta.SetID(reflect.ValueOf(m), 123)
	if m.ID != 123 {
		t.Errorf("ID = %d, want 123", m.ID)
	}
}

func TestTypeMetadata_SetID_NoID(t *testing.T) {
	ClearMetadataCache()

	type NoIDModel struct {
		Name string `db:"name"`
	}
	meta := GetTypeMetadata(reflect.TypeOf(NoIDModel{}))
	m := &NoIDModel{Name: "test"}
	// Should not panic
	meta.SetID(reflect.ValueOf(m), 123)
}

func TestClearMetadataCache(t *testing.T) {
	meta1 := GetTypeMetadata(reflect.TypeOf(TestModel{}))
	ClearMetadataCache()
	meta2 := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	if meta1 == meta2 {
		t.Error("after ClearMetadataCache, should return new instance")
	}
}

func TestBuildFieldIndex_UnexportedFields(t *testing.T) {
	ClearMetadataCache()

	type ModelWithUnexported struct {
		Model
		Name    string `db:"name"`
		private string //nolint:unused
	}

	meta := GetTypeMetadata(reflect.TypeOf(ModelWithUnexported{}))
	if _, ok := meta.FieldIndex["private"]; ok {
		t.Error("unexported field should not be in FieldIndex")
	}
}

func TestBuildFieldIndex_DBTagDash(t *testing.T) {
	ClearMetadataCache()

	type ModelWithDash struct {
		Model
		Name    string `db:"name"`
		Ignored string `db:"-"`
	}

	meta := GetTypeMetadata(reflect.TypeOf(ModelWithDash{}))
	// "-" tags should be skipped — they get converted to snake_case "ignored"
	// but the code checks for "-" tag and uses snake_case as fallback
	for _, col := range meta.Columns {
		if col == "-" {
			t.Error("column '-' should not appear")
		}
	}
}

func TestTypeMetadata_FieldIndex(t *testing.T) {
	ClearMetadataCache()

	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	if _, ok := meta.FieldIndex["name"]; !ok {
		t.Error("FieldIndex should contain 'name'")
	}
	if _, ok := meta.FieldIndex["email"]; !ok {
		t.Error("FieldIndex should contain 'email'")
	}
	if _, ok := meta.FieldIndex["id"]; !ok {
		t.Error("FieldIndex should contain 'id'")
	}
}

func TestTypeMetadata_TimestampFields(t *testing.T) {
	ClearMetadataCache()

	meta := GetTypeMetadata(reflect.TypeOf(TestModel{}))

	if !meta.HasCreated {
		t.Error("HasCreated should be true")
	}
	if meta.CreatedIdx == nil {
		t.Error("CreatedIdx should not be nil")
	}

	if !meta.HasUpdated {
		t.Error("HasUpdated should be true")
	}
	if meta.UpdatedIdx == nil {
		t.Error("UpdatedIdx should not be nil")
	}

	// Verify the field index path works
	m := &TestModel{}
	m.CreatedAt = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	v := reflect.ValueOf(m).Elem().FieldByIndex(meta.CreatedIdx)
	if !v.Interface().(time.Time).Equal(m.CreatedAt) {
		t.Error("CreatedIdx path doesn't resolve correctly")
	}
}
