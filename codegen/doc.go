// Package codegen generates type-safe Go code from struct definitions.
//
// [Generator] parses Go source files using AST, extracts struct field
// information from db struct tags, and produces typed field definitions
// and scanner functions that eliminate runtime reflection.
package codegen
