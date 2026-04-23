package model

import (
	"encoding/json"
	"testing"
)

func TestAllowedModelRulesMatchSupportsPatternTypes(t *testing.T) {
	provider := &Provider{AllowedModels: AllowedModelRules{
		{MatchType: "exact", Pattern: "gpt-5.4"},
		{MatchType: "prefix", Pattern: "claude-"},
		{MatchType: "suffix", Pattern: "-mini"},
		{MatchType: "contains", Pattern: "sonnet"},
		{MatchType: "regex", Pattern: `^o\d+$`},
	}}

	tests := []struct {
		model string
		want  bool
	}{
		{model: "gpt-5.4", want: true},
		{model: "claude-opus-4", want: true},
		{model: "gpt-4o-mini", want: true},
		{model: "claude-sonnet-4", want: true},
		{model: "o3", want: true},
		{model: "gemini-2.5-pro", want: false},
	}

	for _, tc := range tests {
		if got := provider.SupportsModel(tc.model); got != tc.want {
			t.Fatalf("SupportsModel(%q) = %v, want %v", tc.model, got, tc.want)
		}
	}
}

func TestAllowedModelRulesUnmarshalSupportsStringAndObjectRules(t *testing.T) {
	var rules AllowedModelRules
	if err := json.Unmarshal([]byte(`[
		"gpt-5.4",
		{"matchType":"prefix","pattern":"claude-"},
		{"pattern":"gpt-4o-mini"}
	]`), &rules); err != nil {
		t.Fatalf("unmarshal rules: %v", err)
	}

	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[0].MatchType != "exact" || rules[0].Pattern != "gpt-5.4" {
		t.Fatalf("unexpected first rule: %+v", rules[0])
	}
	if rules[1].MatchType != "prefix" || rules[1].Pattern != "claude-" {
		t.Fatalf("unexpected second rule: %+v", rules[1])
	}
	if rules[2].MatchType != "exact" || rules[2].Pattern != "gpt-4o-mini" {
		t.Fatalf("unexpected third rule default exact: %+v", rules[2])
	}
}

func TestAllowedModelRulesExactModelNamesOnlyReturnsExactRules(t *testing.T) {
	rules := AllowedModelRules{
		{MatchType: "exact", Pattern: "gpt-5.4"},
		{MatchType: "prefix", Pattern: "claude-"},
		{MatchType: "exact", Pattern: "gpt-4o-mini"},
	}

	got := rules.ExactModelNames()
	if len(got) != 2 || got[0] != "gpt-5.4" || got[1] != "gpt-4o-mini" {
		t.Fatalf("unexpected exact model names: %+v", got)
	}
}

func TestProviderModelRedirectRulesMatchSupportsPatternTypes(t *testing.T) {
	rules := ProviderModelRedirectRules{
		{MatchType: "exact", Source: "gpt-5.4", Target: "gpt-5.4-nano"},
		{MatchType: "prefix", Source: "claude-", Target: "glm-4.6"},
		{MatchType: "suffix", Source: "-mini", Target: "mini-router"},
		{MatchType: "contains", Source: "sonnet", Target: "sonnet-router"},
		{MatchType: "regex", Source: `^o\d+$`, Target: "reasoning-router"},
	}

	tests := []struct {
		model string
		want  string
	}{
		{model: "gpt-5.4", want: "gpt-5.4-nano"},
		{model: "claude-opus-4", want: "glm-4.6"},
		{model: "gpt-4o-mini", want: "mini-router"},
		{model: "claude-sonnet-4", want: "glm-4.6"},
		{model: "o3", want: "reasoning-router"},
		{model: "gemini-2.5-pro", want: ""},
	}

	for _, tc := range tests {
		got, ok := rules.Match(tc.model)
		if tc.want == "" {
			if ok {
				t.Fatalf("Match(%q) unexpectedly matched %q", tc.model, got)
			}
			continue
		}
		if !ok || got != tc.want {
			t.Fatalf("Match(%q) = (%q, %v), want (%q, true)", tc.model, got, ok, tc.want)
		}
	}
}

func TestProviderModelRedirectRulesUnmarshalSupportsMapAndArray(t *testing.T) {
	var legacy ProviderModelRedirectRules
	if err := json.Unmarshal([]byte(`{"gpt-5.4":"gpt-5.4-nano"}`), &legacy); err != nil {
		t.Fatalf("unmarshal legacy map: %v", err)
	}
	if len(legacy) != 1 || legacy[0].MatchType != "exact" || legacy[0].Source != "gpt-5.4" || legacy[0].Target != "gpt-5.4-nano" {
		t.Fatalf("unexpected legacy redirect rules: %+v", legacy)
	}

	var modern ProviderModelRedirectRules
	if err := json.Unmarshal([]byte(`[
		{"matchType":"prefix","source":"claude-","target":"glm-4.6"},
		{"source":"gpt-4o","target":"gpt-4o-mini"}
	]`), &modern); err != nil {
		t.Fatalf("unmarshal modern array: %v", err)
	}
	if len(modern) != 2 || modern[0].MatchType != "prefix" || modern[1].MatchType != "exact" {
		t.Fatalf("unexpected modern redirect rules: %+v", modern)
	}
}
