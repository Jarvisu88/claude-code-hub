package notification

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEventType_String(t *testing.T) {
	tests := []struct {
		event    EventType
		expected string
	}{
		{EventCircuitBreakerOpen, "circuit_breaker_open"},
		{EventCircuitBreakerClose, "circuit_breaker_close"},
		{EventDailyLeaderboard, "daily_leaderboard"},
		{EventCostAlert, "cost_alert"},
		{EventCacheHitRateAlert, "cache_hit_rate_alert"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.event.String())
	}
}

func TestEventPayload_Struct(t *testing.T) {
	payload := EventPayload{
		EventType: EventCostAlert,
		Data: CostAlertPayload{
			UserID:          1,
			UserName:        "alice",
			CurrentCost:     "$8.50",
			CostLimit:       "$10.00",
			UsagePercentage: "85.0%",
			Period:          "daily",
		},
	}

	assert.Equal(t, EventCostAlert, payload.EventType)
	data, ok := payload.Data.(CostAlertPayload)
	assert.True(t, ok)
	assert.Equal(t, "alice", data.UserName)
	assert.Equal(t, "85.0%", data.UsagePercentage)
}

func TestCircuitBreakerPayload(t *testing.T) {
	p := CircuitBreakerPayload{
		ProviderID:   42,
		ProviderName: "anthropic-1",
		State:        "open",
		FailureCount: 5,
		Reason:       "consecutive 5xx errors",
	}
	assert.Equal(t, 42, p.ProviderID)
	assert.Equal(t, "open", p.State)
}

func TestDailyLeaderboardPayload(t *testing.T) {
	p := DailyLeaderboardPayload{
		Date: "2025-01-15",
		TopN: 3,
		Entries: []LeaderboardEntry{
			{Rank: 1, UserID: 1, UserName: "alice", RequestCount: 100, TotalTokens: 50000, TotalCost: "$5.00"},
			{Rank: 2, UserID: 2, UserName: "bob", RequestCount: 80, TotalTokens: 40000, TotalCost: "$4.00"},
		},
		Summary: LeaderboardSummary{
			TotalUsers:    10,
			TotalRequests: 500,
			TotalCost:     "$50.00",
		},
	}
	assert.Equal(t, 3, p.TopN)
	assert.Len(t, p.Entries, 2)
	assert.Equal(t, "alice", p.Entries[0].UserName)
}

func TestCacheHitRatePayload(t *testing.T) {
	p := CacheHitRatePayload{
		CurrentRate: 0.45,
		Threshold:   0.60,
		Window:      "1h",
		TotalHits:   450,
		TotalMisses: 550,
	}
	assert.Equal(t, 0.45, p.CurrentRate)
	assert.Equal(t, int64(550), p.TotalMisses)
}
