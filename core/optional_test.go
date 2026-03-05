package core

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"testing"
)

func TestSome(t *testing.T) {
	opt := Some("hello")
	if !opt.IsPresent() {
		t.Error("Some should be present")
	}
	if opt.IsAbsent() {
		t.Error("Some should not be absent")
	}
	if opt.Get() != "hello" {
		t.Errorf("Get() = %q, want %q", opt.Get(), "hello")
	}
}

func TestNone(t *testing.T) {
	opt := None[string]()
	if opt.IsPresent() {
		t.Error("None should not be present")
	}
	if !opt.IsAbsent() {
		t.Error("None should be absent")
	}
}

func TestNone_Get_Panics(t *testing.T) {
	opt := None[string]()
	defer func() {
		if r := recover(); r == nil {
			t.Error("Get() on None should panic")
		}
	}()
	opt.Get()
}

func TestGetOrDefault_Present(t *testing.T) {
	opt := Some(42)
	if got := opt.GetOrDefault(0); got != 42 {
		t.Errorf("GetOrDefault() = %d, want 42", got)
	}
}

func TestGetOrDefault_Absent(t *testing.T) {
	opt := None[int]()
	if got := opt.GetOrDefault(99); got != 99 {
		t.Errorf("GetOrDefault() = %d, want 99", got)
	}
}

func TestGetOrZero_Present(t *testing.T) {
	opt := Some(42)
	if got := opt.GetOrZero(); got != 42 {
		t.Errorf("GetOrZero() = %d, want 42", got)
	}
}

func TestGetOrZero_Absent(t *testing.T) {
	opt := None[int]()
	if got := opt.GetOrZero(); got != 0 {
		t.Errorf("GetOrZero() = %d, want 0", got)
	}
}

func TestFromPtr_NonNil(t *testing.T) {
	val := "hello"
	opt := FromPtr(&val)
	if !opt.IsPresent() {
		t.Error("FromPtr with non-nil should be present")
	}
	if opt.Get() != "hello" {
		t.Errorf("Get() = %q, want %q", opt.Get(), "hello")
	}
}

func TestFromPtr_Nil(t *testing.T) {
	opt := FromPtr[string](nil)
	if opt.IsPresent() {
		t.Error("FromPtr with nil should not be present")
	}
}

func TestPtr_Present(t *testing.T) {
	opt := Some("hello")
	ptr := opt.Ptr()
	if ptr == nil {
		t.Fatal("Ptr() should not be nil for present value")
	}
	if *ptr != "hello" {
		t.Errorf("*Ptr() = %q, want %q", *ptr, "hello")
	}
}

func TestPtr_Absent(t *testing.T) {
	opt := None[string]()
	if opt.Ptr() != nil {
		t.Error("Ptr() should be nil for absent value")
	}
}

func TestMap_Present(t *testing.T) {
	opt := Some(5)
	mapped := Map(opt, func(i int) string {
		return "value"
	})
	if !mapped.IsPresent() {
		t.Error("Map on present should be present")
	}
	if mapped.Get() != "value" {
		t.Errorf("Map result = %q, want %q", mapped.Get(), "value")
	}
}

func TestMap_Absent(t *testing.T) {
	opt := None[int]()
	mapped := Map(opt, func(i int) string {
		return "value"
	})
	if mapped.IsPresent() {
		t.Error("Map on absent should be absent")
	}
}

func TestFlatMap_Present(t *testing.T) {
	opt := Some(5)
	result := FlatMap(opt, func(i int) Optional[string] {
		return Some("mapped")
	})
	if !result.IsPresent() {
		t.Error("FlatMap on present should be present")
	}
	if result.Get() != "mapped" {
		t.Errorf("FlatMap result = %q, want %q", result.Get(), "mapped")
	}
}

