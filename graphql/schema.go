package graphql

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// GraphQLType representa um tipo GraphQL.
type GraphQLType string

const (
	TypeString   GraphQLType = "String"
	TypeInt      GraphQLType = "Int"
	TypeFloat    GraphQLType = "Float"
	TypeBoolean  GraphQLType = "Boolean"
	TypeID       GraphQLType = "ID"
	TypeDateTime GraphQLType = "DateTime"
)

// FieldDefinition define um campo GraphQL.
type FieldDefinition struct {
	Name        string
	Type        string
	NonNull     bool
	IsList      bool
	Description string
	Deprecated  string
	Arguments   []ArgumentDefinition
}

// ArgumentDefinition define um argumento de campo.
type ArgumentDefinition struct {
	Name         string
	Type         string
	NonNull      bool
	DefaultValue string
	Description  string
}

// TypeDefinition define um tipo GraphQL.
type TypeDefinition struct {
	Name        string
	Description string
	Fields      []FieldDefinition
	Implements  []string
}

// InputDefinition define um input type GraphQL.
type InputDefinition struct {
	Name        string
	Description string
	Fields      []FieldDefinition
}

// EnumDefinition define um enum GraphQL.
type EnumDefinition struct {
	Name        string
	Description string
	Values      []EnumValue
}

// EnumValue define um valor de enum.
type EnumValue struct {
	Name        string
	Description string
	Deprecated  string
}

// SchemaGenerator gera schemas GraphQL a partir de structs Go.
type SchemaGenerator struct {
	types       map[string]*TypeDefinition
	inputs      map[string]*InputDefinition
	enums       map[string]*EnumDefinition
	scalars     map[string]bool
	connections map[string]bool
}

// NewSchemaGenerator cria um novo gerador de schema.
func NewSchemaGenerator() *SchemaGenerator {
	return &SchemaGenerator{
		types:       make(map[string]*TypeDefinition),
		inputs:      make(map[string]*InputDefinition),
		enums:       make(map[string]*EnumDefinition),
		scalars:     make(map[string]bool),
		connections: make(map[string]bool),
	}
}

// RegisterType registra um tipo Go para geração de schema.
func (g *SchemaGenerator) RegisterType(model interface{}) error {
	typ := reflect.TypeOf(model)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	if typ.Kind() != reflect.Struct {
		return fmt.Errorf("model must be a struct, got %s", typ.Kind())
	}

	typeDef := &TypeDefinition{
		Name:   typ.Name(),
		Fields: make([]FieldDefinition, 0),
	}

	inputDef := &InputDefinition{
		Name:   typ.Name() + "Input",
		Fields: make([]FieldDefinition, 0),
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		// Pula campos não exportados
		if !field.IsExported() {
			continue
		}

		// Pula campos embedded Model
		if field.Anonymous {
			// Processa campos do Model embedded
			if field.Type.Kind() == reflect.Struct {
				for j := 0; j < field.Type.NumField(); j++ {
					embeddedField := field.Type.Field(j)
					if !embeddedField.IsExported() {
						continue
					}
					if fieldDef := g.parseField(embeddedField); fieldDef != nil {
						typeDef.Fields = append(typeDef.Fields, *fieldDef)
						// Não inclui ID e timestamps no input
						if fieldDef.Name != "id" && fieldDef.Name != "createdAt" && fieldDef.Name != "updatedAt" {
							inputDef.Fields = append(inputDef.Fields, *fieldDef)
						}
					}
				}
			}
			continue
		}

		// Pula campos com db:"-"
		if tag := field.Tag.Get("db"); tag == "-" {
			continue
		}

		if fieldDef := g.parseField(field); fieldDef != nil {
			typeDef.Fields = append(typeDef.Fields, *fieldDef)

			// Não inclui campos de relacionamento no input
			if !strings.HasSuffix(fieldDef.Type, "Connection") && !fieldDef.IsList {
				inputDef.Fields = append(inputDef.Fields, *fieldDef)
			}
		}
	}

	g.types[typeDef.Name] = typeDef
	g.inputs[inputDef.Name] = inputDef

	return nil
}

