package v1

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

const (
	providerGroupAll     = "all"
	providerGroupDefault = "default"
)

type responseFormat string
type clientFormat string

const (
	responseFormatOpenAI    responseFormat = "openai"
	responseFormatAnthropic responseFormat = "anthropic"
	responseFormatGemini    responseFormat = "gemini"

	clientFormatClaude    clientFormat = "claude"
	clientFormatOpenAI    clientFormat = "openai"
	clientFormatGemini    clientFormat = "gemini"
	clientFormatGeminiCLI clientFormat = "gemini-cli"
	clientFormatResponse  clientFormat = "response"
)

type fetchedModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
}

func (h *Handler) handleModels(c *gin.Context) {
	authResult, ok := GetAuthResult(c)
	if !ok || authResult == nil {
		appErr := appErrors.NewInternalError("代理鉴权上下文缺失")
		c.AbortWithStatusJSON(appErr.HTTPStatus, appErr.ToResponse())
		return
	}

	models, err := h.getAvailableModels(c, authResult)
	if err != nil {
		writeAppError(c, err)
		return
	}

	switch detectResponseFormat(c) {
	case responseFormatAnthropic:
		c.JSON(http.StatusOK, formatAnthropicModelsResponse(models))
	case responseFormatGemini:
		c.JSON(http.StatusOK, formatGeminiModelsResponse(models))
	default:
		c.JSON(http.StatusOK, formatOpenAIModelsResponse(models))
	}
}

func (h *Handler) getAvailableModels(c *gin.Context, authResult *authsvc.AuthResult) ([]fetchedModel, error) {
	if h == nil || h.providers == nil {
		return nil, appErrors.NewInternalError("/v1/models provider repository 未初始化")
	}

	providers, err := h.providers.GetActiveProviders(c.Request.Context())
	if err != nil {
		return nil, err
	}

	effectiveGroup := effectiveProviderGroup(authResult)
	allowedTypes := allowedProviderTypes(detectClientFormat(c))
	allowedTypeSet := make(map[string]struct{}, len(allowedTypes))
	for _, providerType := range allowedTypes {
		allowedTypeSet[providerType] = struct{}{}
	}

	seen := make(map[string]fetchedModel)
	for _, provider := range providers {
		if provider == nil || !provider.IsActive() {
			continue
		}
		if _, ok := allowedTypeSet[provider.ProviderType]; !ok {
			continue
		}
		if effectiveGroup != "" && !providerGroupMatches(provider.GroupTag, effectiveGroup) {
			continue
		}

		for _, modelID := range provider.AllowedModels {
			modelID = strings.TrimSpace(modelID)
			if modelID == "" {
				continue
			}
			if _, exists := seen[modelID]; exists {
				continue
			}
			seen[modelID] = fetchedModel{ID: modelID, DisplayName: modelID}
		}
	}

	models := make([]fetchedModel, 0, len(seen))
	for _, item := range seen {
		models = append(models, item)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	return models, nil
}

func effectiveProviderGroup(authResult *authsvc.AuthResult) string {
	if authResult == nil {
		return ""
	}
	if authResult.Key != nil && authResult.Key.ProviderGroup != nil && strings.TrimSpace(*authResult.Key.ProviderGroup) != "" {
		return strings.TrimSpace(*authResult.Key.ProviderGroup)
	}
	if authResult.User != nil && authResult.User.ProviderGroup != nil && strings.TrimSpace(*authResult.User.ProviderGroup) != "" {
		return strings.TrimSpace(*authResult.User.ProviderGroup)
	}
	if authResult.Key != nil || authResult.User != nil {
		return providerGroupDefault
	}
	return ""
}

func providerGroupMatches(providerGroupTag *string, effectiveGroup string) bool {
	if strings.TrimSpace(effectiveGroup) == "" {
		return true
	}

	userGroups := parseCommaSeparatedValues(effectiveGroup)
	for _, group := range userGroups {
		if group == providerGroupAll {
			return true
		}
	}

	providerGroups := []string{providerGroupDefault}
	if providerGroupTag != nil && strings.TrimSpace(*providerGroupTag) != "" {
		providerGroups = parseCommaSeparatedValues(*providerGroupTag)
	}

	for _, providerGroup := range providerGroups {
		for _, userGroup := range userGroups {
			if providerGroup == userGroup {
				return true
			}
		}
	}

	return false
}

func parseCommaSeparatedValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(strings.ToLower(part))
		if trimmed == "" {
			continue
		}
		values = append(values, trimmed)
	}
	if len(values) == 0 {
		return []string{providerGroupDefault}
	}
	return values
}

