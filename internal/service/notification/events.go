package notification

import "time"

// EventType defines the type of notification event.
type EventType string

const (
	EventCircuitBreakerOpen  EventType = "circuit_breaker_open"
	EventCircuitBreakerClose EventType = "circuit_breaker_close"
	EventDailyLeaderboard    EventType = "daily_leaderboard"
	EventCostAlert           EventType = "cost_alert"
	EventCacheHitRateAlert   EventType = "cache_hit_rate_alert"
)

// String returns the string representation of the event type.
func (e EventType) String() string {
	return string(e)
}

// EventPayload is the common payload sent through the notification queue.
type EventPayload struct {
	EventType EventType   `json:"event_type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// CircuitBreakerPayload carries data for circuit breaker events.
type CircuitBreakerPayload struct {
	ProviderID   int    `json:"provider_id"`
	ProviderName string `json:"provider_name"`
	State        string `json:"state"` // "open" or "closed"
	FailureCount int    `json:"failure_count"`
	Reason       string `json:"reason,omitempty"`
}

// DailyLeaderboardPayload carries data for the daily usage leaderboard.
type DailyLeaderboardPayload struct {
	Date    string               `json:"date"` // YYYY-MM-DD
	TopN    int                  `json:"top_n"`
	Entries []LeaderboardEntry   `json:"entries"`
	Summary LeaderboardSummary   `json:"summary"`
}

// LeaderboardEntry represents a single user entry in the leaderboard.
type LeaderboardEntry struct {
	Rank         int    `json:"rank"`
	UserID       int    `json:"user_id"`
	UserName     string `json:"user_name"`
	RequestCount int    `json:"request_count"`
	TotalTokens  int64  `json:"total_tokens"`
	TotalCost    string `json:"total_cost"`
}

// LeaderboardSummary provides aggregate statistics for the leaderboard.
type LeaderboardSummary struct {
	TotalUsers    int    `json:"total_users"`
	TotalRequests int    `json:"total_requests"`
	TotalCost     string `json:"total_cost"`
}

// CostAlertPayload carries data for cost alert events.
type CostAlertPayload struct {
	UserID          int    `json:"user_id"`
	UserName        string `json:"user_name"`
	CurrentCost     string `json:"current_cost"`
	CostLimit       string `json:"cost_limit"`
	UsagePercentage string `json:"usage_percentage"` // e.g. "85.0%"
	Period          string `json:"period"`            // e.g. "daily", "monthly"
}

// CacheHitRatePayload carries data for cache hit rate alert events.
type CacheHitRatePayload struct {
	CurrentRate float64 `json:"current_rate"` // 0.0 - 1.0
	Threshold   float64 `json:"threshold"`    // 0.0 - 1.0
	Window      string  `json:"window"`       // e.g. "1h", "24h"
	TotalHits   int64   `json:"total_hits"`
	TotalMisses int64   `json:"total_misses"`
}
