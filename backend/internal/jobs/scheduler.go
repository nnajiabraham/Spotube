package jobs

import (
	"context"
	"database/sql"
	"sync"

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

// Start begins the job scheduler and registers jobs.
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	// Register analysis job (every minute)
	_, err := s.cron.AddFunc("0 * * * * *", s.runAnalysisJob)
	if err != nil {
		return err
	}

	// Register executor job (every 10 seconds)
	_, err = s.cron.AddFunc("*/10 * * * * *", s.runExecutorJob)
	if err != nil {
		return err
	}

	s.cron.Start()
	s.started = true

	s.logger.Info().Msg("job scheduler started")
	s.activityLogger.RecordInfo("Job scheduler started", "", "system")

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

	s.activityLogger.RecordInfo("Job scheduler stopped", "", "system")

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

// runExecutorJob is the cron job function for execution (placeholder).
func (s *Scheduler) runExecutorJob() {
	s.wg.Add(1)
	defer s.wg.Done()

	select {
	case <-s.ctx.Done():
		return
	default:
		// TODO: Implement executor logic in next task
		s.logger.Debug().Msg("executor job triggered (placeholder)")
	}
}
