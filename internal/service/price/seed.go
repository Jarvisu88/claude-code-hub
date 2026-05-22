package price

import (
	"context"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

// SeedDefaultPrices inserts a curated set of common model prices when the
// model_prices table is empty. It is idempotent: calling it when records
// already exist is a no-op (the caller should check HasAnyRecords first).
func SeedDefaultPrices(ctx context.Context, repo PriceRepo) error {
	has, err := repo.HasAnyRecords(ctx)
	if err != nil {
		return err
	}
	if has {
		logger.Debug().Msg("price seed: records already exist, skipping")
		return nil
	}

	seeds := defaultSeeds()
	inserted := 0
	for _, mp := range seeds {
		if _, err := repo.Create(ctx, mp); err != nil {
			logger.Warn().Err(err).Str("model", mp.ModelName).Msg("price seed: failed to insert")
			continue
		}
		inserted++
	}
	logger.Info().Int("inserted", inserted).Msg("price seed: default prices seeded")
	return nil
}

// defaultSeeds returns the curated list of commonly used model prices.
// Prices are per-token (not per-million).
func defaultSeeds() []*model.ModelPrice {
	return []*model.ModelPrice{
		// --- Anthropic Claude ---
		seedEntry("claude-opus-4-20250514", "anthropic", "chat",
			15e-6, 75e-6, // input / output per token
			intPtr(200000), intPtr(32000),
			withCache(18.75e-6, 1.875e-6),
			withFeatures(true, true, true, true, true, true, true)),

		seedEntry("claude-sonnet-4-20250514", "anthropic", "chat",
			3e-6, 15e-6,
			intPtr(200000), intPtr(64000),
			withCache(3.75e-6, 0.3e-6),
			withFeatures(true, true, true, true, true, true, true)),

		seedEntry("claude-3-5-haiku-20241022", "anthropic", "chat",
			0.8e-6, 4e-6,
			intPtr(200000), intPtr(8192),
			withCache(1e-6, 0.08e-6),
			withFeatures(true, false, true, true, true, false, true)),

		// --- OpenAI GPT ---
		seedEntry("gpt-4o", "openai", "chat",
			2.5e-6, 10e-6,
			intPtr(128000), intPtr(16384),
			noCache(),
			withFeatures(false, false, true, false, false, true, true)),

		seedEntry("gpt-4o-mini", "openai", "chat",
			0.15e-6, 0.6e-6,
			intPtr(128000), intPtr(16384),
			noCache(),
			withFeatures(false, false, true, false, false, true, true)),

		seedEntry("o3", "openai", "chat",
			10e-6, 40e-6,
			intPtr(200000), intPtr(100000),
			noCache(),
			withFeatures(false, false, true, false, false, true, false)),

		seedEntry("o4-mini", "openai", "chat",
			1.1e-6, 4.4e-6,
			intPtr(200000), intPtr(100000),
			noCache(),
			withFeatures(false, false, true, false, false, true, false)),

		// --- Google Gemini ---
		seedEntry("gemini/gemini-2.5-pro-preview-05-06", "gemini", "chat",
			1.25e-6, 10e-6,
			intPtr(1048576), intPtr(65536),
			noCache(),
			withFeatures(false, false, true, false, false, true, true)),

		seedEntry("gemini/gemini-2.5-flash-preview-04-17", "gemini", "chat",
			0.15e-6, 0.6e-6,
			intPtr(1048576), intPtr(65536),
			noCache(),
			withFeatures(false, false, true, false, false, true, true)),
	}
}

// --- helpers ---

type cacheOpts struct {
	creation *float64
	read     *float64
}

func withCache(creation, read float64) cacheOpts {
	return cacheOpts{creation: f64Ptr(creation), read: f64Ptr(read)}
}

func noCache() cacheOpts { return cacheOpts{} }

type featureOpts struct {
	assistantPrefill bool
	computerUse      bool
	functionCalling  bool
	pdfInput         bool
	promptCaching    bool
	responseSchema   bool
	vision           bool
}

func withFeatures(prefill, computer, funcCall, pdf, cache, schema, vision bool) featureOpts {
	return featureOpts{
		assistantPrefill: prefill,
		computerUse:      computer,
		functionCalling:  funcCall,
		pdfInput:         pdf,
		promptCaching:    cache,
		responseSchema:   schema,
		vision:           vision,
	}
}

func seedEntry(
	name string,
	provider string,
	mode string,
	inputCost float64,
	outputCost float64,
	maxInput *int,
	maxOutput *int,
	cache cacheOpts,
	features featureOpts,
) *model.ModelPrice {
	pd := model.PriceData{
		InputCostPerToken:                  f64Ptr(inputCost),
		OutputCostPerToken:                 f64Ptr(outputCost),
		LitellmProvider:                    strPtr(provider),
		Mode:                               strPtr(mode),
		MaxInputTokens:                     maxInput,
		MaxOutputTokens:                    maxOutput,
		CacheCreationInputTokenCost:        cache.creation,
		CacheReadInputTokenCost:            cache.read,
		SupportsAssistantPrefill:           boolPtr(features.assistantPrefill),
		SupportsComputerUse:                boolPtr(features.computerUse),
		SupportsFunctionCalling:            boolPtr(features.functionCalling),
		SupportsPdfInput:                   boolPtr(features.pdfInput),
		SupportsPromptCaching:              boolPtr(features.promptCaching),
		SupportsResponseSchema:             boolPtr(features.responseSchema),
		SupportsVision:                     boolPtr(features.vision),
	}
	return &model.ModelPrice{
		ModelName: name,
		PriceData: pd,
		Source:    "seed",
	}
}

func f64Ptr(v float64) *float64 { return &v }
func intPtr(v int) *int         { return &v }
func strPtr(v string) *string   { return &v }
func boolPtr(v bool) *bool      { return &v }
