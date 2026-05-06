package cost

import (
	"context"
	"fmt"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// ModelPriceRepository 模型价格查询接口
type ModelPriceRepository interface {
	GetLatestByModelName(ctx context.Context, modelName string) (*model.ModelPrice, error)
}

// Calculator 成本计算器
type Calculator struct {
	priceRepo ModelPriceRepository
}

// NewCalculator 创建成本计算器
func NewCalculator(priceRepo ModelPriceRepository) *Calculator {
	return &Calculator{
		priceRepo: priceRepo,
	}
}

// CalculateRequest 成本计算请求
type CalculateRequest struct {
	// 模型名称
	Model string

	// Token 数量
	InputTokens             int
	OutputTokens            int
	CacheCreationTokens     int
	CacheReadTokens         int
	CacheCreationTokens1h   int // 1小时以上的缓存创建 tokens
	CacheCreationTokens200k int // 200k 以上的缓存创建 tokens
	CacheReadTokens200k     int // 200k 以上的缓存读取 tokens

	// 成本倍率（来自 Provider 配置）
	CostMultiplier udecimal.Decimal

	// 图片数量（用于图片生成模型）
	OutputImages int

	// 搜索上下文查询次数
	SearchContextQueries int
	SearchContextSize    string // "low", "medium", "high"
}

// CalculateResult 成本计算结果
type CalculateResult struct {
	// 总成本（已应用 cost_multiplier）
	TotalCost udecimal.Decimal

	// 分项成本（未应用 cost_multiplier）
	InputCost             udecimal.Decimal
	OutputCost            udecimal.Decimal
	CacheCreationCost     udecimal.Decimal
	CacheReadCost         udecimal.Decimal
	ImageCost             udecimal.Decimal
	SearchContextCost     udecimal.Decimal

	// 使用的价格信息
	PriceData *model.PriceData
}

// Calculate 计算成本
func (c *Calculator) Calculate(ctx context.Context, req CalculateRequest) (*CalculateResult, error) {
	// 1. 获取模型价格
	modelPrice, err := c.priceRepo.GetLatestByModelName(ctx, req.Model)
	if err != nil {
		return nil, fmt.Errorf("failed to get model price: %w", err)
	}

	priceData := &modelPrice.PriceData

	// 2. 计算各项成本
	result := &CalculateResult{
		PriceData: priceData,
	}

	// 2.1 计算 Input Tokens 成本
	if req.InputTokens > 0 {
		inputCost, err := calculateInputCost(req.InputTokens, priceData)
		if err != nil {
			return nil, err
		}
		result.InputCost = inputCost
	}

	// 2.2 计算 Output Tokens 成本
	if req.OutputTokens > 0 {
		outputCost, err := calculateOutputCost(req.OutputTokens, priceData)
		if err != nil {
			return nil, err
		}
		result.OutputCost = outputCost
	}

	// 2.3 计算 Cache Creation Tokens 成本
	if req.CacheCreationTokens > 0 || req.CacheCreationTokens1h > 0 || req.CacheCreationTokens200k > 0 {
		cacheCreationCost, err := calculateCacheCreationCost(
			req.CacheCreationTokens,
			req.CacheCreationTokens1h,
			req.CacheCreationTokens200k,
			priceData,
		)
		if err != nil {
			return nil, err
		}
		result.CacheCreationCost = cacheCreationCost
	}

	// 2.4 计算 Cache Read Tokens 成本
	if req.CacheReadTokens > 0 || req.CacheReadTokens200k > 0 {
		cacheReadCost, err := calculateCacheReadCost(
			req.CacheReadTokens,
			req.CacheReadTokens200k,
			priceData,
		)
		if err != nil {
			return nil, err
		}
		result.CacheReadCost = cacheReadCost
	}

	// 2.5 计算图片生成成本
	if req.OutputImages > 0 {
		imageCost, err := calculateImageCost(req.OutputImages, priceData)
		if err != nil {
			return nil, err
		}
		result.ImageCost = imageCost
	}

	// 2.6 计算搜索上下文成本
	if req.SearchContextQueries > 0 {
		searchCost, err := calculateSearchContextCost(
			req.SearchContextQueries,
			req.SearchContextSize,
			priceData,
		)
		if err != nil {
			return nil, err
		}
		result.SearchContextCost = searchCost
	}

	// 3. 计算总成本（未应用倍率）
	baseCost := result.InputCost.
		Add(result.OutputCost).
		Add(result.CacheCreationCost).
		Add(result.CacheReadCost).
		Add(result.ImageCost).
		Add(result.SearchContextCost)

	// 4. 应用成本倍率
	costMultiplier := req.CostMultiplier
	if costMultiplier.IsZero() {
		costMultiplier = udecimal.MustParse("1.0")
	}

	result.TotalCost = baseCost.Mul(costMultiplier)

	return result, nil
}

// calculateInputCost 计算输入 tokens 成本
func calculateInputCost(tokens int, priceData *model.PriceData) (udecimal.Decimal, error) {
	if priceData.InputCostPerToken == nil {
		return udecimal.Zero, nil
	}

	costPerToken := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.InputCostPerToken))
	tokenCount := udecimal.MustParse(fmt.Sprintf("%d", tokens))

	return costPerToken.Mul(tokenCount), nil
}

