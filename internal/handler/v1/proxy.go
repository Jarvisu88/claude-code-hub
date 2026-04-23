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
	"github.com/ding113/claude-code-hub/internal/repository"
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
	UpdateTerminal(ctx context.Context, id int, update repository.MessageRequestTerminalUpdate) error
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
			for _, modelName := range provider.AllowedModels.ExactModelNames() {
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

	originalModel := requestStringValue(requestBody["model"])
	provider, err := h.selectProviderForEndpoint(c.Request.Context(), endpointKind, originalModel)
	if err != nil {
		writeAppError(c, err)
		return
	}
	baseProviderChain := buildInitialProviderChain(provider)
	effectiveModel := originalModel
	if redirectedModel := provider.GetRedirectedModel(originalModel); redirectedModel != "" && redirectedModel != originalModel {
		requestBody["model"] = redirectedModel
		requestBodyBytes, err = json.Marshal(requestBody)
		if err != nil {
			errorMessage := "重写上游模型请求失败"
			h.finalizeMessageRequest(c, 0, repository.MessageRequestTerminalUpdate{
				StatusCode:   http.StatusInternalServerError,
				DurationMs:   0,
				ErrorMessage: &errorMessage,
			})
			writeAppError(c, appErrors.NewInternalError("重写上游模型请求失败").WithError(err))
			return
		}
		effectiveModel = redirectedModel
	}

	sessionID, _ := GetProxySessionID(c)
	requestSequence := 1
	if sessionID != "" && h.sessions != nil {
		requestSequence = h.sessions.GetNextRequestSequence(c.Request.Context(), sessionID)
	}
	startedAt := time.Now()
	messageRequestID := h.createMessageRequest(c, authResult, provider, baseProviderChain, requestBody, originalModel, effectiveModel, sessionID, requestSequence)

	upstreamURL, err := buildProxyURL(provider.URL, c.Request.URL)
	if err != nil {
		errorMessage := "构建上游代理地址失败"
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    http.StatusInternalServerError,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ErrorMessage:  &errorMessage,
			ProviderChain: finalizeProviderChain(baseProviderChain, http.StatusInternalServerError, &errorMessage),
		})
		writeAppError(c, appErrors.NewInternalError("构建上游代理地址失败").WithError(err))
		return
	}

	upstreamReq, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, bytes.NewReader(requestBodyBytes))
	if err != nil {
		errorMessage := "构建上游请求失败"
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    http.StatusInternalServerError,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ErrorMessage:  &errorMessage,
			ProviderChain: finalizeProviderChain(baseProviderChain, http.StatusInternalServerError, &errorMessage),
		})
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
			h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
				StatusCode:    http.StatusGatewayTimeout,
				DurationMs:    int(time.Since(startedAt).Milliseconds()),
				ErrorMessage:  &timeoutMessage,
				ProviderChain: finalizeProviderChain(baseProviderChain, http.StatusGatewayTimeout, &timeoutMessage),
			})
			writeAppError(c, (&appErrors.AppError{
				Type:       appErrors.ErrorTypeProviderError,
				Message:    timeoutMessage,
				Code:       appErrors.CodeProviderTimeout,
				HTTPStatus: http.StatusGatewayTimeout,
			}).WithError(err))
			return
		}
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    http.StatusBadGateway,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ErrorMessage:  &errMsg,
			ProviderChain: finalizeProviderChain(baseProviderChain, http.StatusBadGateway, &errMsg),
		})
		writeAppError(c, appErrors.NewProviderError(errMsg, appErrors.CodeProviderError).WithError(err))
		return
	}
	defer upstreamResp.Body.Close()

	copyProxyResponseHeaders(c.Writer.Header(), upstreamResp.Header)
	if isStreamingResponse(upstreamResp.Header) {
		c.Status(upstreamResp.StatusCode)
		if _, err := io.Copy(c.Writer, upstreamResp.Body); err != nil {
			c.Error(err)
		}
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    upstreamResp.StatusCode,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ProviderChain: finalizeProviderChain(baseProviderChain, upstreamResp.StatusCode, nil),
		})
		return
	}

	responseBody, err := io.ReadAll(upstreamResp.Body)
	if err != nil {
		errorMessage := "读取上游响应失败"
		h.finalizeMessageRequest(c, messageRequestID, repository.MessageRequestTerminalUpdate{
			StatusCode:    http.StatusBadGateway,
			DurationMs:    int(time.Since(startedAt).Milliseconds()),
			ErrorMessage:  &errorMessage,
			ProviderChain: finalizeProviderChain(baseProviderChain, http.StatusBadGateway, &errorMessage),
		})
		writeAppError(c, appErrors.NewProviderError(errorMessage, appErrors.CodeProviderError).WithError(err))
		return
	}
	terminalUpdate := buildTerminalUpdate(endpointKind, upstreamResp.StatusCode, time.Since(startedAt), responseBody)
	terminalUpdate.ProviderChain = finalizeProviderChain(baseProviderChain, terminalUpdate.StatusCode, terminalUpdate.ErrorMessage)
	finalStatusCode := terminalUpdate.StatusCode
	c.Status(finalStatusCode)
	if finalStatusCode != upstreamResp.StatusCode {
		c.Writer.Header().Del("Content-Length")
	}
	if _, err := c.Writer.Write(responseBody); err != nil {
		c.Error(err)
	}
	h.finalizeMessageRequest(c, messageRequestID, terminalUpdate)
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

		leftCostMultiplier := 1.0
		if candidates[i].CostMultiplier != nil {
			leftCostMultiplier = candidates[i].CostMultiplier.InexactFloat64()
		}
		rightCostMultiplier := 1.0
		if candidates[j].CostMultiplier != nil {
			rightCostMultiplier = candidates[j].CostMultiplier.InexactFloat64()
		}
		if leftCostMultiplier != rightCostMultiplier {
			return leftCostMultiplier < rightCostMultiplier
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
		return providerType == string(model.ProviderTypeCodex)
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
		if provider.ProviderType == string(model.ProviderTypeGemini) || provider.ProviderType == string(model.ProviderTypeGeminiCli) {
			dst.Del("Authorization")
			dst.Set("x-goog-api-key", provider.Key)
			return
		}
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

func isStreamingResponse(headers http.Header) bool {
	if headers == nil {
		return false
	}
	return strings.Contains(strings.ToLower(headers.Get("Content-Type")), "text/event-stream")
}

func (h *Handler) createMessageRequest(
	c *gin.Context,
	authResult *authsvc.AuthResult,
	provider *model.Provider,
	providerChain []model.ProviderChainItem,
	requestBody map[string]any,
	originalModel string,
	effectiveModel string,
	sessionID string,
	requestSequence int,
) int {
	if h == nil || h.requestLogs == nil || authResult == nil || authResult.User == nil || authResult.Key == nil || provider == nil {
		return 0
	}

	endpoint := ""
	userAgent := ""
	clientIP := ""
	messagesCount := countRequestPayloadItems(extractRequestMessages(requestBody))
	if c != nil {
		if c.FullPath() != "" {
			endpoint = c.FullPath()
		}
		if c.Request != nil {
			userAgent = c.Request.UserAgent()
			clientIP = c.ClientIP()
		}
	}

	messageRequest := &model.MessageRequest{
		ProviderID:      provider.ID,
		UserID:          authResult.User.ID,
		Key:             authResult.Key.Key,
		Model:           effectiveModel,
		CostMultiplier:  provider.CostMultiplier,
		SessionID:       stringPointer(sessionID),
		RequestSequence: requestSequence,
		ApiType:         stringPointer(string(resolveAPIType(endpoint))),
		Endpoint:        stringPointer(endpoint),
		OriginalModel:   stringPointer(originalModel),
		ProviderChain:   providerChain,
		UserAgent:       stringPointer(userAgent),
		ClientIP:        stringPointer(clientIP),
		MessagesCount:   intPointer(messagesCount),
	}

	if _, err := h.requestLogs.Create(c.Request.Context(), messageRequest); err != nil {
		// persistence is best-effort for the minimal slice
		return 0
	}
	return messageRequest.ID
}

func providerCostMultiplier(provider *model.Provider) *float64 {
	if provider == nil || provider.CostMultiplier == nil {
		return nil
	}
	value := provider.CostMultiplier.InexactFloat64()
	return &value
}

func buildInitialProviderChain(provider *model.Provider) []model.ProviderChainItem {
	if provider == nil {
		return nil
	}
	return []model.ProviderChainItem{{
		ID:              provider.ID,
		Name:            provider.Name,
		ProviderType:    stringPointer(provider.ProviderType),
		EndpointURL:     stringPointer(provider.URL),
		Reason:          stringPointer("initial_selection"),
		SelectionMethod: stringPointer("weighted_random"),
		Priority:        provider.Priority,
		Weight:          provider.Weight,
		CostMultiplier:  providerCostMultiplier(provider),
		Timestamp:       int64Pointer(time.Now().UnixMilli()),
	}}
}

func finalizeProviderChain(base []model.ProviderChainItem, statusCode int, errorMessage *string) []model.ProviderChainItem {
	if len(base) == 0 {
		return nil
	}
	finalized := make([]model.ProviderChainItem, len(base))
	copy(finalized, base)
	finalized[len(finalized)-1].StatusCode = intPointer(statusCode)
	finalized[len(finalized)-1].ErrorMessage = errorMessage
	return finalized
}

func buildTerminalUpdate(endpointKind proxyEndpointKind, statusCode int, duration time.Duration, responseBody []byte) repository.MessageRequestTerminalUpdate {
	update := repository.MessageRequestTerminalUpdate{
		StatusCode: statusCode,
		DurationMs: int(duration.Milliseconds()),
	}

	if len(bytes.TrimSpace(responseBody)) == 0 {
		return update
	}

	var payload map[string]any
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		if fakeStatusCode, fakeErrorMessage, ok := detectFake200HTMLResponse(statusCode, responseBody); ok {
			update.StatusCode = fakeStatusCode
			update.ErrorMessage = &fakeErrorMessage
		} else if fakeStatusCode, fakeErrorMessage, ok := detectFake200PlainTextResponse(statusCode, responseBody); ok {
			update.StatusCode = fakeStatusCode
			update.ErrorMessage = &fakeErrorMessage
		}
		return update
	}
	if fakeStatusCode, fakeErrorMessage, ok := detectFake200JSONResponse(statusCode, payload, responseBody); ok {
		update.StatusCode = fakeStatusCode
		update.ErrorMessage = &fakeErrorMessage
		return update
	}

	if statusCode >= http.StatusBadRequest {
		if errorMessage := extractErrorMessage(payload); errorMessage != "" {
			update.ErrorMessage = &errorMessage
		}
		return update
	}

	switch endpointKind {
	case proxyEndpointMessages:
		populateAnthropicUsageUpdate(&update, payload)
	case proxyEndpointMessagesCount:
		if inputTokens, ok := lookupInt(payload, "input_tokens"); ok {
			update.InputTokens = &inputTokens
		}
	case proxyEndpointResponses:
		populateResponsesUsageUpdate(&update, payload)
	case proxyEndpointChatCompletions:
		populateChatCompletionsUsageUpdate(&update, payload)
	}

	return update
}

