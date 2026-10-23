package jobs

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"

	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog"

	"github.com/manlikeabro/spotube/internal/activitylogger"
	"github.com/manlikeabro/spotube/internal/auth"
)

// Scheduler manages background jobs using robfig/cron.
type Scheduler struct {
	cron           *cron.Cron
	db             *sql.DB
	logger         zerolog.Logger
	activityLogger *activitylogger.Logger
	clientFactory  *auth.ClientFactory
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
}

// New creates a new job scheduler with default configuration.
func New(deps JobDeps, clientFactory *auth.ClientFactory) *Scheduler {
	ctx, cancel := context.WithCancel(context.Background())

	// Create cron with second precision for fine-grained scheduling
	cronInstance := cron.New(cron.WithSeconds())

	// Create job instances
	analysisJob := NewAnalysisJob(deps, clientFactory)

	return &Scheduler{
		cron:           cronInstance,
		db:             deps.DB,
		logger:         deps.Logger,
		activityLogger: deps.ActivityLogger,
		clientFactory:  clientFactory,
		analysisJob:    analysisJob,
		ctx:            ctx,
		cancel:         cancel,
	}
}

// Start registers the sync run on cronSpec (6-field syntax, seconds included). When applier is
// non-nil, each run applies queued items right after analysing a mapping.
func (s *Scheduler) Start(cronSpec string, applier *Executor) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	if applier != nil {
		s.analysisJob.EnableAutoApply(applier, defaultApplyLimit)
	}

	if _, err := s.cron.AddFunc(cronSpec, s.runAnalysisJob); err != nil {
		return err
	}

	s.cron.Start()
	s.started = true

	s.logger.Info().
		Str("cron", cronSpec).
		Bool("auto_apply", applier != nil).
		Msg("sync scheduler started")
	s.activityLogger.RecordInfo("Sync scheduler started", "", "system")

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

	s.activityLogger.RecordInfo("Sync scheduler stopped", "", "system")

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
