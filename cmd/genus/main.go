package main

import (
	"fmt"
	"os"

	"github.com/GabrielOnRails/genus/codegen"
)

const version = "1.0.0"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "generate":
		if err := runGenerate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "migrate":
		if err := runMigrate(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "repl":
		if err := runREPL(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "playground":
		if err := runPlayground(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "version":
		fmt.Printf("genus version %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func runGenerate() error {
	args := os.Args[2:]

	// Parse flags
	var (
		outputDir = "."
		pkgName   = ""
		paths     []string
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]

		if arg == "-o" || arg == "--output" {
			if i+1 >= len(args) {
				return fmt.Errorf("flag %s requires a value", arg)
			}
			outputDir = args[i+1]
			i++
		} else if arg == "-p" || arg == "--package" {
			if i+1 >= len(args) {
				return fmt.Errorf("flag %s requires a value", arg)
			}
			pkgName = args[i+1]
			i++
		} else if arg == "-h" || arg == "--help" {
			printGenerateUsage()
			return nil
		} else {
			paths = append(paths, arg)
		}
	}

	if len(paths) == 0 {
		paths = []string{"."}
	}

	generator := codegen.NewGenerator(codegen.Config{
		OutputDir:   outputDir,
		PackageName: pkgName,
	})

	fmt.Printf("Generating field definitions...\n")

	for _, path := range paths {
		fmt.Printf("Processing: %s\n", path)
		if err := generator.GenerateFromPath(path); err != nil {
			return fmt.Errorf("failed to generate from %s: %w", path, err)
		}
	}

	fmt.Printf("Code generation completed successfully!\n")
	return nil
}

func printUsage() {
	fmt.Println(`Genus - Type-safe ORM for Go

Usage:
  genus <command> [arguments]

Commands:
  generate    Generate type-safe field definitions from Go structs
  migrate     Manage database migrations (up, down, status, create)
  repl        Interactive query builder REPL
  playground  Start web-based query playground
  version     Print version information
  help        Show this help message

Run 'genus <command> --help' for more information on a command.`)
}

func printGenerateUsage() {
	fmt.Println(`Generate type-safe field definitions from Go structs

Usage:
  genus generate [flags] [paths...]

Flags:
  -o, --output <dir>     Output directory for generated files (default: ".")
  -p, --package <name>   Package name for generated code (default: auto-detect)
  -h, --help             Show this help message

Examples:
  genus generate                    # Generate from current directory
  genus generate ./models           # Generate from models directory
  genus generate -o ./generated     # Generate to specific output directory
  genus generate -p mypackage ./models  # Generate with custom package name

The generator will:
1. Scan Go files for structs with 'db' tags
2. Generate type-safe field definitions (e.g., UserFields, ProductFields)
3. Save generated code to *_fields.gen.go files`)
}
