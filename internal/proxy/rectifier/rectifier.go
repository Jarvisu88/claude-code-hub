// Package rectifier provides request body rectifiers that normalize or fix
// incoming request payloads before they are forwarded to upstream providers.
//
// Each rectifier follows a consistent pattern:
//   - A public function that takes the request body (map[string]any) and returns a Result
//   - An "enabled" check function that consults system settings
//   - Results include an Applied bool and diagnostic details
//
// The RectifierChain aggregates multiple rectifiers and runs them in order.
package rectifier

import (
	"github.com/ding113/claude-code-hub/internal/model"
)

// Result describes the outcome of a single rectifier invocation.
type Result struct {
	// Name identifies which rectifier produced the result.
	Name string

	// Applied is true when the rectifier modified the request body.
	Applied bool

	// Details contains rectifier-specific diagnostic information.
	Details map[string]any
}

// SettingsProvider abstracts how system settings are fetched.
// Typically backed by a cached database lookup.
type SettingsProvider interface {
	Get() (*model.SystemSettings, error)
}

// Chain runs multiple rectifiers in order.
type Chain struct {
	settings SettingsProvider
}

// NewChain creates a rectifier chain with the given settings provider.
func NewChain(settings SettingsProvider) *Chain {
	return &Chain{settings: settings}
}

// Surface is a hint about which API surface the request targets, so that
// rectifiers can decide whether they are applicable.
type Surface string

const (
	SurfaceClaude Surface = "claude"
	SurfaceCodex  Surface = "codex"
	SurfaceOpenAI Surface = "openai"
	SurfaceGemini Surface = "gemini"
)

// Apply runs all enabled rectifiers against the request body and returns
// the list of results. It returns (results, anyApplied).
func (c *Chain) Apply(body map[string]any, surface Surface) ([]Result, bool) {
	if body == nil {
		return nil, false
	}

	var settings *model.SystemSettings
	if c.settings != nil {
		s, err := c.settings.Get()
		if err == nil {
			settings = s
		}
	}

	var results []Result
	anyApplied := false

	// 1. Thinking signature rectifier (Claude surface only)
	if surface == SurfaceClaude {
		if thinkingSignatureEnabled(settings) {
			r := RectifyThinkingSignature(body)
			results = append(results, r)
			if r.Applied {
				anyApplied = true
			}
		}
	}

	// 2. Thinking budget rectifier (Claude surface only)
	if surface == SurfaceClaude {
		if thinkingBudgetEnabled(settings) {
			r := RectifyThinkingBudget(body)
			results = append(results, r)
			if r.Applied {
				anyApplied = true
			}
		}
	}

	// 3. Billing header rectifier (Claude surface only)
	if surface == SurfaceClaude {
		if billingHeaderEnabled(settings) {
			r := RectifyBillingHeader(body)
			results = append(results, r)
			if r.Applied {
				anyApplied = true
			}
		}
	}

	// 4. Response input rectifier (Codex surface only)
	if surface == SurfaceCodex {
		if responseInputEnabled(settings) {
			r := RectifyResponseInput(body)
			results = append(results, r)
			if r.Applied {
				anyApplied = true
			}
		}
	}

	return results, anyApplied
}
