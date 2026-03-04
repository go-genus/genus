package query

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// JSON representa um campo JSON/JSONB no banco de dados.
type JSON[T any] struct {
	Data  T
	Valid bool
}

// NewJSON cria um novo campo JSON com dados.
func NewJSON[T any](data T) JSON[T] {
	return JSON[T]{Data: data, Valid: true}
}

// NullJSON cria um JSON nulo.
func NullJSON[T any]() JSON[T] {
	return JSON[T]{Valid: false}
}

// Scan implementa sql.Scanner.
func (j *JSON[T]) Scan(value interface{}) error {
	if value == nil {
		j.Valid = false
		return nil
	}

	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into JSON", value)
	}

	j.Valid = true
	return json.Unmarshal(data, &j.Data)
}

// Value implementa driver.Valuer.
func (j JSON[T]) Value() (driver.Value, error) {
	if !j.Valid {
		return nil, nil
	}
	return json.Marshal(j.Data)
}

// MarshalJSON implementa json.Marshaler.
func (j JSON[T]) MarshalJSON() ([]byte, error) {
	if !j.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(j.Data)
}

// UnmarshalJSON implementa json.Unmarshaler.
func (j *JSON[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		j.Valid = false
		return nil
	}
	j.Valid = true
	return json.Unmarshal(data, &j.Data)
}

// JSONField representa um campo JSON para queries.
type JSONField struct {
	Column string
}

// NewJSONField cria um campo JSON para queries.
func NewJSONField(column string) JSONField {
	return JSONField{Column: column}
}

// Path cria uma condição para acessar um caminho JSON (PostgreSQL: ->, MySQL: ->).
// Exemplo: metadata.Path("user", "name") -> metadata->'user'->'name'
func (f JSONField) Path(keys ...string) JSONPath {
	return JSONPath{
		Column: f.Column,
		Path:   keys,
	}
}

// JSONPath representa um caminho dentro de um JSON.
type JSONPath struct {
	Column string
	Path   []string
}

// Eq verifica igualdade do valor no caminho JSON.
// Usa Raw para gerar SQL diretamente.
func (p JSONPath) Eq(value interface{}) Condition {
	jsonVal, _ := json.Marshal(value)
	return Condition{
		Field:    p.toSQLText(),
		Operator: OpEq,
		Value:    string(jsonVal),
	}
}

// EqText verifica igualdade do valor como texto.
func (p JSONPath) EqText(value string) Condition {
	return Condition{
		Field:    p.toSQLText(),
		Operator: OpEq,
		Value:    value,
	}
}

// Contains cria condição de JSON contains (PostgreSQL: @>).
// Retorna uma RawCondition que pode ser usada com Where.
func (f JSONField) Contains(value interface{}) RawCondition {
	jsonVal, _ := json.Marshal(value)
	return RawCondition{
		SQL:  fmt.Sprintf("%s @> ?", f.Column),
		Args: []interface{}{string(jsonVal)},
	}
}

// ContainedBy cria condição de JSON contained by (PostgreSQL: <@).
func (f JSONField) ContainedBy(value interface{}) RawCondition {
	jsonVal, _ := json.Marshal(value)
	return RawCondition{
		SQL:  fmt.Sprintf("%s <@ ?", f.Column),
		Args: []interface{}{string(jsonVal)},
	}
}

// HasKey cria condição de verificação de chave (PostgreSQL: ?).
func (f JSONField) HasKey(key string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("%s ? ?", f.Column),
		Args: []interface{}{key},
	}
}

// HasAnyKey cria condição para verificar se tem alguma das chaves (PostgreSQL: ?|).
func (f JSONField) HasAnyKey(keys ...string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("%s ?| ?", f.Column),
		Args: []interface{}{keys},
	}
}

// HasAllKeys cria condição para verificar se tem todas as chaves (PostgreSQL: ?&).
func (f JSONField) HasAllKeys(keys ...string) RawCondition {
	return RawCondition{
		SQL:  fmt.Sprintf("%s ?& ?", f.Column),
		Args: []interface{}{keys},
	}
}

// toSQL converte o path para SQL com ->.
func (p JSONPath) toSQL() string {
	result := p.Column
	for _, key := range p.Path {
		result += fmt.Sprintf("->'%s'", key)
	}
	return result
}

// toSQLText converte o path para SQL com ->> no último elemento.
func (p JSONPath) toSQLText() string {
	if len(p.Path) == 0 {
		return p.Column
	}

	result := p.Column
	for i, key := range p.Path {
		if i == len(p.Path)-1 {
			// Último elemento usa ->>
			result += fmt.Sprintf("->>'%s'", key)
		} else {
			result += fmt.Sprintf("->'%s'", key)
		}
	}
	return result
}

// RawCondition representa uma condição SQL raw.
type RawCondition struct {
	SQL  string
	Args []interface{}
}

// JSONRaw representa JSON bruto como string.
type JSONRaw string

// Scan implementa sql.Scanner.
func (j *JSONRaw) Scan(value interface{}) error {
	if value == nil {
		*j = ""
		return nil
	}

	switch v := value.(type) {
	case []byte:
		*j = JSONRaw(v)
	case string:
		*j = JSONRaw(v)
	default:
		return fmt.Errorf("cannot scan %T into JSONRaw", value)
	}
	return nil
}

// Value implementa driver.Valuer.
func (j JSONRaw) Value() (driver.Value, error) {
	if j == "" {
		return nil, nil
	}
	return string(j), nil
}

// Unmarshal decodifica o JSON para o tipo especificado.
func (j JSONRaw) Unmarshal(v interface{}) error {
	return json.Unmarshal([]byte(j), v)
}

// String retorna o JSON como string.
func (j JSONRaw) String() string {
	return string(j)
}

// IsNull verifica se o JSON é nulo ou vazio.
func (j JSONRaw) IsNull() bool {
	return j == "" || j == "null"
}
