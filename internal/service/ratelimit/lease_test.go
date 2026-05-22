package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/quagmt/udecimal"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---- helpers ----

func setupLeaseTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(func() { mr.Close() })

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return client, mr
}

func newTestLeaseService(t *testing.T) (*LeaseService, *redis.Client, *miniredis.Miniredis) {
	t.Helper()
	client, mr := setupLeaseTestRedis(t)
	cfg := DefaultLeaseConfig()
	cfg.Timezone = "UTC"
	svc := NewLeaseService(client, cfg)
	return svc, client, mr
}

// ---- Time Utils Tests ----

func TestGet5hWindowStart(t *testing.T) {
	now := time.Date(2025, 5, 22, 15, 30, 0, 0, time.UTC)
	start := Get5hWindowStart(now)
	expected := time.Date(2025, 5, 22, 10, 30, 0, 0, time.UTC)
	assert.Equal(t, expected, start)
}

func TestGetDailyWindowStart_Fixed(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	tests := []struct {
		name      string
		now       time.Time
		resetTime string
		expected  time.Time
	}{
		{
			name:      "after reset time today",
			now:       time.Date(2025, 5, 22, 14, 0, 0, 0, time.UTC),
			resetTime: "00:00",
			expected:  time.Date(2025, 5, 22, 0, 0, 0, 0, time.UTC),
		},
		{
			name:      "before reset time today - use yesterday",
			now:       time.Date(2025, 5, 22, 5, 0, 0, 0, time.UTC),
			resetTime: "18:00",
			expected:  time.Date(2025, 5, 21, 18, 0, 0, 0, time.UTC),
		},
		{
			name:      "exactly at reset time",
			now:       time.Date(2025, 5, 22, 6, 0, 0, 0, time.UTC),
			resetTime: "06:00",
			expected:  time.Date(2025, 5, 22, 6, 0, 0, 0, time.UTC),
		},
		{
			name:      "custom reset time 03:30",
			now:       time.Date(2025, 5, 22, 10, 0, 0, 0, time.UTC),
			resetTime: "03:30",
			expected:  time.Date(2025, 5, 22, 3, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := GetDailyWindowStart(tt.now, loc, ResetModeFixed, tt.resetTime)
			assert.Equal(t, tt.expected, start)
		})
	}
}

func TestGetDailyWindowStart_Rolling(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	now := time.Date(2025, 5, 22, 15, 30, 0, 0, time.UTC)
	start := GetDailyWindowStart(now, loc, ResetModeRolling, "00:00")
	expected := now.Add(-24 * time.Hour)
	assert.Equal(t, expected, start)
}

func TestGetWeeklyWindowStart(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	tests := []struct {
		name     string
		now      time.Time
		expected time.Time
	}{
		{
			name:     "Thursday -> Monday of same week",
			now:      time.Date(2025, 5, 22, 15, 0, 0, 0, time.UTC), // Thursday
			expected: time.Date(2025, 5, 19, 0, 0, 0, 0, time.UTC),  // Monday
		},
		{
			name:     "Monday -> same Monday",
			now:      time.Date(2025, 5, 19, 10, 0, 0, 0, time.UTC), // Monday
			expected: time.Date(2025, 5, 19, 0, 0, 0, 0, time.UTC),  // Monday
		},
		{
			name:     "Sunday -> previous Monday",
			now:      time.Date(2025, 5, 25, 10, 0, 0, 0, time.UTC), // Sunday
			expected: time.Date(2025, 5, 19, 0, 0, 0, 0, time.UTC),  // Monday
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := GetWeeklyWindowStart(tt.now, loc)
			assert.Equal(t, tt.expected, start)
		})
	}
}

func TestGetMonthlyWindowStart(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	now := time.Date(2025, 5, 22, 15, 0, 0, 0, time.UTC)
	start := GetMonthlyWindowStart(now, loc)
	expected := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, start)
}

