package ratelimit

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/quagmt/udecimal"
)

// LeaseStatus represents the lifecycle state of a quota lease.
type LeaseStatus string

const (
	LeaseStatusActive    LeaseStatus = "active"
	LeaseStatusCompleted LeaseStatus = "completed"
	LeaseStatusExpired   LeaseStatus = "expired"
)

// LeaseEntityType indicates which kind of entity a lease belongs to.
type LeaseEntityType string

const (
	LeaseEntityKey      LeaseEntityType = "key"
	LeaseEntityUser     LeaseEntityType = "user"
	LeaseEntityProvider LeaseEntityType = "provider"
)

// Lease represents a reserved quota slice. It is created when a request is
// accepted (Acquire) and resolved when the request finishes (Release/Expire).
type Lease struct {
	ID            string          `json:"id"`
	EntityType    LeaseEntityType `json:"entityType"`
	EntityID      int             `json:"entityId"`
	Period        Period          `json:"period"`
	ResetMode     DailyResetMode  `json:"resetMode"`
	ResetTime     string          `json:"resetTime"`
	EstimatedCost udecimal.Decimal `json:"estimatedCost"`
	ActualCost    udecimal.Decimal `json:"actualCost"`
	AcquiredAt    time.Time       `json:"acquiredAt"`
	CompletedAt   *time.Time      `json:"completedAt,omitempty"`
	Status        LeaseStatus     `json:"status"`
}

// Complete marks the lease as completed with the actual cost.
func (l *Lease) Complete(actualCost udecimal.Decimal) {
	l.ActualCost = actualCost
	l.Status = LeaseStatusCompleted
	now := time.Now()
	l.CompletedAt = &now
}

// Expire marks the lease as expired.
func (l *Lease) Expire() {
	l.Status = LeaseStatusExpired
	now := time.Now()
	l.CompletedAt = &now
}

// IsExpired returns true if the lease has been active for longer than the
// given maximum lease duration.
func (l *Lease) IsExpired(now time.Time, maxDuration time.Duration) bool {
	return now.Sub(l.AcquiredAt) > maxDuration
}

// CostDelta returns actual - estimated; positive means we under-reserved.
func (l *Lease) CostDelta() udecimal.Decimal {
	return l.ActualCost.Sub(l.EstimatedCost)
}

// BudgetLease is a lightweight structure cached in Redis. It represents the
// remaining budget slice for an entity+window combination.
type BudgetLease struct {
	EntityType      LeaseEntityType `json:"entityType"`
	EntityID        int             `json:"entityId"`
	Period          Period          `json:"window"`
	ResetMode       DailyResetMode  `json:"resetMode"`
	ResetTime       string          `json:"resetTime"`
	SnapshotAtMs    int64           `json:"snapshotAtMs"`
	CurrentUsage    float64         `json:"currentUsage"`
	LimitAmount     float64         `json:"limitAmount"`
	RemainingBudget float64         `json:"remainingBudget"`
	TTLSeconds      int             `json:"ttlSeconds"`
	CostResetAtMs   *int64          `json:"costResetAtMs,omitempty"`
}

// BuildLeaseKey returns the Redis key for a BudgetLease.
// Format: lease:{entityType}:{entityId}:{window}
func BuildLeaseKey(entityType LeaseEntityType, entityID int, period Period) string {
	return fmt.Sprintf("lease:%s:%d:%s", entityType, entityID, period)
}

// IsExpired returns true if the budget lease snapshot has passed its TTL.
func (bl *BudgetLease) IsExpired(nowMs int64) bool {
	return nowMs >= bl.SnapshotAtMs+int64(bl.TTLSeconds)*1000
}

// Serialize returns a JSON-encoded representation of the BudgetLease.
func (bl *BudgetLease) Serialize() (string, error) {
	data, err := json.Marshal(bl)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// DeserializeBudgetLease parses a JSON string into a BudgetLease.
// Returns nil if the data is invalid.
func DeserializeBudgetLease(data string) *BudgetLease {
	var bl BudgetLease
	if err := json.Unmarshal([]byte(data), &bl); err != nil {
		return nil
	}
	// Basic validation
	if bl.EntityType == "" || bl.Period == "" {
		return nil
	}
	return &bl
}

// CalculateLeaseSliceParams holds parameters for calculating a lease slice.
type CalculateLeaseSliceParams struct {
	LimitAmount  float64
	CurrentUsage float64
	Percent      float64
	CapUSD       *float64 // optional cap in USD
}

// CalculateLeaseSlice computes the budget slice as:
//
//	min(limit * percent, remaining, capUSD)
//
// Rounded to 4 decimal places.
func CalculateLeaseSlice(params CalculateLeaseSliceParams) float64 {
	remaining := params.LimitAmount - params.CurrentUsage
	if remaining <= 0 {
		return 0
	}

	// Clamp percent to [0, 1]
	pct := params.Percent
	if pct < 0 {
		pct = 0
	}
	if pct > 1 {
		pct = 1
	}

	slice := params.LimitAmount * pct

	// Cap by remaining
	if slice > remaining {
		slice = remaining
	}

	// Cap by USD limit if provided
	if params.CapUSD != nil {
		cap := *params.CapUSD
		if cap < 0 {
			cap = 0
		}
		if slice > cap {
			slice = cap
		}
	}

	// Round to 4 decimal places, ensure non-negative
	result := math.Round(slice*10000) / 10000
	if result < 0 {
		return 0
	}
	return result
}
