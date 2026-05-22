package selector

import (
	"math/rand"
	"sort"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// SelectWeightedRandom implements Stage 7: cost-sorted, then weighted random selection.
// Providers are first sorted by cost multiplier ascending (lower cost preferred),
// then selected using weighted random within that sorted order.
//
// Returns nil if providers is empty.
func SelectWeightedRandom(providers []*model.Provider) *model.Provider {
	if len(providers) == 0 {
		return nil
	}
	if len(providers) == 1 {
		return providers[0]
	}

	// Sort by cost multiplier ascending
	sorted := make([]*model.Provider, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		costI := getEffectiveCostMultiplier(sorted[i])
		costJ := getEffectiveCostMultiplier(sorted[j])
		return costI.Cmp(costJ) < 0
	})

	return weightedRandom(sorted)
}

// weightedRandom performs a weighted random selection from the given providers.
// Weight defaults to 1 if nil or <= 0.
func weightedRandom(providers []*model.Provider) *model.Provider {
	if len(providers) == 0 {
		return nil
	}
	if len(providers) == 1 {
		return providers[0]
	}

	totalWeight := 0
	for _, p := range providers {
		totalWeight += getEffectiveWeight(p)
	}

	if totalWeight == 0 {
		// All weights are 0, fall back to uniform random
		return providers[rand.Intn(len(providers))]
	}

	r := rand.Intn(totalWeight)
	cumulative := 0
	for _, p := range providers {
		cumulative += getEffectiveWeight(p)
		if r < cumulative {
			return p
		}
	}

	// Safety fallback (should not be reached)
	return providers[len(providers)-1]
}

// getEffectiveWeight returns the weight for selection, defaulting to 1.
func getEffectiveWeight(p *model.Provider) int {
	if p.Weight == nil || *p.Weight <= 0 {
		return 1
	}
	return *p.Weight
}

// getEffectiveCostMultiplier returns the cost multiplier, defaulting to 1.0.
func getEffectiveCostMultiplier(p *model.Provider) udecimal.Decimal {
	if p.CostMultiplier == nil || p.CostMultiplier.IsZero() {
		return udecimal.MustParse("1.0")
	}
	return *p.CostMultiplier
}

// SelectWeightedRandomWithSeed is like SelectWeightedRandom but uses a seeded random source.
// Useful for deterministic testing.
func SelectWeightedRandomWithSeed(providers []*model.Provider, seed int64) *model.Provider {
	if len(providers) == 0 {
		return nil
	}
	if len(providers) == 1 {
		return providers[0]
	}

	// Sort by cost multiplier ascending
	sorted := make([]*model.Provider, len(providers))
	copy(sorted, providers)
	sort.Slice(sorted, func(i, j int) bool {
		costI := getEffectiveCostMultiplier(sorted[i])
		costJ := getEffectiveCostMultiplier(sorted[j])
		return costI.Cmp(costJ) < 0
	})

	return weightedRandomSeeded(sorted, seed)
}

// weightedRandomSeeded performs weighted random selection with a seed.
func weightedRandomSeeded(providers []*model.Provider, seed int64) *model.Provider {
	if len(providers) == 0 {
		return nil
	}
	if len(providers) == 1 {
		return providers[0]
	}

	rng := rand.New(rand.NewSource(seed))

	totalWeight := 0
	for _, p := range providers {
		totalWeight += getEffectiveWeight(p)
	}

	if totalWeight == 0 {
		return providers[rng.Intn(len(providers))]
	}

	r := rng.Intn(totalWeight)
	cumulative := 0
	for _, p := range providers {
		cumulative += getEffectiveWeight(p)
		if r < cumulative {
			return p
		}
	}

	return providers[len(providers)-1]
}
