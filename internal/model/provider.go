package model

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/quagmt/udecimal"
	"github.com/uptrace/bun"
)

// Provider 供应商模型
type Provider struct {
	bun.BaseModel `bun:"table:providers,alias:p"`

	ID          int     `bun:"id,pk,autoincrement" json:"id"`
	Name        string  `bun:"name,notnull" json:"name"`
	Description *string `bun:"description" json:"description"`
	URL         string  `bun:"url,notnull" json:"url"`
	Key         string  `bun:"key,notnull" json:"-"` // 不序列化
	IsEnabled   *bool   `bun:"is_enabled,notnull,default:true" json:"isEnabled"`
	Weight      *int    `bun:"weight,notnull,default:1" json:"weight"`

	// 优先级和分组配置
	Priority       *int              `bun:"priority,notnull,default:0" json:"priority"`
	CostMultiplier *udecimal.Decimal `bun:"cost_multiplier,type:numeric(10,4),default:1.0" json:"costMultiplier"`

	GroupTag *string `bun:"group_tag" json:"groupTag"`

	// 供应商类型
	ProviderType     string `bun:"provider_type,notnull,default:'claude'" json:"providerType"` // claude, claude-auth, codex, gemini-cli, gemini, openai-compatible
	PreserveClientIp bool   `bun:"preserve_client_ip,notnull,default:false" json:"preserveClientIp"`

	// 模型重定向
	ModelRedirects ProviderModelRedirectRules `bun:"model_redirects,type:jsonb" json:"modelRedirects"`
	AllowedModels  AllowedModelRules          `bun:"allowed_models,type:jsonb" json:"allowedModels"`
	JoinClaudePool bool                       `bun:"join_claude_pool,default:false" json:"joinClaudePool"`

	// Codex instructions 策略（已废弃但保留兼容）
	CodexInstructionsStrategy *string `bun:"codex_instructions_strategy,default:'auto'" json:"codexInstructionsStrategy"` // auto, force_official, keep_original

	// MCP 透传配置
	McpPassthroughType string  `bun:"mcp_passthrough_type,notnull,default:'none'" json:"mcpPassthroughType"` // none, minimax, glm, custom
	McpPassthroughUrl  *string `bun:"mcp_passthrough_url" json:"mcpPassthroughUrl"`

	// 金额限流配置
	Limit5hUSD              *udecimal.Decimal `bun:"limit_5h_usd,type:numeric(10,2)" json:"limit5hUsd"`
	LimitDailyUSD           *udecimal.Decimal `bun:"limit_daily_usd,type:numeric(10,2)" json:"limitDailyUsd"`
	DailyResetMode          string            `bun:"daily_reset_mode,notnull,default:'fixed'" json:"dailyResetMode"` // fixed, rolling
	DailyResetTime          string            `bun:"daily_reset_time,notnull,default:'00:00'" json:"dailyResetTime"` // HH:mm 格式
	LimitWeeklyUSD          *udecimal.Decimal `bun:"limit_weekly_usd,type:numeric(10,2)" json:"limitWeeklyUsd"`
	LimitMonthlyUSD         *udecimal.Decimal `bun:"limit_monthly_usd,type:numeric(10,2)" json:"limitMonthlyUsd"`
	LimitTotalUSD           *udecimal.Decimal `bun:"limit_total_usd,type:numeric(10,2)" json:"limitTotalUsd"`
	TotalCostResetAt        *time.Time        `bun:"total_cost_reset_at" json:"totalCostResetAt"`
	LimitConcurrentSessions *int              `bun:"limit_concurrent_sessions,default:0" json:"limitConcurrentSessions"`

	// 熔断器配置
	MaxRetryAttempts                       *int `bun:"max_retry_attempts" json:"maxRetryAttempts"`
	CircuitBreakerFailureThreshold         *int `bun:"circuit_breaker_failure_threshold,default:5" json:"circuitBreakerFailureThreshold"`
	CircuitBreakerOpenDuration             *int `bun:"circuit_breaker_open_duration,default:1800000" json:"circuitBreakerOpenDuration"` // ms (30分钟)
	CircuitBreakerHalfOpenSuccessThreshold *int `bun:"circuit_breaker_half_open_success_threshold,default:2" json:"circuitBreakerHalfOpenSuccessThreshold"`

	// 代理配置
	ProxyUrl              *string `bun:"proxy_url" json:"proxyUrl"`
	ProxyFallbackToDirect bool    `bun:"proxy_fallback_to_direct,default:false" json:"proxyFallbackToDirect"`

	// 超时配置（毫秒）
	FirstByteTimeoutStreamingMs  *int `bun:"first_byte_timeout_streaming_ms,notnull,default:0" json:"firstByteTimeoutStreamingMs"`
	StreamingIdleTimeoutMs       *int `bun:"streaming_idle_timeout_ms,notnull,default:0" json:"streamingIdleTimeoutMs"`
	RequestTimeoutNonStreamingMs *int `bun:"request_timeout_non_streaming_ms,notnull,default:0" json:"requestTimeoutNonStreamingMs"`

	// 供应商官网
	WebsiteUrl *string `bun:"website_url" json:"websiteUrl"`
	FaviconUrl *string `bun:"favicon_url" json:"faviconUrl"`

	// Cache TTL override
	CacheTtlPreference *string `bun:"cache_ttl_preference" json:"cacheTtlPreference"`

	// 1M Context Window 偏好配置
	Context1mPreference *string `bun:"context_1m_preference" json:"context1mPreference"`

	// Codex 参数覆写
	CodexReasoningEffortPreference   *string `bun:"codex_reasoning_effort_preference" json:"codexReasoningEffortPreference"`
	CodexReasoningSummaryPreference  *string `bun:"codex_reasoning_summary_preference" json:"codexReasoningSummaryPreference"`
	CodexTextVerbosityPreference     *string `bun:"codex_text_verbosity_preference" json:"codexTextVerbosityPreference"`
	CodexParallelToolCallsPreference *string `bun:"codex_parallel_tool_calls_preference" json:"codexParallelToolCallsPreference"`

	// 废弃字段（保留向后兼容）
	Tpm *int `bun:"tpm,default:0" json:"tpm"`
	Rpm *int `bun:"rpm,default:0" json:"rpm"`
	Rpd *int `bun:"rpd,default:0" json:"rpd"`
	Cc  *int `bun:"cc,default:0" json:"cc"`

	CreatedAt time.Time  `bun:"created_at,notnull,default:current_timestamp" json:"createdAt"`
	UpdatedAt time.Time  `bun:"updated_at,notnull,default:current_timestamp" json:"updatedAt"`
	DeletedAt *time.Time `bun:"deleted_at,soft_delete,nullzero" json:"deletedAt,omitempty"`
}

