package api

import (
	"encoding/json"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type IPGeoHandler struct {
	auth     adminAuthenticator
	settings systemSettingsStore
	http     httpDoer
}

func NewIPGeoHandler(auth adminAuthenticator, settings systemSettingsStore, httpClient httpDoer) *IPGeoHandler {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: ipGeoTimeout()}
	}
	return &IPGeoHandler{auth: auth, settings: settings, http: httpClient}
}

func (h *IPGeoHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/ip-geo/:ip", h.lookup)
}

func (h *IPGeoHandler) lookup(c *gin.Context) {
	if h == nil || h.auth == nil || h.http == nil || h.settings == nil {
		writeAdminError(c, appErrors.NewInternalError("IP 地理位置服务未初始化"))
		return
	}
	authResult, err := h.auth.AuthenticateAdminToken(resolveAdminToken(c))
	if err != nil {
		writeAdminError(c, err)
		return
	}
	if authResult == nil || !authResult.IsAdmin {
		writeAdminError(c, appErrors.NewPermissionDenied("权限不足", appErrors.CodePermissionDenied))
		return
	}
	settings, err := h.settings.Get(c.Request.Context())
	if err != nil {
		writeAdminError(c, err)
		return
	}
	if settings != nil && !settings.IpGeoLookupEnabled {
		c.JSON(http.StatusNotFound, gin.H{"error": "ip geolocation disabled"})
		return
	}
	c.Header("Cache-Control", "private, max-age=60")

	ip := strings.TrimSpace(c.Param("ip"))
	if ip == "" {
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": "invalid ip"})
		return
	}
	if parsed := net.ParseIP(ip); parsed == nil {
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": "invalid ip"})
		return
	} else if isPrivateIP(parsed) {
		c.JSON(http.StatusOK, gin.H{"status": "private", "data": gin.H{"ip": ip, "kind": "private"}})
		return
	}

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("IP_GEO_API_URL")), "/")
	if baseURL == "" {
		baseURL = "https://ip-api.claude-code-hub.app"
	}
	upstreamURL := baseURL + "/v1/ip2location/" + url.PathEscape(ip)
	lang := strings.TrimSpace(c.Query("lang"))
	if lang == "" {
		lang = "en"
	}
	upstreamURL += "?lang=" + url.QueryEscape(lang)

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, upstreamURL, nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": "request build failed"})
		return
	}
	req.Header.Set("Accept", "application/json")
	if token := strings.TrimSpace(os.Getenv("IP_GEO_API_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := h.http.Do(req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": err.Error()})
		return
	}
	defer resp.Body.Close()

	var payload any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": "upstream invalid json"})
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		c.JSON(http.StatusOK, gin.H{"status": "error", "error": "upstream status " + resp.Status})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok", "data": payload})
}

func isPrivateIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()
}

func ipGeoTimeout() time.Duration {
	if raw := strings.TrimSpace(os.Getenv("IP_GEO_TIMEOUT_MS")); raw != "" {
		if millis, err := time.ParseDuration(raw + "ms"); err == nil && millis > 0 {
			return millis
		}
	}
	return 1500 * time.Millisecond
}
