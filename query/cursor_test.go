package query

import (
	"testing"
)

func TestEncodeCursor(t *testing.T) {
	cursor := EncodeCursor("created_at", "2024-01-01", 42)
	if cursor == "" {
		t.Error("EncodeCursor should not return empty")
	}
}

func TestDecodeCursor(t *testing.T) {
	cursor := EncodeCursor("created_at", "2024-01-01", 42)
	data, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor error: %v", err)
	}
	if data.Column != "created_at" {
		t.Errorf("Column = %q, want 'created_at'", data.Column)
	}
	if data.ID != 42 {
		t.Errorf("ID = %d, want 42", data.ID)
	}
}

func TestDecodeCursor_Empty(t *testing.T) {
	data, err := DecodeCursor("")
	if err != nil {
		t.Fatalf("DecodeCursor empty error: %v", err)
	}
	if data != nil {
		t.Error("DecodeCursor empty should return nil")
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	_, err := DecodeCursor("not-valid-base64!!!")
	if err == nil {
		t.Error("DecodeCursor invalid should return error")
	}
}

func TestDecodeCursor_InvalidJSON(t *testing.T) {
	// Valid base64 but invalid JSON
	_, err := DecodeCursor(Cursor("bm90LWpzb24="))
	if err == nil {
		t.Error("DecodeCursor invalid JSON should return error")
	}
}

func TestCursor_IsValid(t *testing.T) {
	// Empty is valid (first page)
	if !Cursor("").IsValid() {
		t.Error("empty cursor should be valid")
	}

	// Valid encoded cursor
	cursor := EncodeCursor("id", 1, 1)
	if !cursor.IsValid() {
		t.Error("encoded cursor should be valid")
	}

	// Invalid cursor
	if Cursor("!!!invalid!!!").IsValid() {
		t.Error("invalid cursor should not be valid")
	}
}

func TestReverseSlice(t *testing.T) {
	// int slice
	ints := []int{1, 2, 3, 4, 5}
	reverseSlice(ints)
	expected := []int{5, 4, 3, 2, 1}
	for i, v := range ints {
		if v != expected[i] {
			t.Errorf("reverseSlice[%d] = %d, want %d", i, v, expected[i])
		}
	}

	// Empty slice
	empty := []int{}
	reverseSlice(empty) // Should not panic

	// Single element
	single := []int{42}
	reverseSlice(single)
	if single[0] != 42 {
		t.Error("reverseSlice single element should remain unchanged")
	}
}

func TestCursorPage_ToConnection(t *testing.T) {
	type Item struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	page := &CursorPage[Item]{
		Items:           []Item{{ID: 1, Name: "A"}, {ID: 2, Name: "B"}},
		HasNextPage:     true,
		HasPreviousPage: false,
		StartCursor:     EncodeCursor("id", 1, 1),
		EndCursor:       EncodeCursor("id", 2, 2),
	}

	conn := page.ToConnection(func(item Item) Cursor {
		return EncodeCursor("id", item.ID, item.ID)
	})

	if len(conn.Edges) != 2 {
		t.Errorf("Edges len = %d, want 2", len(conn.Edges))
	}
	if !conn.PageInfo.HasNextPage {
		t.Error("HasNextPage should be true")
	}
	if conn.PageInfo.HasPreviousPage {
		t.Error("HasPreviousPage should be false")
	}
	if conn.Edges[0].Node.Name != "A" {
		t.Errorf("first edge node = %q, want 'A'", conn.Edges[0].Node.Name)
	}
}

func TestCursorPage_ToConnection_WithTotalCount(t *testing.T) {
	type Item struct{ ID int64 }
	count := int64(100)
	page := &CursorPage[Item]{
		Items:      []Item{{ID: 1}},
		TotalCount: &count,
	}

	conn := page.ToConnection(func(item Item) Cursor {
		return EncodeCursor("id", item.ID, item.ID)
	})

	if conn.TotalCount == nil || *conn.TotalCount != 100 {
		t.Error("TotalCount should be 100")
	}
}

func TestBuilder_Paginate(t *testing.T) {
	b := newTestBuilder()
	cb := b.Paginate(CursorConfig{
		OrderBy:   "created_at",
		OrderDesc: true,
		First:     10,
	})

	if cb == nil {
		t.Fatal("Paginate should not return nil")
	}
	if cb.config.OrderBy != "created_at" {
		t.Errorf("config.OrderBy = %q, want 'created_at'", cb.config.OrderBy)
	}
}

func TestBuilder_BuildCursorSQL(t *testing.T) {
	b := newTestBuilder()
	d := &mockDialect{}

	// With After cursor
	afterCursor := EncodeCursor("created_at", "2024-01-01", 1)
	sql, args, err := b.buildCursorSQL(CursorConfig{
		OrderBy: "created_at",
		After:   afterCursor,
	}, d)
	if err != nil {
		t.Fatalf("buildCursorSQL error: %v", err)
	}
	if sql == "" {
		t.Error("sql should not be empty with After cursor")
	}
	if len(args) != 1 {
		t.Errorf("args len = %d, want 1", len(args))
	}

	// With Before cursor (desc order)
	beforeCursor := EncodeCursor("created_at", "2024-12-31", 100)
	sql2, args2, err := b.buildCursorSQL(CursorConfig{
		OrderBy:   "created_at",
		Before:    beforeCursor,
		OrderDesc: true,
	}, d)
	if err != nil {
		t.Fatalf("buildCursorSQL before error: %v", err)
	}
	if sql2 == "" {
		t.Error("sql should not be empty with Before cursor")
	}
	if len(args2) != 1 {
		t.Errorf("args2 len = %d, want 1", len(args2))
	}
}

func TestBuilder_BuildCursorSQL_NoCursor(t *testing.T) {
	b := newTestBuilder()
	d := &mockDialect{}

	sql, args, err := b.buildCursorSQL(CursorConfig{OrderBy: "id"}, d)
	if err != nil {
		t.Fatalf("buildCursorSQL error: %v", err)
	}
	if sql != "" {
		t.Errorf("sql should be empty without cursors, got %q", sql)
	}
	if len(args) != 0 {
		t.Errorf("args should be empty, got %v", args)
	}
}

func TestCursorBuilder_GenerateCursor(t *testing.T) {
	type userModel struct {
		ID   int64  `db:"id"`
		Name string `db:"name"`
	}

	b := NewBuilder[userModel](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"users",
	)
	cb := b.Paginate(CursorConfig{OrderBy: "name"})

	cursor := cb.generateCursor(userModel{ID: 42, Name: "John"})
	if cursor == "" {
		t.Error("generateCursor should produce a cursor")
	}

	data, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("DecodeCursor error: %v", err)
	}
	if data.Column != "name" {
		t.Errorf("Column = %q, want 'name'", data.Column)
	}
	if data.ID != 42 {
		t.Errorf("ID = %d, want 42", data.ID)
	}
}