func detectFake200HTMLResponse(statusCode int, responseBody []byte) (int, string, bool) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return 0, "", false
	}
	trimmed := strings.TrimSpace(string(responseBody))
	if trimmed == "" {
		return 0, "", false
	}
	lowerTrimmed := strings.ToLower(trimmed)
	if strings.HasPrefix(lowerTrimmed, "<!doctype html") || strings.HasPrefix(lowerTrimmed, "<html") {
		return http.StatusBadGateway, "上游返回了 HTML 错误页", true
	}
	return 0, "", false
}

func detectFake200PlainTextResponse(statusCode int, responseBody []byte) (int, string, bool) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return 0, "", false
	}
	trimmed := strings.TrimSpace(string(responseBody))
	if trimmed == "" {
		return 0, "", false
	}
	if !isLikelyUpstreamErrorMessage(trimmed) {
		return 0, "", false
	}
	return inferUpstreamErrorStatusCode(trimmed, responseBody), trimmed, true
}

func detectFake200JSONResponse(statusCode int, payload map[string]any, responseBody []byte) (int, string, bool) {
	if statusCode < http.StatusOK || statusCode >= http.StatusMultipleChoices {
		return 0, "", false
	}
	if payload == nil {
		return 0, "", false
	}
	errorMessage := extractErrorMessage(payload)
	if errorMessage != "" {
		return inferUpstreamErrorStatusCode(errorMessage, responseBody), errorMessage, true
	}
	if message, ok := payload["message"].(string); ok && isLikelyUpstreamErrorMessage(message) {
		message = strings.TrimSpace(message)
		return inferUpstreamErrorStatusCode(message, responseBody), message, true
	}
	return 0, "", false
}

