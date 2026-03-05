package grpc

import (
	"strings"
	"testing"
	"time"
)

// ========================================
// Test Models
// ========================================

type User struct {
	ID        int64     `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	Email     string    `json:"email" db:"email"`
	Age       int32     `json:"age" db:"age"`
	Active    bool      `json:"active" db:"active"`
	Score     float64   `json:"score" db:"score"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

type Product struct {
	ID    int64   `json:"id" db:"id"`
	Name  string  `json:"name" db:"name"`
	Price float32 `json:"price" db:"price"`
}

type EmbeddedBase struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type ModelWithEmbedded struct {
	EmbeddedBase
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
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type ModelNoJsonTag struct {
	Name  string `db:"name"`
	Email string
}

type ModelWithUint struct {
	Val8  uint8  `json:"val8"`
	Val16 uint16 `json:"val16"`
	Val32 uint32 `json:"val32"`
	Val64 uint64 `json:"val64"`
}

type ModelWithIntSizes struct {
	Val8  int8  `json:"val8"`
	Val16 int16 `json:"val16"`
	Val64 int64 `json:"val64"`
}

type ModelWithFloat32 struct {
	Val float32 `json:"val"`
}

// Note: []byte is handled by the special case in goTypeToProto
// but only when it's not a struct field (it matches reflect.TypeOf([]byte{})).
// As a struct field, []byte is first detected as reflect.Slice of uint8.

type ModelWithPtrSlice struct {
	Items []*Product `json:"items"`
}

// ========================================
// Tests: NewProtoGenerator
// ========================================

func TestNewProtoGenerator(t *testing.T) {
	gen := NewProtoGenerator("myservice", "github.com/myorg/myservice/pb")
	if gen == nil {
		t.Fatal("NewProtoGenerator returned nil")
	}
	if gen.packageName != "myservice" {
		t.Errorf("packageName = %q, want %q", gen.packageName, "myservice")
	}
	if gen.goPackage != "github.com/myorg/myservice/pb" {
		t.Errorf("goPackage = %q", gen.goPackage)
	}
}

// ========================================
// Tests: RegisterMessage
// ========================================

func TestRegisterMessage_BasicStruct(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	err := gen.RegisterMessage(Product{})
	if err != nil {
		t.Fatalf("RegisterMessage error = %v", err)
	}

	msg, ok := gen.messages["Product"]
	if !ok {
		t.Fatal("Product message not registered")
	}
	if len(msg.Fields) != 3 {
		t.Errorf("expected 3 fields, got %d", len(msg.Fields))
	}

	// Check CRUD messages were generated
	for _, name := range []string{
		"GetProductRequest", "GetProductResponse",
		"ListProductsRequest", "ListProductsResponse",
		"CreateProductRequest", "CreateProductResponse",
		"UpdateProductRequest", "UpdateProductResponse",
		"DeleteProductRequest", "DeleteProductResponse",
	} {
		if _, ok := gen.messages[name]; !ok {
			t.Errorf("CRUD message %q not generated", name)
		}
	}

	// Check service was generated
	if _, ok := gen.services["ProductService"]; !ok {
		t.Error("ProductService not generated")
	}
}

func TestRegisterMessage_NotAStruct(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	err := gen.RegisterMessage("not a struct")
	if err == nil {
		t.Error("expected error for non-struct")
	}
}

func TestRegisterMessage_Pointer(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	err := gen.RegisterMessage(&Product{})
	if err != nil {
		t.Fatalf("RegisterMessage error = %v", err)
	}
	if _, ok := gen.messages["Product"]; !ok {
		t.Error("should register pointer to struct")
	}
}

func TestRegisterMessage_WithTimeField(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(User{})

	if !gen.imports["google/protobuf/timestamp.proto"] {
		t.Error("timestamp import should be registered")
	}
}

func TestRegisterMessage_WithEmbedded(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithEmbedded{})

	msg := gen.messages["ModelWithEmbedded"]
	if msg == nil {
		t.Fatal("message not registered")
	}

	hasID := false
	hasName := false
	hasExtra := false
	for _, f := range msg.Fields {
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

func TestRegisterMessage_SkipDBDash(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithSkippedField{})

	msg := gen.messages["ModelWithSkippedField"]
	for _, f := range msg.Fields {
		if f.Name == "Ignored" || f.Name == "-" {
			t.Error("field with db:\"-\" should be skipped")
		}
	}
}

func TestRegisterMessage_WithPointer(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithPointer{})

	msg := gen.messages["ModelWithPointer"]
	for _, f := range msg.Fields {
		if f.Name == "email" {
			if !f.Optional {
				t.Error("pointer field should be optional")
			}
			return
		}
	}
	t.Error("email field not found")
}

func TestRegisterMessage_WithSlice(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithSlice{})

	msg := gen.messages["ModelWithSlice"]
	for _, f := range msg.Fields {
		if f.Name == "tags" {
			if !f.Repeated {
				t.Error("tags should be repeated")
			}
			if f.Type != "string" {
				t.Errorf("tags type = %q, want string", f.Type)
			}
			return
		}
	}
	t.Error("tags field not found")
}

