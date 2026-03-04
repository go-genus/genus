package grpc

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// ProtoType representa um tipo Protobuf.
type ProtoType string

const (
	ProtoString   ProtoType = "string"
	ProtoInt32    ProtoType = "int32"
	ProtoInt64    ProtoType = "int64"
	ProtoUint32   ProtoType = "uint32"
	ProtoUint64   ProtoType = "uint64"
	ProtoFloat    ProtoType = "float"
	ProtoDouble   ProtoType = "double"
	ProtoBool     ProtoType = "bool"
	ProtoBytes    ProtoType = "bytes"
	ProtoTimestamp ProtoType = "google.protobuf.Timestamp"
)

// ProtoFieldDefinition define um campo Protobuf.
type ProtoFieldDefinition struct {
	Name     string
	Type     string
	Number   int
	Repeated bool
	Optional bool
	Comment  string
}

// ProtoMessageDefinition define uma mensagem Protobuf.
type ProtoMessageDefinition struct {
	Name     string
	Fields   []ProtoFieldDefinition
	Comment  string
	Nested   []*ProtoMessageDefinition
}

// ProtoServiceDefinition define um serviço gRPC.
type ProtoServiceDefinition struct {
	Name    string
	Methods []ProtoMethodDefinition
	Comment string
}

// ProtoMethodDefinition define um método RPC.
type ProtoMethodDefinition struct {
	Name       string
	Input      string
	Output     string
	Comment    string
	Streaming  bool
	ClientStream bool
	ServerStream bool
}

// ProtoEnumDefinition define um enum Protobuf.
type ProtoEnumDefinition struct {
	Name   string
	Values []ProtoEnumValue
}

// ProtoEnumValue define um valor de enum.
type ProtoEnumValue struct {
	Name   string
	Number int
}

// ProtoGenerator gera arquivos .proto a partir de structs Go.
type ProtoGenerator struct {
	packageName string
	goPackage   string
	messages    map[string]*ProtoMessageDefinition
	services    map[string]*ProtoServiceDefinition
	enums       map[string]*ProtoEnumDefinition
	imports     map[string]bool
}

// NewProtoGenerator cria um novo gerador de proto.
func NewProtoGenerator(packageName, goPackage string) *ProtoGenerator {
	return &ProtoGenerator{
		packageName: packageName,
		goPackage:   goPackage,
		messages:    make(map[string]*ProtoMessageDefinition),
		services:    make(map[string]*ProtoServiceDefinition),
		enums:       make(map[string]*ProtoEnumDefinition),
		imports:     make(map[string]bool),
	}
}

// RegisterMessage registra um tipo Go como mensagem Protobuf.
func (g *ProtoGenerator) RegisterMessage(model interface{}) error {
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("model must be a struct, got %s", typ.Kind())
	}

	msg := &ProtoMessageDefinition{
		Name:   typ.Name(),
		Fields: make([]ProtoFieldDefinition, 0),
	}

	fieldNumber := 1

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Pula campos não exportados
		if !field.IsExported() {
			continue
		}

		// Processa campos embedded
		if field.Anonymous {
			if field.Type.Kind() == reflect.Struct {
				for j := 0; j < field.Type.NumField(); j++ {
					embeddedField := field.Type.Field(j)
					if !embeddedField.IsExported() {
						continue
					}
					if fieldDef := g.parseField(embeddedField, fieldNumber); fieldDef != nil {
						msg.Fields = append(msg.Fields, *fieldDef)
						fieldNumber++
					}
				}
			}
			continue
		}

		// Pula campos com db:"-"
		if tag := field.Tag.Get("db"); tag == "-" {
			continue
		}

		if fieldDef := g.parseField(field, fieldNumber); fieldDef != nil {
			msg.Fields = append(msg.Fields, *fieldDef)
			fieldNumber++
		}
	}

	g.messages[msg.Name] = msg

	// Gera mensagens de request/response
	g.generateCRUDMessages(typ.Name())

	// Gera serviço
	g.generateCRUDService(typ.Name())

	return nil
}

// parseField converte um campo Go para definição Protobuf.
func (g *ProtoGenerator) parseField(field reflect.StructField, number int) *ProtoFieldDefinition {
	// Obtém nome do campo
	name := field.Tag.Get("json")
	if name == "" {
		name = field.Tag.Get("db")
	}
	if name == "" || name == "-" {
		name = toSnakeCaseProto(field.Name)
	}
	// Remove opções do json tag
	if idx := strings.Index(name, ","); idx != -1 {
		name = name[:idx]
	}

	def := &ProtoFieldDefinition{
		Name:   name,
		Number: number,
	}

	// Converte tipo Go para Protobuf
	def.Type, def.Repeated, def.Optional = g.goTypeToProto(field.Type)

	return def
}

