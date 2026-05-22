package ratelimit

import (
	"fmt"
	"regexp"
	"strconv"
	"time"
)

// DailyResetMode defines how daily limits reset.
type DailyResetMode string

const (
	// ResetModeFixed resets at a fixed time each day (e.g. "00:00" in configured timezone).
	ResetModeFixed DailyResetMode = "fixed"
	// ResetModeRolling uses a rolling 24h window from first usage.
	ResetModeRolling DailyResetMode = "rolling"
)

// TimeRange represents a start/end pair for a rate limit window.
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// resetTimePat matches "HH:MM" format.
var resetTimePat = regexp.MustCompile(`^([0-9]{1,2}):([0-9]{2})$`)

// parseResetTime parses a "HH:MM" string into hours and minutes.
// Returns (0, 0) for invalid input.
func parseResetTime(resetTime string) (hours, minutes int) {
	matches := resetTimePat.FindStringSubmatch(resetTime)
	if matches == nil {
		return 0, 0
	}
	h, err := strconv.Atoi(matches[1])
	if err != nil || h < 0 || h > 23 {
		h = 0
	}
	m, err := strconv.Atoi(matches[2])
	if err != nil || m < 0 || m > 59 {
		m = 0
	}
	return h, m
}

// NormalizeResetTime normalises a reset time string to "HH:MM" format.
func NormalizeResetTime(resetTime string) string {
	if resetTime == "" {
		return "00:00"
	}
	h, m := parseResetTime(resetTime)
	return fmt.Sprintf("%02d:%02d", h, m)
}

// GetPeriodTimeRange returns the time window [start, end) for a given period.
// timezone should be a valid IANA timezone string (e.g. "Asia/Shanghai").
// resetMode and resetTime only affect PeriodDaily.
func GetPeriodTimeRange(period Period, now time.Time, timezone string, resetMode DailyResetMode, resetTime string) (TimeRange, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return TimeRange{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	end := now
	var start time.Time

	switch period {
	case Period5H:
		start = Get5hWindowStart(now)
	case PeriodDaily:
		start = GetDailyWindowStart(now, loc, resetMode, resetTime)
	case PeriodWeekly:
		start = GetWeeklyWindowStart(now, loc)
	case PeriodMonthly:
		start = GetMonthlyWindowStart(now, loc)
	case PeriodTotal:
		// Total: since the beginning of time
		start = time.Time{}
	default:
		return TimeRange{}, fmt.Errorf("unsupported period: %s", period)
	}

	return TimeRange{Start: start, End: end}, nil
}

// GetPeriodEnd returns the end of the current period window.
func GetPeriodEnd(period Period, start time.Time, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	switch period {
	case Period5H:
		return start.Add(5 * time.Hour), nil
	case PeriodDaily:
		return start.Add(24 * time.Hour), nil
	case PeriodWeekly:
		return start.AddDate(0, 0, 7), nil
	case PeriodMonthly:
		// Go to the same day next month in the configured timezone
		zonedStart := start.In(loc)
		nextMonth := zonedStart.AddDate(0, 1, 0)
		return nextMonth.In(time.UTC), nil
	case PeriodTotal:
		// Total never expires; use a far-future sentinel
		return time.Date(9999, 12, 31, 23, 59, 59, 0, time.UTC), nil
	default:
		return time.Time{}, fmt.Errorf("unsupported period: %s", period)
	}
}

// GetTTLForPeriod returns the TTL (in seconds) from now until the end of the period window.
func GetTTLForPeriod(period Period, now time.Time, timezone string, resetMode DailyResetMode, resetTime string) (int, error) {
	tr, err := GetPeriodTimeRange(period, now, timezone, resetMode, resetTime)
	if err != nil {
		return 0, err
	}

	end, err := GetPeriodEnd(period, tr.Start, timezone)
	if err != nil {
		return 0, err
	}

	ttl := int(end.Sub(now).Seconds())
	if ttl < 1 {
		ttl = 1
	}
	return ttl, nil
}

// Get5hWindowStart returns the start of the rolling 5-hour window.
func Get5hWindowStart(now time.Time) time.Time {
	return now.Add(-5 * time.Hour)
}

// GetDailyWindowStart returns the start of the daily window, supporting fixed
// and rolling modes.
func GetDailyWindowStart(now time.Time, loc *time.Location, resetMode DailyResetMode, resetTime string) time.Time {
	if resetMode == ResetModeRolling {
		// Rolling: past 24 hours
		return now.Add(-24 * time.Hour)
	}

	// Fixed: reset at the configured time each day in the given timezone
	h, m := parseResetTime(NormalizeResetTime(resetTime))
	zonedNow := now.In(loc)

	resetToday := time.Date(
		zonedNow.Year(), zonedNow.Month(), zonedNow.Day(),
		h, m, 0, 0, loc,
	)

	if now.Before(resetToday) {
		// Before today's reset -> window started at yesterday's reset time
		return resetToday.AddDate(0, 0, -1)
	}
	return resetToday
}

// GetWeeklyWindowStart returns Monday 00:00 of the current week in the given
// timezone.
func GetWeeklyWindowStart(now time.Time, loc *time.Location) time.Time {
	zonedNow := now.In(loc)

	// time.Monday == 1; compute offset to get to Monday
	weekday := zonedNow.Weekday()
	if weekday == time.Sunday {
		weekday = 7
	}
	daysSinceMonday := int(weekday) - 1

	monday := time.Date(
		zonedNow.Year(), zonedNow.Month(), zonedNow.Day()-daysSinceMonday,
		0, 0, 0, 0, loc,
	)
	return monday
}

// GetMonthlyWindowStart returns the 1st of the current month at 00:00 in the
// given timezone.
func GetMonthlyWindowStart(now time.Time, loc *time.Location) time.Time {
	zonedNow := now.In(loc)
	return time.Date(
		zonedNow.Year(), zonedNow.Month(), 1,
		0, 0, 0, 0, loc,
	)
}
