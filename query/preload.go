package query

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-genus/genus/core"
)

// PreloadSpec especifica quais relacionamentos eager load.
type PreloadSpec struct {
	Relation string
	Nested   []*PreloadSpec // Para preloads aninhados: Preload("Posts.Comments")
}

// parsePreloadPath converte "Posts.Comments" em PreloadSpecs aninhados.
func parsePreloadPath(path string) *PreloadSpec {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	spec := &PreloadSpec{Relation: parts[0]}
	if len(parts) > 1 {
		spec.Nested = []*PreloadSpec{parsePreloadPath(strings.Join(parts[1:], "."))}
	}

	return spec
}

// executePreload executa o eager loading de relacionamentos.
func executePreload[T any](
	ctx context.Context,
	executor core.Executor,
	dialect core.Dialect,
	logger core.Logger,
	results []T,
	specs []*PreloadSpec,
) error {
	if len(results) == 0 {
		return nil
	}

	// Pega o tipo do model
	var zero T
	modelType := reflect.TypeOf(zero)
	if modelType.Kind() == reflect.Ptr {
		modelType = modelType.Elem()
	}

	// Pega relacionamentos registrados
	rels := core.GetRelationships(modelType)
	if rels == nil {
		return fmt.Errorf("no relationships registered for %s. Did you call genus.RegisterModel()?", modelType.Name())
	}

	// Processa cada relacionamento a ser preloaded
	for _, spec := range specs {
		relMeta, ok := rels.Relationships[spec.Relation]
		if !ok {
			return fmt.Errorf("relationship '%s' not found on %s", spec.Relation, modelType.Name())
		}

		// Carrega baseado no tipo de relacionamento
		switch relMeta.Type {
		case core.HasMany:
			if err := preloadHasMany(ctx, executor, dialect, logger, results, relMeta, spec.Nested); err != nil {
				return err
			}
		case core.BelongsTo:
			if err := preloadBelongsTo(ctx, executor, dialect, logger, results, relMeta, spec.Nested); err != nil {
				return err
			}
		case core.ManyToMany:
			if err := preloadManyToMany(ctx, executor, dialect, logger, results, relMeta, spec.Nested); err != nil {
				return err
			}
		case core.Polymorphic:
			if err := preloadPolymorphic(ctx, executor, dialect, logger, results, relMeta, spec.Nested); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported relationship type: %s", relMeta.Type)
		}
	}

	return nil
}

// preloadHasMany carrega relacionamento has_many.
// Exemplo: User has many Posts
func preloadHasMany[T any](
	ctx context.Context,
	executor core.Executor,
	dialect core.Dialect,
	logger core.Logger,
	parents []T,
	meta *core.RelationshipMeta,
	nested []*PreloadSpec,
) error {
	// 1. Coleta todos os IDs dos parents
	parentIDs := make([]int64, 0, len(parents))
	for _, parent := range parents {
		v := reflect.ValueOf(parent)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		idField := v.FieldByName("ID")
		if !idField.IsValid() {
			continue
		}
		parentIDs = append(parentIDs, idField.Int())
	}

	if len(parentIDs) == 0 {
		return nil
	}

	// 2. Query: SELECT * FROM child_table WHERE foreign_key IN (parent_ids)
	childTableName := getTableNameFromType(meta.FieldType)

	// Monta IN clause
	placeholders := make([]string, len(parentIDs))
	args := make([]interface{}, len(parentIDs))
	for i, id := range parentIDs {
		placeholders[i] = dialect.Placeholder(i + 1)
		args[i] = id
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s IN (%s)",
		dialect.QuoteIdentifier(childTableName),
		meta.ForeignKey,
		strings.Join(placeholders, ", "),
	)

	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		logger.LogError(query, args, err)
		return fmt.Errorf("failed to preload %s: %w", meta.FieldName, err)
	}
	defer rows.Close()

	// 3. Scan resultados e agrupa por foreign_key
	childrenMap := make(map[int64][]reflect.Value)

	for rows.Next() {
		// Cria uma nova instância do tipo child
		childType := meta.FieldType
		if childType.Kind() == reflect.Slice {
			childType = childType.Elem()
		}

		child := reflect.New(childType).Interface()

		if err := scanStruct(rows, child); err != nil {
			return fmt.Errorf("failed to scan child: %w", err)
		}

		// Pega o valor da foreign_key
		childValue := reflect.ValueOf(child).Elem()
		fkField := childValue.FieldByName(toPascalCase(meta.ForeignKey))
		if !fkField.IsValid() {
			return fmt.Errorf("foreign key field not found: %s", meta.ForeignKey)
		}

		fkValue := fkField.Int()
		childrenMap[fkValue] = append(childrenMap[fkValue], childValue)
	}

	// 4. Atribui children aos parents
	for i := range parents {
		v := reflect.ValueOf(&parents[i]).Elem()
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		idField := v.FieldByName("ID")
		if !idField.IsValid() {
			continue
		}
		parentID := idField.Int()

		// Pega field do relacionamento
		relField := v.FieldByName(meta.FieldName)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}

		// Atribui os children
		if children, ok := childrenMap[parentID]; ok {
			slice := reflect.MakeSlice(relField.Type(), len(children), len(children))
			for i, child := range children {
				slice.Index(i).Set(child)
			}
			relField.Set(slice)
		}
	}

	return nil
}

