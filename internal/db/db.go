package db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

var ErrPathTraversal = errors.New("path traversal detected")

type DB struct {
	*sql.DB
	emailsPath string // Root path for resolving relative .eml file paths
}

// Open opens a connection to the SQLite database and initializes the schema
func Open(dbPath string) (*DB, error) {
	// Ensure the directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open SQLite database with time format pragma
	// The _time_format=sqlite parameter tells the driver to parse RFC3339 timestamps
	dsn := dbPath + "?_time_format=sqlite"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxOpenConns(1) // SQLite works best with single connection
	sqlDB.SetMaxIdleConns(1)

	db := &DB{
		DB: sqlDB,
		// emailsPath will be set via SetEmailsPath after opening
	}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// SetEmailsPath sets the root path for resolving relative .eml file paths
// This must be called after opening the database for GetEmailWithFullContent to work
func (db *DB) SetEmailsPath(path string) {
	db.emailsPath = path
}

// GetEmailsPath returns the configured emails root path
func (db *DB) GetEmailsPath() string {
	return db.emailsPath
}

// ResolveEmailPath converts a relative .eml file path to an absolute path
// Returns an error if the path attempts traversal outside the emails directory
func (db *DB) ResolveEmailPath(relativePath string) (string, error) {
	// Reject absolute paths for security
	if filepath.IsAbs(relativePath) {
		return "", ErrPathTraversal
	}

	// Clean the path to remove . and ..
	cleaned := filepath.Clean(relativePath)

	// Check for path traversal attempts after cleaning
	if strings.Contains(cleaned, "..") {
		return "", ErrPathTraversal
	}

	// Get absolute path of emails directory
	absEmailsPath, err := filepath.Abs(db.emailsPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute emails path: %w", err)
	}

	// Resolve the file path
	resolved := filepath.Join(absEmailsPath, cleaned)

	// Canonicalize the resolved path
	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to canonicalize path: %w", err)
	}

	// Verify the resolved path is within emailsPath boundary
	// Must either be a child of emailsPath or be emailsPath itself
	if !strings.HasPrefix(absResolved, absEmailsPath+string(filepath.Separator)) &&
		absResolved != absEmailsPath {
		return "", ErrPathTraversal
	}

	return absResolved, nil
}

// initSchema creates all tables, indexes, and triggers
func (db *DB) initSchema() error {
	_, err := db.Exec(schema)
	if err != nil {
		return fmt.Errorf("failed to execute schema: %w", err)
	}
	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	return db.DB.Close()
}

// GetSetting retrieves a setting value by key
func (db *DB) GetSetting(key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get setting: %w", err)
	}
	return value, nil
}

// SetSetting sets or updates a setting
func (db *DB) SetSetting(key, value string) error {
	_, err := db.Exec(`
		INSERT INTO settings (key, value, updated_at)
		VALUES (?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(key) DO UPDATE SET value = ?, updated_at = CURRENT_TIMESTAMP
	`, key, value, value)
	if err != nil {
		return fmt.Errorf("failed to set setting: %w", err)
	}
	return nil
}

// Vacuum reclaims unused space in the database
// This should be run after deleting emails or running migrations
func (db *DB) Vacuum() error {
	_, err := db.Exec("VACUUM")
	if err != nil {
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	return nil
}

// Analyze updates database statistics for better query planning
func (db *DB) Analyze() error {
	_, err := db.Exec("ANALYZE")
	if err != nil {
		return fmt.Errorf("failed to analyze database: %w", err)
	}
	return nil
}

// GetDatabaseSize returns the database file size in bytes
func (db *DB) GetDatabaseSize() (int64, error) {
	var pageCount, pageSize int64
	err := db.QueryRow("PRAGMA page_count").Scan(&pageCount)
	if err != nil {
		return 0, fmt.Errorf("failed to get page count: %w", err)
	}

	err = db.QueryRow("PRAGMA page_size").Scan(&pageSize)
	if err != nil {
		return 0, fmt.Errorf("failed to get page size: %w", err)
	}

	return pageCount * pageSize, nil
}

// MigrateToOptimizedSchema migrates an existing database to the optimized schema
// This removes duplicate data (body_html, raw_headers, cc, bcc, attachment BLOBs)
func (db *DB) MigrateToOptimizedSchema() error {
	// Check if migration is needed by checking if old columns exist
	var hasBodyHTML bool
	err := db.QueryRow(`
		SELECT COUNT(*) > 0
		FROM pragma_table_info('emails')
		WHERE name = 'body_html'
	`).Scan(&hasBodyHTML)
	if err != nil {
		return fmt.Errorf("failed to check schema: %w", err)
	}

	if !hasBodyHTML {
		// Already migrated
		return nil
	}

	// Run migration
	_, err = db.Exec(migrationSchema)
	if err != nil {
		return fmt.Errorf("failed to run migration: %w", err)
	}

	return nil
}
