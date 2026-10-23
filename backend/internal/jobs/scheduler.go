package jobs

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/quota"
	"github.com/manlikeabro/spotube/internal/settings"
)

// Scheduler runs the sync runner on a cron tick. It always runs; each tick checks the runtime
// settings (background sync, auto-apply) to decide what to do.
type Scheduler struct {
	cron           *cron.Cron
	logger         zerolog.Logger
	activityLogger *activitylogger.Logger
	analysisJob    *AnalysisJob
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	started        bool
	running        atomic.Bool
	mu             sync.RWMutex
}

// JobDeps contains dependencies required by background jobs.
type JobDeps struct {
	DB             *sql.DB
	Logger         zerolog.Logger
	ActivityLogger *activitylogger.Logger
	// Cache, Budget and Settings are optional (nil in most tests).
	Cache    *PlaylistCache
	Budget   *quota.Budget
	Settings *settings.Store
}

// New creates a scheduler for the shared runner.
func New(runner *AnalysisJob) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())
	return &Scheduler{
		// Second precision for fine-grained scheduling.
		cron:           cron.New(cron.WithSeconds()),
		logger:         runner.logger,
		activityLogger: runner.activityLogger,
		analysisJob:    runner,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start registers the tick on cronSpec (6-field syntax, seconds included).
func (s *Scheduler) Start(cronSpec string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}
	if _, err := s.cron.AddFunc(cronSpec, s.runAnalysisJob); err != nil {
		return err
	}
	s.cron.Start()
	s.started = true

	s.logger.Info().
		Str("cron", cronSpec).
		Bool("background_sync", s.analysisJob.workersEnabled()).
		Bool("auto_apply", s.analysisJob.autoApplyEnabled()).
		Msg("sync scheduler started")
	return nil
}

// Stop gracefully shuts down the job scheduler.
func (s *Scheduler) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return nil
	}

	// Stop accepting new jobs
	cronCtx := s.cron.Stop()

	// Cancel ongoing jobs
	s.cancel()

	// Wait for jobs to complete (with timeout from cron.Stop())
	select {
	case <-cronCtx.Done():
		s.logger.Info().Msg("job scheduler stopped gracefully")
	}

	s.wg.Wait()
	s.started = false

	return nil
}

// IsRunning returns whether the scheduler is currently active.
func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// runAnalysisJob is the cron job function for analysis.
func (s *Scheduler) runAnalysisJob() {
	// Skip this tick if the previous run is still going (large playlists, slow APIs).
	if !s.running.CompareAndSwap(false, true) {
		s.logger.Debug().Msg("sync run still in progress, skipping tick")
		return
	}
	defer s.running.Store(false)

	s.wg.Add(1)
	defer s.wg.Done()

	select {
	case <-s.ctx.Done():
		return
	default:
		if err := s.analysisJob.Run(s.ctx); err != nil {
			s.logger.Error().Err(err).Msg("analysis job failed")
		}
	}
}
