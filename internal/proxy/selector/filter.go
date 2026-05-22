package selector

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// DefaultProviderGroup is the default group name when no group is specified.
const DefaultProviderGroup = "default"

// AllGroup is a special group name that matches all providers.
const AllGroup = "all"

// TimeOfDay represents a time within a day as minutes since midnight.
type TimeOfDay struct {
	Hour   int
	Minute int
}

// Minutes returns the total minutes since midnight.
func (t TimeOfDay) Minutes() int {
	return t.Hour*60 + t.Minute
}

// defaultNowFunc returns the current time of day in UTC.
func defaultNowFunc() TimeOfDay {
	now := time.Now().UTC()
	return TimeOfDay{Hour: now.Hour(), Minute: now.Minute()}
}

// parseHHMM parses a "HH:mm" string into minutes since midnight. Returns -1 on error.
func parseHHMM(s string) int {
	if s == "" {
		return -1
	}
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return -1
	}
	h, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil || h < 0 || h > 23 {
		return -1
	}
	m, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil || m < 0 || m > 59 {
		return -1
	}
	return h*60 + m
}

// getCurrentMinutesInTimezone returns the current minutes-since-midnight in the given timezone.
// If timezone is empty or invalid, falls back to UTC.
func getCurrentMinutesInTimezone(now TimeOfDay, timezone string) int {
	// For now, timezone shifting is simplified: the caller provides a TimeOfDay
	// already computed in the correct timezone. The timezone parameter is kept
	// for forward compatibility when we add full tz support.
	_ = timezone
	return now.Minutes()
}

// IsProviderActiveNow checks if a provider is within its active time window.
// If start or end is nil, the provider is always active (fail-open).
// If start == end, the provider is never active (disabled schedule).
func IsProviderActiveNow(startTime, endTime *string, now TimeOfDay, timezone string) bool {
	if startTime == nil || endTime == nil {
		return true
	}
	if *startTime == *endTime {
		return false
	}

	nowMinutes := getCurrentMinutesInTimezone(now, timezone)
	startMinutes := parseHHMM(*startTime)
	endMinutes := parseHHMM(*endTime)

	// Fail-open: malformed values => treat as always active
	if startMinutes < 0 || endMinutes < 0 {
		return true
	}

	if startMinutes < endMinutes {
		// Same-day window: start <= now < end
		return nowMinutes >= startMinutes && nowMinutes < endMinutes
	}

	// Cross-day window: now >= start || now < end
	return nowMinutes >= startMinutes || nowMinutes < endMinutes
}

// formatToProviderTypes maps a request format to compatible provider types.
func formatToProviderTypes(format string) []string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "claude":
		return []string{"claude", "claude-auth"}
	case "codex", "response":
		return []string{"codex"}
	case "openai":
		return []string{"openai-compatible"}
	case "gemini":
		return []string{"gemini"}
	case "gemini-cli":
		return []string{"gemini-cli"}
	default:
		return nil // no filter
	}
}

// CheckFormatCompatibility checks if a provider type is compatible with a request format.
func CheckFormatCompatibility(format, providerType string) bool {
	compatibleTypes := formatToProviderTypes(format)
	if len(compatibleTypes) == 0 {
		return true // unknown format, allow all
	}
	for _, t := range compatibleTypes {
		if t == providerType {
			return true
		}
	}
	return false
}

