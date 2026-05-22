package selector

import (
	"math"

	"github.com/ding113/claude-code-hub/internal/model"
)

// ResolveEffectivePriority returns the effective priority for a provider,
// considering group-specific priority overrides.
//
// Logic:
//   - If userGroup is non-empty and the provider has groupPriorities entries
//     matching any of the user's groups, return the minimum override value.
//   - Otherwise, return the provider's global Priority (default 0).
//
// Lower priority values indicate higher precedence (priority 0 is highest).
func ResolveEffectivePriority(provider *model.Provider, userGroup string) int {
	if userGroup != "" && len(provider.GroupPriorities) > 0 {
		groups := parseProviderGroups(userGroup)
		minOverride := math.MaxInt
		found := false
		for _, g := range groups {
			if v, ok := provider.GroupPriorities[g]; ok {
				if v < minOverride {
					minOverride = v
				}
				found = true
			}
		}
		if found {
			return minOverride
		}
	}

	if provider.Priority != nil {
		return *provider.Priority
	}
	return 0
}

// SelectTopPriority implements Stage 6: select providers at the highest priority tier.
// Supports groupPriorities overrides via the userGroup parameter.
// Lower priority values take precedence (priority 0 is selected over priority 10).
func SelectTopPriority(providers []*model.Provider, userGroup string) []*model.Provider {
	if len(providers) == 0 {
		return nil
	}

	// Find minimum priority
	minPriority := math.MaxInt
	for _, p := range providers {
		pri := ResolveEffectivePriority(p, userGroup)
		if pri < minPriority {
			minPriority = pri
		}
	}

	// Filter to top priority tier
	var result []*model.Provider
	for _, p := range providers {
		if ResolveEffectivePriority(p, userGroup) == minPriority {
			result = append(result, p)
		}
	}
	return result
}

// GroupByPriority groups providers into tiers by effective priority (ascending).
// Each tier is a slice of providers sharing the same effective priority.
// Tiers are returned in ascending priority order (highest precedence first).
func GroupByPriority(providers []*model.Provider, userGroup string) [][]*model.Provider {
	if len(providers) == 0 {
		return nil
	}

	// Build priority -> providers map
	tierMap := make(map[int][]*model.Provider)
	var priorities []int

	for _, p := range providers {
		pri := ResolveEffectivePriority(p, userGroup)
		if _, exists := tierMap[pri]; !exists {
			priorities = append(priorities, pri)
		}
		tierMap[pri] = append(tierMap[pri], p)
	}

	// Sort priorities ascending (lower = higher precedence)
	sortInts(priorities)

	tiers := make([][]*model.Provider, 0, len(priorities))
	for _, pri := range priorities {
		tiers = append(tiers, tierMap[pri])
	}
	return tiers
}

// sortInts sorts a slice of ints in ascending order (insertion sort for small slices).
func sortInts(a []int) {
	for i := 1; i < len(a); i++ {
		key := a[i]
		j := i - 1
		for j >= 0 && a[j] > key {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = key
	}
}