func TestGetPeriodTimeRange_AllPeriods(t *testing.T) {
	now := time.Date(2025, 5, 22, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		period    Period
		resetMode DailyResetMode
	}{
		{Period5H, ResetModeFixed},
		{PeriodDaily, ResetModeFixed},
		{PeriodDaily, ResetModeRolling},
		{PeriodWeekly, ResetModeFixed},
		{PeriodMonthly, ResetModeFixed},
		{PeriodTotal, ResetModeFixed},
	}

	for _, tt := range tests {
		name := string(tt.period)
		if tt.period == PeriodDaily {
			name += "_" + string(tt.resetMode)
		}
		t.Run(name, func(t *testing.T) {
			tr, err := GetPeriodTimeRange(tt.period, now, "UTC", tt.resetMode, "00:00")
			require.NoError(t, err)
			assert.True(t, tr.Start.Before(tr.End) || tr.Start.IsZero(),
				"start (%v) should be before end (%v)", tr.Start, tr.End)
		})
	}
}

func TestGetPeriodTimeRange_InvalidTimezone(t *testing.T) {
	now := time.Now()
	_, err := GetPeriodTimeRange(PeriodDaily, now, "Invalid/Timezone", ResetModeFixed, "00:00")
	assert.Error(t, err)
}

func TestGetTTLForPeriod(t *testing.T) {
	now := time.Date(2025, 5, 22, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		period Period
		minTTL int
	}{
		{Period5H, 1},     // at least 1 second
		{PeriodDaily, 1},  // at least 1 second
		{PeriodWeekly, 1}, // at least 1 second
		{PeriodMonthly, 1},
	}

	for _, tt := range tests {
		t.Run(string(tt.period), func(t *testing.T) {
			ttl, err := GetTTLForPeriod(tt.period, now, "UTC", ResetModeFixed, "00:00")
			require.NoError(t, err)
			assert.GreaterOrEqual(t, ttl, tt.minTTL)
		})
	}
}

func TestNormalizeResetTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", "00:00"},
		{"00:00", "00:00"},
		{"18:30", "18:30"},
		{"9:05", "09:05"},
		{"invalid", "00:00"},
		{"25:00", "00:00"},
		{"12:61", "12:00"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, NormalizeResetTime(tt.input))
		})
	}
}

func TestGetWeeklyWindowStart_WithTimezone(t *testing.T) {
	// Test with Asia/Shanghai timezone (UTC+8)
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)

	// Thursday 2025-05-22 03:00 UTC = Thursday 2025-05-22 11:00 Shanghai
	now := time.Date(2025, 5, 22, 3, 0, 0, 0, time.UTC)
	start := GetWeeklyWindowStart(now, loc)

	// Should be Monday 00:00 Shanghai time = Sunday 16:00 UTC
	expectedShanghai := time.Date(2025, 5, 19, 0, 0, 0, 0, loc)
	assert.Equal(t, expectedShanghai, start)
}

// ---- Lease Structure Tests ----

func TestLease_Complete(t *testing.T) {
	lease := &Lease{
		ID:            "test-1",
		EntityType:    LeaseEntityUser,
		EntityID:      42,
		Period:        PeriodDaily,
		EstimatedCost: udecimal.MustParse("1.50"),
		Status:        LeaseStatusActive,
		AcquiredAt:    time.Now(),
	}

	actualCost := udecimal.MustParse("1.20")
	lease.Complete(actualCost)

	assert.Equal(t, LeaseStatusCompleted, lease.Status)
	assert.True(t, lease.ActualCost.Equal(actualCost))
	assert.NotNil(t, lease.CompletedAt)
}

func TestLease_Expire(t *testing.T) {
	lease := &Lease{
		ID:            "test-2",
		EntityType:    LeaseEntityKey,
		EntityID:      1,
		Period:        Period5H,
		EstimatedCost: udecimal.MustParse("0.50"),
		Status:        LeaseStatusActive,
		AcquiredAt:    time.Now(),
	}

	lease.Expire()

	assert.Equal(t, LeaseStatusExpired, lease.Status)
	assert.NotNil(t, lease.CompletedAt)
}

