// Package main provides the entry point for the Spotube server application.
package main

import (
	"log"
	"os"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	// Import migrations to register them
	"github.com/manlikeabro/spotube/internal/pbext/googleauth"
	"github.com/manlikeabro/spotube/internal/pbext/mappings"
	"github.com/manlikeabro/spotube/internal/pbext/setupwizard"
	"github.com/manlikeabro/spotube/internal/pbext/spotifyauth"
	_ "github.com/manlikeabro/spotube/migrations"
)

func main() {
	app := pocketbase.New()

	// Register setup wizard routes and hooks
	setupwizard.Register(app)
	setupwizard.RegisterHooks(app)

	// Register Spotify auth routes
	spotifyauth.Register(app)

	// Register Google auth routes
	googleauth.Register(app)

	// Register mappings hooks
	mappings.RegisterHooks(app)

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
