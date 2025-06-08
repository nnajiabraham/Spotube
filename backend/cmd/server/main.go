// Package main provides the entry point for the Spotube server application.
package main

import (
	"fmt"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	// Initialize zerolog with pretty output for development
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	fmt.Println("hello world")
	log.Info().Msg("Spotube server - minimal scaffold")

	// For development, run a simple HTTP server instead of exiting
	if len(os.Args) > 1 && os.Args[1] == "serve" {
		log.Info().Msg("Starting HTTP server on :8090")
		// Simple placeholder server - will be replaced in RFC-002
		select {} // Block forever for now
	}

	// Exit with 0 to satisfy CI requirements when not in serve mode
	os.Exit(0)
}
