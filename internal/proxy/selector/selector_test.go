package selector

import (
	"context"
	"fmt"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// --- Test helpers ---

func boolPtr(b bool) *bool       { return &b }
func intPtr(i int) *int          { return &i }
func stringPtr(s string) *string { return &s }

func decimalPtr(s string) *udecimal.Decimal {
	d := udecimal.MustParse(s)
	return &d
}

func makeProvider(id int, name string, opts ...func(*model.Provider)) *model.Provider {
	p := &model.Provider{
		ID:           id,
		Name:         name,
		IsEnabled:    boolPtr(true),
		Weight:       intPtr(1),
		Priority:     intPtr(0),
		ProviderType: "claude",
		AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func withWeight(w int) func(*model.Provider) {
	return func(p *model.Provider) { p.Weight = intPtr(w) }
}

func withPriority(pri int) func(*model.Provider) {
	return func(p *model.Provider) { p.Priority = intPtr(pri) }
}

func withGroupTag(tag string) func(*model.Provider) {
	return func(p *model.Provider) { p.GroupTag = stringPtr(tag) }
}

func withGroupPriorities(gp map[string]int) func(*model.Provider) {
	return func(p *model.Provider) { p.GroupPriorities = gp }
}

func withProviderType(pt string) func(*model.Provider) {
	return func(p *model.Provider) { p.ProviderType = pt }
}

func withModels(models ...string) func(*model.Provider) {
	return func(p *model.Provider) { p.AllowedModels = model.ExactAllowedModelRules(models...) }
}

func withAllowedModelRules(rules model.AllowedModelRules) func(*model.Provider) {
	return func(p *model.Provider) { p.AllowedModels = rules }
}

func withDisabled() func(*model.Provider) {
	return func(p *model.Provider) { p.IsEnabled = boolPtr(false) }
}

func withSchedule(start, end string) func(*model.Provider) {
	return func(p *model.Provider) {
		p.ActiveTimeStart = stringPtr(start)
		p.ActiveTimeEnd = stringPtr(end)
	}
}

func withAllowedClients(clients ...string) func(*model.Provider) {
	return func(p *model.Provider) { p.AllowedClients = clients }
}

func withBlockedClients(clients ...string) func(*model.Provider) {
	return func(p *model.Provider) { p.BlockedClients = clients }
}

func withCostMultiplier(cost string) func(*model.Provider) {
	return func(p *model.Provider) { p.CostMultiplier = decimalPtr(cost) }
}

func withConcurrentLimit(limit int) func(*model.Provider) {
	return func(p *model.Provider) { p.LimitConcurrentSessions = intPtr(limit) }
}

func withDisableSessionReuse() func(*model.Provider) {
	return func(p *model.Provider) { p.DisableSessionReuse = true }
}

// --- Mock implementations ---

type mockProviderRepo struct {
	providers []*model.Provider
	byID      map[int]*model.Provider
}

func newMockProviderRepo(providers ...*model.Provider) *mockProviderRepo {
	byID := make(map[int]*model.Provider)
	for _, p := range providers {
		byID[p.ID] = p
	}
	return &mockProviderRepo{providers: providers, byID: byID}
}

func (r *mockProviderRepo) GetActiveProviders(_ context.Context) ([]*model.Provider, error) {
	return r.providers, nil
}

func (r *mockProviderRepo) GetByID(_ context.Context, id int) (*model.Provider, error) {
	p, ok := r.byID[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return p, nil
}

type mockGroupRepo struct {
	multipliers map[string]udecimal.Decimal
}

func (r *mockGroupRepo) GetCostMultiplier(_ context.Context, group string) (udecimal.Decimal, error) {
	m, ok := r.multipliers[group]
	if !ok {
		return udecimal.MustParse("1.0"), nil
	}
	return m, nil
}

type mockCircuitBreaker struct {
	open map[int]bool
}

func newMockCircuitBreaker() *mockCircuitBreaker {
	return &mockCircuitBreaker{open: make(map[int]bool)}
}

func (cb *mockCircuitBreaker) IsOpen(p *model.Provider) bool {
	return cb.open[p.ID]
}

func (cb *mockCircuitBreaker) setOpen(id int) {
	cb.open[id] = true
}

type mockSessionService struct {
	bindings map[string]int
}

func newMockSessionService() *mockSessionService {
	return &mockSessionService{bindings: make(map[string]int)}
}

func (s *mockSessionService) GetBoundProvider(_ context.Context, sessionID string) (int, error) {
	id, ok := s.bindings[sessionID]
	if !ok {
		return 0, nil
	}
	return id, nil
}

func (s *mockSessionService) BindProvider(_ context.Context, sessionID string, providerID int) {
	s.bindings[sessionID] = providerID
}

type mockCostChecker struct {
	blocked map[int]bool
}

func newMockCostChecker() *mockCostChecker {
	return &mockCostChecker{blocked: make(map[int]bool)}
}

func (c *mockCostChecker) CheckCostLimits(_ context.Context, p *model.Provider) (bool, error) {
	return !c.blocked[p.ID], nil
}

type mockConcurrency struct {
	full map[int]bool
}

func newMockConcurrency() *mockConcurrency {
	return &mockConcurrency{full: make(map[int]bool)}
}

func (c *mockConcurrency) CheckAndTrack(_ context.Context, providerID int, _ string, _ int) (bool, int, error) {
	if c.full[providerID] {
		return false, 999, nil
	}
	return true, 1, nil
}

// --- Filter tests ---

func TestIsProviderActiveNow(t *testing.T) {
	tests := []struct {
		name   string
		start  *string
		end    *string
		now    TimeOfDay
		expect bool
	}{
		{"nil start", nil, stringPtr("18:00"), TimeOfDay{12, 0}, true},
		{"nil end", stringPtr("09:00"), nil, TimeOfDay{12, 0}, true},
		{"both nil", nil, nil, TimeOfDay{12, 0}, true},
		{"same start and end (disabled)", stringPtr("09:00"), stringPtr("09:00"), TimeOfDay{12, 0}, false},
		{"within same-day window", stringPtr("09:00"), stringPtr("18:00"), TimeOfDay{12, 0}, true},
		{"before same-day window", stringPtr("09:00"), stringPtr("18:00"), TimeOfDay{7, 0}, false},
		{"after same-day window", stringPtr("09:00"), stringPtr("18:00"), TimeOfDay{19, 0}, false},
		{"cross-day: within after start", stringPtr("22:00"), stringPtr("06:00"), TimeOfDay{23, 0}, true},
		{"cross-day: within before end", stringPtr("22:00"), stringPtr("06:00"), TimeOfDay{3, 0}, true},
		{"cross-day: outside", stringPtr("22:00"), stringPtr("06:00"), TimeOfDay{12, 0}, false},
		{"malformed start", stringPtr("xx:00"), stringPtr("18:00"), TimeOfDay{12, 0}, true},
		{"malformed end", stringPtr("09:00"), stringPtr("xx:00"), TimeOfDay{12, 0}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsProviderActiveNow(tt.start, tt.end, tt.now, "UTC")
			if got != tt.expect {
				t.Errorf("IsProviderActiveNow(%v, %v, %v) = %v, want %v",
					tt.start, tt.end, tt.now, got, tt.expect)
			}
		})
	}
}

func TestCheckFormatCompatibility(t *testing.T) {
	tests := []struct {
		format       string
		providerType string
		expect       bool
	}{
		{"claude", "claude", true},
		{"claude", "claude-auth", true},
		{"claude", "openai-compatible", false},
		{"claude", "codex", false},
		{"codex", "codex", true},
		{"response", "codex", true},
		{"codex", "claude", false},
		{"openai", "openai-compatible", true},
		{"openai", "claude", false},
		{"gemini", "gemini", true},
		{"gemini", "gemini-cli", false},
		{"gemini-cli", "gemini-cli", true},
		{"gemini-cli", "gemini", false},
		{"", "anything", true},      // empty format = no filter
		{"unknown", "claude", true},  // unknown format = no filter
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s+%s", tt.format, tt.providerType)
		t.Run(name, func(t *testing.T) {
			got := CheckFormatCompatibility(tt.format, tt.providerType)
			if got != tt.expect {
				t.Errorf("CheckFormatCompatibility(%q, %q) = %v, want %v",
					tt.format, tt.providerType, got, tt.expect)
			}
		})
	}
}

func TestCheckProviderGroupMatch(t *testing.T) {
	tests := []struct {
		name        string
		providerTag *string
		userGroups  string
		expect      bool
	}{
		{"no user groups", stringPtr("premium"), "", true},
		{"user all group", stringPtr("premium"), "all", true},
		{"exact match", stringPtr("premium"), "premium", true},
		{"no match", stringPtr("premium"), "standard", false},
		{"provider nil tag vs default", nil, "default", true},
		{"provider nil tag vs non-default", nil, "premium", false},
		{"multi-user-groups with match", stringPtr("premium"), "standard,premium", true},
		{"multi-provider-tags with match", stringPtr("cli,chat"), "cli", true},
		{"multi-provider-tags no match", stringPtr("cli,chat"), "premium", false},
		{"provider empty tag vs default", stringPtr(""), "default", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckProviderGroupMatch(tt.providerTag, tt.userGroups)
			if got != tt.expect {
				t.Errorf("CheckProviderGroupMatch(%v, %q) = %v, want %v",
					tt.providerTag, tt.userGroups, got, tt.expect)
			}
		})
	}
}

func TestIsClientAllowed(t *testing.T) {
	tests := []struct {
		name      string
		allowed   []string
		blocked   []string
		userAgent string
		expect    bool
	}{
		{"no restrictions", nil, nil, "anything", true},
		{"blocked exact", nil, []string{"badbot"}, "BadBot/1.0", false},
		{"blocked glob", nil, []string{"bad*"}, "BadBot/1.0", false},
		{"not blocked", nil, []string{"badbot"}, "GoodBot/1.0", true},
		{"allowlist match", []string{"goodbot"}, nil, "GoodBot/1.0", true},
		{"allowlist miss", []string{"goodbot"}, nil, "OtherBot/1.0", false},
		{"blocklist takes precedence", []string{"bot"}, []string{"badbot"}, "BadBot/1.0", false},
		{"allowlist with glob", []string{"claude*"}, nil, "Claude Code/1.0", true},
		{"empty user agent", []string{"claude"}, nil, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsClientAllowed(tt.allowed, tt.blocked, tt.userAgent)
			if got != tt.expect {
				t.Errorf("IsClientAllowed(%v, %v, %q) = %v, want %v",
					tt.allowed, tt.blocked, tt.userAgent, got, tt.expect)
			}
		})
	}
}

func TestFilterByGroup(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Premium", withGroupTag("premium")),
		makeProvider(2, "Standard", withGroupTag("standard")),
		makeProvider(3, "Default"),
	}

	t.Run("no group filter", func(t *testing.T) {
		result := FilterByGroup(providers, "")
		if len(result) != 3 {
			t.Errorf("expected 3, got %d", len(result))
		}
	})

	t.Run("premium group", func(t *testing.T) {
		result := FilterByGroup(providers, "premium")
		if len(result) != 1 || result[0].ID != 1 {
			t.Errorf("expected [premium], got %v", result)
		}
	})

	t.Run("default group includes nil tag", func(t *testing.T) {
		result := FilterByGroup(providers, "default")
		if len(result) != 1 || result[0].ID != 3 {
			t.Errorf("expected [Default], got %v", result)
		}
	})

	t.Run("all group", func(t *testing.T) {
		result := FilterByGroup(providers, "all")
		if len(result) != 3 {
			t.Errorf("expected 3, got %d", len(result))
		}
	})
}

