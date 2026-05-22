package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/quagmt/udecimal"
	"github.com/redis/go-redis/v9"

	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

// Default lease configuration values.
const (
	DefaultLeasePercent    = 0.05
	DefaultLeaseTTLSeconds = 10
	DefaultLeaseMaxAge     = 10 * time.Minute
)

// LeaseConfig holds configurable parameters for the lease system.
type LeaseConfig struct {
	// Timezone for window calculations (IANA, e.g. "Asia/Shanghai").
	Timezone string
	// TTLSeconds for the budget lease cache in Redis.
	TTLSeconds int
	// LeasePercent per window type.
	Percent5h      float64
	PercentDaily   float64
	PercentWeekly  float64
	PercentMonthly float64
	// CapUSD optional global cap in USD per lease slice.
	CapUSD *float64
	// MaxLeaseDuration is the maximum time a lease can stay active.
	MaxLeaseDuration time.Duration
}

// DefaultLeaseConfig returns sensible defaults.
func DefaultLeaseConfig() LeaseConfig {
	return LeaseConfig{
		Timezone:         "UTC",
		TTLSeconds:       DefaultLeaseTTLSeconds,
		Percent5h:        DefaultLeasePercent,
		PercentDaily:     DefaultLeasePercent,
		PercentWeekly:    DefaultLeasePercent,
		PercentMonthly:   DefaultLeasePercent,
		CapUSD:           nil,
		MaxLeaseDuration: DefaultLeaseMaxAge,
	}
}

// AcquireRequest is the input for acquiring a quota lease.
type AcquireRequest struct {
	EntityType    LeaseEntityType
	EntityID      int
	Period        Period
	ResetMode     DailyResetMode
	ResetTime     string
	EstimatedCost udecimal.Decimal
	LimitAmount   udecimal.Decimal
	// CostResetAt optionally clips the window start forward (limits-only reset).
	CostResetAt *time.Time
}

// AcquireResult is returned from a successful Acquire call.
type AcquireResult struct {
	Lease     *Lease
	Remaining udecimal.Decimal
}

// DecrementResult is returned from a budget lease decrement.
type DecrementResult struct {
	Success      bool
	NewRemaining float64
	FailOpen     bool
}

// LeaseService manages quota leases backed by Redis.
type LeaseService struct {
	client *redis.Client
	config LeaseConfig
}

// NewLeaseService creates a new LeaseService.
func NewLeaseService(client *redis.Client, config LeaseConfig) *LeaseService {
	if config.Timezone == "" {
		config.Timezone = "UTC"
	}
	if config.TTLSeconds <= 0 {
		config.TTLSeconds = DefaultLeaseTTLSeconds
	}
	if config.MaxLeaseDuration <= 0 {
		config.MaxLeaseDuration = DefaultLeaseMaxAge
	}
	return &LeaseService{
		client: client,
		config: config,
	}
}

// Acquire reserves estimated cost against the quota for the given entity+period.
// It atomically checks and reserves via a Lua script.
//
// Flow:
//  1. Build the counter key for this entity+period.
//  2. Compute TTL for the counter based on the period window.
//  3. Run the atomic check-and-reserve Lua script.
//  4. If OK, create a Lease and return it.
//  5. If the limit would be exceeded, return an error.
func (s *LeaseService) Acquire(ctx context.Context, req AcquireRequest) (*AcquireResult, error) {
	if req.LimitAmount.IsZero() {
		// No limit configured -> always allow (no lease needed)
		lease := s.newLease(req)
		return &AcquireResult{
			Lease:     lease,
			Remaining: udecimal.Zero,
		}, nil
	}

	counterKey := s.counterKey(req.EntityType, req.EntityID, req.Period)
	now := time.Now()

	// Compute TTL for the counter
	ttlSec, err := GetTTLForPeriod(
		req.Period, now, s.config.Timezone,
		req.ResetMode, NormalizeResetTime(req.ResetTime),
	)
	if err != nil {
		logger.Warn().Err(err).
			Str("entityType", string(req.EntityType)).
			Int("entityId", req.EntityID).
			Str("period", string(req.Period)).
			Msg("[LeaseService] Failed to compute TTL, falling back to period duration")
		ttlSec = int(req.Period.GetDuration().Seconds())
	}

	// For PeriodTotal, use 0 TTL (no expiry)
	if req.Period == PeriodTotal {
		ttlSec = 0
	}

	costFloat := req.EstimatedCost.InexactFloat64()
	limitFloat := req.LimitAmount.InexactFloat64()

	// Atomic check-and-reserve
	result, err := luaCheckAndReserve.Run(ctx, s.client,
		[]string{counterKey},
		fmt.Sprintf("%.10f", costFloat),
		fmt.Sprintf("%.10f", limitFloat),
		strconv.Itoa(ttlSec),
	).Slice()

	if err != nil {
		logger.Error().Err(err).
			Str("key", counterKey).
			Msg("[LeaseService] Lua check-and-reserve failed, fail-open")
		// Fail-open: allow the request
		lease := s.newLease(req)
		return &AcquireResult{Lease: lease, Remaining: udecimal.Zero}, nil
	}

	// Parse result: {status, newTotal/currentTotal}
	status, _ := toInt64(result[0])
	totalStr := toString(result[1])

	if status == 0 {
		// Limit would be exceeded
		return nil, fmt.Errorf("quota exceeded for %s:%d period=%s (current=%s, limit=%s, requested=%s)",
			req.EntityType, req.EntityID, req.Period,
			totalStr, req.LimitAmount.String(), req.EstimatedCost.String())
	}

	// Success
	lease := s.newLease(req)
	remaining := computeRemaining(totalStr, limitFloat)

	return &AcquireResult{
		Lease:     lease,
		Remaining: remaining,
	}, nil
}