func detectResponseFormat(c *gin.Context) responseFormat {
	if c == nil {
		return responseFormatOpenAI
	}
	if strings.TrimSpace(c.GetHeader("anthropic-version")) != "" {
		return responseFormatAnthropic
	}
	if strings.TrimSpace(c.GetHeader("x-goog-api-key")) != "" || strings.Contains(strings.ToLower(c.Request.URL.Path), "/v1beta/") {
		return responseFormatGemini
	}
	return responseFormatOpenAI
}

func detectClientFormat(c *gin.Context) clientFormat {
	if override := detectClientFormatOverride(c); override != "" {
		return override
	}
	switch detectResponseFormat(c) {
	case responseFormatAnthropic:
		return clientFormatClaude
	case responseFormatGemini:
		return clientFormatGemini
	default:
		return clientFormatOpenAI
	}
}

func detectClientFormatOverride(c *gin.Context) clientFormat {
	if c == nil {
		return ""
	}
	raw := firstNonEmpty(
		c.GetHeader("x-openai-api-type"),
		c.GetHeader("x-cch-api-type"),
		c.GetHeader("openai-beta"),
		c.Query("api_type"),
		c.Query("apiType"),
		c.Query("format"),
	)
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "response", "responses", "codex":
		return clientFormatResponse
	case "openai", "chat":
		return clientFormatOpenAI
	case "claude", "anthropic":
		return clientFormatClaude
	case "gemini":
		return clientFormatGemini
	case "gemini-cli", "geminicli":
		return clientFormatGeminiCLI
	default:
		return ""
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func allowedProviderTypes(format clientFormat) []string {
	switch format {
	case clientFormatClaude:
		return []string{string(model.ProviderTypeClaude), string(model.ProviderTypeClaudeAuth)}
	case clientFormatGemini, clientFormatGeminiCLI:
		return []string{string(model.ProviderTypeGemini), string(model.ProviderTypeGeminiCli)}
	case clientFormatResponse:
		return []string{string(model.ProviderTypeCodex)}
	default:
		return []string{string(model.ProviderTypeCodex), string(model.ProviderTypeOpenAICompatible)}
	}
}

func formatOpenAIModelsResponse(models []fetchedModel) gin.H {
	now := time.Now().Unix()
	data := make([]gin.H, 0, len(models))
	for _, item := range models {
		data = append(data, gin.H{
			"id":       item.ID,
			"object":   "model",
			"created":  now,
			"owned_by": inferOwner(item.ID),
		})
	}
	return gin.H{
		"object": "list",
		"data":   data,
	}
}

func formatAnthropicModelsResponse(models []fetchedModel) gin.H {
	now := time.Now().UTC().Format(time.RFC3339)
	data := make([]gin.H, 0, len(models))
	for _, item := range models {
		createdAt := item.CreatedAt
		if createdAt == "" {
			createdAt = now
		}
		displayName := item.DisplayName
		if displayName == "" {
			displayName = item.ID
		}
		data = append(data, gin.H{
			"id":           item.ID,
			"type":         "model",
			"display_name": displayName,
			"created_at":   createdAt,
		})
	}
	return gin.H{
		"data":     data,
		"has_more": false,
	}
}

func formatGeminiModelsResponse(models []fetchedModel) gin.H {
	items := make([]gin.H, 0, len(models))
	for _, item := range models {
		displayName := item.DisplayName
		if displayName == "" {
			displayName = item.ID
		}
		items = append(items, gin.H{
			"name":                       "models/" + item.ID,
			"displayName":                displayName,
			"supportedGenerationMethods": []string{"generateContent"},
		})
	}
	return gin.H{"models": items}
}

func inferOwner(modelID string) string {
	switch {
	case strings.HasPrefix(modelID, "claude-"):
		return "anthropic"
	case strings.HasPrefix(modelID, "gpt-"), strings.HasPrefix(modelID, "o1"), strings.HasPrefix(modelID, "o3"):
		return "openai"
	case strings.HasPrefix(modelID, "gemini-"):
		return "google"
	case strings.HasPrefix(modelID, "deepseek"):
		return "deepseek"
	case strings.HasPrefix(modelID, "qwen"):
		return "alibaba"
	default:
		return "unknown"
	}
}
