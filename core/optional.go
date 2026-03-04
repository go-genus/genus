package core

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Optional representa um valor que pode ou não estar presente.
// Resolve o problema da manipulação de sql.Null* e ponteiros em JSON
// com uma API limpa e unificada.
//
// Exemplo de uso:
//
//	type User struct {
//	    core.Model
//	    Name  string
//	    Email Optional[string] // pode ser NULL no banco
//	    Age   Optional[int]    // pode ser NULL no banco
//	}
//
//	// Criar um valor presente
//	email := Some("user@example.com")
//
//	// Criar um valor ausente
//	age := None[int]()
//
//	// Verificar se tem valor
//	if email.IsPresent() {
//	    fmt.Println(email.Get()) // user@example.com
//	}
//
//	// Obter com valor padrão
//	value := age.GetOrDefault(18) // retorna 18
type Optional[T any] struct {
	value T
	valid bool
}

// Some cria um Optional com um valor presente.
func Some[T any](value T) Optional[T] {
	return Optional[T]{
		value: value,
		valid: true,
	}
}

// None cria um Optional sem valor (NULL).
func None[T any]() Optional[T] {
	return Optional[T]{
		valid: false,
	}
}

// FromPtr cria um Optional a partir de um ponteiro.
// Se o ponteiro for nil, retorna None. Caso contrário, retorna Some com o valor.
func FromPtr[T any](ptr *T) Optional[T] {
	if ptr == nil {
		return None[T]()
	}
	return Some(*ptr)
}

// IsPresent retorna true se o Optional contém um valor.
func (o Optional[T]) IsPresent() bool {
	return o.valid
}

// IsAbsent retorna true se o Optional não contém um valor.
func (o Optional[T]) IsAbsent() bool {
	return !o.valid
}

// Get retorna o valor contido no Optional.
// Entra em pânico se o Optional não contiver um valor.
// Use IsPresent() para verificar antes de chamar Get().
func (o Optional[T]) Get() T {
	if !o.valid {
		panic("Optional.Get() chamado em um Optional vazio")
	}
	return o.value
}

// GetOrDefault retorna o valor se presente, ou o valor padrão fornecido.
func (o Optional[T]) GetOrDefault(defaultValue T) T {
	if o.valid {
		return o.value
	}
	return defaultValue
}

// GetOrZero retorna o valor se presente, ou o valor zero do tipo.
func (o Optional[T]) GetOrZero() T {
	return o.value
}

// Ptr retorna um ponteiro para o valor se presente, ou nil.
func (o Optional[T]) Ptr() *T {
	if !o.valid {
		return nil
	}
	return &o.value
}

// Map aplica uma função ao valor se presente, retornando um novo Optional.
// Se o Optional estiver vazio, retorna um Optional vazio do novo tipo.
func Map[T any, U any](o Optional[T], fn func(T) U) Optional[U] {
	if !o.valid {
		return None[U]()
	}
	return Some(fn(o.value))
}

// FlatMap aplica uma função que retorna um Optional ao valor se presente.
// Se o Optional estiver vazio, retorna um Optional vazio do novo tipo.
func FlatMap[T any, U any](o Optional[T], fn func(T) Optional[U]) Optional[U] {
	if !o.valid {
		return None[U]()
	}
	return fn(o.value)
}

// Filter retorna o Optional se o predicado for verdadeiro, ou None caso contrário.
func (o Optional[T]) Filter(predicate func(T) bool) Optional[T] {
	if !o.valid || !predicate(o.value) {
		return None[T]()
	}
	return o
}

// IfPresent executa uma função se o Optional contiver um valor.
func (o Optional[T]) IfPresent(fn func(T)) {
	if o.valid {
		fn(o.value)
	}
}

// IfPresentOrElse executa fnPresent se o Optional contiver um valor,
// ou fnAbsent caso contrário.
func (o Optional[T]) IfPresentOrElse(fnPresent func(T), fnAbsent func()) {
	if o.valid {
		fnPresent(o.value)
	} else {
		fnAbsent()
	}
}

// --- Suporte a database/sql ---

// Scan implementa sql.Scanner para permitir scan direto do banco de dados.
func (o *Optional[T]) Scan(value any) error {
	if value == nil {
		o.valid = false
		return nil
	}

	o.valid = true

	// Tenta converter o valor para o tipo T
	switch v := value.(type) {
	case T:
		o.value = v
		return nil
	}

	// Fallback: tenta usar sql.Scanner se T implementar
	if scanner, ok := any(&o.value).(sql.Scanner); ok {
		return scanner.Scan(value)
	}

	// Common conversions for primitive types
	switch any(&o.value).(type) {
	case *string:
		str, err := convertToString(value)
		if err != nil {
			return err
		}
		o.value = any(str).(T)
		return nil
	case *int:
		i, err := convertToInt(value)
		if err != nil {
			return err
		}
		o.value = any(i).(T)
		return nil
	case *int64:
		i64, err := convertToInt64(value)
		if err != nil {
			return err
		}
		o.value = any(i64).(T)
		return nil
	case *bool:
		b, err := convertToBool(value)
		if err != nil {
			return err
		}
		o.value = any(b).(T)
		return nil
	case *float64:
		f, err := convertToFloat64(value)
		if err != nil {
			return err
		}
		o.value = any(f).(T)
		return nil
	}

	return fmt.Errorf("não foi possível converter %T para %T", value, o.value)
}

// Value implementa driver.Valuer para permitir inserção no banco de dados.
func (o Optional[T]) Value() (driver.Value, error) {
	if !o.valid {
		return nil, nil
	}

	// Se T implementa driver.Valuer, usa isso
	if valuer, ok := any(o.value).(driver.Valuer); ok {
		return valuer.Value()
	}

	// Retorna o valor diretamente para tipos primitivos
	return any(o.value), nil
}

// --- Suporte a JSON ---

// MarshalJSON implementa json.Marshaler.
// Se o Optional estiver vazio, serializa como null.
// Caso contrário, serializa o valor contido.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.valid {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON implementa json.Unmarshaler.
// Se o JSON for null, define o Optional como vazio.
// Caso contrário, desserializa para o valor contido.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		o.valid = false
		return nil
	}

	o.valid = true
	return json.Unmarshal(data, &o.value)
}

// --- Helper conversion functions ---

func convertToString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("não foi possível converter %T para string", value)
	}
}

func convertToInt(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case int32:
		return int(v), nil
	case []byte:
		var i int64
		_, err := fmt.Sscanf(string(v), "%d", &i)
		return int(i), err
	default:
		return 0, fmt.Errorf("não foi possível converter %T para int", value)
	}
}

func convertToInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case []byte:
		var i int64
		_, err := fmt.Sscanf(string(v), "%d", &i)
		return i, err
	default:
		return 0, fmt.Errorf("não foi possível converter %T para int64", value)
	}
}

func convertToBool(value any) (bool, error) {
	switch v := value.(type) {
	case bool:
		return v, nil
	case int64:
		return v != 0, nil
	case []byte:
		var b bool
		_, err := fmt.Sscanf(string(v), "%t", &b)
		return b, err
	default:
		return false, fmt.Errorf("não foi possível converter %T para bool", value)
	}
}

func convertToFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case []byte:
		var f float64
		_, err := fmt.Sscanf(string(v), "%f", &f)
		return f, err
	default:
		return 0, fmt.Errorf("não foi possível converter %T para float64", value)
	}
}