// Release completes a lease by adjusting the counter to reflect actual cost.
// delta = actual - estimated. If positive, the counter is incremented further;
// if negative, the counter is decremented (we over-reserved).
func (s *LeaseService) Release(ctx context.Context, lease *Lease, actualCost udecimal.Decimal) error {
	if lease == nil {
		return fmt.Errorf("cannot release nil lease")
	}

	lease.Complete(actualCost)

	delta := lease.CostDelta()
	if delta.IsZero() {
		// Perfect estimate - nothing to adjust
		return nil
	}

	counterKey := s.counterKey(lease.EntityType, lease.EntityID, lease.Period)
	deltaFloat := delta.InexactFloat64()

	_, err := luaAdjustCounter.Run(ctx, s.client,
		[]string{counterKey},
		fmt.Sprintf("%.10f", deltaFloat),
	).Text()

	if err != nil {
		logger.Error().Err(err).
			Str("leaseId", lease.ID).
			Str("key", counterKey).
			Str("delta", delta.String()).
			Msg("[LeaseService] Failed to adjust counter on release")
		return fmt.Errorf("failed to adjust counter: %w", err)
	}

	return nil
}

// Expire marks a lease as expired and refunds the estimated cost (full refund).
func (s *LeaseService) Expire(ctx context.Context, lease *Lease) error {
	if lease == nil {
		return fmt.Errorf("cannot expire nil lease")
	}

	lease.Expire()

	// Refund the full estimated cost
	if lease.EstimatedCost.IsZero() {
		return nil
	}

	counterKey := s.counterKey(lease.EntityType, lease.EntityID, lease.Period)
	refundFloat := lease.EstimatedCost.InexactFloat64()

	_, err := luaAdjustCounter.Run(ctx, s.client,
		[]string{counterKey},
		fmt.Sprintf("%.10f", -refundFloat),
	).Text()

	if err != nil {
		logger.Error().Err(err).
			Str("leaseId", lease.ID).
			Str("key", counterKey).
			Msg("[LeaseService] Failed to refund on expire")
		return fmt.Errorf("failed to refund on expire: %w", err)
	}

	return nil
}

// GetBudgetLease retrieves or refreshes a cached BudgetLease from Redis.
// If the cached lease is missing, expired, or has a stale limit, it is
// refreshed from the provided currentUsage.
func (s *LeaseService) GetBudgetLease(ctx context.Context, req AcquireRequest, currentUsage float64) (*BudgetLease, error) {
	leaseKey := BuildLeaseKey(req.EntityType, req.EntityID, req.Period)
	limitFloat := req.LimitAmount.InexactFloat64()
	nowMs := time.Now().UnixMilli()

	// Try Redis cache first
	cached, err := s.client.Get(ctx, leaseKey).Result()
	if err == nil && cached != "" {
		bl := DeserializeBudgetLease(cached)
		if bl != nil && !bl.IsExpired(nowMs) && bl.LimitAmount == limitFloat {
			return bl, nil
		}
	}

	// Cache miss or stale - build a fresh lease
	return s.refreshBudgetLease(ctx, req, currentUsage, leaseKey)
}

