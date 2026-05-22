package proxy

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
)

// mockCircuitBreaker 模拟熔断器
type mockCircuitBreaker struct {
	openProviders map[int]bool
}

func newMockCircuitBreaker() *mockCircuitBreaker {
	return &mockCircuitBreaker{
		openProviders: make(map[int]bool),
	}
}

func (m *mockCircuitBreaker) IsOpen(providerID int) bool {
	return m.openProviders[providerID]
}

func (m *mockCircuitBreaker) setOpen(providerID int) {
	m.openProviders[providerID] = true
}

// helper functions
func boolPtr(b bool) *bool       { return &b }
func intPtr(i int) *int          { return &i }
func stringPtr(s string) *string { return &s }

// TestSelect_BasicSelection 测试基础选择
func TestSelect_BasicSelection(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Provider 1",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	result, err := selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 1 {
		t.Errorf("Expected provider ID 1, got %d", result.Provider.ID)
	}

	if result.RedirectedModel != "claude-opus-4" {
		t.Errorf("Expected model claude-opus-4, got %s", result.RedirectedModel)
	}
}

// TestSelect_ModelMatching 测试模型匹配
func TestSelect_ModelMatching(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:        1,
			Name:      "Claude Provider",
			IsEnabled: boolPtr(true),
			Weight:    intPtr(1),
			Priority:  intPtr(0),
			AllowedModels: model.AllowedModelRules{
				{MatchType: "prefix", Pattern: "claude-"},
			},
		},
		{
			ID:        2,
			Name:      "GPT Provider",
			IsEnabled: boolPtr(true),
			Weight:    intPtr(1),
			Priority:  intPtr(0),
			AllowedModels: model.AllowedModelRules{
				{MatchType: "prefix", Pattern: "gpt-"},
			},
		},
	}

	// 测试 Claude 模型
	result, err := selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 1 {
		t.Errorf("Expected Claude provider (ID 1), got %d", result.Provider.ID)
	}

	// 测试 GPT 模型
	result, err = selector.Select(ctx, SelectRequest{
		Model:     "gpt-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 2 {
		t.Errorf("Expected GPT provider (ID 2), got %d", result.Provider.ID)
	}
}

// TestSelect_Priority 测试优先级选择
func TestSelect_Priority(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Low Priority",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            2,
			Name:          "High Priority",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(10),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            3,
			Name:          "Medium Priority",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(5),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	// 应该选择高优先级的供应商
	result, err := selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 2 {
		t.Errorf("Expected high priority provider (ID 2), got %d", result.Provider.ID)
	}
}

// TestSelect_Weight 测试权重选择
func TestSelect_Weight(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Weight 1",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            2,
			Name:          "Weight 9",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(9),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	// 多次选择，统计分布
	counts := make(map[int]int)
	iterations := 1000

	for i := 0; i < iterations; i++ {
		result, err := selector.Select(ctx, SelectRequest{
			Model:     "claude-opus-4",
			Providers: providers,
		})
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}
		counts[result.Provider.ID]++
	}

	// 权重 9 的供应商应该被选中约 90% 的次数
	ratio := float64(counts[2]) / float64(iterations)
	if ratio < 0.8 || ratio > 0.95 {
		t.Errorf("Expected weight 9 provider to be selected ~90%% of the time, got %.2f%%", ratio*100)
	}
}

// TestSelect_GroupTag 测试分组标签
func TestSelect_GroupTag(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Premium Provider",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			GroupTag:      stringPtr("premium"),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            2,
			Name:          "Standard Provider",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			GroupTag:      stringPtr("standard"),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	// 选择 premium 组
	result, err := selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		GroupTag:  "premium",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 1 {
		t.Errorf("Expected premium provider (ID 1), got %d", result.Provider.ID)
	}

	// 选择 standard 组
	result, err = selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		GroupTag:  "standard",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 2 {
		t.Errorf("Expected standard provider (ID 2), got %d", result.Provider.ID)
	}
}

