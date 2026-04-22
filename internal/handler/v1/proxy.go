package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	stderrors "errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	sessionsvc "github.com/ding113/claude-code-hub/internal/service/session"
	"github.com/gin-gonic/gin"
)

const authResultContextKey = "proxy_auth_result"
const proxySessionIDContextKey = "proxy_session_id"

type sessionManager interface {
	ExtractClientSessionID(requestBody map[string]any, headers http.Header) sessionsvc.ClientSessionExtractionResult
	GetOrCreateSessionID(ctx context.Context, keyID int, messages any, clientSessionID string) string
	GetNextRequestSequence(ctx context.Context, sessionID string) int
	IncrementConcurrentCount(ctx context.Context, sessionID string)
	DecrementConcurrentCount(ctx context.Context, sessionID string)
}

type providerRepository interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
}

type messageRequestRepository interface {
	Create(ctx context.Context, messageRequest *model.MessageRequest) (*model.MessageRequest, error)
	UpdateTerminal(ctx context.Context, id int, statusCode int, durationMs int, errorMessage *string) error
}

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Handler 承载 /v1 代理入口的最小可用接线。
// 当前阶段先把鉴权链接入 Gin，后续再逐步替换 501 占位逻辑。
type Handler struct {
	auth        *authsvc.Service
	sessions    sessionManager
	providers   providerRepository
	requestLogs messageRequestRepository
	http        httpDoer
}

type proxyEndpointKind string

const (
	proxyEndpointMessages        proxyEndpointKind = "messages"
	proxyEndpointMessagesCount   proxyEndpointKind = "messages_count_tokens"
	proxyEndpointResponses       proxyEndpointKind = "responses"
	proxyEndpointChatCompletions proxyEndpointKind = "chat_completions"
)

func NewHandler(auth *authsvc.Service, sessions sessionManager, providers providerRepository, requestLogs messageRequestRepository, httpClient httpDoer) *Handler {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Handler{
		auth:        auth,
		sessions:    sessions,
		providers:   providers,
		requestLogs: requestLogs,
		http:        httpClient,
	}
}

func (h *Handler) RegisterRoutes(group *gin.RouterGroup) {
	protected := group.Group("")
	protected.Use(h.AuthMiddleware())
	protected.Use(h.SessionLifecycleMiddleware())
	protected.POST("/messages", h.messages)
	protected.POST("/messages/count_tokens", h.messagesCountTokens)
	protected.POST("/chat/completions", h.chatCompletions)
	protected.POST("/responses", h.responses)
	protected.GET("/models", h.models(""))
	protected.GET("/responses/models", h.models(proxyEndpointResponses))
	protected.GET("/chat/completions/models", h.models(proxyEndpointChatCompletions))
	protected.GET("/chat/models", h.models(proxyEndpointChatCompletions))
}

func (h *Handler) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.auth == nil {
			appErr := appErrors.NewInternalError("代理鉴权服务未初始化")
			c.AbortWithStatusJSON(appErr.HTTPStatus, appErr.ToResponse())
			return
		}

		result, err := h.auth.AuthenticateProxy(c.Request.Context(), authsvc.ProxyAuthInput{
			AuthorizationHeader: c.GetHeader("Authorization"),
			APIKeyHeader:        c.GetHeader("x-api-key"),
			GeminiAPIKeyHeader:  c.GetHeader("x-goog-api-key"),
			GeminiAPIKeyQuery:   c.Query("key"),
		})
		if err != nil {
			writeAppError(c, err)
			return
		}

		c.Set(authResultContextKey, result)
		c.Next()
	}
}

func GetAuthResult(c *gin.Context) (*authsvc.AuthResult, bool) {
	if c == nil {
		return nil, false
	}
	value, ok := c.Get(authResultContextKey)
	if !ok {
		return nil, false
	}
	result, ok := value.(*authsvc.AuthResult)
	return result, ok && result != nil
}