func TestFlatMap_Absent(t *testing.T) {
	opt := None[int]()
	result := FlatMap(opt, func(i int) Optional[string] {
		return Some("mapped")
	})
	if result.IsPresent() {
		t.Error("FlatMap on absent should be absent")
	}
}

func TestFlatMap_ReturnsNone(t *testing.T) {
	opt := Some(5)
	result := FlatMap(opt, func(i int) Optional[string] {
		return None[string]()
	})
	if result.IsPresent() {
		t.Error("FlatMap returning None should be absent")
	}
}

func TestFilter_Present_Passes(t *testing.T) {
	opt := Some(10)
	filtered := opt.Filter(func(i int) bool { return i > 5 })
	if !filtered.IsPresent() {
		t.Error("Filter should keep value when predicate passes")
	}
}

func TestFilter_Present_Fails(t *testing.T) {
	opt := Some(3)
	filtered := opt.Filter(func(i int) bool { return i > 5 })
	if filtered.IsPresent() {
		t.Error("Filter should remove value when predicate fails")
	}
}

func TestFilter_Absent(t *testing.T) {
	opt := None[int]()
	filtered := opt.Filter(func(i int) bool { return true })
	if filtered.IsPresent() {
		t.Error("Filter on absent should remain absent")
	}
}

func TestIfPresent_Present(t *testing.T) {
	opt := Some(42)
	called := false
	opt.IfPresent(func(v int) {
		called = true
		if v != 42 {
			t.Errorf("IfPresent value = %d, want 42", v)
		}
	})
	if !called {
		t.Error("IfPresent callback should be called")
	}
}

func TestIfPresent_Absent(t *testing.T) {
	opt := None[int]()
	called := false
	opt.IfPresent(func(v int) {
		called = true
	})
	if called {
		t.Error("IfPresent callback should not be called on absent")
	}
}

func TestIfPresentOrElse_Present(t *testing.T) {
	opt := Some(42)
	presentCalled := false
	absentCalled := false
	opt.IfPresentOrElse(
		func(v int) { presentCalled = true },
		func() { absentCalled = true },
	)
	if !presentCalled {
		t.Error("present callback should be called")
	}
	if absentCalled {
		t.Error("absent callback should not be called")
	}
}

func TestIfPresentOrElse_Absent(t *testing.T) {
	opt := None[int]()
	presentCalled := false
	absentCalled := false
	opt.IfPresentOrElse(
		func(v int) { presentCalled = true },
		func() { absentCalled = true },
	)
	if presentCalled {
		t.Error("present callback should not be called")
	}
	if !absentCalled {
		t.Error("absent callback should be called")
	}
}

// Scan tests

func TestScan_Nil(t *testing.T) {
	var opt Optional[string]
	err := opt.Scan(nil)
	if err != nil {
		t.Errorf("Scan(nil) error = %v", err)
	}
	if opt.IsPresent() {
		t.Error("Scan(nil) should set absent")
	}
}

func TestScan_DirectType(t *testing.T) {
	var opt Optional[string]
	err := opt.Scan("hello")
	if err != nil {
		t.Errorf("Scan(string) error = %v", err)
	}
	if !opt.IsPresent() {
		t.Fatal("should be present")
	}
	if opt.Get() != "hello" {
		t.Errorf("Get() = %q, want %q", opt.Get(), "hello")
	}
}

func TestScan_StringFromBytes(t *testing.T) {
	var opt Optional[string]
	err := opt.Scan([]byte("hello"))
	if err != nil {
		t.Errorf("Scan([]byte) error = %v", err)
	}
	if !opt.IsPresent() || opt.Get() != "hello" {
		t.Errorf("expected present 'hello', got present=%v val=%v", opt.IsPresent(), opt.GetOrZero())
	}
}

func TestScan_IntFromInt64(t *testing.T) {
	var opt Optional[int]
	err := opt.Scan(int64(42))
	if err != nil {
		t.Errorf("Scan(int64) error = %v", err)
	}
	if !opt.IsPresent() || opt.Get() != 42 {
		t.Errorf("expected present 42, got present=%v val=%v", opt.IsPresent(), opt.GetOrZero())
	}
}