func TestRegisterMessage_NoJsonTag(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelNoJsonTag{})

	msg := gen.messages["ModelNoJsonTag"]
	hasName := false
	hasEmail := false
	for _, f := range msg.Fields {
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
		t.Error("field without tags should use snake_case name")
	}
}

// ========================================
// Tests: goTypeToProto
// ========================================

func TestGoTypeToProto_AllTypes(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(User{})

	msg := gen.messages["User"]
	typeMap := make(map[string]string)
	for _, f := range msg.Fields {
		typeMap[f.Name] = f.Type
	}

	if typeMap["id"] != "int64" {
		t.Errorf("id type = %q, want int64", typeMap["id"])
	}
	if typeMap["name"] != "string" {
		t.Errorf("name type = %q, want string", typeMap["name"])
	}
	if typeMap["age"] != "int32" {
		t.Errorf("age type = %q, want int32", typeMap["age"])
	}
	if typeMap["active"] != "bool" {
		t.Errorf("active type = %q, want bool", typeMap["active"])
	}
	if typeMap["score"] != "double" {
		t.Errorf("score type = %q, want double", typeMap["score"])
	}
	if typeMap["created_at"] != "google.protobuf.Timestamp" {
		t.Errorf("created_at type = %q, want google.protobuf.Timestamp", typeMap["created_at"])
	}
}

func TestGoTypeToProto_UintTypes(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithUint{})

	msg := gen.messages["ModelWithUint"]
	typeMap := make(map[string]string)
	for _, f := range msg.Fields {
		typeMap[f.Name] = f.Type
	}

	if typeMap["val8"] != "uint32" {
		t.Errorf("uint8 type = %q, want uint32", typeMap["val8"])
	}
	if typeMap["val16"] != "uint32" {
		t.Errorf("uint16 type = %q, want uint32", typeMap["val16"])
	}
	if typeMap["val32"] != "uint32" {
		t.Errorf("uint32 type = %q, want uint32", typeMap["val32"])
	}
	if typeMap["val64"] != "uint64" {
		t.Errorf("uint64 type = %q, want uint64", typeMap["val64"])
	}
}

func TestGoTypeToProto_IntSizes(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithIntSizes{})

	msg := gen.messages["ModelWithIntSizes"]
	typeMap := make(map[string]string)
	for _, f := range msg.Fields {
		typeMap[f.Name] = f.Type
	}

	if typeMap["val8"] != "int32" {
		t.Errorf("int8 type = %q, want int32", typeMap["val8"])
	}
	if typeMap["val16"] != "int32" {
		t.Errorf("int16 type = %q, want int32", typeMap["val16"])
	}
	if typeMap["val64"] != "int64" {
		t.Errorf("int64 type = %q, want int64", typeMap["val64"])
	}
}

func TestGoTypeToProto_Float32(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithFloat32{})

	msg := gen.messages["ModelWithFloat32"]
	if msg.Fields[0].Type != "float" {
		t.Errorf("float32 type = %q, want float", msg.Fields[0].Type)
	}
}

func TestGoTypeToProto_Bytes(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithBytes{})

	msg := gen.messages["ModelWithBytes"]
	for _, f := range msg.Fields {
		if f.Name == "data" {
			if f.Type != "bytes" {
				t.Errorf("[]byte type = %q, want bytes", f.Type)
			}
			return
		}
	}
	t.Error("data field not found")
}

func TestGoTypeToProto_PtrSlice(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(ModelWithPtrSlice{})

	msg := gen.messages["ModelWithPtrSlice"]
	for _, f := range msg.Fields {
		if f.Name == "items" {
			if !f.Repeated {
				t.Error("items should be repeated")
			}
			if f.Type != "Product" {
				t.Errorf("type = %q, want Product", f.Type)
			}
			return
		}
	}
	t.Error("items field not found")
}

