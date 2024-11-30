package sqlite

import (
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"sort"
	"strings"

	_ "modernc.org/sqlite"
)

func Connect(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not open database: %w", err)
	}

	err = db.Ping()
	if err != nil {
		db.Close() // Make sure to close the database if the ping fails
		return nil, fmt.Errorf("could not ping database: %w", err)
	}

	return db, nil
}

func MigrateUp(db *sql.DB, migrations embed.FS) error {
	if err := createSchemaVersionTable(db); err != nil {
		return fmt.Errorf("could not create schema_version table: %w", err)
	}

	files, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return fmt.Errorf("failed to read migrations directory: %w", err)
	}

	// Sort migrations by filename
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name() < files[j].Name()
	})

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".sql") {
			continue
		}

		applied, err := isMigrationApplied(db, f.Name())
		if err != nil {
			return fmt.Errorf("could not check migration status: %w", err)
		}

		if applied {
			log.Printf("DEBUG skip migration: %s", f.Name())
			continue
		}

		log.Printf("INFO applying migration: %s", f.Name())
		content, err := migrations.ReadFile(f.Name())
		if err != nil {
			return fmt.Errorf("could not read migration %s: %w", f.Name(), err)
		}

		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("could not start transaction: %w", err)
		}

		_, err = tx.Exec(string(content))
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("could not execute migration %s: %w", f.Name(), err)
		}

		err = recordMigration(tx, f.Name())
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("could not record migration %s: %w", f.Name(), err)
		}

		err = tx.Commit()
		if err != nil {
			return fmt.Errorf("could not commit transaction: %w", err)
		}
	}
	return nil
}

func createSchemaVersionTable(db *sql.DB) error {
	const query = `
	CREATE TABLE IF NOT EXISTS schema_version (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := db.Exec(query)
	return err
}

func isMigrationApplied(db *sql.DB, version string) (bool, error) {
	version = strings.TrimSuffix(version, ".sql")
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM schema_version WHERE version = ?", version).Scan(&count)
	return count > 0, err
}

func recordMigration(tx *sql.Tx, version string) error {
	version = strings.TrimSuffix(version, ".sql")
	_, err := tx.Exec("INSERT INTO schema_version (version) VALUES (?)", version)
	return err
}
