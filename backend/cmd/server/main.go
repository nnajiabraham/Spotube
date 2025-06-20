// Package main provides the entry point for the Spotube server application.
package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/plugins/migratecmd"

	// Import migrations to register them
	"github.com/manlikeabro/spotube/internal/jobs"
	"github.com/manlikeabro/spotube/internal/pbext/googleauth"
	"github.com/manlikeabro/spotube/internal/pbext/mappings"
	"github.com/manlikeabro/spotube/internal/pbext/setupwizard"
	"github.com/manlikeabro/spotube/internal/pbext/spotifyauth"
	_ "github.com/manlikeabro/spotube/migrations"
)

func main() {
	// Load .env file if it exists (for development convenience)
	// This loads environment variables from .env file in the current working directory
	// Production deployments should use actual environment variables
	if err := godotenv.Load(); err != nil {
		// Only log if the error is not "file not found" since .env is optional
		if !os.IsNotExist(err) {
			log.Printf("Warning: Error loading .env file: %v", err)
		}
	}

	fmt.Println("PUBLIC_URL", os.Getenv("PUBLIC_URL"))

	app := pocketbase.New()

	// Enable debug logging if LOG_LEVEL is set to debug
	if strings.ToLower(os.Getenv("LOG_LEVEL")) == "debug" {
		app.Settings().Logs.MaxDays = 7
		log.Println("Debug logging enabled")
	}

	// Register setup wizard routes and hooks
	setupwizard.Register(app)
	setupwizard.RegisterHooks(app)

	// Register Spotify auth routes
	spotifyauth.Register(app)

	// Register Google auth routes
	googleauth.Register(app)

	// Register mappings hooks
	mappings.RegisterHooks(app)

	// Register analysis job scheduler
	jobs.RegisterAnalysis(app)

	// Register executor job scheduler (RFC-008)
	jobs.RegisterExecutor(app)

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