type AllowedModelRule struct {
	MatchType string `json:"matchType"`
	Pattern   string `json:"pattern"`
}

type AllowedModelRules []AllowedModelRule

type ProviderModelRedirectRule struct {
	MatchType string `json:"matchType"`
	Source    string `json:"source"`
	Target    string `json:"target"`
}

type ProviderModelRedirectRules []ProviderModelRedirectRule

func ExactAllowedModelRules(models ...string) AllowedModelRules {
	rules := make(AllowedModelRules, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		rules = append(rules, AllowedModelRule{
			MatchType: "exact",
			Pattern:   model,
		})
	}
	return rules
}

func (r *AllowedModelRules) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rules := make(AllowedModelRules, 0, len(raw))
	for _, item := range raw {
		if len(item) == 0 {
			continue
		}
		var exact string
		if err := json.Unmarshal(item, &exact); err == nil {
			exact = strings.TrimSpace(exact)
			if exact != "" {
				rules = append(rules, AllowedModelRule{MatchType: "exact", Pattern: exact})
			}
			continue
		}
		var rule AllowedModelRule
		if err := json.Unmarshal(item, &rule); err != nil {
			return err
		}
		rule.MatchType = strings.TrimSpace(rule.MatchType)
		rule.Pattern = strings.TrimSpace(rule.Pattern)
		if rule.MatchType == "" {
			rule.MatchType = "exact"
		}
		if rule.Pattern == "" {
			continue
		}
		rules = append(rules, rule)
	}
	*r = rules
	return nil
}