func GetProxySessionID(c *gin.Context) (string, bool) {
	if c == nil {
		return "", false
	}
	value, ok := c.Get(proxySessionIDContextKey)
	if !ok {
		return "", false
	}
	sessionID, ok := value.(string)
	return sessionID, ok && sessionID != ""
}

func (h *Handler) SessionLifecycleMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !shouldTrackConcurrentRequests(c) || h == nil || h.sessions == nil {
			c.Next()
			return
		}

		authResult, ok := GetAuthResult(c)
		if !ok || authResult == nil || authResult.Key == nil {
			c.Next()
			return
		}

		requestBody, ok := decodeRequestBody(c)
		if !ok {
			c.Next()
			return
		}

		extracted := h.sessions.ExtractClientSessionID(requestBody, c.Request.Header)
		sessionID := h.sessions.GetOrCreateSessionID(c.Request.Context(), authResult.Key.ID, extractRequestMessages(requestBody), extracted.SessionID)
		if sessionID == "" {
			c.Next()
			return
		}

		c.Set(proxySessionIDContextKey, sessionID)
		h.sessions.IncrementConcurrentCount(c.Request.Context(), sessionID)
		defer h.sessions.DecrementConcurrentCount(c.Request.Context(), sessionID)

		c.Next()
	}
}

func (h *Handler) notImplemented(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": gin.H{
			"type":    "not_implemented",
			"message": "This endpoint is not yet implemented",
		},
	})
}

func (h *Handler) messages(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointMessages)
}

func (h *Handler) messagesCountTokens(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointMessagesCount)
}

func (h *Handler) models(endpointKind proxyEndpointKind) gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.providers == nil {
			h.notImplemented(c)
			return
		}

		providers, err := h.providers.GetActiveProviders(c.Request.Context())
		if err != nil {
			writeAppError(c, err)
			return
		}

		seen := map[string]struct{}{}
		models := make([]gin.H, 0)
		for _, provider := range providers {
			if provider == nil || !provider.IsActive() {
				continue
			}
			if endpointKind != "" && !supportsEndpointKind(provider.ProviderType, endpointKind) {
				continue
			}
			for _, modelName := range provider.AllowedModels {
				modelName = strings.TrimSpace(modelName)
				if modelName == "" {
					continue
				}
				if _, ok := seen[modelName]; ok {
					continue
				}
				seen[modelName] = struct{}{}
				models = append(models, gin.H{
					"id":     modelName,
					"object": "model",
				})
			}
		}

		sort.Slice(models, func(i, j int) bool {
			return models[i]["id"].(string) < models[j]["id"].(string)
		})

		c.JSON(http.StatusOK, gin.H{"data": models, "object": "list"})
	}
}

func (h *Handler) responses(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointResponses)
}

func (h *Handler) chatCompletions(c *gin.Context) {
	h.proxyEndpoint(c, proxyEndpointChatCompletions)
}

