package core

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Optional represents a value that may or may not be present.
// It solves the problem of handling sql.Null* and pointers in JSON
// with a clean and unified API.
//
// Usage example:
//
//	type User struct {
//	    core.Model
//	    Name  string
//	    Email Optional[string] // can be NULL in database
//	    Age   Optional[int]    // can be NULL in database
//	}
//
//	// Create a present value
//	email := Some("user@example.com")
//
//	// Create an absent value
//	age := None[int]()
//
//	// Check if it has a value
//	if email.IsPresent() {
//	    fmt.Println(email.Get()) // user@example.com
//	}
//
//	// Get with default value
//	value := age.GetOrDefault(18) // returns 18
type Optional[T any] struct {
	value T
	valid bool
}

// Some creates an Optional with a present value.
func Some[T any](value T) Optional[T] {
	return Optional[T]{
		value: value,
		valid: true,
	}
}

// None creates an Optional without a value (NULL).
func None[T any]() Optional[T] {
	return Optional[T]{
		valid: false,
	}
}

// FromPtr creates an Optional from a pointer.
// If the pointer is nil, returns None. Otherwise, returns Some with the value.
func FromPtr[T any](ptr *T) Optional[T] {
	if ptr == nil {
		return None[T]()
	}
	return Some(*ptr)
}

// IsPresent returns true if the Optional contains a value.
func (o Optional[T]) IsPresent() bool {
	return o.valid
}

// IsAbsent returns true if the Optional does not contain a value.
func (o Optional[T]) IsAbsent() bool {
	return !o.valid
}

// Get returns the value contained in the Optional.
// Panics if the Optional does not contain a value.
// Use IsPresent() to check before calling Get().
func (o Optional[T]) Get() T {
	if !o.valid {
		panic("Optional.Get() called on an empty Optional")
	}
	return o.value
}

// GetOrDefault returns the value if present, or the provided default value.
func (o Optional[T]) GetOrDefault(defaultValue T) T {
	if o.valid {
		return o.value
	}
	return defaultValue
}

// GetOrZero returns the value if present, or the zero value of the type.
func (o Optional[T]) GetOrZero() T {
	return o.value
}

// Ptr returns a pointer to the value if present, or nil.
func (o Optional[T]) Ptr() *T {
	if !o.valid {
		return nil
	}
	return &o.value
}

// Map applies a function to the value if present, returning a new Optional.
// If the Optional is empty, returns an empty Optional of the new type.
func Map[T any, U any](o Optional[T], fn func(T) U) Optional[U] {
	if !o.valid {
		return None[U]()
	}
	return Some(fn(o.value))
}

// FlatMap applies a function that returns an Optional to the value if present.
// If the Optional is empty, returns an empty Optional of the new type.
func FlatMap[T any, U any](o Optional[T], fn func(T) Optional[U]) Optional[U] {
	if !o.valid {
		return None[U]()
	}
	return fn(o.value)
}

// Filter returns the Optional if the predicate is true, or None otherwise.
func (o Optional[T]) Filter(predicate func(T) bool) Optional[T] {
	if !o.valid || !predicate(o.value) {
		return None[T]()
	}
	return o
}

// IfPresent executes a function if the Optional contains a value.
func (o Optional[T]) IfPresent(fn func(T)) {
	if o.valid {
		fn(o.value)
	}
}

// IfPresentOrElse executes fnPresent if the Optional contains a value,
// or fnAbsent otherwise.
func (o Optional[T]) IfPresentOrElse(fnPresent func(T), fnAbsent func()) {
	if o.valid {
		fnPresent(o.value)
	} else {
		fnAbsent()
	}
}

// --- database/sql support ---

// Scan implements sql.Scanner to allow direct scan from database.
func (o *Optional[T]) Scan(value any) error {
	if value == nil {
		o.valid = false
		return nil
	}

	o.valid = true

	// Try to convert value to type T
	switch v := value.(type) {
	case T:
		o.value = v
		return nil
	}

	// Fallback: try to use sql.Scanner if T implements it
	if scanner, ok := any(&o.value).(sql.Scanner); ok {
		return scanner.Scan(value)
	}

	// Common type conversions for primitive types
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

	return fmt.Errorf("could not convert %T to %T", value, o.value)
}

// Value implements driver.Valuer to allow insertion into database.
func (o Optional[T]) Value() (driver.Value, error) {
	if !o.valid {
		return nil, nil
	}

	// If T implements driver.Valuer, use that
	if valuer, ok := any(o.value).(driver.Valuer); ok {
		return valuer.Value()
	}

	// Return value directly for primitive types
	return any(o.value), nil
}

// --- JSON support ---

// MarshalJSON implements json.Marshaler.
// If the Optional is empty, serializes as null.
// Otherwise, serializes the contained value.
func (o Optional[T]) MarshalJSON() ([]byte, error) {
	if !o.valid {
		return []byte("null"), nil
	}
	return json.Marshal(o.value)
}

// UnmarshalJSON implements json.Unmarshaler.
// If the JSON is null, sets the Optional as empty.
// Otherwise, deserializes into the contained value.
func (o *Optional[T]) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		o.valid = false
		return nil
	}

	o.valid = true
	return json.Unmarshal(data, &o.value)
}

// --- Helper type conversion functions ---

func convertToString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		return "", fmt.Errorf("could not convert %T to string", value)
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
		return 0, fmt.Errorf("could not convert %T to int", value)
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
		return 0, fmt.Errorf("could not convert %T to int64", value)
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
		return false, fmt.Errorf("could not convert %T to bool", value)
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
		return 0, fmt.Errorf("could not convert %T to float64", value)
	}
}
