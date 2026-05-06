package proxy

import (
	"context"
	"fmt"
	"math/rand"
	"sort"

	"github.com/ding113/claude-code-hub/internal/model"
)

// CircuitBreakerChecker 熔断器检查接口
type CircuitBreakerChecker interface {
	IsOpen(providerID int) bool
}

// ProviderSelector 供应商选择器
type ProviderSelector struct {
	circuitBreaker CircuitBreakerChecker
}

// NewProviderSelector 创建供应商选择器
func NewProviderSelector(circuitBreaker CircuitBreakerChecker) *ProviderSelector {
	return &ProviderSelector{
		circuitBreaker: circuitBreaker,
	}
}

// SelectRequest 选择请求
type SelectRequest struct {
	// 模型名称
	Model string

	// 分组标签（可选）
	GroupTag string

	// 排除的供应商 ID（已失败的供应商）
	ExcludeProviderIDs []int

	// 候选供应商列表
	Providers []*model.Provider
}

// SelectResult 选择结果
type SelectResult struct {
	// 选中的供应商
	Provider *model.Provider

	// 重定向后的模型名称（如果有模型重定向）
	RedirectedModel string
}

// Select 选择供应商
func (s *ProviderSelector) Select(ctx context.Context, req SelectRequest) (*SelectResult, error) {
	if len(req.Providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// 1. 过滤供应商
	candidates := s.filterProviders(req)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no suitable providers found after filtering")
	}

	// 2. 按优先级分组
	priorityGroups := s.groupByPriority(candidates)

	// 3. 从最高优先级组开始选择
	for _, group := range priorityGroups {
		if provider := s.selectFromGroup(group); provider != nil {
			// 检查模型重定向
			redirectedModel := req.Model
			if target, ok := provider.ModelRedirects.Match(req.Model); ok {
				redirectedModel = target
			}

			return &SelectResult{
				Provider:        provider,
				RedirectedModel: redirectedModel,
			}, nil
		}
	}

	return nil, fmt.Errorf("all providers are unavailable (circuit breaker open)")
}

// filterProviders 过滤供应商
func (s *ProviderSelector) filterProviders(req SelectRequest) []*model.Provider {
	var candidates []*model.Provider

	for _, provider := range req.Providers {
		// 1. 检查是否被排除
		if s.isExcluded(provider.ID, req.ExcludeProviderIDs) {
			continue
		}

		// 2. 检查是否启用
		if provider.IsEnabled == nil || !*provider.IsEnabled {
			continue
		}

		// 3. 检查分组标签（如果指定）
		if req.GroupTag != "" {
			if provider.GroupTag == nil || *provider.GroupTag != req.GroupTag {
				continue
			}
		}

		// 4. 检查模型匹配
		if !provider.AllowedModels.Match(req.Model) {
			continue
		}

		// 5. 检查熔断器状态
		if s.circuitBreaker != nil && s.circuitBreaker.IsOpen(provider.ID) {
			continue
		}

		candidates = append(candidates, provider)
	}

	return candidates
}

// isExcluded 检查供应商是否被排除
func (s *ProviderSelector) isExcluded(providerID int, excludeIDs []int) bool {
	for _, id := range excludeIDs {
		if id == providerID {
			return true
		}
	}
	return false
}

// groupByPriority 按优先级分组
func (s *ProviderSelector) groupByPriority(providers []*model.Provider) [][]*model.Provider {
	// 按优先级排序（降序）
	sorted := make([]*model.Provider, len(providers))
	copy(sorted, providers)

	sort.Slice(sorted, func(i, j int) bool {
		pi := 0
		if sorted[i].Priority != nil {
			pi = *sorted[i].Priority
		}
		pj := 0
		if sorted[j].Priority != nil {
			pj = *sorted[j].Priority
		}
		return pi > pj // 降序
	})

	// 分组
	var groups [][]*model.Provider
	var currentGroup []*model.Provider
	var currentPriority *int

	for _, provider := range sorted {
		priority := 0
		if provider.Priority != nil {
			priority = *provider.Priority
		}

		if currentPriority == nil {
			// 第一个供应商
			currentPriority = &priority
			currentGroup = []*model.Provider{provider}
		} else if *currentPriority == priority {
			// 同一优先级
			currentGroup = append(currentGroup, provider)
		} else {
			// 新的优先级
			groups = append(groups, currentGroup)
			currentPriority = &priority
			currentGroup = []*model.Provider{provider}
		}
	}

	// 添加最后一组
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// selectFromGroup 从同一优先级组中选择供应商（加权随机）
func (s *ProviderSelector) selectFromGroup(providers []*model.Provider) *model.Provider {
	if len(providers) == 0 {
		return nil
	}

	if len(providers) == 1 {
		return providers[0]
	}

	// 计算总权重
	totalWeight := 0
	for _, provider := range providers {
		weight := 1
		if provider.Weight != nil && *provider.Weight > 0 {
			weight = *provider.Weight
		}
		totalWeight += weight
	}

	if totalWeight == 0 {
		// 所有权重都是 0，随机选择
		return providers[rand.Intn(len(providers))]
	}

	// 加权随机选择
	r := rand.Intn(totalWeight)
	cumulative := 0

	for _, provider := range providers {
		weight := 1
		if provider.Weight != nil && *provider.Weight > 0 {
			weight = *provider.Weight
		}
		cumulative += weight
		if r < cumulative {
			return provider
		}
	}

	// 理论上不会到这里，但为了安全返回最后一个
	return providers[len(providers)-1]
}

// SelectWithRetry 选择供应商（支持故障转移，最多 3 次）
func (s *ProviderSelector) SelectWithRetry(ctx context.Context, req SelectRequest, maxAttempts int) ([]*SelectResult, error) {
	if maxAttempts <= 0 {
		maxAttempts = 3 // 默认最多 3 次
	}

	var results []*SelectResult
	excludeIDs := make([]int, 0)

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// 更新排除列表
		req.ExcludeProviderIDs = excludeIDs

		// 选择供应商
		result, err := s.Select(ctx, req)
		if err != nil {
			// 没有可用供应商了
			break
		}

		results = append(results, result)

		// 添加到排除列表（为下一次选择准备）
		excludeIDs = append(excludeIDs, result.Provider.ID)
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no providers available after %d attempts", maxAttempts)
	}

	return results, nil
}