// calculateOutputCost 计算输出 tokens 成本
func calculateOutputCost(tokens int, priceData *model.PriceData) (udecimal.Decimal, error) {
	if priceData.OutputCostPerToken == nil {
		return udecimal.Zero, nil
	}

	costPerToken := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.OutputCostPerToken))
	tokenCount := udecimal.MustParse(fmt.Sprintf("%d", tokens))

	return costPerToken.Mul(tokenCount), nil
}

// calculateCacheCreationCost 计算缓存创建 tokens 成本
func calculateCacheCreationCost(
	tokens int,
	tokens1h int,
	tokens200k int,
	priceData *model.PriceData,
) (udecimal.Decimal, error) {
	totalCost := udecimal.Zero

	// 基础缓存创建成本
	if tokens > 0 && priceData.CacheCreationInputTokenCost != nil {
		costPerToken := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.CacheCreationInputTokenCost))
		tokenCount := udecimal.MustParse(fmt.Sprintf("%d", tokens))
		totalCost = totalCost.Add(costPerToken.Mul(tokenCount))
	}

	// 1小时以上的缓存创建成本
	if tokens1h > 0 && priceData.CacheCreationInputTokenCostAbove1h != nil {
		costPerToken := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.CacheCreationInputTokenCostAbove1h))
		tokenCount := udecimal.MustParse(fmt.Sprintf("%d", tokens1h))
		totalCost = totalCost.Add(costPerToken.Mul(tokenCount))
	}

	// 200k 以上的缓存创建成本
	if tokens200k > 0 && priceData.CacheCreationInputTokenCostAbove200kTokens != nil {
		costPerToken := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.CacheCreationInputTokenCostAbove200kTokens))
		tokenCount := udecimal.MustParse(fmt.Sprintf("%d", tokens200k))
		totalCost = totalCost.Add(costPerToken.Mul(tokenCount))
	}

	return totalCost, nil
}

// calculateCacheReadCost 计算缓存读取 tokens 成本
func calculateCacheReadCost(
	tokens int,
	tokens200k int,
	priceData *model.PriceData,
) (udecimal.Decimal, error) {
	totalCost := udecimal.Zero

	// 基础缓存读取成本
	if tokens > 0 && priceData.CacheReadInputTokenCost != nil {
		costPerToken := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.CacheReadInputTokenCost))
		tokenCount := udecimal.MustParse(fmt.Sprintf("%d", tokens))
		totalCost = totalCost.Add(costPerToken.Mul(tokenCount))
	}

	// 200k 以上的缓存读取成本
	if tokens200k > 0 && priceData.CacheReadInputTokenCostAbove200kTokens != nil {
		costPerToken := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.CacheReadInputTokenCostAbove200kTokens))
		tokenCount := udecimal.MustParse(fmt.Sprintf("%d", tokens200k))
		totalCost = totalCost.Add(costPerToken.Mul(tokenCount))
	}

	return totalCost, nil
}

// calculateImageCost 计算图片生成成本
func calculateImageCost(imageCount int, priceData *model.PriceData) (udecimal.Decimal, error) {
	if priceData.OutputCostPerImage == nil {
		return udecimal.Zero, nil
	}

	costPerImage := udecimal.MustParse(fmt.Sprintf("%.10f", *priceData.OutputCostPerImage))
	count := udecimal.MustParse(fmt.Sprintf("%d", imageCount))

	return costPerImage.Mul(count), nil
}

// calculateSearchContextCost 计算搜索上下文成本
func calculateSearchContextCost(
	queries int,
	size string,
	priceData *model.PriceData,
) (udecimal.Decimal, error) {
	if priceData.SearchContextCostPerQuery == nil {
		return udecimal.Zero, nil
	}

	var costPerQuery *float64
	switch size {
	case "low":
		costPerQuery = priceData.SearchContextCostPerQuery.SearchContextSizeLow
	case "medium":
		costPerQuery = priceData.SearchContextCostPerQuery.SearchContextSizeMedium
	case "high":
		costPerQuery = priceData.SearchContextCostPerQuery.SearchContextSizeHigh
	default:
		// 默认使用 medium
		costPerQuery = priceData.SearchContextCostPerQuery.SearchContextSizeMedium
	}

	if costPerQuery == nil {
		return udecimal.Zero, nil
	}

	cost := udecimal.MustParse(fmt.Sprintf("%.10f", *costPerQuery))
	queryCount := udecimal.MustParse(fmt.Sprintf("%d", queries))

	return cost.Mul(queryCount), nil
}
