package cost

import (
	"context"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// mockPriceRepository 模拟价格仓库
type mockPriceRepository struct {
	prices map[string]*model.ModelPrice
}

func newMockPriceRepository() *mockPriceRepository {
	return &mockPriceRepository{
		prices: make(map[string]*model.ModelPrice),
	}
}

func (m *mockPriceRepository) GetLatestByModelName(ctx context.Context, modelName string) (*model.ModelPrice, error) {
	if price, ok := m.prices[modelName]; ok {
		return price, nil
	}
	return nil, nil
}

func (m *mockPriceRepository) addPrice(modelName string, priceData *model.PriceData) {
	m.prices[modelName] = &model.ModelPrice{
		ModelName: modelName,
		PriceData: *priceData, // 解引用指针
	}
}

// TestCalculate_BasicInputOutput 测试基础输入输出成本计算
func TestCalculate_BasicInputOutput(t *testing.T) {
	// 准备测试数据
	repo := newMockPriceRepository()
	inputCost := 0.000003  // $3 per million tokens
	outputCost := 0.000015 // $15 per million tokens

	repo.addPrice("claude-opus-4", &model.PriceData{
		InputCostPerToken:  &inputCost,
		OutputCostPerToken: &outputCost,
	})

	calculator := NewCalculator(repo)

	// 测试用例
	tests := []struct {
		name         string
		req          CalculateRequest
		wantTotal    string
		wantInput    string
		wantOutput   string
	}{
		{
			name: "1000 input + 500 output tokens",
			req: CalculateRequest{
				Model:          "claude-opus-4",
				InputTokens:    1000,
				OutputTokens:   500,
				CostMultiplier: udecimal.MustParse("1.0"),
			},
			wantTotal:  "0.0105",    // (1000 * 0.000003) + (500 * 0.000015) = 0.003 + 0.0075 = 0.0105
			wantInput:  "0.003",     // 1000 * 0.000003
			wantOutput: "0.0075",    // 500 * 0.000015
		},
		{
			name: "with cost multiplier 1.2",
			req: CalculateRequest{
				Model:          "claude-opus-4",
				InputTokens:    1000,
				OutputTokens:   500,
				CostMultiplier: udecimal.MustParse("1.2"),
			},
			wantTotal:  "0.0126",    // 0.0105 * 1.2
			wantInput:  "0.003",
			wantOutput: "0.0075",
		},
		{
			name: "only input tokens",
			req: CalculateRequest{
				Model:          "claude-opus-4",
				InputTokens:    5000,
				CostMultiplier: udecimal.MustParse("1.0"),
			},
			wantTotal:  "0.015",     // 5000 * 0.000003
			wantInput:  "0.015",
			wantOutput: "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculator.Calculate(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Calculate() error = %v", err)
			}

			// 验证总成本
			if result.TotalCost.String() != tt.wantTotal {
				t.Errorf("TotalCost = %v, want %v", result.TotalCost.String(), tt.wantTotal)
			}

			// 验证输入成本
			if result.InputCost.String() != tt.wantInput {
				t.Errorf("InputCost = %v, want %v", result.InputCost.String(), tt.wantInput)
			}

			// 验证输出成本
			if result.OutputCost.String() != tt.wantOutput {
				t.Errorf("OutputCost = %v, want %v", result.OutputCost.String(), tt.wantOutput)
			}
		})
	}
}