func TestScan_Int64(t *testing.T) {
	var opt Optional[int64]
	err := opt.Scan(int64(100))
	if err != nil {
		t.Errorf("Scan(int64) error = %v", err)
	}
	if !opt.IsPresent() || opt.Get() != 100 {
		t.Errorf("expected present 100")
	}
}

func TestScan_Int64FromInt(t *testing.T) {
	var opt Optional[int64]
	err := opt.Scan(int(50))
	if err != nil {
		t.Errorf("Scan(int) error = %v", err)
	}
	if !opt.IsPresent() || opt.Get() != 50 {
		t.Errorf("expected present 50")
	}
}

func TestScan_Bool(t *testing.T) {
	var opt Optional[bool]
	err := opt.Scan(true)
	if err != nil {
		t.Errorf("Scan(bool) error = %v", err)
	}
	if !opt.IsPresent() || !opt.Get() {
		t.Error("expected present true")
	}
}

func TestScan_BoolFromInt64(t *testing.T) {
	var opt Optional[bool]
	err := opt.Scan(int64(1))
	if err != nil {
		t.Errorf("Scan(int64 for bool) error = %v", err)
	}
	if !opt.IsPresent() || !opt.Get() {
		t.Error("expected present true")
	}
}

func TestScan_Float64(t *testing.T) {
	var opt Optional[float64]
	err := opt.Scan(float64(3.14))
	if err != nil {
		t.Errorf("Scan(float64) error = %v", err)
	}
	if !opt.IsPresent() || opt.Get() != 3.14 {
		t.Error("expected present 3.14")
	}
}

func TestScan_Float64FromFloat32(t *testing.T) {
	var opt Optional[float64]
	err := opt.Scan(float32(2.5))
	if err != nil {
		t.Errorf("Scan(float32) error = %v", err)
	}
	if !opt.IsPresent() {
		t.Error("expected present")
	}
}

func TestScan_UnsupportedType(t *testing.T) {
	var opt Optional[struct{}]
	err := opt.Scan("unsupported")
	if err == nil {
		t.Error("Scan with unsupported type should return error")
	}
}

// Value tests

func TestValue_Present(t *testing.T) {
	opt := Some("hello")
	val, err := opt.Value()
	if err != nil {
		t.Errorf("Value() error = %v", err)
	}
	if val != "hello" {
		t.Errorf("Value() = %v, want %q", val, "hello")
	}
}

func TestValue_Absent(t *testing.T) {
	opt := None[string]()
	val, err := opt.Value()
	if err != nil {
		t.Errorf("Value() error = %v", err)
	}
	if val != nil {
		t.Errorf("Value() = %v, want nil", val)
	}
}

func TestValue_Int(t *testing.T) {
	opt := Some(42)
	val, err := opt.Value()
	if err != nil {
		t.Errorf("Value() error = %v", err)
	}
	if val != 42 {
		t.Errorf("Value() = %v, want 42", val)
	}
}

// JSON tests

func TestMarshalJSON_Present(t *testing.T) {
	opt := Some("hello")
	data, err := json.Marshal(opt)
	if err != nil {
		t.Fatalf("MarshalJSON error = %v", err)
	}
	if string(data) != `"hello"` {
		t.Errorf("MarshalJSON = %s, want %q", data, `"hello"`)
	}
}

func TestMarshalJSON_Absent(t *testing.T) {
	opt := None[string]()
	data, err := json.Marshal(opt)
	if err != nil {
		t.Fatalf("MarshalJSON error = %v", err)
	}
	if string(data) != "null" {
		t.Errorf("MarshalJSON = %s, want null", data)
	}
}

