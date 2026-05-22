package guard

import (
	"context"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/model"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// SystemSettingsRepo provides access to system settings.
type SystemSettingsRepo interface {
	Get(ctx context.Context) (*model.SystemSettings, error)
}

// VersionGuard checks the client version against the minimum required version.
type VersionGuard struct {
	settingsRepo SystemSettingsRepo
	minVersion   string
}

// NewVersionGuard creates a new VersionGuard.
// minVersion is the minimum required client version (e.g. "1.0.0").
func NewVersionGuard(settingsRepo SystemSettingsRepo, minVersion string) *VersionGuard {
	return &VersionGuard{
		settingsRepo: settingsRepo,
		minVersion:   minVersion,
	}
}

// Name returns the guard name.
func (g *VersionGuard) Name() string {
	return "VersionGuard"
}

// Check validates the client version.
// Fail-open: on parse error or settings fetch error, pass through.
func (g *VersionGuard) Check(ctx context.Context, req *Request) error {
	// If no min version configured, pass through
	if g.minVersion == "" {
		return nil
	}

	// Check if version check is enabled in system settings
	settings, err := g.settingsRepo.Get(ctx)
	if err != nil {
		// Fail-open
		return nil
	}
	if !settings.EnableClientVersionCheck {
		return nil
	}

	// If no client version provided, fail-open
	clientVersion := req.ClientVersion
	if clientVersion == "" {
		return nil
	}

	// Compare versions
	cmp, err := compareVersions(clientVersion, g.minVersion)
	if err != nil {
		// Fail-open on parse error
		return nil
	}

	if cmp < 0 {
		return appErrors.NewInvalidRequest("Client upgrade required")
	}

	return nil
}

// compareVersions compares two semver-like version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareVersions(a, b string) (int, error) {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aNum, bNum int
		var err error

		if i < len(aParts) {
			aNum, err = strconv.Atoi(aParts[i])
			if err != nil {
				return 0, err
			}
		}
		if i < len(bParts) {
			bNum, err = strconv.Atoi(bParts[i])
			if err != nil {
				return 0, err
			}
		}

		if aNum < bNum {
			return -1, nil
		}
		if aNum > bNum {
			return 1, nil
		}
	}

	return 0, nil
}