func TestCursorBuilder_GenerateCursor_EmbeddedStruct(t *testing.T) {
	type Base struct {
		ID int64 `db:"id"`
	}
	type Model struct {
		Base
		CreatedAt string `db:"created_at"`
	}

	b := NewBuilder[Model](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"models",
	)
	cb := b.Paginate(CursorConfig{OrderBy: "created_at"})

	cursor := cb.generateCursor(Model{Base: Base{ID: 10}, CreatedAt: "2024-01-01"})
	data, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if data.ID != 10 {
		t.Errorf("ID = %d, want 10", data.ID)
	}
}

func TestCursorBuilder_FindFieldValue(t *testing.T) {
	type Base struct {
		ID int64 `db:"id"`
	}
	type Model struct {
		Base
		Score int `db:"score"`
	}

	b := NewBuilder[Model](
		&mockExecutor{},
		&mockDialect{},
		newMockLogger(),
		"models",
	)
	cb := b.Paginate(CursorConfig{OrderBy: "id"})

	// Generate cursor which exercises findFieldValue for embedded fields
	cursor := cb.generateCursor(Model{Base: Base{ID: 99}, Score: 50})
	data, err := DecodeCursor(cursor)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if data.ID != 99 {
		t.Errorf("ID = %d, want 99", data.ID)
	}
}