// parseField converte um campo Go para definição GraphQL.
func (g *SchemaGenerator) parseField(field reflect.StructField) *FieldDefinition {
	// Obtém nome do campo
	name := field.Tag.Get("json")
	if name == "" {
		name = field.Tag.Get("db")
	}
	if name == "" || name == "-" {
		name = toGraphQLName(field.Name)
	}
	// Remove opções do json tag
	if idx := strings.Index(name, ","); idx != -1 {
		name = name[:idx]
	}

	def := &FieldDefinition{
		Name: name,
	}

	// Verifica descrição
	if desc := field.Tag.Get("description"); desc != "" {
		def.Description = desc
	}

	// Verifica deprecated
	if dep := field.Tag.Get("deprecated"); dep != "" {
		def.Deprecated = dep
	}

	// Converte tipo Go para GraphQL
	def.Type, def.NonNull, def.IsList = g.goTypeToGraphQL(field.Type)

	return def
}

// goTypeToGraphQL converte um tipo Go para GraphQL.
func (g *SchemaGenerator) goTypeToGraphQL(typ reflect.Type) (string, bool, bool) {
	// Ponteiros são nullable
	nonNull := true
	if typ.Kind() == reflect.Ptr {
		nonNull = false
		typ = typ.Elem()
	}

	// Slices são listas
	isList := false
	if typ.Kind() == reflect.Slice {
		isList = true
		typ = typ.Elem()
		if typ.Kind() == reflect.Ptr {
			typ = typ.Elem()
		}
	}

	// Tipos básicos
	switch typ.Kind() {
	case reflect.String:
		return "String", nonNull, isList
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// ID para campos que terminam com ID ou são chamados id
		return "Int", nonNull, isList
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "Int", nonNull, isList
	case reflect.Float32, reflect.Float64:
		return "Float", nonNull, isList
	case reflect.Bool:
		return "Boolean", nonNull, isList
	}

	// Tipos especiais
	switch typ {
	case reflect.TypeOf(time.Time{}):
		g.scalars["DateTime"] = true
		return "DateTime", nonNull, isList
	}

	// Tipos customizados - usa o nome do tipo
	if typ.Kind() == reflect.Struct {
		return typ.Name(), nonNull, isList
	}

	return "String", nonNull, isList
}

// toGraphQLName converte nome Go para convenção GraphQL (camelCase).
func toGraphQLName(name string) string {
	if len(name) == 0 {
		return name
	}
	// Primeira letra minúscula
	return strings.ToLower(name[:1]) + name[1:]
}