func isLikelyUpstreamErrorMessage(message string) bool {
	lowerMessage := strings.ToLower(strings.TrimSpace(message))
	if lowerMessage == "" {
		return false
	}
	keywords := []string{
		"error",
		"rate limit",
		"too many requests",
		"forbidden",
		"unauthorized",
		"not found",
		"invalid",
		"timeout",
		"timed out",
		"service unavailable",
		"overloaded",
		"限流",
		"未授权",
		"无权限",
		"超时",
		"不可用",
	}
	for _, keyword := range keywords {
		if strings.Contains(lowerMessage, keyword) {
			return true
		}
	}
	return false
}

func inferUpstreamErrorStatusCode(message string, responseBody []byte) int {
	lowerText := strings.ToLower(strings.TrimSpace(message + "\n" + string(responseBody)))
	matchers := []struct {
		statusCode int
		keywords   []string
	}{
		{statusCode: http.StatusTooManyRequests, keywords: []string{"too many requests", "rate limit", "rate limited", "thrott", "resource_exhausted", "限流", "请求过于频繁"}},
		{statusCode: http.StatusUnauthorized, keywords: []string{"unauthorized", "unauthenticated", "invalid api key", "invalid token", "expired token", "未授权", "鉴权失败", "密钥无效"}},
		{statusCode: http.StatusForbidden, keywords: []string{"forbidden", "permission denied", "access denied", "无权限", "权限不足", "禁止访问"}},
		{statusCode: http.StatusNotFound, keywords: []string{"not found", "unknown model", "does not exist", "未找到", "不存在", "模型不存在"}},
		{statusCode: http.StatusBadRequest, keywords: []string{"bad request", "invalid json", "json parse", "invalid argument", "无效请求", "格式错误"}},
		{statusCode: http.StatusServiceUnavailable, keywords: []string{"service unavailable", "server is busy", "temporarily unavailable", "maintenance", "overloaded", "服务不可用", "系统繁忙", "维护中"}},
		{statusCode: http.StatusGatewayTimeout, keywords: []string{"gateway timeout", "timed out", "deadline exceeded", "网关超时", "上游超时"}},
	}
	for _, matcher := range matchers {
		for _, keyword := range matcher.keywords {
			if strings.Contains(lowerText, keyword) {
				return matcher.statusCode
			}
		}
	}
	return http.StatusBadGateway
}

