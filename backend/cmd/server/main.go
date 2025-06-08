// Package main provides the entry point for the Spotube server application.
package main

import (
	"log"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	// Import migrations to register them
	_ "github.com/manlikeabro/spotube/pb_migrations"
)

func main() {
	app := pocketbase.New()

	// Register `pb migrate` sub-command so we can run `go run ./cmd/server migrate up`.
	isGoRun := strings.HasPrefix(os.Args[0], os.TempDir())
	migratecmd.MustRegister(app, app.RootCmd, migratecmd.Config{
		Automigrate: isGoRun, // Dev: auto-generate migrations when using Admin UI
	})

	// Serve PocketBase (defaults to :8090) – production port defined via ENV PORT.
	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
