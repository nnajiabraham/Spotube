package main

import (
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/pressly/goose/v3"

	"github.com/manlikeabro/spotube/internal/migrate"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("usage: migrate [up|down|status|...]")
	}

	command := os.Args[1]

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/spotube.db"
	}

	db, err := sqliteconn.OpenWithPragmas(dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	goose.SetDialect("sqlite3")

	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		log.Fatalf("unable to determine migrate binary directory")
	}

	backendCmdDir := filepath.Dir(filepath.Dir(currentFile))
	backendDir := filepath.Dir(backendCmdDir)
	migrationsDir := filepath.Join(backendDir, "migrations")

	goose.SetDialect("sqlite3")

	switch command {
	case "up":
		if err := migrate.Up(db); err != nil {
			log.Fatalf("failed goose up: %v", err)
		}
	case "down":
		if err := migrate.Down(db); err != nil {
			log.Fatalf("failed goose down: %v", err)
		}
	default:
		if err := goose.Run(command, db, migrationsDir, os.Args[2:]...); err != nil {
			log.Fatalf("goose %s: %v", command, err)
		}
	}
}