func extractErrorMessage(payload map[string]any) string {
	if payload == nil {
		return ""
	}
	if errorValue, ok := payload["error"]; ok {
		switch value := errorValue.(type) {
		case string:
			return strings.TrimSpace(value)
		case map[string]any:
			if message, ok := value["message"].(string); ok {
				return strings.TrimSpace(message)
			}
			if code, ok := value["code"].(string); ok && strings.TrimSpace(code) != "" {
				return strings.TrimSpace(code)
			}
		}
	}
	if message, ok := payload["message"].(string); ok {
		return strings.TrimSpace(message)
	}
	return ""
}

func populateAnthropicUsageUpdate(update *repository.MessageRequestTerminalUpdate, payload map[string]any) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return
	}
	if inputTokens, ok := lookupInt(usage, "input_tokens"); ok {
		update.InputTokens = &inputTokens
	}
	if outputTokens, ok := lookupInt(usage, "output_tokens"); ok {
		update.OutputTokens = &outputTokens
	}
	if cacheCreationInputTokens, ok := lookupInt(usage, "cache_creation_input_tokens"); ok {
		update.CacheCreationInputTokens = &cacheCreationInputTokens
	}
	if cacheReadInputTokens, ok := lookupInt(usage, "cache_read_input_tokens"); ok {
		update.CacheReadInputTokens = &cacheReadInputTokens
	}
	if cacheCreationDetails, ok := usage["cache_creation"].(map[string]any); ok {
		if cacheCreation5mInputTokens, ok := lookupInt(cacheCreationDetails, "ephemeral_5m_input_tokens"); ok {
			update.CacheCreation5mInputTokens = &cacheCreation5mInputTokens
		}
		if cacheCreation1hInputTokens, ok := lookupInt(cacheCreationDetails, "ephemeral_1h_input_tokens"); ok {
			update.CacheCreation1hInputTokens = &cacheCreation1hInputTokens
		}
	}
}

