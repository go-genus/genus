# Generated Scanner Example

Demonstrates zero-reflection scanning using code-generated scanner functions.

## What it demonstrates

- Model definitions for code generation (`models.go`)
- Generated scanner code (`scanners.gen.go`) that eliminates runtime reflection
- Benchmark tests comparing generated scanners vs reflection-based scanning

## Files

| File | Description |
|------|-------------|
| `models.go` | Model definitions (User, Post, Comment) |
| `scanners.gen.go` | Generated scanner functions |
| `scanner_test.go` | Tests for generated scanners |
| `scanner_bench_test.go` | Benchmarks: generated vs reflection |

## Running benchmarks

```bash
go test -bench=. -benchmem ./examples/scanner/
```

## Generating scanners

```bash
genus generate --input=examples/scanner/models.go --output=examples/scanner/scanners.gen.go
```
