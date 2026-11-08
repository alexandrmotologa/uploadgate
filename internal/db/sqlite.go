package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// OpenDatabase initializes SQLite with WAL mode and runs migrations.
func OpenDatabase(dbPath string) (*sql.DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create db directory %q: %w", dir, err)
		}
	}

	// SQLite connection string with busy timeout and WAL mode
	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	// Configure pool limits for low memory and concurrent thread safety
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("database migrations failed: %w", err)
	}

	return db, nil
}

func runMigrations(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS uploads (
		id TEXT PRIMARY KEY,
		filename TEXT NOT NULL,
		original_size INTEGER NOT NULL,
		declared_mime TEXT NOT NULL,
		detected_mime TEXT NOT NULL DEFAULT '',
		storage_provider TEXT NOT NULL,
		storage_bucket TEXT NOT NULL DEFAULT '',
		storage_key TEXT NOT NULL,
		public_url TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		sha256 TEXT NOT NULL DEFAULT '',
		width INTEGER NOT NULL DEFAULT 0,
		height INTEGER NOT NULL DEFAULT 0,
		duration_seconds REAL NOT NULL DEFAULT 0.0,
		error_reason TEXT NOT NULL DEFAULT '',
		metadata_json TEXT NOT NULL DEFAULT '{}',
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_uploads_status ON uploads(status);
	CREATE INDEX IF NOT EXISTS idx_uploads_created_at ON uploads(created_at DESC);

	CREATE TABLE IF NOT EXISTS thumbnails (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		upload_id TEXT NOT NULL REFERENCES uploads(id) ON DELETE CASCADE,
		variant_name TEXT NOT NULL,
		format TEXT NOT NULL,
		width INTEGER NOT NULL,
		height INTEGER NOT NULL,
		storage_key TEXT NOT NULL,
		public_url TEXT NOT NULL,
		size_bytes INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_thumbnails_upload_id ON thumbnails(upload_id);

	CREATE TABLE IF NOT EXISTS webhook_deliveries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		upload_id TEXT NOT NULL,
		event TEXT NOT NULL,
		endpoint TEXT NOT NULL,
		payload TEXT NOT NULL,
		status_code INTEGER NOT NULL DEFAULT 0,
		attempt INTEGER NOT NULL DEFAULT 1,
		response_body TEXT NOT NULL DEFAULT '',
		error_message TEXT NOT NULL DEFAULT '',
		delivered_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_webhooks_upload_id ON webhook_deliveries(upload_id);
	CREATE INDEX IF NOT EXISTS idx_webhooks_created_at ON webhook_deliveries(created_at DESC);
	`

	_, err := db.Exec(schema)
	return err
}