func (h *Handler) proxyEndpoint(c *gin.Context, endpointKind proxyEndpointKind) {
	if h == nil || h.providers == nil || h.http == nil {
		writeAppError(c, appErrors.NewInternalError("代理转发服务未初始化"))
		return
	}

	authResult, ok := GetAuthResult(c)
	if !ok || authResult == nil || authResult.Key == nil {
		writeAppError(c, appErrors.NewInternalError("代理鉴权上下文缺失"))
		return
	}

	requestBody, requestBodyBytes, ok := decodeRequestBodyBytes(c)
	if !ok {
		writeAppError(c, appErrors.NewInvalidRequest("请求体不是合法 JSON"))
		return
	}

	provider, err := h.selectProviderForEndpoint(c.Request.Context(), endpointKind, requestStringValue(requestBody["model"]))
	if err != nil {
		writeAppError(c, err)
		return
	}

	sessionID, _ := GetProxySessionID(c)
	requestSequence := 1
	if sessionID != "" && h.sessions != nil {
		requestSequence = h.sessions.GetNextRequestSequence(c.Request.Context(), sessionID)
	}
	startedAt := time.Now()
	messageRequestID := h.createMessageRequest(c, authResult, provider, requestBody, sessionID, requestSequence)

	upstreamURL, err := buildProxyURL(provider.URL, c.Request.URL)
	if err != nil {
		errorMessage := "构建上游代理地址失败"
		h.finalizeMessageRequest(c, messageRequestID, http.StatusInternalServerError, time.Since(startedAt), &errorMessage)
		writeAppError(c, appErrors.NewInternalError("构建上游代理地址失败").WithError(err))
		return
	}

	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, bytes.NewReader(requestBodyBytes))
	if err != nil {
		errorMessage := "构建上游请求失败"
		h.finalizeMessageRequest(c, messageRequestID, http.StatusInternalServerError, time.Since(startedAt), &errorMessage)
		writeAppError(c, appErrors.NewInternalError("构建上游请求失败").WithError(err))
		return
	}

	copyProxyRequestHeaders(upstreamReq.Header, c.Request.Header)
	applyProviderAuthHeaders(upstreamReq.Header, provider, endpointKind)

	upstreamResp, err := h.http.Do(upstreamReq)
	if err != nil {
		errMsg := "上游 Responses 供应商请求失败"
		if endpointKind == proxyEndpointMessages {
			errMsg = "上游 Messages 供应商请求失败"
		}
		if endpointKind == proxyEndpointMessagesCount {
			errMsg = "上游 Count Tokens 供应商请求失败"
		}
		if endpointKind == proxyEndpointChatCompletions {
			errMsg = "上游 Chat Completions 供应商请求失败"
		}
		if isTimeoutError(err) {
			timeoutMessage := errMsg + "：请求超时"
			h.finalizeMessageRequest(c, messageRequestID, http.StatusGatewayTimeout, time.Since(startedAt), &timeoutMessage)
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeProviderError,
				Message:    timeoutMessage,
				Code:       appErrors.CodeProviderTimeout,
				HTTPStatus: http.StatusGatewayTimeout,
			}).WithError(err))
			return
		}
		h.finalizeMessageRequest(c, messageRequestID, http.StatusBadGateway, time.Since(startedAt), &errMsg)
		writeAppError(c, appErrors.NewProviderError(errMsg, appErrors.CodeProviderError).WithError(err))
		return
	}
	defer upstreamResp.Body.Close()

	copyProxyResponseHeaders(c.Writer.Header(), upstreamResp.Header)
	c.Status(upstreamResp.StatusCode)
	if _, err := io.Copy(c.Writer, upstreamResp.Body); err != nil {
		c.Error(err)
	}

	h.finalizeMessageRequest(c, messageRequestID, upstreamResp.StatusCode, time.Since(startedAt), nil)
}

func writeAppError(c *gin.Context, err error) {
	var appErr *appErrors.AppError
	if stderrors.As(err, &appErr) {
		c.AbortWithStatusJSON(appErr.HTTPStatus, appErr.ToResponse())
		return
	}

	fallback := appErrors.NewInternalError("代理鉴权失败")
	c.AbortWithStatusJSON(fallback.HTTPStatus, fallback.ToResponse())
}

func shouldTrackConcurrentRequests(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	if c.Request.Method != http.MethodPost {
		return false
	}

	switch c.FullPath() {
	case "/v1/messages", "/v1/chat/completions", "/v1/responses":
		return true
	default:
		return false
	}
}

func decodeRequestBody(c *gin.Context) (map[string]any, bool) {
	requestBody, _, ok := decodeRequestBodyBytes(c)
	return requestBody, ok
}

func decodeRequestBodyBytes(c *gin.Context) (map[string]any, []byte, bool) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, nil, false
	}

	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, nil, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))

	if len(bytes.TrimSpace(bodyBytes)) == 0 {
		return map[string]any{}, bodyBytes, true
	}

	var requestBody map[string]any
	if err := json.Unmarshal(bodyBytes, &requestBody); err != nil {
		return nil, nil, false
	}

	return requestBody, bodyBytes, true
}