func TestFilterBasic(t *testing.T) {
	now := TimeOfDay{12, 0}
	providers := []*model.Provider{
		makeProvider(1, "Enabled Claude", withProviderType("claude")),
		makeProvider(2, "Disabled", withDisabled()),
		makeProvider(3, "OpenAI Provider", withProviderType("openai-compatible"), withModels("gpt-4")),
		makeProvider(4, "Active Schedule", withSchedule("09:00", "18:00")),
		makeProvider(5, "Inactive Schedule", withSchedule("20:00", "06:00")),
	}

	t.Run("filter by format", func(t *testing.T) {
		result := FilterBasic(providers, "claude-opus-4", "claude", nil, now, "UTC")
		// Expect: 1 (enabled claude), 4 (active schedule, claude type)
		for _, p := range result {
			if p.ID == 2 || p.ID == 3 || p.ID == 5 {
				t.Errorf("unexpected provider %d in result", p.ID)
			}
		}
	})

	t.Run("filter by exclude", func(t *testing.T) {
		result := FilterBasic(providers, "claude-opus-4", "claude", []int{1}, now, "UTC")
		for _, p := range result {
			if p.ID == 1 {
				t.Error("expected provider 1 to be excluded")
			}
		}
	})

	t.Run("filter disabled", func(t *testing.T) {
		result := FilterBasic(providers, "", "", nil, now, "UTC")
		for _, p := range result {
			if p.ID == 2 {
				t.Error("disabled provider should be filtered")
			}
		}
	})
}