// TestSelect_CircuitBreaker 测试熔断器
func TestSelect_CircuitBreaker(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Provider 1",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            2,
			Name:          "Provider 2",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	// Provider 1 熔断
	cb.setOpen(1)

	// 应该选择 Provider 2
	result, err := selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 2 {
		t.Errorf("Expected provider 2 (1 is open), got %d", result.Provider.ID)
	}
}

// TestSelect_ExcludeProviders 测试排除供应商
func TestSelect_ExcludeProviders(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Provider 1",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            2,
			Name:          "Provider 2",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	// 排除 Provider 1
	result, err := selector.Select(ctx, SelectRequest{
		Model:              "claude-opus-4",
		Providers:          providers,
		ExcludeProviderIDs: []int{1},
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.Provider.ID != 2 {
		t.Errorf("Expected provider 2 (1 is excluded), got %d", result.Provider.ID)
	}
}

// TestSelect_ModelRedirect 测试模型重定向
func TestSelect_ModelRedirect(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:        1,
			Name:      "Provider with Redirect",
			IsEnabled: boolPtr(true),
			Weight:    intPtr(1),
			Priority:  intPtr(0),
			AllowedModels: model.AllowedModelRules{
				{MatchType: "exact", Pattern: "claude-opus-4"},
			},
			ModelRedirects: model.ProviderModelRedirectRules{
				{MatchType: "exact", Source: "claude-opus-4", Target: "claude-opus-20250514"},
			},
		},
	}

	result, err := selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if result.RedirectedModel != "claude-opus-20250514" {
		t.Errorf("Expected redirected model claude-opus-20250514, got %s", result.RedirectedModel)
	}
}

// TestSelect_NoProvidersAvailable 测试无可用供应商
func TestSelect_NoProvidersAvailable(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	// 空列表
	_, err := selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: []*model.Provider{},
	})

	if err == nil {
		t.Error("Expected error for empty provider list")
	}

	// 所有供应商都被禁用
	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Disabled Provider",
			IsEnabled:     boolPtr(false),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	_, err = selector.Select(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	})

	if err == nil {
		t.Error("Expected error for all disabled providers")
	}
}

// TestSelectWithRetry 测试故障转移
func TestSelectWithRetry(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Provider 1",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            2,
			Name:          "Provider 2",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
		{
			ID:            3,
			Name:          "Provider 3",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	// 最多 3 次尝试
	results, err := selector.SelectWithRetry(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	}, 3)

	if err != nil {
		t.Fatalf("SelectWithRetry() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}

	// 验证每次选择的供应商不同
	ids := make(map[int]bool)
	for _, result := range results {
		if ids[result.Provider.ID] {
			t.Errorf("Provider %d selected multiple times", result.Provider.ID)
		}
		ids[result.Provider.ID] = true
	}
}

// TestSelectWithRetry_InsufficientProviders 测试供应商不足
func TestSelectWithRetry_InsufficientProviders(t *testing.T) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := []*model.Provider{
		{
			ID:            1,
			Name:          "Provider 1",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		},
	}

	// 尝试 3 次，但只有 1 个供应商
	results, err := selector.SelectWithRetry(ctx, SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	}, 3)

	if err != nil {
		t.Fatalf("SelectWithRetry() error = %v", err)
	}

	if len(results) != 1 {
		t.Errorf("Expected 1 result (only 1 provider available), got %d", len(results))
	}
}

// BenchmarkSelect 性能基准测试
func BenchmarkSelect(b *testing.B) {
	cb := newMockCircuitBreaker()
	selector := NewProviderSelector(cb)
	ctx := context.Background()

	providers := make([]*model.Provider, 10)
	for i := 0; i < 10; i++ {
		providers[i] = &model.Provider{
			ID:            i + 1,
			Name:          "Provider",
			IsEnabled:     boolPtr(true),
			Weight:        intPtr(1),
			Priority:      intPtr(0),
			AllowedModels: model.ExactAllowedModelRules("claude-opus-4"),
		}
	}

	req := SelectRequest{
		Model:     "claude-opus-4",
		Providers: providers,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = selector.Select(ctx, req)
	}
}