func extractRequestMessages(requestBody map[string]any) any {
	if requestBody == nil {
		return nil
	}
	if messages, ok := requestBody["messages"]; ok {
		return messages
	}
	if input, ok := requestBody["input"]; ok {
		return input
	}
	return nil
}

func requestStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func (h *Handler) selectProviderForEndpoint(ctx context.Context, endpointKind proxyEndpointKind, requestedModel string) (*model.Provider, error) {
	providers, err := h.providers.GetActiveProviders(ctx)
	if err != nil {
		return nil, err
	}

	candidates := make([]*model.Provider, 0, len(providers))
	for _, provider := range providers {
		if provider == nil || !provider.IsActive() {
			continue
		}
		if !supportsEndpointKind(provider.ProviderType, endpointKind) {
			continue
		}
		if requestedModel != "" && !provider.SupportsModel(requestedModel) {
			continue
		}
		candidates = append(candidates, provider)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		leftPriority := 0
		if candidates[i].Priority != nil {
			leftPriority = *candidates[i].Priority
		}
		rightPriority := 0
		if candidates[j].Priority != nil {
			rightPriority = *candidates[j].Priority
		}
		if leftPriority != rightPriority {
			return leftPriority < rightPriority
		}

		leftWeight := 1
		if candidates[i].Weight != nil {
			leftWeight = *candidates[i].Weight
		}
		rightWeight := 1
		if candidates[j].Weight != nil {
			rightWeight = *candidates[j].Weight
		}
		return leftWeight > rightWeight
	})

	if len(candidates) == 0 {
		message := "没有可用的 Responses 供应商。"
		if endpointKind == proxyEndpointMessages {
			message = "没有可用的 Messages 供应商。"
		} else if endpointKind == proxyEndpointMessagesCount {
			message = "没有可用的 Count Tokens 供应商。"
		} else if endpointKind == proxyEndpointChatCompletions {
			message = "没有可用的 Chat Completions 供应商。"
		}
		return nil, (&appErrors.AppError{
			Type:       appErrors.ErrorTypeProviderError,
			Message:    message,
			Code:       appErrors.CodeNoProviderAvailable,
			HTTPStatus: http.StatusServiceUnavailable,
		})
	}

	return candidates[0], nil
}

func buildProxyURL(baseURL string, requestURL *url.URL) (string, error) {
	parsedBaseURL, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	if requestURL == nil {
		return parsedBaseURL.String(), nil
	}

	basePath := strings.TrimRight(parsedBaseURL.Path, "/")
	requestPath := requestURL.Path

	if requestPath == basePath || strings.HasPrefix(requestPath, basePath+"/") {
		parsedBaseURL.Path = requestPath
		parsedBaseURL.RawQuery = requestURL.RawQuery
		return parsedBaseURL.String(), nil
	}

	if strings.HasSuffix(basePath, "/responses") || strings.HasSuffix(basePath, "/v1/responses") ||
		strings.HasSuffix(basePath, "/messages") || strings.HasSuffix(basePath, "/v1/messages") ||
		strings.HasSuffix(basePath, "/chat/completions") || strings.HasSuffix(basePath, "/v1/chat/completions") {
		parsedBaseURL.Path = basePath
		parsedBaseURL.RawQuery = requestURL.RawQuery
		return parsedBaseURL.String(), nil
	}

	parsedBaseURL.Path = basePath + requestPath
	parsedBaseURL.RawQuery = requestURL.RawQuery
	return parsedBaseURL.String(), nil
}

func copyProxyRequestHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		normalized := strings.ToLower(key)
		if normalized == "authorization" || normalized == "x-api-key" || normalized == "x-goog-api-key" || normalized == "host" || normalized == "content-length" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func supportsEndpointKind(providerType string, endpointKind proxyEndpointKind) bool {
	switch endpointKind {
	case proxyEndpointMessages:
		fallthrough
	case proxyEndpointMessagesCount:
		return providerType == string(model.ProviderTypeClaude) || providerType == string(model.ProviderTypeClaudeAuth)
	case proxyEndpointResponses:
		return providerType == string(model.ProviderTypeCodex) || providerType == string(model.ProviderTypeOpenAICompatible)
	case proxyEndpointChatCompletions:
		return providerType == string(model.ProviderTypeOpenAICompatible)
	default:
		return false
	}
}