func TestLease_IsExpired(t *testing.T) {
	lease := &Lease{
		AcquiredAt: time.Now().Add(-15 * time.Minute),
	}

	assert.True(t, lease.IsExpired(time.Now(), 10*time.Minute))
	assert.False(t, lease.IsExpired(time.Now(), 20*time.Minute))
}

func TestLease_CostDelta(t *testing.T) {
	tests := []struct {
		name          string
		estimated     string
		actual        string
		expectedDelta string
	}{
		{"exact match", "1.00", "1.00", "0"},
		{"under-reserved", "1.00", "1.50", "0.5"},
		{"over-reserved", "1.50", "1.00", "-0.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lease := &Lease{
				EstimatedCost: udecimal.MustParse(tt.estimated),
				ActualCost:    udecimal.MustParse(tt.actual),
			}
			delta := lease.CostDelta()
			expected := udecimal.MustParse(tt.expectedDelta)
			assert.True(t, delta.Equal(expected), "expected %s, got %s", expected, delta)
		})
	}
}

// ---- BudgetLease Tests ----

func TestBudgetLease_SerializeDeserialize(t *testing.T) {
	bl := &BudgetLease{
		EntityType:      LeaseEntityUser,
		EntityID:        42,
		Period:          PeriodDaily,
		ResetMode:       ResetModeFixed,
		ResetTime:       "00:00",
		SnapshotAtMs:    time.Now().UnixMilli(),
		CurrentUsage:    10.5,
		LimitAmount:     100.0,
		RemainingBudget: 4.5,
		TTLSeconds:      10,
	}

	data, err := bl.Serialize()
	require.NoError(t, err)

	parsed := DeserializeBudgetLease(data)
	require.NotNil(t, parsed)

	assert.Equal(t, bl.EntityType, parsed.EntityType)
	assert.Equal(t, bl.EntityID, parsed.EntityID)
	assert.Equal(t, bl.Period, parsed.Period)
	assert.InDelta(t, bl.CurrentUsage, parsed.CurrentUsage, 0.001)
	assert.InDelta(t, bl.LimitAmount, parsed.LimitAmount, 0.001)
	assert.InDelta(t, bl.RemainingBudget, parsed.RemainingBudget, 0.001)
}

func TestDeserializeBudgetLease_Invalid(t *testing.T) {
	assert.Nil(t, DeserializeBudgetLease(""))
	assert.Nil(t, DeserializeBudgetLease("not json"))
	assert.Nil(t, DeserializeBudgetLease(`{"entityType":"","entityId":1}`))
}

func TestBudgetLease_IsExpired(t *testing.T) {
	bl := &BudgetLease{
		SnapshotAtMs: time.Now().UnixMilli() - 15000, // 15 seconds ago
		TTLSeconds:   10,
	}
	assert.True(t, bl.IsExpired(time.Now().UnixMilli()))

	bl.SnapshotAtMs = time.Now().UnixMilli() - 5000 // 5 seconds ago
	assert.False(t, bl.IsExpired(time.Now().UnixMilli()))
}

func TestBuildLeaseKey(t *testing.T) {
	key := BuildLeaseKey(LeaseEntityUser, 42, PeriodDaily)
	assert.Equal(t, "lease:user:42:daily", key)

	key = BuildLeaseKey(LeaseEntityKey, 1, Period5H)
	assert.Equal(t, "lease:key:1:5h", key)

	key = BuildLeaseKey(LeaseEntityProvider, 7, PeriodMonthly)
	assert.Equal(t, "lease:provider:7:monthly", key)
}

// ---- CalculateLeaseSlice Tests ----