func populateResponsesUsageUpdate(update *repository.MessageRequestTerminalUpdate, payload map[string]any) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return
	}
	if inputTokens, ok := lookupInt(usage, "input_tokens"); ok {
		update.InputTokens = &inputTokens
	}
	if outputTokens, ok := lookupInt(usage, "output_tokens"); ok {
		update.OutputTokens = &outputTokens
	}
	if inputTokensDetails, ok := usage["input_tokens_details"].(map[string]any); ok {
		if cacheReadInputTokens, ok := lookupInt(inputTokensDetails, "cached_tokens"); ok {
			update.CacheReadInputTokens = &cacheReadInputTokens
		}
	}
}

func populateChatCompletionsUsageUpdate(update *repository.MessageRequestTerminalUpdate, payload map[string]any) {
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		return
	}
	if inputTokens, ok := lookupInt(usage, "prompt_tokens"); ok {
		update.InputTokens = &inputTokens
	}
	if outputTokens, ok := lookupInt(usage, "completion_tokens"); ok {
		update.OutputTokens = &outputTokens
	}
	if promptTokensDetails, ok := usage["prompt_tokens_details"].(map[string]any); ok {
		if cacheReadInputTokens, ok := lookupInt(promptTokensDetails, "cached_tokens"); ok {
			update.CacheReadInputTokens = &cacheReadInputTokens
		}
	}
}

func lookupInt(payload map[string]any, key string) (int, bool) {
	if payload == nil {
		return 0, false
	}
	value, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch number := value.(type) {
	case float64:
		return int(number), true
	case int:
		return number, true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	default:
		return 0, false
	}
}

func (h *Handler) finalizeMessageRequest(c *gin.Context, id int, update repository.MessageRequestTerminalUpdate) {
	if h == nil || h.requestLogs == nil || id <= 0 {
		return
	}
	reqCtx := context.Background()
	if c != nil && c.Request != nil {
		reqCtx = c.Request.Context()
	}
	_ = h.requestLogs.UpdateTerminal(reqCtx, id, update)
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

func int64Pointer(value int64) *int64 {
	return &value
}