func (r AllowedModelRules) Match(model string) bool {
	if len(r) == 0 {
		return true
	}
	for _, rule := range r {
		if rule.matches(model) {
			return true
		}
	}
	return false
}

func (r AllowedModelRules) ExactModelNames() []string {
	models := make([]string, 0, len(r))
	for _, rule := range r {
		if strings.EqualFold(rule.MatchType, "exact") && strings.TrimSpace(rule.Pattern) != "" {
			models = append(models, strings.TrimSpace(rule.Pattern))
		}
	}
	return models
}

func (r AllowedModelRule) matches(model string) bool {
	return matchModelPattern(model, r.MatchType, r.Pattern)
}

func (r *ProviderModelRedirectRules) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*r = nil
		return nil
	}
	var legacyMap map[string]string
	if err := json.Unmarshal(data, &legacyMap); err == nil && legacyMap != nil {
		rules := make(ProviderModelRedirectRules, 0, len(legacyMap))
		for source, target := range legacyMap {
			source = strings.TrimSpace(source)
			target = strings.TrimSpace(target)
			if source == "" || target == "" {
				continue
			}
			rules = append(rules, ProviderModelRedirectRule{
				MatchType: "exact",
				Source:    source,
				Target:    target,
			})
		}
		*r = rules
		return nil
	}

	var raw []ProviderModelRedirectRule
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	rules := make(ProviderModelRedirectRules, 0, len(raw))
	for _, rule := range raw {
		rule.MatchType = strings.TrimSpace(rule.MatchType)
		rule.Source = strings.TrimSpace(rule.Source)
		rule.Target = strings.TrimSpace(rule.Target)
		if rule.MatchType == "" {
			rule.MatchType = "exact"
		}
		if rule.Source == "" || rule.Target == "" {
			continue
		}
		rules = append(rules, rule)
	}
	*r = rules
	return nil
}

func (r ProviderModelRedirectRules) Match(model string) (string, bool) {
	for _, rule := range r {
		if rule.matches(model) {
			return rule.Target, true
		}
	}
	return "", false
}

func (r ProviderModelRedirectRule) matches(model string) bool {
	return matchModelPattern(model, r.MatchType, r.Source)
}

func matchModelPattern(model string, matchType string, pattern string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(matchType)) {
	case "", "exact":
		return model == pattern
	case "prefix":
		return strings.HasPrefix(model, pattern)
	case "suffix":
		return strings.HasSuffix(model, pattern)
	case "contains":
		return strings.Contains(model, pattern)
	case "regex":
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return re.MatchString(model)
	default:
		return false
	}
}

// SupportsModel 检查供应商是否支持指定模型
func (p *Provider) SupportsModel(model string) bool {
	// 如果没有配置允许的模型列表，则支持所有模型
	if len(p.AllowedModels) == 0 {
		return true
	}
	return p.AllowedModels.Match(model)
}

// GetRedirectedModel 获取重定向后的模型名称
func (p *Provider) GetRedirectedModel(model string) string {
	if redirected, ok := p.ModelRedirects.Match(model); ok {
		return redirected
	}
	return model
}

// IsActive 检查供应商是否处于活跃状态
func (p *Provider) IsActive() bool {
	enabled := true
	if p.IsEnabled != nil {
		enabled = *p.IsEnabled
	}
	return enabled && p.DeletedAt == nil
}