func TestCalculateLeaseSlice(t *testing.T) {
	tests := []struct {
		name     string
		params   CalculateLeaseSliceParams
		expected float64
	}{
		{
			name: "basic 5% of 100 with low usage",
			params: CalculateLeaseSliceParams{
				LimitAmount: 100.0, CurrentUsage: 10.0, Percent: 0.05,
			},
			expected: 5.0,
		},
		{
			name: "remaining less than slice",
			params: CalculateLeaseSliceParams{
				LimitAmount: 100.0, CurrentUsage: 97.0, Percent: 0.05,
			},
			expected: 3.0,
		},
		{
			name: "fully used",
			params: CalculateLeaseSliceParams{
				LimitAmount: 100.0, CurrentUsage: 100.0, Percent: 0.05,
			},
			expected: 0,
		},
		{
			name: "over-used",
			params: CalculateLeaseSliceParams{
				LimitAmount: 100.0, CurrentUsage: 110.0, Percent: 0.05,
			},
			expected: 0,
		},
		{
			name: "cap applied",
			params: CalculateLeaseSliceParams{
				LimitAmount: 1000.0, CurrentUsage: 0, Percent: 0.1,
				CapUSD: ptrFloat64(2.0),
			},
			expected: 2.0,
		},
		{
			name: "negative percent clamped",
			params: CalculateLeaseSliceParams{
				LimitAmount: 100.0, CurrentUsage: 0, Percent: -0.5,
			},
			expected: 0,
		},
		{
			name: "percent > 1 clamped",
			params: CalculateLeaseSliceParams{
				LimitAmount: 100.0, CurrentUsage: 0, Percent: 1.5,
			},
			expected: 100.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateLeaseSlice(tt.params)
			assert.InDelta(t, tt.expected, result, 0.001)
		})
	}
}

// ---- Lease Service Tests ----

func TestLeaseService_AcquireRelease(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	req := AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      1,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("5.00"),
		LimitAmount:   udecimal.MustParse("100.00"),
	}

	// Acquire
	result, err := svc.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Lease)
	assert.Equal(t, LeaseStatusActive, result.Lease.Status)
	assert.True(t, result.Lease.EstimatedCost.Equal(udecimal.MustParse("5.00")))
	assert.NotEmpty(t, result.Lease.ID)

	// Release with actual cost = estimated (no adjustment)
	err = svc.Release(ctx, result.Lease, udecimal.MustParse("5.00"))
	require.NoError(t, err)
	assert.Equal(t, LeaseStatusCompleted, result.Lease.Status)
}

func TestLeaseService_AcquireRelease_WithAdjustment(t *testing.T) {
	svc, client, _ := newTestLeaseService(t)
	ctx := context.Background()

	req := AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      2,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("10.00"),
		LimitAmount:   udecimal.MustParse("100.00"),
	}

	// Acquire reserves 10.00
	result, err := svc.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Check counter was set to 10
	counterKey := "ratelimit:cost:daily:user:2"
	val, err := client.Get(ctx, counterKey).Float64()
	require.NoError(t, err)
	assert.InDelta(t, 10.0, val, 0.01)

	// Release with actual cost = 7.00 (over-reserved by 3.00, counter should decrease)
	err = svc.Release(ctx, result.Lease, udecimal.MustParse("7.00"))
	require.NoError(t, err)

	val, err = client.Get(ctx, counterKey).Float64()
	require.NoError(t, err)
	assert.InDelta(t, 7.0, val, 0.01)
}

func TestLeaseService_AcquireRelease_UnderReserved(t *testing.T) {
	svc, client, _ := newTestLeaseService(t)
	ctx := context.Background()

	req := AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      3,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("5.00"),
		LimitAmount:   udecimal.MustParse("100.00"),
	}

	result, err := svc.Acquire(ctx, req)
	require.NoError(t, err)

	// Release with actual cost = 8.00 (under-reserved by 3.00, counter should increase)
	err = svc.Release(ctx, result.Lease, udecimal.MustParse("8.00"))
	require.NoError(t, err)

	counterKey := "ratelimit:cost:daily:user:3"
	val, err := client.Get(ctx, counterKey).Float64()
	require.NoError(t, err)
	assert.InDelta(t, 8.0, val, 0.01)
}