// goTypeToProto converte um tipo Go para Protobuf.
func (g *ProtoGenerator) goTypeToProto(typ reflect.Type) (string, bool, bool) {
	// Ponteiros são optional
	optional := false
	if typ.Kind() == reflect.Ptr {
		optional = true
		typ = typ.Elem()
	}

	// Slices são repeated
	repeated := false
	if typ.Kind() == reflect.Slice {
		repeated = true
		typ = typ.Elem()
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
	}

	// Tipos básicos
	switch typ.Kind() {
	case reflect.String:
		return "string", repeated, optional
	case reflect.Int, reflect.Int32:
		return "int32", repeated, optional
	case reflect.Int64:
		return "int64", repeated, optional
	case reflect.Int8, reflect.Int16:
		return "int32", repeated, optional
	case reflect.Uint, reflect.Uint32:
		return "uint32", repeated, optional
	case reflect.Uint64:
		return "uint64", repeated, optional
	case reflect.Uint8, reflect.Uint16:
		return "uint32", repeated, optional
	case reflect.Float32:
		return "float", repeated, optional
	case reflect.Float64:
		return "double", repeated, optional
	case reflect.Bool:
		return "bool", repeated, optional
	}

	// Tipos especiais
	switch typ {
	case reflect.TypeOf(time.Time{}):
		g.imports["google/protobuf/timestamp.proto"] = true
		return "google.protobuf.Timestamp", repeated, optional
	case reflect.TypeOf([]byte{}):
		return "bytes", false, optional
	}

	// Tipos customizados - usa o nome do tipo
	if typ.Kind() == reflect.Struct {
		return typ.Name(), repeated, optional
	}

	return "string", repeated, optional
}

// generateCRUDMessages gera mensagens de request/response para CRUD.
func (g *ProtoGenerator) generateCRUDMessages(typeName string) {
	// Get request
	g.messages[fmt.Sprintf("Get%sRequest", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Get%sRequest", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: "id", Type: "int64", Number: 1},
		},
	}

	// Get response
	g.messages[fmt.Sprintf("Get%sResponse", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Get%sResponse", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: toSnakeCaseProto(typeName), Type: typeName, Number: 1},
		},
	}

	// List request
	g.messages[fmt.Sprintf("List%ssRequest", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("List%ssRequest", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: "page_size", Type: "int32", Number: 1},
			{Name: "page_token", Type: "string", Number: 2},
			{Name: "filter", Type: "string", Number: 3, Optional: true},
			{Name: "order_by", Type: "string", Number: 4, Optional: true},
		},
	}

	// List response
	g.messages[fmt.Sprintf("List%ssResponse", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("List%ssResponse", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: toSnakeCaseProto(typeName) + "s", Type: typeName, Number: 1, Repeated: true},
			{Name: "next_page_token", Type: "string", Number: 2},
			{Name: "total_count", Type: "int32", Number: 3},
		},
	}

	// Create request
	g.messages[fmt.Sprintf("Create%sRequest", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Create%sRequest", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: toSnakeCaseProto(typeName), Type: typeName, Number: 1},
		},
	}

	// Create response
	g.messages[fmt.Sprintf("Create%sResponse", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Create%sResponse", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: toSnakeCaseProto(typeName), Type: typeName, Number: 1},
		},
	}

	// Update request
	g.messages[fmt.Sprintf("Update%sRequest", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Update%sRequest", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: toSnakeCaseProto(typeName), Type: typeName, Number: 1},
			{Name: "update_mask", Type: "google.protobuf.FieldMask", Number: 2, Optional: true},
		},
	}
	g.imports["google/protobuf/field_mask.proto"] = true

	// Update response
	g.messages[fmt.Sprintf("Update%sResponse", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Update%sResponse", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: toSnakeCaseProto(typeName), Type: typeName, Number: 1},
		},
	}

	// Delete request
	g.messages[fmt.Sprintf("Delete%sRequest", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Delete%sRequest", typeName),
		Fields: []ProtoFieldDefinition{
			{Name: "id", Type: "int64", Number: 1},
		},
	}

	// Delete response
	g.messages[fmt.Sprintf("Delete%sResponse", typeName)] = &ProtoMessageDefinition{
		Name: fmt.Sprintf("Delete%sResponse", typeName),
		Fields: []ProtoFieldDefinition{},
	}
}

// generateCRUDService gera serviço gRPC para CRUD.
func (g *ProtoGenerator) generateCRUDService(typeName string) {
	serviceName := typeName + "Service"

	g.services[serviceName] = &ProtoServiceDefinition{
		Name: serviceName,
		Methods: []ProtoMethodDefinition{
			{
				Name:   fmt.Sprintf("Get%s", typeName),
				Input:  fmt.Sprintf("Get%sRequest", typeName),
				Output: fmt.Sprintf("Get%sResponse", typeName),
			},
			{
				Name:   fmt.Sprintf("List%ss", typeName),
				Input:  fmt.Sprintf("List%ssRequest", typeName),
				Output: fmt.Sprintf("List%ssResponse", typeName),
			},
			{
				Name:   fmt.Sprintf("Create%s", typeName),
				Input:  fmt.Sprintf("Create%sRequest", typeName),
				Output: fmt.Sprintf("Create%sResponse", typeName),
			},
			{
				Name:   fmt.Sprintf("Update%s", typeName),
				Input:  fmt.Sprintf("Update%sRequest", typeName),
				Output: fmt.Sprintf("Update%sResponse", typeName),
			},
			{
				Name:   fmt.Sprintf("Delete%s", typeName),
				Input:  fmt.Sprintf("Delete%sRequest", typeName),
				Output: fmt.Sprintf("Delete%sResponse", typeName),
			},
			{
				Name:         fmt.Sprintf("Watch%ss", typeName),
				Input:        fmt.Sprintf("List%ssRequest", typeName),
				Output:       typeName,
				ServerStream: true,
				Comment:      "Stream updates for real-time sync",
			},
		},
	}
}

// GenerateProto gera o conteúdo do arquivo .proto.
func (g *ProtoGenerator) GenerateProto() string {
	var sb strings.Builder

	// Header
	sb.WriteString("syntax = \"proto3\";\n\n")
	sb.WriteString(fmt.Sprintf("package %s;\n\n", g.packageName))

	// Go package option
	if g.goPackage != "" {
		sb.WriteString(fmt.Sprintf("option go_package = \"%s\";\n\n", g.goPackage))
	}

	// Imports
	for imp := range g.imports {
		sb.WriteString(fmt.Sprintf("import \"%s\";\n", imp))
	}
	if len(g.imports) > 0 {
		sb.WriteString("\n")
	}

	// Enums
	for _, enum := range g.enums {
		sb.WriteString(g.generateEnumProto(enum))
		sb.WriteString("\n")
	}

	// Messages
	for _, msg := range g.messages {
		sb.WriteString(g.generateMessageProto(msg))
		sb.WriteString("\n")
	}

	// Services
	for _, svc := range g.services {
		sb.WriteString(g.generateServiceProto(svc))
		sb.WriteString("\n")
	}

	return sb.String()
}

// generateEnumProto gera definição de enum.
func (g *ProtoGenerator) generateEnumProto(enum *ProtoEnumDefinition) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("enum %s {\n", enum.Name))
	for _, val := range enum.Values {
		sb.WriteString(fmt.Sprintf("  %s = %d;\n", val.Name, val.Number))
	}
	sb.WriteString("}\n")

	return sb.String()
}