func TestMarshalJSON_Int(t *testing.T) {
	opt := Some(42)
	data, err := json.Marshal(opt)
	if err != nil {
		t.Fatalf("MarshalJSON error = %v", err)
	}
	if string(data) != "42" {
		t.Errorf("MarshalJSON = %s, want 42", data)
	}
}

func TestUnmarshalJSON_Present(t *testing.T) {
	var opt Optional[string]
	err := json.Unmarshal([]byte(`"hello"`), &opt)
	if err != nil {
		t.Fatalf("UnmarshalJSON error = %v", err)
	}
	if !opt.IsPresent() || opt.Get() != "hello" {
		t.Errorf("expected present 'hello'")
	}
}

func TestUnmarshalJSON_Null(t *testing.T) {
	var opt Optional[string]
	err := json.Unmarshal([]byte("null"), &opt)
	if err != nil {
		t.Fatalf("UnmarshalJSON error = %v", err)
	}
	if opt.IsPresent() {
		t.Error("UnmarshalJSON(null) should set absent")
	}
}

func TestUnmarshalJSON_Int(t *testing.T) {
	var opt Optional[int]
	err := json.Unmarshal([]byte("42"), &opt)
	if err != nil {
		t.Fatalf("UnmarshalJSON error = %v", err)
	}
	if !opt.IsPresent() || opt.Get() != 42 {
		t.Error("expected present 42")
	}
}

func TestJSON_Struct(t *testing.T) {
	type TestStruct struct {
		Name  string           `json:"name"`
		Email Optional[string] `json:"email"`
		Age   Optional[int]    `json:"age"`
	}

	// Marshal with present values
	s := TestStruct{
		Name:  "Alice",
		Email: Some("alice@test.com"),
		Age:   Some(30),
	}
	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// Unmarshal back
	var decoded TestStruct
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded.Name != "Alice" {
		t.Errorf("Name = %q, want Alice", decoded.Name)
	}
	if !decoded.Email.IsPresent() || decoded.Email.Get() != "alice@test.com" {
		t.Error("Email should be present with alice@test.com")
	}
	if !decoded.Age.IsPresent() || decoded.Age.Get() != 30 {
		t.Error("Age should be present with 30")
	}

	// Marshal with absent values
	s2 := TestStruct{
		Name:  "Bob",
		Email: None[string](),
		Age:   None[int](),
	}
	data2, err := json.Marshal(s2)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded2 TestStruct
	err = json.Unmarshal(data2, &decoded2)
	if err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if decoded2.Email.IsPresent() {
		t.Error("Email should be absent")
	}
	if decoded2.Age.IsPresent() {
		t.Error("Age should be absent")
	}
}

// Helper conversion function tests

