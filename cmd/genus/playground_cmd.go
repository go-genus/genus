package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/GabrielOnRails/genus/playground"

	_ "github.com/lib/pq"
)

func runPlayground() error {
	args := os.Args[2:]

	// Parse flags
	var (
		dsn      = os.Getenv("DATABASE_URL")
		driver   = "postgres"
		port     = 8765
		readOnly = false
	)

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--dsn":
			if i+1 < len(args) {
				dsn = args[i+1]
				i++
			}
		case "--driver":
			if i+1 < len(args) {
				driver = args[i+1]
				i++
			}
		case "--port":
			if i+1 < len(args) {
				port, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--read-only":
			readOnly = true
		case "-h", "--help":
			printPlaygroundUsage()
			return nil
		}
	}

	if dsn == "" {
		return fmt.Errorf("database connection string required (--dsn or DATABASE_URL)")
	}

	// Conecta ao banco
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer db.Close()

	// Testa conexão
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Cria e inicia o servidor
	server := playground.NewPlaygroundServer(playground.PlaygroundConfig{
		Port:         port,
		DB:           db,
		Driver:       driver,
		ReadOnly:     readOnly,
		MaxQueryTime: 30 * time.Second,
		MaxResults:   1000,
	})

	return server.Start()
}

func printPlaygroundUsage() {
	fmt.Println(`Web-based query playground

Usage:
  genus playground [flags]

Flags:
  --dsn <string>       Database connection string (or use DATABASE_URL env)
  --driver <string>    Database driver: postgres, mysql, sqlite3 (default: postgres)
  --port <int>         Server port (default: 8765)
  --read-only          Only allow SELECT queries
  -h, --help           Show this help message

Examples:
  genus playground --dsn "postgres://user:pass@localhost/db"
  DATABASE_URL="postgres://..." genus playground --port 3000
  genus playground --read-only --dsn "..."

Features:
  - Execute SQL queries with syntax highlighting
  - View database schema and table structures
  - Export results as CSV or JSON
  - Query history and explain plans
  - Real-time query timing`)
}
