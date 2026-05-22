package probe

import (
	"context"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/ding113/claude-code-hub/internal/repository"
	"github.com/ding113/claude-code-hub/internal/service/endpointprobe"
)

const (
	// DefaultInterval is the default delay between probe cycles.
	DefaultInterval = 30 * time.Second
)

// EndpointRepo is the subset of repository.ProviderEndpointRepository
// needed by the scheduler.
type EndpointRepo interface {
	List(ctx context.Context, opts *repository.ListOptions) ([]*model.ProviderEndpoint, error)
	UpdateProbeSnapshot(ctx context.Context, id int, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpoint, error)
}

// Scheduler runs periodic health probes against all enabled endpoints.
type Scheduler struct {
	interval     time.Duration
	prober       *Prober
	endpointRepo EndpointRepo
	batcher      *LogBatcher
	stopCh       chan struct{}
	stopped      chan struct{}
	once         sync.Once
}

// NewScheduler creates a Scheduler. If interval <= 0, DefaultInterval is used.
func NewScheduler(interval time.Duration, prober *Prober, endpointRepo EndpointRepo, batcher *LogBatcher) *Scheduler {
	if interval <= 0 {
		interval = DefaultInterval
	}
	if prober == nil {
		prober = NewProber(0)
	}
	return &Scheduler{
		interval:     interval,
		prober:       prober,
		endpointRepo: endpointRepo,
		batcher:      batcher,
		stopCh:       make(chan struct{}),
		stopped:      make(chan struct{}),
	}
}

// Start launches the background probe loop. It blocks until Stop is called or
// ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	go s.loop(ctx)
}

// Stop signals the probe loop to exit and waits for it to finish.
func (s *Scheduler) Stop() {
	s.once.Do(func() {
		close(s.stopCh)
	})
	<-s.stopped
}

// Done returns a channel that is closed when the scheduler has fully stopped.
func (s *Scheduler) Done() <-chan struct{} {
	return s.stopped
}

func (s *Scheduler) loop(ctx context.Context) {
	defer close(s.stopped)

	// Run immediately on start.
	if err := s.RunOnce(ctx); err != nil {
		logger.Error().Err(err).Msg("probe scheduler: initial run failed")
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			s.flushBatcher(ctx)
			return
		case <-ctx.Done():
			s.flushBatcher(ctx)
			return
		case <-ticker.C:
			if err := s.RunOnce(ctx); err != nil {
				logger.Error().Err(err).Msg("probe scheduler: cycle failed")
			}
		}
	}
}

// RunOnce performs a single probe cycle: fetch all enabled endpoints, probe
// each one, record results, and update the endpoint snapshot.
func (s *Scheduler) RunOnce(ctx context.Context) error {
	opts := repository.NewListOptions().
		WithPagination(1, 1000) // fetch up to 1000 endpoints per cycle
	endpoints, err := s.endpointRepo.List(ctx, opts)
	if err != nil {
		return err
	}

	for _, ep := range endpoints {
		if !ep.IsActive() {
			continue
		}

		result := s.prober.Probe(ctx, ep)
		now := time.Now()

		logEntry := s.resultToLog(ep.ID, result, now)

		// Record in the fast in-memory / Redis store (for real-time dashboard).
		recordToProbeStore(ep.ID, result, now)

		// Buffer for DB persistence.
		if s.batcher != nil {
			s.batcher.Add(logEntry)
		}

		// Update the endpoint snapshot columns in-place.
		if _, updateErr := s.endpointRepo.UpdateProbeSnapshot(ctx, ep.ID, logEntry); updateErr != nil {
			logger.Warn().Err(updateErr).Int("endpointId", ep.ID).Msg("probe scheduler: failed to update snapshot")
		}
	}

	// Flush any remaining buffered logs.
	s.flushBatcher(ctx)

	return nil
}

// resultToLog converts a ProbeResult to a ProviderEndpointProbeLog model.
func (s *Scheduler) resultToLog(endpointID int, r *ProbeResult, ts time.Time) *model.ProviderEndpointProbeLog {
	log := &model.ProviderEndpointProbeLog{
		EndpointID: endpointID,
		Source:     "scheduled",
		Ok:         r.OK,
		CreatedAt:  ts,
	}
	if r.StatusCode != 0 {
		code := r.StatusCode
		log.StatusCode = &code
	}
	if r.LatencyMs > 0 {
		ms := int(r.LatencyMs)
		log.LatencyMs = &ms
	}
	if r.ErrorType != "" {
		et := r.ErrorType
		log.ErrorType = &et
	}
	if r.ErrorMsg != "" {
		em := r.ErrorMsg
		log.ErrorMessage = &em
	}
	return log
}

// recordToProbeStore writes to the existing endpointprobe in-memory/Redis store.
func recordToProbeStore(endpointID int, r *ProbeResult, ts time.Time) {
	var statusCode, latencyMs *int
	var errorType, errorMsg *string
	if r.StatusCode != 0 {
		sc := r.StatusCode
		statusCode = &sc
	}
	if r.LatencyMs > 0 {
		ms := int(r.LatencyMs)
		latencyMs = &ms
	}
	if r.ErrorType != "" {
		et := r.ErrorType
		errorType = &et
	}
	if r.ErrorMsg != "" {
		em := r.ErrorMsg
		errorMsg = &em
	}
	endpointprobe.Record(endpointID, "scheduled", r.OK, statusCode, latencyMs, errorType, errorMsg, ts)
}

func (s *Scheduler) flushBatcher(ctx context.Context) {
	if s.batcher != nil {
		if err := s.batcher.Flush(ctx); err != nil {
			logger.Error().Err(err).Msg("probe scheduler: final flush failed")
		}
	}
}
