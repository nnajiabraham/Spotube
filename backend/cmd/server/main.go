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

	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"

	"github.com/manlikeabro/spotube/internal/auth"
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

	// Token repository
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	auth.SetTokenRepository(tokenRepo)

	// Session store
	sessionStore := sessions.NewCookieStore([]byte(cfg.SessionCookieName))
	sessionStore.Options.Path = "/"
	sessionStore.Options.MaxAge = cfg.SessionTTLSeconds
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = cfg.SessionSecure

	// Settings repository for OAuth handlers
	settingsRepo := &sqliteSettingsRepo{db: db}

	// Spotify OAuth
	spotifyHandler := handlers.NewSpotifyOAuthHandler(
		settingsRepo,
		tokenRepo,
		sessionStore,
		cfg.PublicURL+"/api/auth/spotify/callback",
	)
	spotifyGroup := srv.Group("/api/auth/spotify")
	handlers.RegisterSpotifyRoutes(spotifyGroup, spotifyHandler)

	// YouTube OAuth
	youtubeHandler := handlers.NewYouTubeOAuthHandler(
		settingsRepo,
		tokenRepo,
		sessionStore,
		cfg.PublicURL+"/api/auth/youtube/callback",
	)
	youtubeGroup := srv.Group("/api/auth/youtube")
	handlers.RegisterYouTubeRoutes(youtubeGroup, youtubeHandler)

	// Mappings CRUD
	mappingsHandler := handlers.NewMappingsHandler(db)
	mappingsGroup := srv.Group("/api/collections/mappings/records")
	handlers.RegisterMappingsRoutes(mappingsGroup, mappingsHandler)

	// Blacklist CRUD
	blacklistHandler := handlers.NewBlacklistHandler(db)
	blacklistGroup := srv.Group("/api/collections/blacklist/records")
	handlers.RegisterBlacklistRoutes(blacklistGroup, blacklistHandler)

	// Activity Logs (read-only)
	activityLogsHandler := handlers.NewActivityLogsHandler(db)
	activityLogsGroup := srv.Group("/api/collections/activity_logs/records")
	handlers.RegisterActivityLogsRoutes(activityLogsGroup, activityLogsHandler)

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