// ToPascalCase converte para PascalCase.
func ToPascalCase(name string) string {
	if len(name) == 0 {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// GenerateSchema gera o schema GraphQL completo.
func (g *SchemaGenerator) GenerateSchema() string {
	var sb strings.Builder

	// Scalars customizados
	if g.scalars["DateTime"] {
		sb.WriteString("scalar DateTime\n\n")
	}

	// Enums
	for _, enum := range g.enums {
		sb.WriteString(g.generateEnum(enum))
		sb.WriteString("\n")
	}

	// Types
	for _, typ := range g.types {
		sb.WriteString(g.generateType(typ))
		sb.WriteString("\n")
	}

	// Inputs
	for _, input := range g.inputs {
		if len(input.Fields) > 0 {
			sb.WriteString(g.generateInput(input))
			sb.WriteString("\n")
		}
	}

	// Connections (Relay)
	for typeName := range g.connections {
		sb.WriteString(g.generateConnection(typeName))
		sb.WriteString("\n")
	}

	// Query type
	sb.WriteString(g.generateQueryType())
	sb.WriteString("\n")

	// Mutation type
	sb.WriteString(g.generateMutationType())

	return sb.String()
}

// generateEnum gera definição de enum.
func (g *SchemaGenerator) generateEnum(enum *EnumDefinition) string {
	var sb strings.Builder

	if enum.Description != "" {
		sb.WriteString(fmt.Sprintf("\"\"\"%s\"\"\"\n", enum.Description))
	}

	sb.WriteString(fmt.Sprintf("enum %s {\n", enum.Name))
	for _, val := range enum.Values {
		if val.Description != "" {
			sb.WriteString(fmt.Sprintf("  \"\"\"%s\"\"\"\n", val.Description))
		}
		sb.WriteString(fmt.Sprintf("  %s", val.Name))
		if val.Deprecated != "" {
			sb.WriteString(fmt.Sprintf(" @deprecated(reason: \"%s\")", val.Deprecated))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("}\n")

	return sb.String()
}

// generateType gera definição de tipo.
func (g *SchemaGenerator) generateType(typ *TypeDefinition) string {
	var sb strings.Builder

	if typ.Description != "" {
		sb.WriteString(fmt.Sprintf("\"\"\"%s\"\"\"\n", typ.Description))
	}

	sb.WriteString(fmt.Sprintf("type %s", typ.Name))
	if len(typ.Implements) > 0 {
		sb.WriteString(" implements ")
		sb.WriteString(strings.Join(typ.Implements, " & "))
	}
	sb.WriteString(" {\n")

	for _, field := range typ.Fields {
		sb.WriteString(g.generateField(field))
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateField gera definição de campo.
func (g *SchemaGenerator) generateField(field FieldDefinition) string {
	var sb strings.Builder

	if field.Description != "" {
		sb.WriteString(fmt.Sprintf("  \"\"\"%s\"\"\"\n", field.Description))
	}

	sb.WriteString(fmt.Sprintf("  %s", field.Name))

	// Arguments
	if len(field.Arguments) > 0 {
		sb.WriteString("(")
		args := make([]string, len(field.Arguments))
		for i, arg := range field.Arguments {
			argStr := fmt.Sprintf("%s: %s", arg.Name, arg.Type)
			if arg.NonNull {
				argStr += "!"
			}
			if arg.DefaultValue != "" {
				argStr += fmt.Sprintf(" = %s", arg.DefaultValue)
			}
			args[i] = argStr
		}
		sb.WriteString(strings.Join(args, ", "))
		sb.WriteString(")")
	}

	sb.WriteString(": ")

	// Type
	if field.IsList {
		sb.WriteString("[")
	}
	sb.WriteString(field.Type)
	if field.NonNull && !field.IsList {
		sb.WriteString("!")
	}
	if field.IsList {
		sb.WriteString("]")
		if field.NonNull {
			sb.WriteString("!")
		}
	}

	if field.Deprecated != "" {
		sb.WriteString(fmt.Sprintf(" @deprecated(reason: \"%s\")", field.Deprecated))
	}

	sb.WriteString("\n")

	return sb.String()
}

// generateInput gera definição de input.
func (g *SchemaGenerator) generateInput(input *InputDefinition) string {
	var sb strings.Builder

	if input.Description != "" {
		sb.WriteString(fmt.Sprintf("\"\"\"%s\"\"\"\n", input.Description))
	}

	sb.WriteString(fmt.Sprintf("input %s {\n", input.Name))

	for _, field := range input.Fields {
		sb.WriteString(g.generateField(field))
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateConnection gera tipo Connection (Relay).
func (g *SchemaGenerator) generateConnection(typeName string) string {
	var sb strings.Builder

	// Edge
	sb.WriteString(fmt.Sprintf("type %sEdge {\n", typeName))
	sb.WriteString(fmt.Sprintf("  node: %s!\n", typeName))
	sb.WriteString("  cursor: String!\n")
	sb.WriteString("}\n\n")

	// Connection
	sb.WriteString(fmt.Sprintf("type %sConnection {\n", typeName))
	sb.WriteString(fmt.Sprintf("  edges: [%sEdge!]!\n", typeName))
	sb.WriteString("  pageInfo: PageInfo!\n")
	sb.WriteString("  totalCount: Int\n")
	sb.WriteString("}\n")

	return sb.String()
}

// generateQueryType gera o tipo Query.
func (g *SchemaGenerator) generateQueryType() string {
	var sb strings.Builder

	// PageInfo para paginação
	sb.WriteString("type PageInfo {\n")
	sb.WriteString("  hasNextPage: Boolean!\n")
	sb.WriteString("  hasPreviousPage: Boolean!\n")
	sb.WriteString("  startCursor: String\n")
	sb.WriteString("  endCursor: String\n")
	sb.WriteString("}\n\n")

	sb.WriteString("type Query {\n")

	for typeName := range g.types {
		lowerName := toGraphQLName(typeName)

		// Single item query
		sb.WriteString(fmt.Sprintf("  %s(id: ID!): %s\n", lowerName, typeName))

		// List query with pagination
		sb.WriteString(fmt.Sprintf("  %ss(\n", lowerName))
		sb.WriteString("    first: Int\n")
		sb.WriteString("    after: String\n")
		sb.WriteString("    last: Int\n")
		sb.WriteString("    before: String\n")
		sb.WriteString(fmt.Sprintf("    filter: %sFilter\n", typeName))
		sb.WriteString(fmt.Sprintf("    orderBy: %sOrderBy\n", typeName))
		sb.WriteString(fmt.Sprintf("  ): %sConnection!\n\n", typeName))

		// Registra connection
		g.connections[typeName] = true
	}

	sb.WriteString("}\n")

	// Filter inputs
	for typeName, typ := range g.types {
		sb.WriteString(g.generateFilterInput(typeName, typ))
		sb.WriteString(g.generateOrderByEnum(typeName, typ))
	}

	return sb.String()
}

// generateFilterInput gera input de filtro.
func (g *SchemaGenerator) generateFilterInput(typeName string, typ *TypeDefinition) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\ninput %sFilter {\n", typeName))

	for _, field := range typ.Fields {
		// Adiciona filtros baseados no tipo
		switch field.Type {
		case "String":
			sb.WriteString(fmt.Sprintf("  %s: String\n", field.Name))
			sb.WriteString(fmt.Sprintf("  %s_contains: String\n", field.Name))
			sb.WriteString(fmt.Sprintf("  %s_startsWith: String\n", field.Name))
		case "Int", "Float":
			sb.WriteString(fmt.Sprintf("  %s: %s\n", field.Name, field.Type))
			sb.WriteString(fmt.Sprintf("  %s_gt: %s\n", field.Name, field.Type))
			sb.WriteString(fmt.Sprintf("  %s_gte: %s\n", field.Name, field.Type))
			sb.WriteString(fmt.Sprintf("  %s_lt: %s\n", field.Name, field.Type))
			sb.WriteString(fmt.Sprintf("  %s_lte: %s\n", field.Name, field.Type))
		case "Boolean":
			sb.WriteString(fmt.Sprintf("  %s: Boolean\n", field.Name))
		case "DateTime":
			sb.WriteString(fmt.Sprintf("  %s: DateTime\n", field.Name))
			sb.WriteString(fmt.Sprintf("  %s_gt: DateTime\n", field.Name))
			sb.WriteString(fmt.Sprintf("  %s_lt: DateTime\n", field.Name))
		}
	}

	sb.WriteString("  AND: [" + typeName + "Filter!]\n")
	sb.WriteString("  OR: [" + typeName + "Filter!]\n")
	sb.WriteString("}\n")

	return sb.String()
}

// generateOrderByEnum gera enum de ordenação.
func (g *SchemaGenerator) generateOrderByEnum(typeName string, typ *TypeDefinition) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("\nenum %sOrderBy {\n", typeName))

	for _, field := range typ.Fields {
		upperName := strings.ToUpper(toSnakeCase(field.Name))
		sb.WriteString(fmt.Sprintf("  %s_ASC\n", upperName))
		sb.WriteString(fmt.Sprintf("  %s_DESC\n", upperName))
	}

	sb.WriteString("}\n")

	return sb.String()
}

// generateMutationType gera o tipo Mutation.
func (g *SchemaGenerator) generateMutationType() string {
	var sb strings.Builder

	sb.WriteString("type Mutation {\n")

	for typeName := range g.types {
		lowerName := toGraphQLName(typeName)

		// Create
		sb.WriteString(fmt.Sprintf("  create%s(input: %sInput!): %s!\n",
			typeName, typeName, typeName))

		// Update
		sb.WriteString(fmt.Sprintf("  update%s(id: ID!, input: %sInput!): %s!\n",
			typeName, typeName, typeName))

		// Delete
		sb.WriteString(fmt.Sprintf("  delete%s(id: ID!): Boolean!\n", typeName))

		// Batch create
		sb.WriteString(fmt.Sprintf("  create%ss(inputs: [%sInput!]!): [%s!]!\n",
			typeName, typeName, typeName))

		// Batch delete
		sb.WriteString(fmt.Sprintf("  delete%ss(ids: [ID!]!): Int!\n\n", typeName))

		_ = lowerName
	}

	sb.WriteString("}\n")

	return sb.String()
}

// toSnakeCase converte para snake_case.
func toSnakeCase(s string) string {
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
func (g *SchemaGenerator) AddEnum(name string, values []string, descriptions ...string) {
	enum := &EnumDefinition{
		Name:   name,
		Values: make([]EnumValue, len(values)),
	}

	for i, val := range values {
		enum.Values[i] = EnumValue{Name: val}
		if i < len(descriptions) {
			enum.Values[i].Description = descriptions[i]
		}
	}

	g.enums[name] = enum
}

// AddScalar registra um scalar customizado.
func (g *SchemaGenerator) AddScalar(name string) {
	g.scalars[name] = true
}