func TestGoTypeToProto_StructType(t *testing.T) {
	type Inner struct {
		X string `json:"x"`
	}
	type Outer struct {
		Inner Inner `json:"inner"`
	}

	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(Outer{})

	msg := gen.messages["Outer"]
	for _, f := range msg.Fields {
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
// Tests: toSnakeCaseProto
// ========================================

func TestToSnakeCaseProto(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"UserName", "user_name"},
		{"userName", "user_name"},
		{"name", "name"},
		{"ID", "i_d"},
		{"", ""},
	}

	for _, tt := range tests {
		result := toSnakeCaseProto(tt.input)
		if result != tt.expected {
			t.Errorf("toSnakeCaseProto(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// ========================================
// Tests: GenerateProto
// ========================================

func TestGenerateProto_Basic(t *testing.T) {
	gen := NewProtoGenerator("myservice", "github.com/myorg/myservice/pb")
	gen.RegisterMessage(Product{})

	proto := gen.GenerateProto()

	if !strings.Contains(proto, "syntax = \"proto3\"") {
		t.Error("should contain proto3 syntax")
	}
	if !strings.Contains(proto, "package myservice") {
		t.Error("should contain package name")
	}
	if !strings.Contains(proto, "option go_package = \"github.com/myorg/myservice/pb\"") {
		t.Error("should contain go_package option")
	}
	if !strings.Contains(proto, "message Product") {
		t.Error("should contain Product message")
	}
	if !strings.Contains(proto, "service ProductService") {
		t.Error("should contain ProductService")
	}
}

func TestGenerateProto_EmptyGoPackage(t *testing.T) {
	gen := NewProtoGenerator("myservice", "")
	gen.RegisterMessage(Product{})

	proto := gen.GenerateProto()
	if strings.Contains(proto, "option go_package") {
		t.Error("should not contain go_package when empty")
	}
}

func TestGenerateProto_WithImports(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(User{})

	proto := gen.GenerateProto()
	if !strings.Contains(proto, "import \"google/protobuf/timestamp.proto\"") {
		t.Error("should contain timestamp import")
	}
	if !strings.Contains(proto, "import \"google/protobuf/field_mask.proto\"") {
		t.Error("should contain field_mask import")
	}
}

func TestGenerateProto_WithEnum(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.AddEnum("Status", "STATUS_UNSPECIFIED", "STATUS_ACTIVE", "STATUS_INACTIVE")
	gen.RegisterMessage(Product{})

	proto := gen.GenerateProto()
	if !strings.Contains(proto, "enum Status") {
		t.Error("should contain Status enum")
	}
	if !strings.Contains(proto, "STATUS_UNSPECIFIED = 0") {
		t.Error("should contain first enum value with number 0")
	}
	if !strings.Contains(proto, "STATUS_ACTIVE = 1") {
		t.Error("should contain second enum value with number 1")
	}
}

func TestGenerateProto_CRUDService(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(Product{})

	proto := gen.GenerateProto()

	expectedMethods := []string{
		"rpc GetProduct(GetProductRequest) returns (GetProductResponse)",
		"rpc ListProducts(ListProductsRequest) returns (ListProductsResponse)",
		"rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse)",
		"rpc UpdateProduct(UpdateProductRequest) returns (UpdateProductResponse)",
		"rpc DeleteProduct(DeleteProductRequest) returns (DeleteProductResponse)",
	}

	for _, method := range expectedMethods {
		if !strings.Contains(proto, method) {
			t.Errorf("should contain method: %s", method)
		}
	}
}

func TestGenerateProto_StreamingMethod(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(Product{})

	proto := gen.GenerateProto()
	if !strings.Contains(proto, "stream Product") {
		t.Error("should contain server streaming method")
	}
}

func TestGenerateProto_CRUDMessages(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(Product{})

	proto := gen.GenerateProto()

	// List request
	if !strings.Contains(proto, "message ListProductsRequest") {
		t.Error("should contain ListProductsRequest")
	}
	if !strings.Contains(proto, "int32 page_size") {
		t.Error("should contain page_size field")
	}
	if !strings.Contains(proto, "string page_token") {
		t.Error("should contain page_token field")
	}

	// Update request should have field_mask
	if !strings.Contains(proto, "google.protobuf.FieldMask update_mask") {
		t.Error("should contain update_mask field")
	}
}

// ========================================
// Tests: AddEnum
// ========================================

func TestAddEnum(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.AddEnum("Color", "RED", "GREEN", "BLUE")

	enum, ok := gen.enums["Color"]
	if !ok {
		t.Fatal("enum not registered")
	}
	if len(enum.Values) != 3 {
		t.Errorf("expected 3 values, got %d", len(enum.Values))
	}
	if enum.Values[0].Number != 0 {
		t.Errorf("first value number = %d, want 0", enum.Values[0].Number)
	}
	if enum.Values[2].Number != 2 {
		t.Errorf("third value number = %d, want 2", enum.Values[2].Number)
	}
}

// ========================================
// Tests: AddService
// ========================================

func TestAddService(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	methods := []ProtoMethodDefinition{
		{Name: "Ping", Input: "PingRequest", Output: "PingResponse"},
		{Name: "Stream", Input: "StreamRequest", Output: "StreamResponse", ServerStream: true},
		{Name: "Upload", Input: "UploadRequest", Output: "UploadResponse", ClientStream: true},
	}
	gen.AddService("HealthService", methods)

	svc, ok := gen.services["HealthService"]
	if !ok {
		t.Fatal("service not registered")
	}
	if len(svc.Methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(svc.Methods))
	}
}

func TestAddService_GenerateProto(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	methods := []ProtoMethodDefinition{
		{Name: "Ping", Input: "PingRequest", Output: "PingResponse", Comment: "Health check"},
		{Name: "Upload", Input: "UploadRequest", Output: "UploadResponse", ClientStream: true},
	}
	gen.AddService("HealthService", methods)

	proto := gen.GenerateProto()
	if !strings.Contains(proto, "// Health check") {
		t.Error("should contain method comment")
	}
	if !strings.Contains(proto, "rpc Upload(stream UploadRequest)") {
		t.Error("should contain client stream method")
	}
}

// ========================================
// Tests: generateMessageProto with nested
// ========================================

func TestGenerateMessageProto_WithNested(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	msg := &ProtoMessageDefinition{
		Name:    "Outer",
		Comment: "Outer message",
		Fields: []ProtoFieldDefinition{
			{Name: "id", Type: "int64", Number: 1},
		},
		Nested: []*ProtoMessageDefinition{
			{
				Name: "Inner",
				Fields: []ProtoFieldDefinition{
					{Name: "value", Type: "string", Number: 1},
				},
			},
		},
	}

	result := gen.generateMessageProto(msg)
	if !strings.Contains(result, "// Outer message") {
		t.Error("should contain message comment")
	}
	if !strings.Contains(result, "message Inner") {
		t.Error("should contain nested message")
	}
}

// ========================================
// Tests: generateFieldProto
// ========================================

func TestGenerateFieldProto_Repeated(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	field := ProtoFieldDefinition{
		Name:     "tags",
		Type:     "string",
		Number:   1,
		Repeated: true,
	}

	result := gen.generateFieldProto(field)
	if !strings.Contains(result, "repeated string tags = 1") {
		t.Errorf("expected repeated field, got: %s", result)
	}
}

func TestGenerateFieldProto_Optional(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	field := ProtoFieldDefinition{
		Name:     "email",
		Type:     "string",
		Number:   1,
		Optional: true,
	}

	result := gen.generateFieldProto(field)
	if !strings.Contains(result, "optional string email = 1") {
		t.Errorf("expected optional field, got: %s", result)
	}
}

func TestGenerateFieldProto_WithComment(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	field := ProtoFieldDefinition{
		Name:    "email",
		Type:    "string",
		Number:  1,
		Comment: "User email address",
	}

	result := gen.generateFieldProto(field)
	if !strings.Contains(result, "// User email address") {
		t.Errorf("should contain comment: %s", result)
	}
}

// ========================================
// Tests: generateServiceProto
// ========================================

func TestGenerateServiceProto_WithComment(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	svc := &ProtoServiceDefinition{
		Name:    "TestService",
		Comment: "A test service",
		Methods: []ProtoMethodDefinition{
			{Name: "Get", Input: "GetReq", Output: "GetResp"},
		},
	}

	result := gen.generateServiceProto(svc)
	if !strings.Contains(result, "// A test service") {
		t.Error("should contain service comment")
	}
}

// ========================================
// Tests: generateEnumProto
// ========================================

func TestGenerateEnumProto(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	enum := &ProtoEnumDefinition{
		Name: "Status",
		Values: []ProtoEnumValue{
			{Name: "UNKNOWN", Number: 0},
			{Name: "ACTIVE", Number: 1},
		},
	}

	result := gen.generateEnumProto(enum)
	if !strings.Contains(result, "enum Status") {
		t.Error("should contain enum name")
	}
	if !strings.Contains(result, "UNKNOWN = 0") {
		t.Error("should contain first value")
	}
	if !strings.Contains(result, "ACTIVE = 1") {
		t.Error("should contain second value")
	}
}

// ========================================
// Tests: Empty proto
// ========================================

func TestGenerateProto_Empty(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	proto := gen.GenerateProto()
	if !strings.Contains(proto, "syntax = \"proto3\"") {
		t.Error("should contain syntax even when empty")
	}
	if !strings.Contains(proto, "package test") {
		t.Error("should contain package even when empty")
	}
}

// ========================================
// Tests: Field number increment
// ========================================

func TestRegisterMessage_FieldNumbers(t *testing.T) {
	gen := NewProtoGenerator("test", "test/pb")
	gen.RegisterMessage(Product{})

	msg := gen.messages["Product"]
	for i, f := range msg.Fields {
		expected := i + 1
		if f.Number != expected {
			t.Errorf("field %q number = %d, want %d", f.Name, f.Number, expected)
		}
	}
}
