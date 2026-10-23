package migrate

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

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

// Pending reports whether migrations exist that have not been applied yet.
func Pending(db *sql.DB) (bool, error) {
	dir, err := migrationsDir()
	if err != nil {
		return false, err
	}
	goose.SetDialect("sqlite3")
	goose.SetBaseFS(nil)
	current, err := goose.GetDBVersion(db)
	if err != nil {
		return false, err
	}
	migrations, err := goose.CollectMigrations(dir, 0, goose.MaxVersion)
	if err != nil {
		return false, err
	}
	last, err := migrations.Last()
	if err != nil {
		return false, nil // no migrations at all
	}
	return last.Version > current, nil
}

// keepBackups is how many pre-migration backups are kept next to the database.
const keepBackups = 10

// Backup writes a consistent copy of the database (WAL included) to
// <db dir>/backups/<name>-<timestamp>-<label>.db and prunes all but the newest keepBackups.
func Backup(db *sql.DB, dbPath, label string) (string, error) {
	dir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(filepath.Base(dbPath), filepath.Ext(dbPath))
	path := filepath.Join(dir, fmt.Sprintf("%s-%s-%s.db", base, time.Now().Format("20060102-150405"), label))
	if _, err := db.Exec(`VACUUM INTO ?`, path); err != nil {
		return "", fmt.Errorf("backup database: %w", err)
	}
	old, _ := filepath.Glob(filepath.Join(dir, base+"-*.db"))
	sort.Strings(old) // timestamped names sort oldest first
	for len(old) > keepBackups {
		_ = os.Remove(old[0])
		old = old[1:]
	}
	return path, nil
}