func applyProviderAuthHeaders(dst http.Header, provider *model.Provider, endpointKind proxyEndpointKind) {
	if provider == nil {
		return
	}

	dst.Set("Content-Type", "application/json")

	switch endpointKind {
	case proxyEndpointMessages:
		fallthrough
	case proxyEndpointMessagesCount:
		dst.Set("Authorization", "Bearer "+provider.Key)
		dst.Set("x-api-key", provider.Key)
		if provider.ProviderType == string(model.ProviderTypeClaudeAuth) {
			dst.Del("x-api-key")
		}
	default:
		dst.Set("Authorization", "Bearer "+provider.Key)
	}
}

func isTimeoutError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}

func copyProxyResponseHeaders(dst http.Header, src http.Header) {
	for key, values := range src {
		normalized := strings.ToLower(key)
		if normalized == "content-length" || normalized == "transfer-encoding" || normalized == "connection" {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

func (h *Handler) createMessageRequest(
	c *gin.Context,
	authResult *authsvc.AuthResult,
	provider *model.Provider,
	requestBody map[string]any,
	sessionID string,
	requestSequence int,
) int {
	if h == nil || h.requestLogs == nil || authResult == nil || authResult.User == nil || authResult.Key == nil || provider == nil {
		return 0
	}

	modelName := requestStringValue(requestBody["model"])
	endpoint := ""
	userAgent := ""
	messagesCount := countRequestPayloadItems(extractRequestMessages(requestBody))
	if c != nil {
		if c.FullPath() != "" {
			endpoint = c.FullPath()
		}
		if c.Request != nil {
			userAgent = c.Request.UserAgent()
		}
	}

	messageRequest := &model.MessageRequest{
		ProviderID:      provider.ID,
		UserID:          authResult.User.ID,
		Key:             authResult.Key.Key,
		Model:           modelName,
		SessionID:       stringPointer(sessionID),
		RequestSequence: requestSequence,
		ApiType:         stringPointer(string(resolveAPIType(endpoint))),
		Endpoint:        stringPointer(endpoint),
		OriginalModel:   stringPointer(modelName),
		ProviderChain: []model.ProviderChainItem{
			{
				ID:   provider.ID,
				Name: provider.Name,
			},
		},
		UserAgent:     stringPointer(userAgent),
		MessagesCount: intPointer(messagesCount),
	}

	if _, err := h.requestLogs.Create(c.Request.Context(), messageRequest); err != nil {
		// persistence is best-effort for the minimal slice
		return 0
	}
	return messageRequest.ID
}

func (h *Handler) finalizeMessageRequest(c *gin.Context, id int, statusCode int, duration time.Duration, errorMessage *string) {
	if h == nil || h.requestLogs == nil || id <= 0 {
		return
	}
	reqCtx := context.Background()
	if c != nil && c.Request != nil {
		reqCtx = c.Request.Context()
	}
	_ = h.requestLogs.UpdateTerminal(reqCtx, id, statusCode, int(duration.Milliseconds()), errorMessage)
}

type proxyAPIType string

const (
	proxyAPITypeClaude  proxyAPIType = "claude"
	proxyAPITypeOpenAI  proxyAPIType = "openai"
	proxyAPITypeCodex   proxyAPIType = "codex"
	proxyAPITypeUnknown proxyAPIType = "unknown"
)

func resolveAPIType(endpoint string) proxyAPIType {
	switch endpoint {
	case "/v1/messages", "/v1/messages/count_tokens":
		return proxyAPITypeClaude
	case "/v1/chat/completions":
		return proxyAPITypeOpenAI
	case "/v1/responses":
		return proxyAPITypeCodex
	default:
		return proxyAPITypeUnknown
	}
}

func countRequestPayloadItems(messages any) int {
	items, ok := messages.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func stringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func intPointer(value int) *int {
	return &value
}