func TestLeaseService_Acquire_ExceedsLimit(t *testing.T) {
	svc, client, _ := newTestLeaseService(t)
	ctx := context.Background()

	// Pre-fill the counter close to limit
	counterKey := "ratelimit:cost:daily:user:4"
	client.Set(ctx, counterKey, "95.0", 0)

	req := AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      4,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("10.00"),
		LimitAmount:   udecimal.MustParse("100.00"),
	}

	// This should fail: 95 + 10 > 100
	result, err := svc.Acquire(ctx, req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "quota exceeded")
}

func TestLeaseService_Acquire_NoLimit(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	req := AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      5,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("999.00"),
		LimitAmount:   udecimal.Zero, // No limit
	}

	result, err := svc.Acquire(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, LeaseStatusActive, result.Lease.Status)
}

func TestLeaseService_Expire(t *testing.T) {
	svc, client, _ := newTestLeaseService(t)
	ctx := context.Background()

	req := AcquireRequest{
		EntityType:    LeaseEntityKey,
		EntityID:      10,
		Period:        Period5H,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("20.00"),
		LimitAmount:   udecimal.MustParse("100.00"),
	}

	// Acquire reserves 20.00
	result, err := svc.Acquire(ctx, req)
	require.NoError(t, err)

	counterKey := "ratelimit:cost:5h:key:10"
	val, err := client.Get(ctx, counterKey).Float64()
	require.NoError(t, err)
	assert.InDelta(t, 20.0, val, 0.01)

	// Expire should refund the full estimated cost
	err = svc.Expire(ctx, result.Lease)
	require.NoError(t, err)
	assert.Equal(t, LeaseStatusExpired, result.Lease.Status)

	val, err = client.Get(ctx, counterKey).Float64()
	require.NoError(t, err)
	assert.InDelta(t, 0.0, val, 0.01)
}

func TestLeaseService_MultipleAcquires(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	limit := udecimal.MustParse("50.00")

	// Acquire multiple leases that together approach the limit
	var leases []*Lease
	for i := 0; i < 4; i++ {
		result, err := svc.Acquire(ctx, AcquireRequest{
			EntityType:    LeaseEntityUser,
			EntityID:      100,
			Period:        PeriodWeekly,
			ResetMode:     ResetModeFixed,
			ResetTime:     "00:00",
			EstimatedCost: udecimal.MustParse("10.00"),
			LimitAmount:   limit,
		})
		require.NoError(t, err, "acquire %d should succeed", i)
		leases = append(leases, result.Lease)
	}

	// 5th should fail: 40 + 10 > 50 (actually 40+10 = 50, not > 50)
	// Actually our script checks > not >=, let's acquire one more at 10 to hit 50
	result, err := svc.Acquire(ctx, AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      100,
		Period:        PeriodWeekly,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("10.00"),
		LimitAmount:   limit,
	})
	require.NoError(t, err, "5th acquire should succeed (40+10=50, not exceeding)")
	leases = append(leases, result.Lease)

	// 6th should fail: 50 + 10 > 50
	_, err = svc.Acquire(ctx, AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      100,
		Period:        PeriodWeekly,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("10.00"),
		LimitAmount:   limit,
	})
	assert.Error(t, err, "6th acquire should fail")

	// Release all with lower actual cost -> should free up quota
	for _, l := range leases {
		err := svc.Release(ctx, l, udecimal.MustParse("5.00"))
		require.NoError(t, err)
	}

	// Now we should be at 25.00 usage; another 10.00 should succeed
	_, err = svc.Acquire(ctx, AcquireRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      100,
		Period:        PeriodWeekly,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("10.00"),
		LimitAmount:   limit,
	})
	assert.NoError(t, err, "acquire after releasing should succeed")
}

func TestLeaseService_AllPeriods(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	periods := []Period{Period5H, PeriodDaily, PeriodWeekly, PeriodMonthly, PeriodTotal}

	for _, p := range periods {
		t.Run(string(p), func(t *testing.T) {
			result, err := svc.Acquire(ctx, AcquireRequest{
				EntityType:    LeaseEntityUser,
				EntityID:      200,
				Period:        p,
				ResetMode:     ResetModeFixed,
				ResetTime:     "00:00",
				EstimatedCost: udecimal.MustParse("1.00"),
				LimitAmount:   udecimal.MustParse("100.00"),
			})
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, LeaseStatusActive, result.Lease.Status)

			err = svc.Release(ctx, result.Lease, udecimal.MustParse("1.00"))
			require.NoError(t, err)
		})
	}
}

