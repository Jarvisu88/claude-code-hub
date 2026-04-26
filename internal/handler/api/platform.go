package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type PlatformHandler struct {
	dbPing    func(ctx context.Context) error
	redisPing func(ctx context.Context) error
	version   string
}

var versionHTTPClient = &http.Client{Timeout: 5 * time.Second}

func NewPlatformHandler(dbPing func(ctx context.Context) error, redisPing func(ctx context.Context) error, version string) *PlatformHandler {
	if version == "" {
		version = "0.1.0"
	}
	return &PlatformHandler{
		dbPing:    dbPing,
		redisPing: redisPing,
		version:   version,
	}
}

func (h *PlatformHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/health", h.health)
	router.GET("/api/health/live", h.live)
	router.GET("/api/health/ready", h.ready)
	router.GET("/api/version", h.versionInfo)
}

func (h *PlatformHandler) health(c *gin.Context) {
	ctx := c.Request.Context()
	if h.dbPing != nil {
		if err := h.dbPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "database": "disconnected", "error": err.Error()})
			return
		}
	}

	redisStatus := "disabled"
	if h.redisPing != nil {
		if err := h.redisPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unhealthy", "database": "connected", "redis": "disconnected", "error": err.Error()})
			return
		}
		redisStatus = "connected"
	}

	c.JSON(http.StatusOK, gin.H{"status": "healthy", "database": "connected", "redis": redisStatus})
}

func (h *PlatformHandler) live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func (h *PlatformHandler) ready(c *gin.Context) {
	ctx := c.Request.Context()
	if h.dbPing != nil {
		if err := h.dbPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "database": "disconnected", "error": err.Error()})
			return
		}
	}
	if h.redisPing != nil {
		if err := h.redisPing(ctx); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "redis": "disconnected", "error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *PlatformHandler) versionInfo(c *gin.Context) {
	version := resolveCurrentVersion(h.version)
	info, err := fetchLatestVersionInfo()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"current":   version,
			"latest":    nil,
			"hasUpdate": false,
			"error":     "无法获取最新版本信息",
		})
		return
	}
	if info.Latest == "" {
		c.JSON(http.StatusOK, gin.H{
			"current":   version,
			"latest":    nil,
			"hasUpdate": false,
			"message":   "暂无发布版本",
		})
		return
	}

	releaseURL := any(nil)
	publishedAt := any(nil)
	if info.ReleaseURL != "" {
		releaseURL = info.ReleaseURL
	}
	if info.PublishedAt != "" {
		publishedAt = info.PublishedAt
	}
	hasUpdate := compareSemverLike(version, info.Latest) < 0
	c.JSON(http.StatusOK, gin.H{
		"name":        "claude-code-hub-go-rewrite",
		"version":     version,
		"current":     version,
		"latest":      info.Latest,
		"hasUpdate":   hasUpdate,
		"releaseUrl":  releaseURL,
		"publishedAt": publishedAt,
	})
}

func resolveCurrentVersion(fallback string) string {
	if value := strings.TrimSpace(os.Getenv("NEXT_PUBLIC_APP_VERSION")); value != "" {
		return value
	}
	if value := strings.TrimSpace(os.Getenv("APP_VERSION")); value != "" {
		return value
	}
	if content, err := os.ReadFile("VERSION"); err == nil {
		if value := strings.TrimSpace(string(content)); value != "" {
			return value
		}
	}
	return fallback
}

type latestVersionInfo struct {
	Latest      string
	ReleaseURL  string
	PublishedAt string
}

func fetchLatestVersionInfo() (latestVersionInfo, error) {
	url := strings.TrimSpace(os.Getenv("VERSION_RELEASE_API_URL"))
	if url == "" {
		url = "https://api.github.com/repos/ding113/claude-code-hub/releases/latest"
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return latestVersionInfo{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "claude-code-hub-go-rewrite")
	resp, err := versionHTTPClient.Do(req)
	if err != nil {
		return latestVersionInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return latestVersionInfo{}, appVersionHTTPError(resp.StatusCode)
	}
	var payload struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return latestVersionInfo{}, err
	}
	return latestVersionInfo{
		Latest:      normalizeVersionForDisplay(payload.TagName),
		ReleaseURL:  strings.TrimSpace(payload.HTMLURL),
		PublishedAt: strings.TrimSpace(payload.PublishedAt),
	}, nil
}

type appVersionHTTPError int

func (e appVersionHTTPError) Error() string { return "version http status " + strconv.Itoa(int(e)) }

func normalizeVersionForDisplay(version string) string {
	trimmed := strings.TrimSpace(version)
	if trimmed == "" {
		return trimmed
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "v") {
		return "v" + trimmed[1:]
	}
	if startsWithDigit(trimmed) {
		return "v" + trimmed
	}
	return trimmed
}

func startsWithDigit(value string) bool {
	if value == "" {
		return false
	}
	_, err := strconv.Atoi(string(value[0]))
	return err == nil
}

func compareSemverLike(current, latest string) int {
	currentParts := parseVersionParts(current)
	latestParts := parseVersionParts(latest)
	if len(currentParts) == 0 || len(latestParts) == 0 {
		return 0
	}
	maxLen := len(currentParts)
	if len(latestParts) > maxLen {
		maxLen = len(latestParts)
	}
	for i := 0; i < maxLen; i++ {
		curr := 0
		if i < len(currentParts) {
			curr = currentParts[i]
		}
		lat := 0
		if i < len(latestParts) {
			lat = latestParts[i]
		}
		if current == latest {
			return 0
		}
		if lat > curr {
			return -1
		}
		if lat < curr {
			return 1
		}
	}
	return 0
}

func parseVersionParts(value string) []int {
	value = strings.TrimSpace(strings.TrimPrefix(strings.ToLower(value), "v"))
	if value == "" {
		return nil
	}
	core := strings.Split(strings.Split(value, "-")[0], ".")
	parts := make([]int, 0, len(core))
	for _, piece := range core {
		piece = strings.TrimSpace(piece)
		if piece == "" {
			return nil
		}
		n, err := strconv.Atoi(piece)
		if err != nil {
			return nil
		}
		parts = append(parts, n)
	}
	return parts
}
