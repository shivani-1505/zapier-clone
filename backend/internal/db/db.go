package db

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/auditcue/integration-framework/internal/config"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3" // Keep SQLite for backwards compatibility
)

// Database represents a database connection
type Database struct {
	db     *sql.DB
	config config.DatabaseConfig
}

// NewDatabase creates a new database connection
func NewDatabase(cfg config.DatabaseConfig) (*Database, error) {
	var db *sql.DB
	var err error

	switch cfg.Driver {
	case "sqlite":
		db, err = sql.Open("sqlite3", cfg.FilePath)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to SQLite database: %w", err)
		}

		// Enable foreign keys in SQLite
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
		}

	case "postgres":
		connStr := fmt.Sprintf(
			"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Name, cfg.SSLMode,
		)
		db, err = sql.Open("postgres", connStr)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to PostgreSQL database: %w", err)
		}

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", cfg.Driver)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Database{
		db:     db,
		config: cfg,
	}, nil
}

// Close closes the database connection
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}
	return nil
}

// Migrate runs database migrations
func (d *Database) Migrate() error {
	var migrationPath string

	// Select the appropriate migration file based on database driver
	if d.config.Driver == "postgres" {
		migrationPath = "internal/db/migrations/init_postgres.sql"
	} else {
		migrationPath = "internal/db/migrations/init.sql"
	}

	// Read migration SQL file
	migrationSQL, err := ioutil.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("failed to read migration file: %w", err)
	}

	// Split the migration into separate statements
	var statements []string

	if d.config.Driver == "postgres" {
		// PostgreSQL uses $ for function declarations, so we need a more sophisticated split
		statements = splitPostgresStatements(string(migrationSQL))
	} else {
		// SQLite can use simple semicolon splitting
		statements = strings.Split(string(migrationSQL), ";")
	}

	// Begin a transaction
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute each statement
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		if _, err := tx.Exec(statement); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// splitPostgresStatements splits PostgreSQL statements that may contain function definitions
// with $ delimiters which can contain semicolons
func splitPostgresStatements(sql string) []string {
	var statements []string
	var currentStatement strings.Builder
	inFunctionBody := false

	lines := strings.Split(sql, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// Skip comments and empty lines
		if strings.HasPrefix(trimmedLine, "--") || trimmedLine == "" {
			continue
		}

		// Check for function body delimiters
		if strings.Contains(trimmedLine, "$") {
			inFunctionBody = !inFunctionBody
			currentStatement.WriteString(line + "\n")
			continue
		}

		// If we're in a function body, just add the line
		if inFunctionBody {
			currentStatement.WriteString(line + "\n")
			continue
		}

		// If line ends with semicolon and we're not in function body, it's end of statement
		if strings.HasSuffix(trimmedLine, ";") && !inFunctionBody {
			currentStatement.WriteString(line + "\n")
			statements = append(statements, currentStatement.String())
			currentStatement.Reset()
			continue
		}

		// Otherwise, just add the line
		currentStatement.WriteString(line + "\n")
	}

	// Add any remaining statement
	if currentStatement.Len() > 0 {
		statements = append(statements, currentStatement.String())
	}

	return statements
}

// DB returns the underlying database connection
func (d *Database) DB() *sql.DB {
	return d.db
}