// preloadBelongsTo carrega relacionamento belongs_to.
// Exemplo: Post belongs to User
func preloadBelongsTo[T any](
	ctx context.Context,
	executor core.Executor,
	dialect core.Dialect,
	logger core.Logger,
	children []T,
	meta *core.RelationshipMeta,
	nested []*PreloadSpec,
) error {
	// 1. Coleta todos os foreign_key values
	fkValues := make([]int64, 0, len(children))
	fkSet := make(map[int64]bool)

	for _, child := range children {
		v := reflect.ValueOf(child)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		fkField := v.FieldByName(toPascalCase(meta.ForeignKey))
		if !fkField.IsValid() {
			continue
		}

		fkVal := fkField.Int()
		if fkVal > 0 && !fkSet[fkVal] {
			fkValues = append(fkValues, fkVal)
			fkSet[fkVal] = true
		}
	}

	if len(fkValues) == 0 {
		return nil
	}

	// 2. Query: SELECT * FROM parent_table WHERE id IN (fk_values)
	parentTableName := getTableNameFromType(meta.FieldType)

	placeholders := make([]string, len(fkValues))
	args := make([]interface{}, len(fkValues))
	for i, fk := range fkValues {
		placeholders[i] = dialect.Placeholder(i + 1)
		args[i] = fk
	}

	query := fmt.Sprintf(
		"SELECT * FROM %s WHERE %s IN (%s)",
		dialect.QuoteIdentifier(parentTableName),
		meta.References,
		strings.Join(placeholders, ", "),
	)

	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		logger.LogError(query, args, err)
		return fmt.Errorf("failed to preload %s: %w", meta.FieldName, err)
	}
	defer rows.Close()

	// 3. Scan e mapeia parents por ID
	parentsMap := make(map[int64]reflect.Value)

	for rows.Next() {
		parentType := meta.FieldType
		if parentType.Kind() == reflect.Ptr {
			parentType = parentType.Elem()
		}

		parent := reflect.New(parentType).Interface()

		if err := scanStruct(rows, parent); err != nil {
			return fmt.Errorf("failed to scan parent: %w", err)
		}

		parentValue := reflect.ValueOf(parent).Elem()
		idField := parentValue.FieldByName("ID")
		if idField.IsValid() {
			parentsMap[idField.Int()] = parentValue
		}
	}

	// 4. Atribui parent a cada child
	for i := range children {
		v := reflect.ValueOf(&children[i]).Elem()
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		fkField := v.FieldByName(toPascalCase(meta.ForeignKey))
		if !fkField.IsValid() {
			continue
		}

		fkVal := fkField.Int()
		if parent, ok := parentsMap[fkVal]; ok {
			relField := v.FieldByName(meta.FieldName)
			if relField.IsValid() && relField.CanSet() {
				// Se o field é pointer
				if relField.Type().Kind() == reflect.Ptr {
					ptr := reflect.New(parent.Type())
					ptr.Elem().Set(parent)
					relField.Set(ptr)
				} else {
					relField.Set(parent)
				}
			}
		}
	}

	return nil
}