func TestLeaseService_AllEntityTypes(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	entityTypes := []LeaseEntityType{LeaseEntityKey, LeaseEntityUser, LeaseEntityProvider}

	for _, et := range entityTypes {
		t.Run(string(et), func(t *testing.T) {
			result, err := svc.Acquire(ctx, AcquireRequest{
				EntityType:    et,
				EntityID:      300,
				Period:        PeriodDaily,
				ResetMode:     ResetModeFixed,
				ResetTime:     "00:00",
				EstimatedCost: udecimal.MustParse("2.00"),
				LimitAmount:   udecimal.MustParse("50.00"),
			})
			require.NoError(t, err)
			require.NotNil(t, result)

			err = svc.Release(ctx, result.Lease, udecimal.MustParse("2.00"))
			require.NoError(t, err)
		})
	}
}

// ---- Budget Lease Cache Tests ----

func TestLeaseService_GetBudgetLease(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	req := AcquireRequest{
		EntityType:  LeaseEntityUser,
		EntityID:    500,
		Period:      PeriodDaily,
		ResetMode:   ResetModeFixed,
		ResetTime:   "00:00",
		LimitAmount: udecimal.MustParse("100.00"),
	}

	bl, err := svc.GetBudgetLease(ctx, req, 30.0)
	require.NoError(t, err)
	require.NotNil(t, bl)

	assert.Equal(t, LeaseEntityUser, bl.EntityType)
	assert.Equal(t, 500, bl.EntityID)
	assert.Equal(t, PeriodDaily, bl.Period)
	assert.InDelta(t, 30.0, bl.CurrentUsage, 0.01)
	assert.InDelta(t, 100.0, bl.LimitAmount, 0.01)
	assert.Greater(t, bl.RemainingBudget, 0.0)

	// Second call should hit cache
	bl2, err := svc.GetBudgetLease(ctx, req, 30.0)
	require.NoError(t, err)
	require.NotNil(t, bl2)
	assert.Equal(t, bl.SnapshotAtMs, bl2.SnapshotAtMs)
}

func TestLeaseService_DecrementBudgetLease(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	// First create a budget lease
	req := AcquireRequest{
		EntityType:  LeaseEntityUser,
		EntityID:    600,
		Period:      PeriodDaily,
		ResetMode:   ResetModeFixed,
		ResetTime:   "00:00",
		LimitAmount: udecimal.MustParse("100.00"),
	}

	bl, err := svc.GetBudgetLease(ctx, req, 0.0)
	require.NoError(t, err)
	initialRemaining := bl.RemainingBudget

	// Decrement
	result := svc.DecrementBudgetLease(ctx, LeaseEntityUser, 600, PeriodDaily, 1.0)
	assert.True(t, result.Success)
	assert.InDelta(t, initialRemaining-1.0, result.NewRemaining, 0.01)
}

func TestLeaseService_DecrementBudgetLease_InsufficientBudget(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	req := AcquireRequest{
		EntityType:  LeaseEntityUser,
		EntityID:    700,
		Period:      PeriodDaily,
		ResetMode:   ResetModeFixed,
		ResetTime:   "00:00",
		LimitAmount: udecimal.MustParse("100.00"),
	}

	// Create budget lease with high usage (small remaining)
	_, err := svc.GetBudgetLease(ctx, req, 99.0)
	require.NoError(t, err)

	// Try to decrement more than remaining (remaining should be ~5% of 100 = 5, but capped by remaining of 1)
	result := svc.DecrementBudgetLease(ctx, LeaseEntityUser, 700, PeriodDaily, 10.0)
	assert.False(t, result.Success)
}