// TestCalculate_CacheTokens 测试缓存 tokens 成本计算
func TestCalculate_CacheTokens(t *testing.T) {
	repo := newMockPriceRepository()
	inputCost := 0.000003
	outputCost := 0.000015
	cacheCreationCost := 0.00000375      // $3.75 per million tokens
	cacheReadCost := 0.0000003           // $0.30 per million tokens

	repo.addPrice("claude-sonnet-4", &model.PriceData{
		InputCostPerToken:           &inputCost,
		OutputCostPerToken:          &outputCost,
		CacheCreationInputTokenCost: &cacheCreationCost,
		CacheReadInputTokenCost:     &cacheReadCost,
	})

	calculator := NewCalculator(repo)

	tests := []struct {
		name              string
		req               CalculateRequest
		wantTotal         string
		wantCacheCreation string
		wantCacheRead     string
	}{
		{
			name: "cache creation and read",
			req: CalculateRequest{
				Model:               "claude-sonnet-4",
				InputTokens:         1000,
				OutputTokens:        500,
				CacheCreationTokens: 2000,
				CacheReadTokens:     3000,
				CostMultiplier:      udecimal.MustParse("1.0"),
			},
			wantTotal:         "0.0189",     // 0.003 + 0.0075 + 0.0075 + 0.0009
			wantCacheCreation: "0.0075",     // 2000 * 0.00000375
			wantCacheRead:     "0.0009",     // 3000 * 0.0000003
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculator.Calculate(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("Calculate() error = %v", err)
			}

			if result.TotalCost.String() != tt.wantTotal {
				t.Errorf("TotalCost = %v, want %v", result.TotalCost.String(), tt.wantTotal)
			}

			if result.CacheCreationCost.String() != tt.wantCacheCreation {
				t.Errorf("CacheCreationCost = %v, want %v", result.CacheCreationCost.String(), tt.wantCacheCreation)
			}

			if result.CacheReadCost.String() != tt.wantCacheRead {
				t.Errorf("CacheReadCost = %v, want %v", result.CacheReadCost.String(), tt.wantCacheRead)
			}
		})
	}
}

// TestCalculate_ImageGeneration 测试图片生成成本计算
func TestCalculate_ImageGeneration(t *testing.T) {
	repo := newMockPriceRepository()
	imageCost := 0.04 // $0.04 per image

	repo.addPrice("dall-e-3", &model.PriceData{
		OutputCostPerImage: &imageCost,
	})

	calculator := NewCalculator(repo)

	result, err := calculator.Calculate(context.Background(), CalculateRequest{
		Model:          "dall-e-3",
		OutputImages:   5,
		CostMultiplier: udecimal.MustParse("1.0"),
	})

	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	wantTotal := "0.2" // 5 * 0.04
	if result.TotalCost.String() != wantTotal {
		t.Errorf("TotalCost = %v, want %v", result.TotalCost.String(), wantTotal)
	}

	if result.ImageCost.String() != wantTotal {
		t.Errorf("ImageCost = %v, want %v", result.ImageCost.String(), wantTotal)
	}
}

// TestCalculate_SearchContext 测试搜索上下文成本计算
func TestCalculate_SearchContext(t *testing.T) {
	repo := newMockPriceRepository()
	lowCost := 0.001
	mediumCost := 0.002
	highCost := 0.003

	repo.addPrice("gemini-search", &model.PriceData{
		SearchContextCostPerQuery: &model.SearchContextCostPerQuery{
			SearchContextSizeLow:    &lowCost,
			SearchContextSizeMedium: &mediumCost,
			SearchContextSizeHigh:   &highCost,
		},
	})

	calculator := NewCalculator(repo)

	tests := []struct {
		name      string
		size      string
		queries   int
		wantTotal string
	}{
		{
			name:      "low size, 10 queries",
			size:      "low",
			queries:   10,
			wantTotal: "0.01", // 10 * 0.001
		},
		{
			name:      "medium size, 5 queries",
			size:      "medium",
			queries:   5,
			wantTotal: "0.01", // 5 * 0.002
		},
		{
			name:      "high size, 3 queries",
			size:      "high",
			queries:   3,
			wantTotal: "0.009", // 3 * 0.003
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := calculator.Calculate(context.Background(), CalculateRequest{
				Model:                "gemini-search",
				SearchContextQueries: tt.queries,
				SearchContextSize:    tt.size,
				CostMultiplier:       udecimal.MustParse("1.0"),
			})

			if err != nil {
				t.Fatalf("Calculate() error = %v", err)
			}

			if result.TotalCost.String() != tt.wantTotal {
				t.Errorf("TotalCost = %v, want %v", result.TotalCost.String(), tt.wantTotal)
			}
		})
	}
}

