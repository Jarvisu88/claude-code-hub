package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
	"github.com/quagmt/udecimal"
)

var allowedWebhookProviderTypes = map[string]struct{}{
	"wechat":   {},
	"feishu":   {},
	"dingtalk": {},
	"telegram": {},
	"custom":   {},
}

var allowedNotificationTypes = map[string]struct{}{
	"circuit_breaker":   {},
	"daily_leaderboard": {},
	"cost_alert":        {},
}

func isValidWebhookProviderType(value string) bool {
	_, ok := allowedWebhookProviderTypes[strings.TrimSpace(value)]
	return ok
}

func isValidNotificationType(value string) bool {
	_, ok := allowedNotificationTypes[strings.TrimSpace(value)]
	return ok
}

func isValidHHMM(value string) bool {
	if _, err := time.Parse("15:04", value); err != nil {
		return false
	}
	return len(value) == len("15:04")
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func parseRequiredBodyID(c *gin.Context) (int, bool) {
	var input struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return 0, false
	}
	if input.ID <= 0 {
		writeAdminError(c, appErrors.NewInvalidRequest("id is required"))
		return 0, false
	}
	return input.ID, true
}

func rewriteBodyForActionUpdate(c *gin.Context, transformer func(map[string]any) (map[string]any, error)) bool {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return false
	}
	var raw map[string]any
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return false
	}
	if transformer != nil {
		raw, err = transformer(raw)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest(err.Error()))
			return false
		}
	}
	normalized, _ := json.Marshal(raw)
	c.Request.Body = io.NopCloser(bytes.NewReader(normalized))
	return true
}

func extractIDAndRewriteBody(c *gin.Context, transformer func(map[string]any) (map[string]any, error)) bool {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return false
	}
	var raw map[string]any
	if err := json.Unmarshal(bodyBytes, &raw); err != nil {
		writeAdminError(c, appErrors.NewInvalidRequest("request body must be valid JSON"))
		return false
	}
	idValue, ok := raw["id"].(float64)
	if !ok || int(idValue) <= 0 {
		writeAdminError(c, appErrors.NewInvalidRequest("id is required"))
		return false
	}
	if transformer != nil {
		raw, err = transformer(raw)
		if err != nil {
			writeAdminError(c, appErrors.NewInvalidRequest(err.Error()))
			return false
		}
	}
	normalized, _ := json.Marshal(raw)
	c.Request.Body = io.NopCloser(bytes.NewReader(normalized))
	c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(int(idValue))}}
	return true
}

func validateWebhookTargetInput(target *model.WebhookTarget, requireName bool) error {
	if target == nil {
		return appErrors.NewInvalidRequest("webhook target is required")
	}
	if requireName && strings.TrimSpace(target.Name) == "" {
		return appErrors.NewInvalidRequest("name is required")
	}
	if target.ProviderType != "" && !isValidWebhookProviderType(target.ProviderType) {
		return appErrors.NewInvalidRequest("providerType is invalid")
	}
	if strings.TrimSpace(target.ProviderType) == "telegram" {
		if trimStringValue(target.TelegramBotToken) == "" || trimStringValue(target.TelegramChatId) == "" {
			return appErrors.NewInvalidRequest("telegramBotToken and telegramChatId are required")
		}
		return nil
	}
	if target.WebhookUrl == nil || strings.TrimSpace(*target.WebhookUrl) == "" {
		return appErrors.NewInvalidRequest("webhookUrl is required")
	}
	if _, err := url.ParseRequestURI(strings.TrimSpace(*target.WebhookUrl)); err != nil {
		return appErrors.NewInvalidRequest("webhookUrl is invalid")
	}
	return nil
}

func parseCostAlertThreshold(value float64) (udecimal.Decimal, error) {
	if value < 0 || value > 1 {
		return udecimal.Zero, fmt.Errorf("costAlertThreshold must be between 0 and 1")
	}
	return udecimal.MustParse(strconv.FormatFloat(value, 'f', -1, 64)), nil
}
