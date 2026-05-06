package v1

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

func (h *Handler) applyGlobalRequestFilters(c *gin.Context, requestBody map[string]any) (bool, error) {
	if h == nil || h.requestFilters == nil {
		return false, nil
	}
	filters, err := h.requestFilters.ListActive(context.Background())
	if err != nil {
		return false, appErrors.NewInternalError("加载全局请求过滤器失败").WithError(err)
	}
	changed := false
	for _, filter := range filters {
		if filter == nil || !filter.IsGlobal() {
			continue
		}
		filterChanged, applyErr := applyRequestFilter(c, requestBody, filter)
		if applyErr != nil {
			return false, applyErr
		}
		changed = filterChanged || changed
	}
	return changed, nil
}

func (h *Handler) applyProviderRequestFilters(c *gin.Context, requestBody map[string]any, provider *model.Provider) (bool, error) {
	if h == nil || h.requestFilters == nil || provider == nil {
		return false, nil
	}
	filters, err := h.requestFilters.ListActive(context.Background())
	if err != nil {
		return false, appErrors.NewInternalError("加载供应商请求过滤器失败").WithError(err)
	}
	changed := false
	for _, filter := range filters {
		if filter == nil || filter.IsGlobal() {
			continue
		}
		if filter.AppliesToProvider(provider.ID, provider.GroupTag) {
			filterChanged, applyErr := applyRequestFilter(c, requestBody, filter)
			if applyErr != nil {
				return false, applyErr
			}
			changed = filterChanged || changed
		}
	}
	return changed, nil
}

func (h *Handler) ensureNoSensitiveWords(requestBody map[string]any) error {
	if h == nil || h.sensitiveWords == nil || requestBody == nil {
		return nil
	}
	words, err := h.sensitiveWords.ListActive(context.Background())
	if err != nil {
		return appErrors.NewInternalError("加载敏感词规则失败").WithError(err)
	}
	if len(words) == 0 {
		return nil
	}
	texts := collectRequestTexts(requestBody)
	if len(texts) == 0 {
		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			return appErrors.NewInternalError("序列化请求内容用于敏感词检测失败").WithError(err)
		}
		texts = []string{string(bodyBytes)}
	}
	for _, word := range words {
		if word == nil || !word.IsActive() {
			continue
		}
		pattern := strings.TrimSpace(word.Word)
		if pattern == "" {
			continue
		}
		switch strings.TrimSpace(word.MatchType) {
		case "exact":
			for _, text := range texts {
				if strings.EqualFold(strings.TrimSpace(text), strings.TrimSpace(pattern)) {
					return appErrors.NewInvalidRequest(`请求包含敏感词："` + pattern + `"`)
				}
			}
		case "regex":
			re, reErr := regexp.Compile(pattern)
			if reErr == nil {
				for _, text := range texts {
					if re.MatchString(text) {
						return appErrors.NewInvalidRequest(`请求包含敏感词："` + pattern + `"`)
					}
				}
			}
		default:
			for _, text := range texts {
				if strings.Contains(strings.ToLower(text), strings.ToLower(pattern)) {
					return appErrors.NewInvalidRequest(`请求包含敏感词："` + pattern + `"`)
				}
			}
		}
	}
	return nil
}

func collectRequestTexts(value any) []string {
	out := make([]string, 0)
	var walk func(v any)
	walk = func(v any) {
		switch typed := v.(type) {
		case string:
			out = append(out, typed)
		case []any:
			for _, item := range typed {
				walk(item)
			}
		case map[string]any:
			for _, item := range typed {
				walk(item)
			}
		}
	}
	walk(value)
	return out
}

func applyRequestFilter(c *gin.Context, requestBody map[string]any, filter *model.RequestFilter) (bool, error) {
	if filter == nil {
		return false, nil
	}
	if err := ensureSupportedRuntimeFilter(filter); err != nil {
		return false, err
	}
	switch strings.TrimSpace(filter.Scope) {
	case "header":
		return applyHeaderFilter(c, filter), nil
	case "body":
		return applyBodyFilter(requestBody, filter), nil
	}
	return false, nil
}

func ensureSupportedRuntimeFilter(filter *model.RequestFilter) error {
	ruleMode := strings.TrimSpace(filter.RuleMode)
	if ruleMode != "" && ruleMode != "simple" {
		return appErrors.NewInternalError("存在当前运行时不支持的 advanced request filter")
	}
	executionPhase := strings.TrimSpace(filter.ExecutionPhase)
	if executionPhase != "" && executionPhase != "guard" {
		return appErrors.NewInternalError("存在当前运行时不支持的 final request filter")
	}
	if len(filter.Operations) > 0 {
		return appErrors.NewInternalError("存在当前运行时不支持的 request filter operations")
	}
	if strings.TrimSpace(filter.Action) == "text_replace" {
		return appErrors.NewInternalError("存在当前运行时不支持的 text_replace request filter")
	}
	return nil
}

func applyHeaderFilter(c *gin.Context, filter *model.RequestFilter) bool {
	if c == nil || c.Request == nil {
		return false
	}
	target := strings.TrimSpace(filter.Target)
	if target == "" {
		return false
	}
	switch strings.TrimSpace(filter.Action) {
	case "remove":
		if c.Request.Header.Get(target) == "" {
			return false
		}
		c.Request.Header.Del(target)
		return true
	case "set":
		value := stringifyFilterValue(filter.Replacement)
		if c.Request.Header.Get(target) == value {
			return false
		}
		c.Request.Header.Set(target, value)
		return true
	}
	return false
}

func applyBodyFilter(requestBody map[string]any, filter *model.RequestFilter) bool {
	if requestBody == nil {
		return false
	}
	target := strings.TrimSpace(filter.Target)
	if target == "" {
		return false
	}
	switch strings.TrimSpace(filter.Action) {
	case "remove":
		return removeBodyPath(requestBody, target)
	case "set", "json_path":
		return setBodyPath(requestBody, target, filter.Replacement)
	case "text_replace":
		if current, ok := getBodyPath(requestBody, target).(string); ok {
			replaced := strings.ReplaceAll(current, current, stringifyFilterValue(filter.Replacement))
			return setBodyPath(requestBody, target, replaced)
		}
	}
	return false
}

func stringifyFilterValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	data, _ := json.Marshal(value)
	return string(data)
}

func splitBodyPath(path string) []string {
	parts := strings.Split(path, ".")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func getBodyPath(body map[string]any, path string) any {
	current := any(body)
	for _, part := range splitBodyPath(path) {
		node, ok := current.(map[string]any)
		if !ok {
			return nil
		}
		current, ok = node[part]
		if !ok {
			return nil
		}
	}
	return current
}

func setBodyPath(body map[string]any, path string, value any) bool {
	parts := splitBodyPath(path)
	if len(parts) == 0 {
		return false
	}
	current := body
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[part] = next
		}
		current = next
	}
	last := parts[len(parts)-1]
	existing, exists := current[last]
	if exists && existing == value {
		return false
	}
	current[last] = value
	return true
}

func removeBodyPath(body map[string]any, path string) bool {
	parts := splitBodyPath(path)
	if len(parts) == 0 {
		return false
	}
	current := body
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	last := parts[len(parts)-1]
	if _, ok := current[last]; !ok {
		return false
	}
	delete(current, last)
	return true
}
