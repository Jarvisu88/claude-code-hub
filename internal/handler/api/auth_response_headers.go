package api

import (
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const authCSPReportOnlyValue = "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'; font-src 'self' data:; frame-ancestors 'none'"

func applyAuthResponseHeaders(c *gin.Context) {
	if c == nil {
		return
	}
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
	c.Header("Pragma", "no-cache")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("X-Frame-Options", "DENY")
	c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
	c.Header("X-DNS-Prefetch-Control", "off")
	c.Header("Content-Security-Policy-Report-Only", authCSPReportOnlyValue)

	if authSecureCookiesEnabled() {
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
	}
}

func authSecureCookiesEnabled() bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv("ENABLE_SECURE_COOKIES")))
	switch raw {
	case "", "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