// TestCalculate_ZeroTokens 测试零 tokens 的情况
func TestCalculate_ZeroTokens(t *testing.T) {
	repo := newMockPriceRepository()
	inputCost := 0.000003
	outputCost := 0.000015

	repo.addPrice("test-model", &model.PriceData{
		InputCostPerToken:  &inputCost,
		OutputCostPerToken: &outputCost,
	})

	calculator := NewCalculator(repo)

	result, err := calculator.Calculate(context.Background(), CalculateRequest{
		Model:          "test-model",
		InputTokens:    0,
		OutputTokens:   0,
		CostMultiplier: udecimal.MustParse("1.0"),
	})

	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	if !result.TotalCost.IsZero() {
		t.Errorf("TotalCost should be zero, got %v", result.TotalCost.String())
	}
}

// TestCalculate_DefaultCostMultiplier 测试默认成本倍率
func TestCalculate_DefaultCostMultiplier(t *testing.T) {
	repo := newMockPriceRepository()
	inputCost := 0.000003

	repo.addPrice("test-model", &model.PriceData{
		InputCostPerToken: &inputCost,
	})

	calculator := NewCalculator(repo)

	// 不设置 CostMultiplier（零值）
	result, err := calculator.Calculate(context.Background(), CalculateRequest{
		Model:       "test-model",
		InputTokens: 1000,
	})

	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	// 应该使用默认倍率 1.0
	wantTotal := "0.003" // 1000 * 0.000003 * 1.0
	if result.TotalCost.String() != wantTotal {
		t.Errorf("TotalCost = %v, want %v", result.TotalCost.String(), wantTotal)
	}
}

// TestCalculate_ComplexScenario 测试复杂场景
func TestCalculate_ComplexScenario(t *testing.T) {
	repo := newMockPriceRepository()
	inputCost := 0.000003
	outputCost := 0.000015
	cacheCreationCost := 0.00000375
	cacheReadCost := 0.0000003

	repo.addPrice("claude-opus-4", &model.PriceData{
		InputCostPerToken:           &inputCost,
		OutputCostPerToken:          &outputCost,
		CacheCreationInputTokenCost: &cacheCreationCost,
		CacheReadInputTokenCost:     &cacheReadCost,
	})

	calculator := NewCalculator(repo)

	// 复杂场景：包含所有类型的 tokens + 成本倍率
	result, err := calculator.Calculate(context.Background(), CalculateRequest{
		Model:               "claude-opus-4",
		InputTokens:         10000,
		OutputTokens:        5000,
		CacheCreationTokens: 20000,
		CacheReadTokens:     30000,
		CostMultiplier:      udecimal.MustParse("1.5"),
	})

	if err != nil {
		t.Fatalf("Calculate() error = %v", err)
	}

	// 验证各项成本
	// Input: 10000 * 0.000003 = 0.03
	// Output: 5000 * 0.000015 = 0.075
	// Cache Creation: 20000 * 0.00000375 = 0.075
	// Cache Read: 30000 * 0.0000003 = 0.009
	// Base Total: 0.03 + 0.075 + 0.075 + 0.009 = 0.189
	// With Multiplier: 0.189 * 1.5 = 0.2835

	if result.InputCost.String() != "0.03" {
		t.Errorf("InputCost = %v, want 0.03", result.InputCost.String())
	}

	if result.OutputCost.String() != "0.075" {
		t.Errorf("OutputCost = %v, want 0.075", result.OutputCost.String())
	}

	if result.CacheCreationCost.String() != "0.075" {
		t.Errorf("CacheCreationCost = %v, want 0.075", result.CacheCreationCost.String())
	}

	if result.CacheReadCost.String() != "0.009" {
		t.Errorf("CacheReadCost = %v, want 0.009", result.CacheReadCost.String())
	}

	if result.TotalCost.String() != "0.2835" {
		t.Errorf("TotalCost = %v, want 0.2835", result.TotalCost.String())
	}
}

// BenchmarkCalculate 性能基准测试
func BenchmarkCalculate(b *testing.B) {
	repo := newMockPriceRepository()
	inputCost := 0.000003
	outputCost := 0.000015

	repo.addPrice("test-model", &model.PriceData{
		InputCostPerToken:  &inputCost,
		OutputCostPerToken: &outputCost,
	})

	calculator := NewCalculator(repo)
	ctx := context.Background()

	req := CalculateRequest{
		Model:          "test-model",
		InputTokens:    1000,
		OutputTokens:   500,
		CostMultiplier: udecimal.MustParse("1.0"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = calculator.Calculate(ctx, req)
	}
}