func TestFilterByClientRestriction(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "No restriction"),
		makeProvider(2, "Allow Claude", withAllowedClients("claude")),
		makeProvider(3, "Block BadBot", withBlockedClients("badbot")),
	}

	t.Run("claude user agent", func(t *testing.T) {
		result := FilterByClientRestriction(providers, "Claude Code/1.0")
		ids := make(map[int]bool)
		for _, p := range result {
			ids[p.ID] = true
		}
		if !ids[1] || !ids[2] || !ids[3] {
			t.Errorf("expected IDs {1,2,3}, got %v", ids)
		}
	})

	t.Run("other user agent", func(t *testing.T) {
		result := FilterByClientRestriction(providers, "OtherAgent/1.0")
		ids := make(map[int]bool)
		for _, p := range result {
			ids[p.ID] = true
		}
		if !ids[1] || ids[2] || !ids[3] {
			t.Errorf("expected IDs {1,3}, got %v", ids)
		}
	})

	t.Run("blocked user agent", func(t *testing.T) {
		result := FilterByClientRestriction(providers, "BadBot/1.0")
		ids := make(map[int]bool)
		for _, p := range result {
			ids[p.ID] = true
		}
		if !ids[1] || ids[2] || ids[3] {
			t.Errorf("expected IDs {1}, got %v", ids)
		}
	})
}

