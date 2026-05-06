package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
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

type authAuditStore interface {
	CreateAsync(ctx context.Context, entry *model.AuditLog) error
}

type AuthHandler struct {
	auth     loginAuthenticator
	sessions authSessionRevoker
	audit    authAuditStore
}

func NewAuthHandler(auth loginAuthenticator, options ...any) *AuthHandler {
	var sessionRevoker authSessionRevoker
	var auditStore authAuditStore
	for _, option := range options {
		switch typed := option.(type) {
		case authSessionRevoker:
			if sessionRevoker == nil {
				sessionRevoker = typed
			}
		case authAuditStore:
			if auditStore == nil {
				auditStore = typed
			}
		}
	}
	return &AuthHandler{auth: auth, sessions: sessionRevoker, audit: auditStore}
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
	applyAuthResponseHeaders(c)
	var input struct {
		Key string `json:"key"`
	}
	if err := bindOptionalJSON(c, &input); err != nil {
		writeAdminError(c, err)
		return
	}
	key := strings.TrimSpace(input.Key)
	if key == "" {
		h.writeAudit(c, &model.AuditLog{
			ActionCategory: "auth",
			ActionType:     "login.failed",
			Success:        false,
			ErrorMessage:   stringPointer("KEY_REQUIRED"),
		})
		c.JSON(http.StatusBadRequest, gin.H{
			"ok":        false,
			"error":     "API key is required",
			"errorCode": "KEY_REQUIRED",
		})
		return
	}

	authResult, err := h.auth.AuthenticateAdminToken(key)
	if err != nil || authResult == nil {
		authResult, err = h.auth.AuthenticateProxy(c.Request.Context(), authsvc.ProxyAuthInput{APIKeyHeader: key})
		if err != nil {
			if isLoginInvalidCredentialError(err) {
				h.writeAudit(c, &model.AuditLog{
					ActionCategory: "auth",
					ActionType:     "login.failed",
					Success:        false,
					ErrorMessage:   stringPointer("KEY_INVALID"),
				})
				c.JSON(http.StatusUnauthorized, gin.H{
					"ok":        false,
					"error":     "Authentication failed",
					"errorCode": "KEY_INVALID",
				})
				return
			}
			h.writeAudit(c, &model.AuditLog{
				ActionCategory: "auth",
				ActionType:     "login.failed",
				Success:        false,
				ErrorMessage:   stringPointer("SERVER_ERROR"),
			})
			c.JSON(http.StatusInternalServerError, gin.H{
				"ok":        false,
				"error":     "Internal server error",
				"errorCode": "SERVER_ERROR",
			})
			return
		}
		if authResult == nil {
			h.writeAudit(c, &model.AuditLog{
				ActionCategory: "auth",
				ActionType:     "login.failed",
				Success:        false,
				ErrorMessage:   stringPointer("KEY_INVALID"),
			})
			c.JSON(http.StatusUnauthorized, gin.H{
				"ok":        false,
				"error":     "Authentication failed",
				"errorCode": "KEY_INVALID",
			})
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

	cookieValue := key
	sessionMode := resolveSessionTokenModeFromEnv()
	if sessionMode != sessionTokenModeLegacy && authResult != nil && authResult.User != nil {
		if h == nil || h.sessions == nil {
			if sessionMode == sessionTokenModeOpaque {
				h.writeAudit(c, &model.AuditLog{
					ActionCategory: "auth",
					ActionType:     "login.failed",
					Success:        false,
					ErrorMessage:   stringPointer("SESSION_CREATE_FAILED"),
				})
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"ok":        false,
					"error":     "Internal server error",
					"errorCode": "SESSION_CREATE_FAILED",
				})
				return
			}
		} else {
			sessionID, err := h.sessions.Create(c.Request.Context(), key, authResult.User.ID, authResult.User.Role)
			if err != nil || (sessionMode == sessionTokenModeOpaque && sessionID == "") {
				if sessionMode == sessionTokenModeOpaque {
					h.writeAudit(c, &model.AuditLog{
						ActionCategory: "auth",
						ActionType:     "login.failed",
						Success:        false,
						ErrorMessage:   stringPointer("SESSION_CREATE_FAILED"),
					})
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"ok":        false,
						"error":     "Internal server error",
						"errorCode": "SESSION_CREATE_FAILED",
					})
					return
				}
			} else if sessionMode == sessionTokenModeOpaque {
				cookieValue = sessionID
			}
		}
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, cookieValue, authCookieMaxAge, "/", "", authSecureCookiesEnabled(), true)
	response := gin.H{
		"ok":         true,
		"redirectTo": redirectTo,
		"loginType":  loginType,
	}
	if authResult != nil && authResult.User != nil {
		response["user"] = gin.H{
			"id":          authResult.User.ID,
			"name":        authResult.User.Name,
			"description": authResult.User.Description,
			"role":        authResult.User.Role,
		}
	}
	h.writeAudit(c, buildLoginAuditLog(authResult))
	c.JSON(http.StatusOK, response)
}

func isLoginInvalidCredentialError(err error) bool {
	return appErrors.IsCode(err, appErrors.CodeInvalidAPIKey) ||
		appErrors.IsCode(err, appErrors.CodeExpiredAPIKey) ||
		appErrors.IsCode(err, appErrors.CodeDisabledAPIKey) ||
		appErrors.IsCode(err, appErrors.CodeDisabledUser) ||
		appErrors.IsCode(err, appErrors.CodeUserExpired) ||
		appErrors.IsCode(err, appErrors.CodeInvalidCredentials)
}

func (h *AuthHandler) logout(c *gin.Context) {
	applyAuthResponseHeaders(c)
	if resolveSessionTokenModeFromEnv() != sessionTokenModeLegacy && h != nil && h.sessions != nil {
		if sessionID, err := c.Cookie(authCookieName); err == nil {
			_ = h.sessions.Revoke(c.Request.Context(), sessionID)
		}
	}
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(authCookieName, "", -1, "/", "", authSecureCookiesEnabled(), true)
	h.writeAudit(c, &model.AuditLog{
		ActionCategory: "auth",
		ActionType:     "logout.success",
		Success:        true,
	})
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func buildLoginAuditLog(authResult *authsvc.AuthResult) *model.AuditLog {
	entry := &model.AuditLog{
		ActionCategory: "auth",
		ActionType:     "login.success",
		Success:        true,
	}
	if authResult != nil {
		if authResult.User != nil {
			entry.OperatorUserID = &authResult.User.ID
			entry.OperatorUserName = stringPointer(authResult.User.Name)
			entry.TargetType = stringPointer("user")
			entry.TargetID = stringPointer(strconv.Itoa(authResult.User.ID))
			entry.TargetName = stringPointer(authResult.User.Name)
		}
		if authResult.Key != nil {
			entry.OperatorKeyID = &authResult.Key.ID
			entry.OperatorKeyName = stringPointer(authResult.Key.Name)
		}
	}
	return entry
}

func (h *AuthHandler) writeAudit(c *gin.Context, entry *model.AuditLog) {
	if h == nil || h.audit == nil || entry == nil {
		return
	}
	if c != nil {
		if ip := strings.TrimSpace(c.ClientIP()); ip != "" {
			entry.OperatorIP = stringPointer(ip)
		}
		if ua := strings.TrimSpace(c.Request.UserAgent()); ua != "" {
			entry.UserAgent = stringPointer(ua)
		}
	}
	_ = h.audit.CreateAsync(c.Request.Context(), entry)
}
