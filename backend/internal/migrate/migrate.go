package migrate

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/pressly/goose/v3"
)

var (
	dirOnce   sync.Once
	cachedDir string
	dirErr    error
)

func migrationsDir() (string, error) {
	dirOnce.Do(func() {
		_, currentFile, _, ok := runtime.Caller(0)
		if !ok {
			dirErr = fmt.Errorf("unable to resolve migrations directory")
			return
		}
		backendDir := filepath.Dir(filepath.Dir(currentFile))
		cachedDir = filepath.Join(filepath.Dir(backendDir), "migrations")
	})
	return cachedDir, dirErr
}

// Up applies all pending migrations to the provided database.
func Up(db *sql.DB) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	goose.SetDialect("sqlite3")
	goose.SetBaseFS(nil)
	return goose.Up(db, dir)
}

// Down rolls back a single migration.
func Down(db *sql.DB) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	goose.SetDialect("sqlite3")
	goose.SetBaseFS(nil)
	return goose.Down(db, dir)
}