// --- Priority tests ---

func TestResolveEffectivePriority(t *testing.T) {
	t.Run("no override", func(t *testing.T) {
		p := makeProvider(1, "P1", withPriority(5))
		got := ResolveEffectivePriority(p, "")
		if got != 5 {
			t.Errorf("expected 5, got %d", got)
		}
	})

	t.Run("with group override", func(t *testing.T) {
		p := makeProvider(1, "P1", withPriority(5), withGroupPriorities(map[string]int{
			"premium": 1,
			"vip":     2,
		}))
		got := ResolveEffectivePriority(p, "premium")
		if got != 1 {
			t.Errorf("expected 1, got %d", got)
		}
	})

	t.Run("multi-group takes minimum", func(t *testing.T) {
		p := makeProvider(1, "P1", withPriority(5), withGroupPriorities(map[string]int{
			"premium": 3,
			"vip":     1,
		}))
		got := ResolveEffectivePriority(p, "premium,vip")
		if got != 1 {
			t.Errorf("expected 1 (min of premium=3, vip=1), got %d", got)
		}
	})

	t.Run("no matching group override", func(t *testing.T) {
		p := makeProvider(1, "P1", withPriority(5), withGroupPriorities(map[string]int{
			"premium": 1,
		}))
		got := ResolveEffectivePriority(p, "standard")
		if got != 5 {
			t.Errorf("expected 5 (fallback), got %d", got)
		}
	})

	t.Run("nil priority defaults to 0", func(t *testing.T) {
		p := makeProvider(1, "P1")
		p.Priority = nil
		got := ResolveEffectivePriority(p, "")
		if got != 0 {
			t.Errorf("expected 0 (default), got %d", got)
		}
	})
}

