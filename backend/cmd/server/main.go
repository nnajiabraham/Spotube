// Package main provides the entry point for the Spotube server application.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"database/sql"

	"github.com/gorilla/sessions"
	"github.com/joho/godotenv"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/apiusage"
	"github.com/manlikeabro/spotube/internal/auth"
	"github.com/manlikeabro/spotube/internal/backups"
	"github.com/manlikeabro/spotube/internal/config"
	"github.com/manlikeabro/spotube/internal/handlers"
	"github.com/manlikeabro/spotube/internal/httpserver"
	"github.com/manlikeabro/spotube/internal/jobs"
	"github.com/manlikeabro/spotube/internal/logging"
	"github.com/manlikeabro/spotube/internal/platform"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/settings"
	"github.com/manlikeabro/spotube/internal/sqliteconn"
)

// backupCheckInterval is how often due playlist backups are looked for; each backup has its
// own interval (24 hours by default).
const backupCheckInterval = 10 * time.Minute

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
	// Every YouTube API call is metered into api_usage so the dashboard shows real quota.
	apiusage.SetDB(db)

	// Runtime settings: .env wins at startup; the Settings page changes the rest (stored) or,
	// for env-pinned ones, until the next restart.
	appSettings, err := settings.Load(db, nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to load settings")
	}
	apiusage.SetLimit(func() int { return appSettings.Int(settings.YouTubeDailyQuota) })
	// The quota budget gates every YouTube call (see internal/quota).
	budget := quota.New(db,
		func() int { return appSettings.Int(settings.YouTubeDailyQuota) },
		func() int { return appSettings.Int(settings.YouTubeInteractiveReserve) },
		nil)
	apiusage.SetGate(budget)

	srv := httpserver.New(cfg, logger)

	handlers.RegisterHealth(srv, &handlers.HealthHandler{
		DB:     db,
		Logger: logger,
		Config: cfg,
	})

	setupDefaults := &handlers.SettingsRecord{
		SpotifyClientID:     nullableString(cfg.SpotifyClientID),
		SpotifyClientSecret: nullableString(cfg.SpotifyClientSecret),
		GoogleClientID:      nullableString(cfg.GoogleClientID),
		GoogleClientSecret:  nullableString(cfg.GoogleClientSecret),
	}
	setupHandler := handlers.NewSetupHandler(db, setupDefaults)
	apiGroup := srv.Group("/api/setup")
	handlers.RegisterSetupRoutes(apiGroup, setupHandler)

	// Token repository
	tokenRepo := auth.NewSQLiteTokenRepository(db)
	auth.SetTokenRepository(tokenRepo)

	// Session store
	sessionStore := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	sessionStore.Options.Path = "/"
	sessionStore.Options.MaxAge = cfg.SessionTTLSeconds
	sessionStore.Options.HttpOnly = true
	sessionStore.Options.Secure = cfg.SessionSecure
	sessionStore.Options.SameSite = http.SameSiteLaxMode

	// Settings repository for OAuth handlers (uses env defaults when DB empty)
	oauthFallback := &auth.SettingsRecord{
		SpotifyClientID:     nullableString(cfg.SpotifyClientID),
		SpotifyClientSecret: nullableString(cfg.SpotifyClientSecret),
		GoogleClientID:      nullableString(cfg.GoogleClientID),
		GoogleClientSecret:  nullableString(cfg.GoogleClientSecret),
	}
	settingsRepo := &sqliteSettingsRepo{db: db, fallback: oauthFallback}

	// Spotify OAuth
	spotifyHandler := handlers.NewSpotifyOAuthHandler(
		settingsRepo,
		tokenRepo,
		sessionStore,
		cfg.PublicURL+"/api/auth/spotify/callback",
		cfg.FrontendURL,
	)
	spotifyGroup := srv.Group("/api/auth/spotify")
	handlers.RegisterSpotifyRoutes(spotifyGroup, spotifyHandler)

	// YouTube OAuth
	youtubeHandler := handlers.NewYouTubeOAuthHandler(
		settingsRepo,
		tokenRepo,
		sessionStore,
		cfg.PublicURL+"/api/auth/youtube/callback",
		cfg.FrontendURL,
	)
	youtubeGroup := srv.Group("/api/auth/youtube")
	handlers.RegisterYouTubeRoutes(youtubeGroup, youtubeHandler)

	// Blacklist CRUD
	blacklistHandler := handlers.NewBlacklistHandler(db)
	blacklistGroup := srv.Group("/api/collections/blacklist/records")
	handlers.RegisterBlacklistRoutes(blacklistGroup, blacklistHandler)

	// Activity Logs (read-only)
	activityLogsHandler := handlers.NewActivityLogsHandler(db)
	activityLogsGroup := srv.Group("/api/collections/activity_logs/records")
	handlers.RegisterActivityLogsRoutes(activityLogsGroup, activityLogsHandler)

	// One sync runner shared by the scheduler, Sync now and new mappings, so per-mapping locks
	// and the playlist cache are shared too.
	activityLogger := activitylogger.New(db)
	clientFactory := auth.NewClientFactory(db, settingsRepo, tokenRepo)
	jobDeps := jobs.JobDeps{
		DB:             db,
		Logger:         logger,
		ActivityLogger: activityLogger,
		Cache:          jobs.NewPlaylistCache(db),
		Budget:         budget,
		Settings:       appSettings,
	}
	syncExecutor := jobs.NewExecutor(jobDeps, clientFactory)
	if n, err := syncExecutor.ResetInterrupted(context.Background()); err != nil {
		logger.Error().Err(err).Msg("failed to requeue interrupted items")
	} else if n > 0 {
		logger.Info().Int64("items", n).Msg("requeued items interrupted by the last shutdown")
	}
	runner := jobs.NewAnalysisJob(jobDeps, clientFactory)
	runner.EnableAutoApply(syncExecutor, 0)
	budget.DailyReads = cachedFor(time.Minute, runner.DailyReadUnits)

	mappingsHandler := handlers.NewMappingsHandler(db)
	mappingsHandler.Syncer = runner
	mappingsHandler.Playlists = &platform.Playlists{Factory: clientFactory}
	mappingsHandler.Budget = budget
	mappingsHandler.SyncNowDefault = func() bool { return appSettings.Bool(settings.SyncNewMappingsNow) }
	mappingsGroup := srv.Group("/api/collections/mappings/records")
	handlers.RegisterMappingsRoutes(mappingsGroup, mappingsHandler)

	handlers.RegisterQuotaRoutes(srv.Group("/api/quota"), &handlers.QuotaHandler{
		DB: db, Budget: budget, Spotify: &platform.Spotify{Factory: clientFactory},
	})
	handlers.RegisterAppSettingsRoutes(srv.Group("/api/settings"), &handlers.AppSettingsHandler{Store: appSettings})

	syncItemsHandler := handlers.NewSyncItemsHandler(db, syncExecutor)
	syncItemsGroup := srv.Group("/api/collections/sync_items/records")
	handlers.RegisterSyncItemsRoutes(syncItemsGroup, syncItemsHandler)

	// In-app track search for the review queue, and remembered track links
	tracksHandler := handlers.NewTracksHandler(db,
		&platform.Spotify{Factory: clientFactory}, &platform.YouTube{Factory: clientFactory})
	tracksHandler.Budget = budget
	handlers.RegisterTracksRoutes(srv.Group("/api"), tracksHandler)

	// Playlist backups: snapshots on their own schedule (and free ones during sync), restore
	backupService := &backups.Service{
		Store:     &backups.Store{DB: db},
		Reader:    &platform.Playlists{Factory: clientFactory},
		Writer:    &platform.Playlists{Factory: clientFactory},
		Activity:  activityLogger,
		Logger:    logger,
		QuotaLeft: func() int { return budget.Status().Available },
	}
	handlers.RegisterBackupsRoutes(srv.Group("/api/backups"), handlers.NewBackupsHandler(backupService))
	backupCtx, stopBackups := context.WithCancel(context.Background())
	defer stopBackups()
	backupService.Start(backupCtx, backupCheckInterval, func() bool { return appSettings.Bool(settings.BackupsEnabled) })

	// Dashboard Stats (unauthenticated)
	dashboardHandler := handlers.NewDashboardHandler(db)
	dashboardGroup := srv.Group("/api/dashboard")
	handlers.RegisterDashboardRoutes(dashboardGroup, dashboardHandler)

	// The scheduler always ticks; the background sync and auto-apply settings decide what a
	// tick does, so they can be switched on the Settings page without a restart.
	jobScheduler := jobs.New(runner)
	if err := jobScheduler.Start(cfg.SyncAnalysisCronSpec); err != nil {
		logger.Fatal().Err(err).Msg("failed to start sync scheduler")
	}

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

	stopBackups()
	if err := jobScheduler.Stop(); err != nil {
		logger.Error().Err(err).Msg("sync worker shutdown failed")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("server shutdown failed")
	}

	logger.Info().Msg("server stopped")
}

// cachedFor memoizes f for ttl (it is read on every quota decision).
func cachedFor(ttl time.Duration, f func() int) func() int {
	var mu sync.Mutex
	var value int
	var at time.Time
	return func() int {
		mu.Lock()
		defer mu.Unlock()
		if at.IsZero() || time.Since(at) > ttl {
			value, at = f(), time.Now()
		}
		return value
	}
}

func nullableString(value string) sql.NullString {
	if strings.TrimSpace(value) == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: strings.TrimSpace(value), Valid: true}
}
