package api

import (
	"context"
	"net/http"
	"strings"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/gin-gonic/gin"
)

const authCookieName = "auth-token"
const authCookieMaxAge = 60 * 60 * 24 * 7

type loginAuthenticator interface {
	AuthenticateAdminToken(token string) (*authsvc.AuthResult, error)
	AuthenticateProxy(ctx context.Context, input authsvc.ProxyAuthInput) (*authsvc.AuthResult, error)
}

type AuthHandler struct {
	auth loginAuthenticator
}

func NewAuthHandler(auth loginAuthenticator) *AuthHandler {
	return &AuthHandler{auth: auth}
}

func (h *AuthHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/api/auth/login", h.login)
	router.POST("/api/auth/logout", h.logout)
}

func (h *AuthHandler) login(c *gin.Context) {
	if h == nil || h.auth == nil {
		writeAdminError(c, appErrors.NewInternalError("登录服务未初始化"))
		return
	}
	var input struct {
		Key string `json:"key"`
	}
	if err := bindOptionalJSON(c, &input); err != nil {
		writeAdminError(c, err)
		return
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		writeAdminError(c, appErrors.NewInvalidRequest("API key is required"))
		return
	}

	authResult, err := h.auth.AuthenticateAdminToken(key)
	if err != nil || authResult == nil {
		authResult, err = h.auth.AuthenticateProxy(c.Request.Context(), authsvc.ProxyAuthInput{APIKeyHeader: key})
		if err != nil {
			writeAdminError(c, appErrors.NewAuthenticationError("Authentication failed", appErrors.CodeInvalidAPIKey))
			return
		}
	}

	loginType := "readonly_user"
	redirectTo := "/my-usage"
	if authResult != nil && authResult.IsAdmin {
		loginType = "admin"
		redirectTo = "/dashboard"
	} else if authResult != nil && authResult.Key != nil && authResult.Key.CanLoginWebUi != nil && *authResult.Key.CanLoginWebUi {
		loginType = "dashboard_user"
		redirectTo = "/dashboard"
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, key, authCookieMaxAge, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{
		"ok":         true,
		"redirectTo": redirectTo,
		"loginType":  loginType,
	})
}

func (h *AuthHandler) logout(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