func TestSelectTopPriority(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Low", withPriority(10)),
		makeProvider(2, "High", withPriority(0)),
		makeProvider(3, "High2", withPriority(0)),
		makeProvider(4, "Medium", withPriority(5)),
	}

	t.Run("selects highest priority tier", func(t *testing.T) {
		result := SelectTopPriority(providers, "")
		if len(result) != 2 {
			t.Fatalf("expected 2 providers at priority 0, got %d", len(result))
		}
		for _, p := range result {
			if p.ID != 2 && p.ID != 3 {
				t.Errorf("unexpected provider ID %d", p.ID)
			}
		}
	})

	t.Run("with group priority override", func(t *testing.T) {
		providers := []*model.Provider{
			makeProvider(1, "GlobalHigh", withPriority(0)),
			makeProvider(2, "GroupOverride", withPriority(10), withGroupPriorities(map[string]int{
				"premium": -1,
			})),
		}
		result := SelectTopPriority(providers, "premium")
		if len(result) != 1 || result[0].ID != 2 {
			t.Errorf("expected provider 2 (group override -1), got %v", result)
		}
	})
}

func TestGroupByPriority(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "P0", withPriority(0)),
		makeProvider(2, "P5a", withPriority(5)),
		makeProvider(3, "P5b", withPriority(5)),
		makeProvider(4, "P10", withPriority(10)),
	}

	tiers := GroupByPriority(providers, "")
	if len(tiers) != 3 {
		t.Fatalf("expected 3 tiers, got %d", len(tiers))
	}

	// Tier 0: priority 0
	if len(tiers[0]) != 1 || tiers[0][0].ID != 1 {
		t.Errorf("tier 0: expected [1], got %v", tiers[0])
	}

	// Tier 1: priority 5
	if len(tiers[1]) != 2 {
		t.Errorf("tier 1: expected 2 providers, got %d", len(tiers[1]))
	}

	// Tier 2: priority 10
	if len(tiers[2]) != 1 || tiers[2][0].ID != 4 {
		t.Errorf("tier 2: expected [4], got %v", tiers[2])
	}
}

// --- Weight tests ---

func TestSelectWeightedRandom_Distribution(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "W1", withWeight(1)),
		makeProvider(2, "W9", withWeight(9)),
	}

	counts := make(map[int]int)
	iterations := 10000

	for i := 0; i < iterations; i++ {
		p := SelectWeightedRandom(providers)
		if p == nil {
			t.Fatal("SelectWeightedRandom returned nil")
		}
		counts[p.ID]++
	}

	ratio := float64(counts[2]) / float64(iterations)
	if ratio < 0.85 || ratio > 0.95 {
		t.Errorf("expected W9 to be selected ~90%%, got %.2f%%", ratio*100)
	}
}

func TestSelectWeightedRandom_CostSorted(t *testing.T) {
	// Use seeded version for deterministic test
	providers := []*model.Provider{
		makeProvider(1, "Expensive", withWeight(10), withCostMultiplier("5.0")),
		makeProvider(2, "Cheap", withWeight(10), withCostMultiplier("1.0")),
	}

	// Both have same weight, so the ordering should be by cost (cheap first).
	// Since they have equal weight, distribution should be ~50/50
	// But the point is both appear in results.
	counts := make(map[int]int)
	for i := 0; i < 1000; i++ {
		p := SelectWeightedRandom(providers)
		counts[p.ID]++
	}

	if counts[1] == 0 || counts[2] == 0 {
		t.Error("expected both providers to be selected at least once")
	}
}