// preloadManyToMany carrega relacionamento many_to_many.
// Exemplo: Post has many Tags through post_tags
func preloadManyToMany[T any](
	ctx context.Context,
	executor core.Executor,
	dialect core.Dialect,
	logger core.Logger,
	parents []T,
	meta *core.RelationshipMeta,
	nested []*PreloadSpec,
) error {
	// 1. Coleta parent IDs
	parentIDs := make([]int64, 0, len(parents))
	for _, parent := range parents {
		v := reflect.ValueOf(parent)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		idField := v.FieldByName("ID")
		if !idField.IsValid() {
			continue
		}
		parentIDs = append(parentIDs, idField.Int())
	}

	if len(parentIDs) == 0 {
		return nil
	}

	// 2. Query com JOIN na join table
	targetTableName := getTableNameFromType(meta.FieldType)

	placeholders := make([]string, len(parentIDs))
	args := make([]interface{}, len(parentIDs))
	for i, id := range parentIDs {
		placeholders[i] = dialect.Placeholder(i + 1)
		args[i] = id
	}

	query := fmt.Sprintf(
		`SELECT %s.*, %s.%s as __parent_id__
		FROM %s
		INNER JOIN %s ON %s.%s = %s.%s
		WHERE %s.%s IN (%s)`,
		dialect.QuoteIdentifier(targetTableName),
		dialect.QuoteIdentifier(meta.JoinTable),
		meta.ForeignKey,
		dialect.QuoteIdentifier(targetTableName),
		dialect.QuoteIdentifier(meta.JoinTable),
		dialect.QuoteIdentifier(targetTableName),
		"id",
		dialect.QuoteIdentifier(meta.JoinTable),
		meta.AssociationForeignKey,
		dialect.QuoteIdentifier(meta.JoinTable),
		meta.ForeignKey,
		strings.Join(placeholders, ", "),
	)

	rows, err := executor.QueryContext(ctx, query, args...)
	if err != nil {
		logger.LogError(query, args, err)
		return fmt.Errorf("failed to preload many_to_many %s: %w", meta.FieldName, err)
	}
	defer rows.Close()

	// 3. Pega os nomes das colunas
	columns, err := rows.Columns()
	if err != nil {
		return fmt.Errorf("failed to get columns: %w", err)
	}

	// 4. Scan e agrupa por parent_id
	targetsMap := make(map[int64][]reflect.Value)

	for rows.Next() {
		targetType := meta.FieldType
		if targetType.Kind() == reflect.Slice {
			targetType = targetType.Elem()
		}
		if targetType.Kind() == reflect.Ptr {
			targetType = targetType.Elem()
		}

		target := reflect.New(targetType).Interface()

		// Cria slice de ponteiros para scan
		// Precisamos capturar __parent_id__ também
		scanValues := make([]interface{}, len(columns))
		var parentID int64

		targetValue := reflect.ValueOf(target).Elem()
		targetFieldMap := buildFieldMapForStruct(targetValue.Type())

		for i, colName := range columns {
			if colName == "__parent_id__" {
				// Captura o parent_id separadamente
				scanValues[i] = &parentID
			} else {
				// Mapeia para o campo do target
				if path, ok := targetFieldMap[colName]; ok {
					field := getFieldByPath(targetValue, path)
					if field.IsValid() && field.CanAddr() {
						scanValues[i] = field.Addr().Interface()
					} else {
						var placeholder interface{}
						scanValues[i] = &placeholder
					}
				} else {
					var placeholder interface{}
					scanValues[i] = &placeholder
				}
			}
		}

		if err := rows.Scan(scanValues...); err != nil {
			return fmt.Errorf("failed to scan many_to_many target: %w", err)
		}

		// Agrupa por parent_id
		targetsMap[parentID] = append(targetsMap[parentID], targetValue)
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("rows iteration error: %w", err)
	}

	// 5. Atribui targets aos parents
	for i := range parents {
		v := reflect.ValueOf(&parents[i]).Elem()
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		idField := v.FieldByName("ID")
		if !idField.IsValid() {
			continue
		}
		parentID := idField.Int()

		// Pega field do relacionamento
		relField := v.FieldByName(meta.FieldName)
		if !relField.IsValid() || !relField.CanSet() {
			continue
		}

		// Atribui os targets
		if targets, ok := targetsMap[parentID]; ok {
			slice := reflect.MakeSlice(relField.Type(), len(targets), len(targets))
			for i, target := range targets {
				slice.Index(i).Set(target)
			}
			relField.Set(slice)
		}
	}

	return nil
}