func TestLeaseService_DecrementBudgetLease_KeyNotFound(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	result := svc.DecrementBudgetLease(ctx, LeaseEntityUser, 999, PeriodDaily, 1.0)
	assert.False(t, result.Success)
	assert.InDelta(t, -1.0, result.NewRemaining, 0.01)
}

// ---- CheckCostLimitsWithLease Integration Test ----

func TestCheckCostLimitsWithLease(t *testing.T) {
	client, mr := setupLeaseTestRedis(t)
	defer mr.Close()

	redisSvc := NewRedisService(client)
	leaseSvc := NewLeaseService(client, DefaultLeaseConfig())
	ctx := context.Background()

	// Should be allowed
	result, err := redisSvc.CheckCostLimitsWithLease(ctx, leaseSvc, CheckCostLimitsRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      1,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("5.00"),
		LimitAmount:   udecimal.MustParse("100.00"),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Allowed)
	assert.NotNil(t, result.Lease)
}

func TestCheckCostLimitsWithLease_NoLimit(t *testing.T) {
	client, mr := setupLeaseTestRedis(t)
	defer mr.Close()

	redisSvc := NewRedisService(client)
	leaseSvc := NewLeaseService(client, DefaultLeaseConfig())
	ctx := context.Background()

	result, err := redisSvc.CheckCostLimitsWithLease(ctx, leaseSvc, CheckCostLimitsRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      1,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("999.00"),
		LimitAmount:   udecimal.Zero,
	})
	require.NoError(t, err)
	assert.True(t, result.Allowed)
	assert.Equal(t, "no limit configured", result.Reason)
}

func TestCheckCostLimitsWithLease_Exceeded(t *testing.T) {
	client, mr := setupLeaseTestRedis(t)
	defer mr.Close()

	redisSvc := NewRedisService(client)
	leaseSvc := NewLeaseService(client, DefaultLeaseConfig())
	ctx := context.Background()

	// Pre-fill counter
	counterKey := "ratelimit:cost:daily:user:2"
	client.Set(ctx, counterKey, "95.0", 0)

	result, err := redisSvc.CheckCostLimitsWithLease(ctx, leaseSvc, CheckCostLimitsRequest{
		EntityType:    LeaseEntityUser,
		EntityID:      2,
		Period:        PeriodDaily,
		ResetMode:     ResetModeFixed,
		ResetTime:     "00:00",
		EstimatedCost: udecimal.MustParse("10.00"),
		LimitAmount:   udecimal.MustParse("100.00"),
	})
	require.NoError(t, err) // method itself does not error; it returns Allowed=false
	assert.False(t, result.Allowed)
	assert.Contains(t, result.Reason, "quota exceeded")
}

// ---- Concurrent Acquire Test ----

func TestLeaseService_ConcurrentAcquire(t *testing.T) {
	svc, _, _ := newTestLeaseService(t)
	ctx := context.Background()

	limit := udecimal.MustParse("10.00")
	cost := udecimal.MustParse("1.00")

	type acquireResult struct {
		ok  bool
		err error
	}

	results := make(chan acquireResult, 20)

	// Fire 20 goroutines trying to acquire 1.00 each against a 10.00 limit
	for i := 0; i < 20; i++ {
		go func() {
			_, err := svc.Acquire(ctx, AcquireRequest{
				EntityType:    LeaseEntityUser,
				EntityID:      9999,
				Period:        PeriodDaily,
				ResetMode:     ResetModeFixed,
				ResetTime:     "00:00",
				EstimatedCost: cost,
				LimitAmount:   limit,
			})
			results <- acquireResult{ok: err == nil, err: err}
		}()
	}

	successes := 0
	failures := 0
	for i := 0; i < 20; i++ {
		r := <-results
		if r.ok {
			successes++
		} else {
			failures++
		}
	}

	// Exactly 10 should succeed (limit=10, cost=1 each)
	assert.Equal(t, 10, successes, "expected exactly 10 successful acquires")
	assert.Equal(t, 10, failures, "expected exactly 10 failed acquires")
}

// ---- helper ----

func ptrFloat64(v float64) *float64 {
	return &v
}