func TestSelectWeightedRandom_SingleProvider(t *testing.T) {
	p := makeProvider(1, "Only")
	result := SelectWeightedRandom([]*model.Provider{p})
	if result == nil || result.ID != 1 {
		t.Error("expected the single provider to be returned")
	}
}

func TestSelectWeightedRandom_Empty(t *testing.T) {
	result := SelectWeightedRandom(nil)
	if result != nil {
		t.Error("expected nil for empty slice")
	}
}

func TestSelectWeightedRandomWithSeed(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "A", withWeight(5)),
		makeProvider(2, "B", withWeight(5)),
	}

	// Same seed should produce same result
	r1 := SelectWeightedRandomWithSeed(providers, 42)
	r2 := SelectWeightedRandomWithSeed(providers, 42)
	if r1.ID != r2.ID {
		t.Errorf("same seed should produce same result: got %d and %d", r1.ID, r2.ID)
	}
}

// --- Integration tests for the full selector pipeline ---

func TestSelect_BasicPipeline(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Provider1"),
	}

	repo := newMockProviderRepo(providers...)
	s := New(repo, nil, nil, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 1 {
		t.Errorf("expected provider 1, got %d", result.Provider.ID)
	}
	if result.SessionReused {
		t.Error("expected SessionReused=false")
	}
}

func TestSelect_SessionReuse(t *testing.T) {
	provider := makeProvider(1, "ReusableProvider")

	repo := newMockProviderRepo(provider)
	sessionSvc := newMockSessionService()
	sessionSvc.bindings["sess-123"] = 1

	s := New(repo, nil, nil, sessionSvc, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		SessionID: "sess-123",
		Providers: []*model.Provider{provider},
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 1 {
		t.Errorf("expected provider 1, got %d", result.Provider.ID)
	}
	if !result.SessionReused {
		t.Error("expected SessionReused=true")
	}
}

func TestSelect_SessionReuse_DisableSessionReuse(t *testing.T) {
	provider := makeProvider(1, "NoReuse", withDisableSessionReuse())

	repo := newMockProviderRepo(provider)
	sessionSvc := newMockSessionService()
	sessionSvc.bindings["sess-123"] = 1

	s := New(repo, nil, nil, sessionSvc, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		SessionID: "sess-123",
		Providers: []*model.Provider{provider},
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	// Should not reuse because DisableSessionReuse=true
	if result.SessionReused {
		t.Error("expected SessionReused=false because DisableSessionReuse=true")
	}
}

func TestSelect_SessionReuse_CircuitBreakerOpen(t *testing.T) {
	provider := makeProvider(1, "OpenCB")

	repo := newMockProviderRepo(provider)
	sessionSvc := newMockSessionService()
	sessionSvc.bindings["sess-123"] = 1
	cb := newMockCircuitBreaker()
	cb.setOpen(1)

	s := New(repo, nil, cb, sessionSvc, nil, nil)
	ctx := context.Background()

	// Provider 1 has circuit breaker open, so:
	// - Session reuse should be skipped (CB open)
	// - Normal selection should also fail (CB open, only provider)
	_, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		SessionID: "sess-123",
		Providers: []*model.Provider{provider},
	})

	if err == nil {
		t.Error("expected error: only provider has circuit breaker open")
	}
}

func TestSelect_CircuitBreakerFilters(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Open"),
		makeProvider(2, "Closed"),
	}

	cb := newMockCircuitBreaker()
	cb.setOpen(1)

	s := New(nil, nil, cb, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 2 {
		t.Errorf("expected provider 2 (1 has open CB), got %d", result.Provider.ID)
	}
}