// getTableNameFromType retorna o nome da tabela a partir do tipo.
func getTableNameFromType(t reflect.Type) string {
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() == reflect.Slice {
		t = t.Elem()
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Converte TypeName para snake_case
	name := t.Name()
	var result strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// toPascalCase converte snake_case para PascalCase.
// Exemplo: "user_id" -> "UserID"
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	var result strings.Builder
	for _, part := range parts {
		if len(part) > 0 {
			result.WriteRune(rune(part[0] - 32)) // Uppercase first char
			result.WriteString(part[1:])
		}
	}
	return result.String()
}

// buildFieldMapForStruct é um wrapper para buildFieldMap do scanner.
func buildFieldMapForStruct(typ reflect.Type) map[string]fieldPath {
	return buildFieldMap(typ)
}

// preloadPolymorphic carrega relacionamentos polimórficos.
// Exemplo: Comment pertence a Post ou User via commentable_type e commentable_id
//
// Table: comments
//
//	id | body | commentable_type | commentable_id
//	1  | Nice | Post             | 5
//	2  | Great| User             | 3
func preloadPolymorphic[T any](
	ctx context.Context,
	executor core.Executor,
	dialect core.Dialect,
	logger core.Logger,
	children []T,
	meta *core.RelationshipMeta,
	nested []*PreloadSpec,
) error {
	if len(children) == 0 {
		return nil
	}

	// 1. Coleta todos os pares (type, id) dos children
	type polyKey struct {
		typeName string
		id       int64
	}

	polyKeys := make([]polyKey, 0, len(children))
	keySet := make(map[polyKey]bool)

	typeFieldName := toPascalCase(meta.PolymorphicType)
	idFieldName := toPascalCase(meta.PolymorphicID)

	for _, child := range children {
		v := reflect.ValueOf(child)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		typeField := v.FieldByName(typeFieldName)
		idField := v.FieldByName(idFieldName)

		if !typeField.IsValid() || !idField.IsValid() {
			continue
		}

		typeName := typeField.String()
		id := idField.Int()

		if typeName != "" && id > 0 {
			key := polyKey{typeName: typeName, id: id}
			if !keySet[key] {
				polyKeys = append(polyKeys, key)
				keySet[key] = true
			}
		}
	}

	if len(polyKeys) == 0 {
		return nil
	}

	// 2. Agrupa por type para fazer queries batch
	typeGroups := make(map[string][]int64)
	for _, key := range polyKeys {
		typeGroups[key.typeName] = append(typeGroups[key.typeName], key.id)
	}

	// 3. Para cada type, faz uma query e armazena os resultados
	parentsMap := make(map[polyKey]reflect.Value)

	for typeName, ids := range typeGroups {
		// Converte type name para table name (Post -> post)
		tableName := toSnakeCasePlural(typeName)

		// Constrói query
		placeholders := make([]string, len(ids))
		args := make([]interface{}, len(ids))
		for i, id := range ids {
			placeholders[i] = dialect.Placeholder(i + 1)
			args[i] = id
		}

		query := fmt.Sprintf(
			"SELECT * FROM %s WHERE id IN (%s)",
			dialect.QuoteIdentifier(tableName),
			strings.Join(placeholders, ", "),
		)

		rows, err := executor.QueryContext(ctx, query, args...)
		if err != nil {
			logger.LogError(query, args, err)
			return fmt.Errorf("failed to preload polymorphic %s for type %s: %w", meta.FieldName, typeName, err)
		}

		// Scan os resultados
		for rows.Next() {
			// Cria instância do tipo relacionado
			parentType := meta.FieldType
			if parentType.Kind() == reflect.Ptr {
				parentType = parentType.Elem()
			}

			parent := reflect.New(parentType).Interface()

			if err := scanStruct(rows, parent); err != nil {
				rows.Close()
				return fmt.Errorf("failed to scan polymorphic parent: %w", err)
			}

			parentValue := reflect.ValueOf(parent).Elem()
			idField := parentValue.FieldByName("ID")
			if idField.IsValid() {
				key := polyKey{typeName: typeName, id: idField.Int()}
				parentsMap[key] = parentValue
			}
		}
		rows.Close()
	}

	// 4. Atribui parents aos children
	for i := range children {
		v := reflect.ValueOf(&children[i]).Elem()
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}

		typeField := v.FieldByName(typeFieldName)
		idField := v.FieldByName(idFieldName)

		if !typeField.IsValid() || !idField.IsValid() {
			continue
		}

		typeName := typeField.String()
		id := idField.Int()
		key := polyKey{typeName: typeName, id: id}

		if parent, ok := parentsMap[key]; ok {
			relField := v.FieldByName(meta.FieldName)
			if relField.IsValid() && relField.CanSet() {
				// Se o field é pointer
				if relField.Type().Kind() == reflect.Ptr {
					ptr := reflect.New(parent.Type())
					ptr.Elem().Set(parent)
					relField.Set(ptr)
				} else if relField.Type().Kind() == reflect.Interface {
					// Para interface{}, apenas define o valor
					relField.Set(parent.Addr())
				} else {
					relField.Set(parent)
				}
			}
		}
	}

	return nil
}

// toSnakeCasePlural converte PascalCase para snake_case (simplificado).
// Para nomes de tabelas polimórficas.
func toSnakeCasePlural(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