// generateMessageProto gera definição de mensagem.
func (g *ProtoGenerator) generateMessageProto(msg *ProtoMessageDefinition) string {
	var sb strings.Builder

	if msg.Comment != "" {
		sb.WriteString(fmt.Sprintf("// %s\n", msg.Comment))
	}

	sb.WriteString(fmt.Sprintf("message %s {\n", msg.Name))

	for _, field := range msg.Fields {
		sb.WriteString(g.generateFieldProto(field))
	}

	// Nested messages
	for _, nested := range msg.Nested {
		nestedStr := g.generateMessageProto(nested)
		// Indent nested message
		lines := strings.Split(nestedStr, "\n")
		for _, line := range lines {
			if line != "" {
				sb.WriteString("  " + line + "\n")
			}
		}
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateFieldProto gera definição de campo.
func (g *ProtoGenerator) generateFieldProto(field ProtoFieldDefinition) string {
	var sb strings.Builder

	if field.Comment != "" {
		sb.WriteString(fmt.Sprintf("  // %s\n", field.Comment))
	}

	sb.WriteString("  ")

	if field.Repeated {
		sb.WriteString("repeated ")
	} else if field.Optional {
		sb.WriteString("optional ")
	}

	sb.WriteString(fmt.Sprintf("%s %s = %d;\n", field.Type, field.Name, field.Number))

	return sb.String()
}

// generateServiceProto gera definição de serviço.
func (g *ProtoGenerator) generateServiceProto(svc *ProtoServiceDefinition) string {
	var sb strings.Builder

	if svc.Comment != "" {
		sb.WriteString(fmt.Sprintf("// %s\n", svc.Comment))
	}

	sb.WriteString(fmt.Sprintf("service %s {\n", svc.Name))

	for _, method := range svc.Methods {
		if method.Comment != "" {
			sb.WriteString(fmt.Sprintf("  // %s\n", method.Comment))
		}

		input := method.Input
		output := method.Output

		if method.ClientStream {
			input = "stream " + input
		}
		if method.ServerStream {
			output = "stream " + output
		}

		sb.WriteString(fmt.Sprintf("  rpc %s(%s) returns (%s);\n",
			method.Name, input, output))
	}

	sb.WriteString("}\n")

	return sb.String()
}

// toSnakeCaseProto converte para snake_case.
func toSnakeCaseProto(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// AddEnum registra um enum.
func (g *ProtoGenerator) AddEnum(name string, values ...string) {
	enum := &ProtoEnumDefinition{
		Name:   name,
		Values: make([]ProtoEnumValue, len(values)),
	}

	for i, val := range values {
		enum.Values[i] = ProtoEnumValue{Name: val, Number: i}
	}

	g.enums[name] = enum
}

// AddService adiciona um serviço customizado.
func (g *ProtoGenerator) AddService(name string, methods []ProtoMethodDefinition) {
	g.services[name] = &ProtoServiceDefinition{
		Name:    name,
		Methods: methods,
	}
}