func TestSelect_CostLimitFilters(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "OverLimit"),
		makeProvider(2, "UnderLimit"),
	}

	costChecker := newMockCostChecker()
	costChecker.blocked[1] = true

	s := New(nil, nil, nil, nil, costChecker, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 2 {
		t.Errorf("expected provider 2 (1 over limit), got %d", result.Provider.ID)
	}
}

func TestSelect_PrioritySelection(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "LowPri", withPriority(10)),
		makeProvider(2, "HighPri", withPriority(0)),
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 2 {
		t.Errorf("expected provider 2 (priority 0), got %d", result.Provider.ID)
	}
}

func TestSelect_GroupPreFilter(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Premium", withGroupTag("premium")),
		makeProvider(2, "Standard", withGroupTag("standard")),
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:         "claude-opus-4",
		ProviderGroup: "premium",
		Providers:     providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 1 {
		t.Errorf("expected provider 1 (premium), got %d", result.Provider.ID)
	}
}

func TestSelect_FormatFiltering(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Claude", withProviderType("claude")),
		makeProvider(2, "OpenAI", withProviderType("openai-compatible"), withModels("claude-opus-4")),
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Format:    "claude",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 1 {
		t.Errorf("expected provider 1 (claude format), got %d", result.Provider.ID)
	}
}

func TestSelect_ModelFiltering(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Claude Only", withModels("claude-opus-4")),
		makeProvider(2, "GPT Only", withModels("gpt-4")),
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 1 {
		t.Errorf("expected provider 1 (claude-opus-4), got %d", result.Provider.ID)
	}
}

func TestSelect_ModelMatching_Patterns(t *testing.T) {
	tests := []struct {
		name      string
		rules     model.AllowedModelRules
		model     string
		expectHit bool
	}{
		{
			"exact match",
			model.ExactAllowedModelRules("claude-opus-4"),
			"claude-opus-4",
			true,
		},
		{
			"exact miss",
			model.ExactAllowedModelRules("claude-opus-4"),
			"claude-sonnet-4",
			false,
		},
		{
			"prefix match",
			model.AllowedModelRules{{MatchType: "prefix", Pattern: "claude-"}},
			"claude-opus-4",
			true,
		},
		{
			"prefix miss",
			model.AllowedModelRules{{MatchType: "prefix", Pattern: "gpt-"}},
			"claude-opus-4",
			false,
		},
		{
			"suffix match",
			model.AllowedModelRules{{MatchType: "suffix", Pattern: "-opus-4"}},
			"claude-opus-4",
			true,
		},
		{
			"contains match",
			model.AllowedModelRules{{MatchType: "contains", Pattern: "opus"}},
			"claude-opus-4",
			true,
		},
		{
			"regex match",
			model.AllowedModelRules{{MatchType: "regex", Pattern: "^claude-.+"}},
			"claude-opus-4",
			true,
		},
		{
			"regex miss",
			model.AllowedModelRules{{MatchType: "regex", Pattern: "^gpt-.+"}},
			"claude-opus-4",
			false,
		},
		{
			"empty rules match all",
			model.AllowedModelRules{},
			"anything",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := makeProvider(1, "Test", withAllowedModelRules(tt.rules))
			got := p.SupportsModel(tt.model)
			if got != tt.expectHit {
				t.Errorf("SupportsModel(%q) = %v, want %v", tt.model, got, tt.expectHit)
			}
		})
	}
}

func TestSelect_ExcludeIDs(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Excluded"),
		makeProvider(2, "Available"),
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:      "claude-opus-4",
		ExcludeIDs: []int{1},
		Providers:  providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 2 {
		t.Errorf("expected provider 2, got %d", result.Provider.ID)
	}
}

