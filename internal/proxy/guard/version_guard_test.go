package guard

import (
	"context"
	"errors"
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

// mockSystemSettingsRepo implements SystemSettingsRepo for testing.
type mockSystemSettingsRepo struct {
	settings *model.SystemSettings
	err      error
}

func (m *mockSystemSettingsRepo) Get(ctx context.Context) (*model.SystemSettings, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.settings, nil
}

func TestVersionGuard_Name(t *testing.T) {
	guard := NewVersionGuard(&mockSystemSettingsRepo{}, "1.0.0")
	assert.Equal(t, "VersionGuard", guard.Name())
}

func TestVersionGuard_Check_EmptyMinVersion(t *testing.T) {
	guard := NewVersionGuard(&mockSystemSettingsRepo{}, "")
	req := &Request{ClientVersion: "0.1.0"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty min version should pass through")
}

func TestVersionGuard_Check_Disabled(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{EnableClientVersionCheck: false},
	}
	guard := NewVersionGuard(repo, "2.0.0")
	req := &Request{ClientVersion: "1.0.0"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Disabled version check should pass through")
}

func TestVersionGuard_Check_SettingsError(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		err: errors.New("db error"),
	}
	guard := NewVersionGuard(repo, "2.0.0")
	req := &Request{ClientVersion: "1.0.0"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should fail-open on settings error")
}

func TestVersionGuard_Check_EmptyClientVersion(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{EnableClientVersionCheck: true},
	}
	guard := NewVersionGuard(repo, "2.0.0")
	req := &Request{ClientVersion: ""}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Empty client version should fail-open")
}

func TestVersionGuard_Check_VersionOK(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{EnableClientVersionCheck: true},
	}
	guard := NewVersionGuard(repo, "1.0.0")
	req := &Request{ClientVersion: "1.5.0"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestVersionGuard_Check_VersionEqual(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{EnableClientVersionCheck: true},
	}
	guard := NewVersionGuard(repo, "1.5.0")
	req := &Request{ClientVersion: "1.5.0"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err)
}

func TestVersionGuard_Check_VersionTooOld(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{EnableClientVersionCheck: true},
	}
	guard := NewVersionGuard(repo, "2.0.0")
	req := &Request{ClientVersion: "1.9.9"}
	err := guard.Check(context.Background(), req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "Client upgrade required")
}

func TestVersionGuard_Check_InvalidVersionFormat(t *testing.T) {
	repo := &mockSystemSettingsRepo{
		settings: &model.SystemSettings{EnableClientVersionCheck: true},
	}
	guard := NewVersionGuard(repo, "2.0.0")
	req := &Request{ClientVersion: "not-a-version"}
	err := guard.Check(context.Background(), req)
	assert.NoError(t, err, "Should fail-open on parse error")
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"2.0.0", "1.0.0", 1},
		{"1.0.0", "2.0.0", -1},
		{"1.2.0", "1.1.0", 1},
		{"1.0.1", "1.0.0", 1},
		{"1.0", "1.0.0", 0},
		{"1.0.0", "1.0", 0},
		{"10.0.0", "9.0.0", 1},
	}

	for _, tt := range tests {
		result, err := compareVersions(tt.a, tt.b)
		assert.NoError(t, err, "comparing %s vs %s", tt.a, tt.b)
		assert.Equal(t, tt.want, result, "comparing %s vs %s", tt.a, tt.b)
	}
}

func TestCompareVersions_Error(t *testing.T) {
	_, err := compareVersions("abc", "1.0.0")
	assert.Error(t, err)
}