func TestConvertToString(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    string
		wantErr bool
	}{
		{"string", "hello", "hello", false},
		{"bytes", []byte("world"), "world", false},
		{"int", 42, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToString(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConvertToInt(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    int
		wantErr bool
	}{
		{"int", int(42), 42, false},
		{"int64", int64(100), 100, false},
		{"int32", int32(50), 50, false},
		{"bytes", []byte("123"), 123, false},
		{"string", "bad", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToInt(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConvertToInt64(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    int64
		wantErr bool
	}{
		{"int64", int64(100), 100, false},
		{"int", int(50), 50, false},
		{"int32", int32(25), 25, false},
		{"bytes", []byte("999"), 999, false},
		{"string", "bad", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToInt64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestConvertToBool(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    bool
		wantErr bool
	}{
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"int64 nonzero", int64(1), true, false},
		{"int64 zero", int64(0), false, false},
		{"bytes true", []byte("true"), true, false},
		{"string", "bad", false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToBool(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToFloat64(t *testing.T) {
	tests := []struct {
		name    string
		input   interface{}
		want    float64
		wantErr bool
	}{
		{"float64", float64(3.14), 3.14, false},
		{"float32", float32(2.5), 2.5, false},
		{"bytes", []byte("1.5"), 1.5, false},
		{"string", "bad", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToFloat64(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("got = %f, want %f", got, tt.want)
			}
		})
	}
}

// customValuer implements driver.Valuer for testing Optional.Value with valuer types.
type customValuer struct {
	val string
}

func (c customValuer) Value() (driver.Value, error) {
	return c.val, nil
}

func TestValue_WithDriverValuer(t *testing.T) {
	opt := Some(customValuer{val: "hello"})
	v, err := opt.Value()
	if err != nil {
		t.Errorf("Value() error = %v", err)
	}
	if v != "hello" {
		t.Errorf("Value() = %v, want %q", v, "hello")
	}
}

// customScanner implements sql.Scanner for testing Optional.Scan with scanner types.
type customScanner struct {
	val string
}

func (c *customScanner) Scan(src interface{}) error {
	switch v := src.(type) {
	case string:
		c.val = v
		return nil
	case []byte:
		c.val = string(v)
		return nil
	default:
		return fmt.Errorf("unsupported type %T", src)
	}
}

func TestScan_WithSQLScanner(t *testing.T) {
	var opt Optional[customScanner]
	err := opt.Scan("hello")
	if err != nil {
		t.Errorf("Scan() error = %v", err)
	}
	if !opt.IsPresent() {
		t.Error("should be present")
	}
}

func TestScan_UnsupportedConversion(t *testing.T) {
	// Use a type that is not string/int/int64/bool/float64 and doesn't implement Scanner
	type customType struct{ x int }
	var opt Optional[customType]
	err := opt.Scan("hello")
	if err == nil {
		t.Error("should return error for unsupported conversion")
	}
}

func TestScan_StringConversionError(t *testing.T) {
	var opt Optional[string]
	// Scan with a type that can't convert to string (non-bytes, non-string, non-int)
	err := opt.Scan(struct{}{})
	if err == nil {
		t.Error("should return error for unsupported conversion to string")
	}
}

func TestScan_IntConversionError(t *testing.T) {
	var opt Optional[int]
	err := opt.Scan("not_a_number")
	if err == nil {
		t.Error("should return error for invalid int conversion")
	}
}

func TestScan_Int64ConversionError(t *testing.T) {
	var opt Optional[int64]
	err := opt.Scan("not_a_number")
	if err == nil {
		t.Error("should return error for invalid int64 conversion")
	}
}

func TestScan_BoolConversionError(t *testing.T) {
	var opt Optional[bool]
	err := opt.Scan("not_a_bool")
	if err == nil {
		t.Error("should return error for invalid bool conversion")
	}
}

func TestScan_Float64ConversionError(t *testing.T) {
	var opt Optional[float64]
	err := opt.Scan("not_a_float")
	if err == nil {
		t.Error("should return error for invalid float64 conversion")
	}
}

func TestConvertToString_Unsupported(t *testing.T) {
	_, err := convertToString(123.456) // float64 is not directly supported
	if err == nil {
		t.Error("should return error for unsupported type")
	}
}

func TestConvertToInt_Unsupported(t *testing.T) {
	_, err := convertToInt(true) // bool is not directly supported for int conversion
	if err == nil {
		t.Error("should return error for unsupported type")
	}
}

func TestConvertToInt64_Unsupported(t *testing.T) {
	_, err := convertToInt64(true) // bool is not directly supported for int64 conversion
	if err == nil {
		t.Error("should return error for unsupported type")
	}
}

func TestConvertToBool_Unsupported(t *testing.T) {
	_, err := convertToBool(123.456) // float64 is not directly supported for bool conversion
	if err == nil {
		t.Error("should return error for unsupported type")
	}
}

func TestConvertToFloat64_Unsupported(t *testing.T) {
	_, err := convertToFloat64(true) // bool is not directly supported for float64 conversion
	if err == nil {
		t.Error("should return error for unsupported type")
	}
}