// parseProviderGroups splits a comma-separated group string into a cleaned slice.
func parseProviderGroups(groupStr string) []string {
	if groupStr == "" {
		return nil
	}
	parts := strings.Split(groupStr, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// CheckProviderGroupMatch checks if a provider's group tag matches any of the user's groups.
// providerGroupTag may be nil (treated as "default").
// userGroups is a comma-separated string of group names.
func CheckProviderGroupMatch(providerGroupTag *string, userGroups string) bool {
	groups := parseProviderGroups(userGroups)
	if len(groups) == 0 {
		return true // no group restriction
	}

	// "all" matches everything
	for _, g := range groups {
		if g == AllGroup {
			return true
		}
	}

	// Provider tags: nil/empty => ["default"]
	var providerTags []string
	if providerGroupTag == nil || strings.TrimSpace(*providerGroupTag) == "" {
		providerTags = []string{DefaultProviderGroup}
	} else {
		providerTags = parseProviderGroups(*providerGroupTag)
	}

	// Check intersection
	for _, pt := range providerTags {
		for _, g := range groups {
			if pt == g {
				return true
			}
		}
	}
	return false
}

// FilterByGroup implements Stage 2: group pre-filtering.
func FilterByGroup(providers []*model.Provider, providerGroup string) []*model.Provider {
	if providerGroup == "" {
		return providers
	}

	var result []*model.Provider
	for _, p := range providers {
		if CheckProviderGroupMatch(p.GroupTag, providerGroup) {
			result = append(result, p)
		}
	}
	return result
}

// IsClientAllowed checks if a user-agent is allowed by a provider's client restrictions.
// Empty allowedClients and blockedClients means no restriction (allowed).
func IsClientAllowed(allowedClients, blockedClients []string, userAgent string) bool {
	if len(allowedClients) == 0 && len(blockedClients) == 0 {
		return true
	}

	ua := strings.TrimSpace(userAgent)
	normalizedUA := strings.ToLower(ua)

	// Check blocklist first
	for _, pattern := range blockedClients {
		if clientPatternMatches(pattern, ua, normalizedUA) {
			return false
		}
	}

	// If no allowlist, and not blocked, allowed
	if len(allowedClients) == 0 {
		return true
	}

	// Check allowlist
	for _, pattern := range allowedClients {
		if clientPatternMatches(pattern, ua, normalizedUA) {
			return true
		}
	}

	return false // not on allowlist
}

// clientPatternMatches checks if a user-agent matches a client pattern.
// Supports simple glob patterns with * wildcard.
func clientPatternMatches(pattern, ua, normalizedUA string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if ua == "" {
		return false
	}

	normalizedPattern := strings.ToLower(pattern)

	if strings.Contains(normalizedPattern, "*") {
		return globMatch(normalizedPattern, normalizedUA)
	}

	return strings.Contains(normalizedUA, normalizedPattern)
}

// globMatch performs a simple glob match where * matches any sequence of characters.
func globMatch(pattern, text string) bool {
	// Simple glob: split pattern on *, and check all parts appear in order
	parts := strings.Split(pattern, "*")

	// Filter empty parts
	var nonEmpty []string
	for _, p := range parts {
		if p != "" {
			nonEmpty = append(nonEmpty, p)
		}
	}

	if len(nonEmpty) == 0 {
		return true // pattern is all wildcards
	}

	// Check prefix constraint (pattern doesn't start with *)
	if !strings.HasPrefix(pattern, "*") {
		if !strings.HasPrefix(text, nonEmpty[0]) {
			return false
		}
	}

	// Check suffix constraint (pattern doesn't end with *)
	if !strings.HasSuffix(pattern, "*") {
		if !strings.HasSuffix(text, nonEmpty[len(nonEmpty)-1]) {
			return false
		}
	}

	// Check all parts appear in order
	pos := 0
	for _, part := range nonEmpty {
		idx := strings.Index(text[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}

	return true
}

// FilterByClientRestriction implements Stage 3: client restriction filtering.
func FilterByClientRestriction(providers []*model.Provider, userAgent string) []*model.Provider {
	var result []*model.Provider
	for _, p := range providers {
		if IsClientAllowed(p.AllowedClients, p.BlockedClients, userAgent) {
			result = append(result, p)
		}
	}
	return result
}

// isExcluded checks if a provider ID is in the exclude list.
func isExcluded(providerID int, excludeIDs []int) bool {
	for _, id := range excludeIDs {
		if id == providerID {
			return true
		}
	}
	return false
}

// FilterBasic implements Stage 4: enabled, schedule, format-type, model, exclude.
func FilterBasic(
	providers []*model.Provider,
	requestedModel string,
	format string,
	excludeIDs []int,
	now TimeOfDay,
	timezone string,
) []*model.Provider {
	var result []*model.Provider
	for _, p := range providers {
		// Enabled check
		if p.IsEnabled == nil || !*p.IsEnabled {
			continue
		}

		// Exclude check
		if isExcluded(p.ID, excludeIDs) {
			continue
		}

		// Schedule check
		if !IsProviderActiveNow(p.ActiveTimeStart, p.ActiveTimeEnd, now, timezone) {
			continue
		}

		// Format-type compatibility
		if format != "" && !CheckFormatCompatibility(format, p.ProviderType) {
			continue
		}

		// Model support
		if requestedModel != "" && !p.SupportsModel(requestedModel) {
			continue
		}

		result = append(result, p)
	}
	return result
}

// FilterEnabled is a convenience filter that removes disabled providers.
func FilterEnabled(providers []*model.Provider) []*model.Provider {
	var result []*model.Provider
	for _, p := range providers {
		if p.IsEnabled != nil && *p.IsEnabled {
			result = append(result, p)
		}
	}
	return result
}

// String returns a debug summary of the format compatibility mapping.
func FormatMappingSummary() string {
	mappings := []struct{ format, types string }{
		{"claude", "claude, claude-auth"},
		{"codex/response", "codex"},
		{"openai", "openai-compatible"},
		{"gemini", "gemini"},
		{"gemini-cli", "gemini-cli"},
	}
	var sb strings.Builder
	for _, m := range mappings {
		sb.WriteString(fmt.Sprintf("  %s -> [%s]\n", m.format, m.types))
	}
	return sb.String()
}
