package core

import (
	"fmt"
	"reflect"
	"sync"
)

// RelationType define o tipo de relacionamento entre models.
type RelationType string

const (
	HasMany     RelationType = "has_many"
	BelongsTo   RelationType = "belongs_to"
	ManyToMany  RelationType = "many_to_many"
	Polymorphic RelationType = "polymorphic"
)

// RelationshipMeta armazena metadata sobre um relacionamento.
type RelationshipMeta struct {
	Type                  RelationType
	FieldName             string       // Nome do campo Go (ex: "Posts")
	FieldType             reflect.Type // Tipo do model relacionado
	ForeignKey            string       // Chave estrangeira (ex: "user_id")
	References            string       // Coluna referenciada (ex: "id")
	JoinTable             string       // Tabela de junção (para ManyToMany)
	AssociationForeignKey string       // FK da associação (para ManyToMany)
	Polymorphic           string       // Nome base para polymorphic (ex: "commentable")
	PolymorphicType       string       // Nome da coluna type (ex: "commentable_type")
	PolymorphicID         string       // Nome da coluna ID (ex: "commentable_id")
}

// ModelRelationships armazena todos os relacionamentos de um model type.
type ModelRelationships struct {
	ModelType     reflect.Type
	Relationships map[string]*RelationshipMeta // key: nome do campo
}

var (
	// Registry global de relacionamentos
	relationshipRegistry = make(map[reflect.Type]*ModelRelationships)
	registryMu           sync.RWMutex
)

// RegisterModel registra os relacionamentos de um model.
// Deve ser chamado na inicialização da aplicação para cada model com relacionamentos.
func RegisterModel(model interface{}) error {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Parse relacionamentos do model
	rels, err := parseModelRelationships(model)
	if err != nil {
		return fmt.Errorf("failed to parse relationships for %s: %w", modelType.Name(), err)
	}

	// Registra no registry global
	registryMu.Lock()
	relationshipRegistry[rels.ModelType] = rels
	registryMu.Unlock()

	return nil
}

// GetRelationships retorna os relacionamentos registrados para um model type.
func GetRelationships(modelType reflect.Type) *ModelRelationships {
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	registryMu.RLock()
	defer registryMu.RUnlock()
	return relationshipRegistry[modelType]
}

// parseModelRelationships extrai relacionamentos das tags de um model.
func parseModelRelationships(model interface{}) (*ModelRelationships, error) {
	modelType := reflect.TypeOf(model)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	relationships := &ModelRelationships{
		ModelType:     modelType,
		Relationships: make(map[string]*RelationshipMeta),
	}

	// Itera pelos fields do struct
	for i := 0; i < modelType.NumField(); i++ {
		field := modelType.Field(i)

		// Pula campos não exportados
		if !field.IsExported() {
			continue
		}

		// Verifica se tem tag relation
		relationTag := field.Tag.Get("relation")
		if relationTag == "" {
			continue
		}

		// Parse a tag
		meta, err := parseRelationTag(field, relationTag)
		if err != nil {
			return nil, fmt.Errorf("failed to parse relation tag for field %s: %w", field.Name, err)
		}

		relationships.Relationships[field.Name] = meta
	}

	return relationships, nil
}

// parseRelationTag faz o parsing de uma tag relation.
// Formato: relation:"has_many,foreign_key=user_id,references=id"
func parseRelationTag(field reflect.StructField, tag string) (*RelationshipMeta, error) {
	parts := splitTag(tag)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty relation tag")
	}

	meta := &RelationshipMeta{
		Type:      RelationType(parts[0]),
		FieldName: field.Name,
		FieldType: field.Type,
	}

	// Parse key=value pairs
	for _, part := range parts[1:] {
		key, value := parseKeyValue(part)
		switch key {
		case "foreign_key":
			meta.ForeignKey = value
		case "references":
			meta.References = value
		case "join_table":
			meta.JoinTable = value
		case "association_foreign_key":
			meta.AssociationForeignKey = value
		case "polymorphic":
			meta.Polymorphic = value
		case "polymorphic_type":
			meta.PolymorphicType = value
		case "polymorphic_id":
			meta.PolymorphicID = value
		}
	}

	// Define defaults
	if meta.References == "" {
		meta.References = "id"
	}

	// Defaults para polymorphic
	if meta.Polymorphic != "" {
		if meta.PolymorphicType == "" {
			meta.PolymorphicType = meta.Polymorphic + "_type"
		}
		if meta.PolymorphicID == "" {
			meta.PolymorphicID = meta.Polymorphic + "_id"
		}
	}

	// Validações básicas
	switch meta.Type {
	case HasMany, BelongsTo:
		if meta.ForeignKey == "" && meta.Polymorphic == "" {
			return nil, fmt.Errorf("foreign_key is required for %s relationship", meta.Type)
		}
	case ManyToMany:
		if meta.JoinTable == "" {
			return nil, fmt.Errorf("join_table is required for many_to_many relationship")
		}
		if meta.ForeignKey == "" || meta.AssociationForeignKey == "" {
			return nil, fmt.Errorf("foreign_key and association_foreign_key are required for many_to_many")
		}
	case Polymorphic:
		if meta.Polymorphic == "" {
			return nil, fmt.Errorf("polymorphic field name is required for polymorphic relationship")
		}
	default:
		return nil, fmt.Errorf("unknown relation type: %s", meta.Type)
	}

	return meta, nil
}

// splitTag divide a tag por vírgulas, respeitando espaços.
func splitTag(tag string) []string {
	var result []string
	var current string

	for _, char := range tag {
		if char == ',' {
			if current != "" {
				result = append(result, trimSpace(current))
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, trimSpace(current))
	}

	return result
}

// parseKeyValue divide uma string "key=value" em key e value.
func parseKeyValue(s string) (string, string) {
	for i, char := range s {
		if char == '=' {
			key := trimSpace(s[:i])
			value := trimSpace(s[i+1:])
			return key, value
		}
	}
	return s, ""
}

// trimSpace remove espaços do início e fim de uma string.
func trimSpace(s string) string {
	start := 0
	end := len(s)

	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}

	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}

	return s[start:end]
}
