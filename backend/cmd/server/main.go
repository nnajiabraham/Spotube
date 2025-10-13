// Package main provides the entry point for the Spotube server application.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/manlikeabro/spotube/internal/config"
	"github.com/manlikeabro/spotube/internal/handlers"
	"github.com/manlikeabro/spotube/internal/httpserver"
	"github.com/manlikeabro/spotube/internal/logging"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	logger := logging.Init(cfg.AppEnv, cfg.LogLevel, cfg.Version)

	db, err := sqliteconn.OpenWithPragmas(cfg.DBPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to open database")
	}
	defer db.Close()

	srv := httpserver.New(cfg, logger)

	handlers.RegisterHealth(srv, &handlers.HealthHandler{
		DB:     db,
		Logger: logger,
		Config: cfg,
	})

	setupHandler := handlers.NewSetupHandler(db)
	apiGroup := srv.Group("/api/setup")
	handlers.RegisterSetupRoutes(apiGroup, setupHandler)

	address := ":" + cfg.Port
	go func() {
		logger.Info().Str("addr", address).Msg("starting server")
		if err := srv.Start(address); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("server error")
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("server shutdown failed")
	}

	logger.Info().Msg("server stopped")
}