// DecrementBudgetLease atomically decrements the remainingBudget in a cached
// BudgetLease, using a Lua script.
func (s *LeaseService) DecrementBudgetLease(ctx context.Context, entityType LeaseEntityType, entityID int, period Period, cost float64) DecrementResult {
	leaseKey := BuildLeaseKey(entityType, entityID, period)

	result, err := luaDecrementBudgetLease.Run(ctx, s.client,
		[]string{leaseKey},
		fmt.Sprintf("%.10f", cost),
	).StringSlice()

	if err != nil {
		logger.Error().Err(err).
			Str("key", leaseKey).
			Msg("[LeaseService] Lua decrement failed, fail-open")
		return DecrementResult{Success: true, NewRemaining: -1, FailOpen: true}
	}

	if len(result) < 2 {
		return DecrementResult{Success: true, NewRemaining: -1, FailOpen: true}
	}

	newRemaining, _ := strconv.ParseFloat(result[0], 64)
	success, _ := strconv.ParseFloat(result[1], 64)

	if success == 1 {
		return DecrementResult{Success: true, NewRemaining: newRemaining}
	}

	// Key not found or insufficient budget
	return DecrementResult{Success: false, NewRemaining: newRemaining}
}

// ---- internal helpers ----

func (s *LeaseService) newLease(req AcquireRequest) *Lease {
	return &Lease{
		ID:            uuid.New().String(),
		EntityType:    req.EntityType,
		EntityID:      req.EntityID,
		Period:        req.Period,
		ResetMode:     req.ResetMode,
		ResetTime:     NormalizeResetTime(req.ResetTime),
		EstimatedCost: req.EstimatedCost,
		ActualCost:    udecimal.Zero,
		AcquiredAt:    time.Now(),
		Status:        LeaseStatusActive,
	}
}

func (s *LeaseService) counterKey(entityType LeaseEntityType, entityID int, period Period) string {
	return fmt.Sprintf("ratelimit:cost:%s:%s:%d", period, entityType, entityID)
}

func (s *LeaseService) refreshBudgetLease(ctx context.Context, req AcquireRequest, currentUsage float64, leaseKey string) (*BudgetLease, error) {
	limitFloat := req.LimitAmount.InexactFloat64()
	pct := s.getLeasePercent(req.Period)

	remaining := CalculateLeaseSlice(CalculateLeaseSliceParams{
		LimitAmount:  limitFloat,
		CurrentUsage: currentUsage,
		Percent:      pct,
		CapUSD:       s.config.CapUSD,
	})

	var costResetAtMs *int64
	if req.CostResetAt != nil {
		ms := req.CostResetAt.UnixMilli()
		costResetAtMs = &ms
	}

	bl := &BudgetLease{
		EntityType:      req.EntityType,
		EntityID:        req.EntityID,
		Period:          req.Period,
		ResetMode:       req.ResetMode,
		ResetTime:       NormalizeResetTime(req.ResetTime),
		SnapshotAtMs:    time.Now().UnixMilli(),
		CurrentUsage:    currentUsage,
		LimitAmount:     limitFloat,
		RemainingBudget: remaining,
		TTLSeconds:      s.config.TTLSeconds,
		CostResetAtMs:   costResetAtMs,
	}

	data, err := bl.Serialize()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize budget lease: %w", err)
	}

	err = s.client.SetEx(ctx, leaseKey, data, time.Duration(s.config.TTLSeconds)*time.Second).Err()
	if err != nil {
		logger.Warn().Err(err).Str("key", leaseKey).Msg("[LeaseService] Failed to cache budget lease")
		// Non-fatal: return the lease anyway
	}

	return bl, nil
}

func (s *LeaseService) getLeasePercent(period Period) float64 {
	switch period {
	case Period5H:
		return s.config.Percent5h
	case PeriodDaily:
		return s.config.PercentDaily
	case PeriodWeekly:
		return s.config.PercentWeekly
	case PeriodMonthly:
		return s.config.PercentMonthly
	default:
		return DefaultLeasePercent
	}
}

// ---- conversion helpers ----

func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case int64:
		return val, true
	case int:
		return int64(val), true
	case float64:
		return int64(val), true
	case string:
		n, err := strconv.ParseInt(val, 10, 64)
		return n, err == nil
	default:
		return 0, false
	}
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int64:
		return strconv.FormatInt(val, 10)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func computeRemaining(totalStr string, limit float64) udecimal.Decimal {
	total, err := strconv.ParseFloat(totalStr, 64)
	if err != nil {
		return udecimal.Zero
	}
	rem := limit - total
	if rem < 0 {
		rem = 0
	}
	return udecimal.MustParse(fmt.Sprintf("%.4f", rem))
}