func TestSelect_ConcurrencyCheck(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Full", withConcurrentLimit(1)),
		makeProvider(2, "Available", withConcurrentLimit(10)),
	}

	conc := newMockConcurrency()
	conc.full[1] = true

	s := New(nil, nil, nil, nil, nil, conc)
	ctx := context.Background()

	// Run multiple times to ensure we always get provider 2
	for i := 0; i < 20; i++ {
		result, err := s.Select(ctx, SelectRequest{
			Model:     "claude-opus-4",
			SessionID: "sess-test",
			Providers: providers,
		})

		if err != nil {
			t.Fatalf("Select() error: %v", err)
		}
		if result.Provider.ID != 2 {
			t.Errorf("attempt %d: expected provider 2, got %d", i, result.Provider.ID)
		}
	}
}

func TestSelect_AllProvidersUnavailable(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Disabled", withDisabled()),
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err == nil {
		t.Error("expected error for all disabled providers")
	}
}

func TestSelect_NoProviders(t *testing.T) {
	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	_, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: []*model.Provider{},
	})

	if err == nil {
		t.Error("expected error for empty provider list")
	}
}

func TestSelect_GroupCostMultiplier(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "P1", withGroupTag("premium")),
	}

	groupRepo := &mockGroupRepo{
		multipliers: map[string]udecimal.Decimal{
			"premium": udecimal.MustParse("1.5"),
		},
	}

	s := New(nil, groupRepo, nil, nil, nil, nil)
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:         "claude-opus-4",
		ProviderGroup: "premium",
		Providers:     providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	expected := udecimal.MustParse("1.5")
	if result.GroupCostMult.Cmp(expected) != 0 {
		t.Errorf("expected GroupCostMult=1.5, got %s", result.GroupCostMult.String())
	}
}

func TestSelect_ScheduleFiltering(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Active", withSchedule("09:00", "18:00")),
		makeProvider(2, "Inactive", withSchedule("20:00", "06:00")),
	}

	noon := func() TimeOfDay { return TimeOfDay{12, 0} }
	s := New(nil, nil, nil, nil, nil, nil, WithNowFunc(noon))
	ctx := context.Background()

	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 1 {
		t.Errorf("expected provider 1 (active at noon), got %d", result.Provider.ID)
	}
}

func TestSelect_ClientRestrictionFiltering(t *testing.T) {
	providers := []*model.Provider{
		makeProvider(1, "Claude Only", withAllowedClients("claude")),
		makeProvider(2, "Open"),
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()

	// Non-Claude user agent should only get provider 2
	result, err := s.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		UserAgent: "OtherClient/1.0",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if result.Provider.ID != 2 {
		t.Errorf("expected provider 2 (open), got %d", result.Provider.ID)
	}
}

// --- Glob match tests ---

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		expect  bool
	}{
		{"*", "anything", true},
		{"claude*", "claudecode", true},
		{"claude*", "other", false},
		{"*code", "claudecode", true},
		{"*code*", "claudecodesomething", true},
		{"claude*code", "claudeismycode", true},
		{"claude*code", "claudeismydog", false},
		{"exact", "exact", true},
		{"exact", "notexact", false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.pattern, tt.text), func(t *testing.T) {
			got := globMatch(tt.pattern, tt.text)
			if got != tt.expect {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.expect)
			}
		})
	}
}

// --- Benchmark ---

func BenchmarkSelect(b *testing.B) {
	providers := make([]*model.Provider, 20)
	for i := 0; i < 20; i++ {
		providers[i] = makeProvider(i+1, fmt.Sprintf("Provider %d", i+1))
	}

	s := New(nil, nil, nil, nil, nil, nil)
	ctx := context.Background()
	req := SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Select(ctx, req)
	}
}

func BenchmarkFilterBasic(b *testing.B) {
	providers := make([]*model.Provider, 100)
	for i := 0; i < 100; i++ {
		providers[i] = makeProvider(i+1, fmt.Sprintf("Provider %d", i+1))
	}
	now := TimeOfDay{12, 0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FilterBasic(providers, "claude-opus-4", "claude", nil, now, "UTC")
	}
}
