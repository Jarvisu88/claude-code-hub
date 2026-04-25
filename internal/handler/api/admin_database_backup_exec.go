package api

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/config"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

type DBBackupExecRunner struct {
	cfg config.DatabaseConfig
}

func NewDBBackupExecRunner(cfg config.DatabaseConfig) *DBBackupExecRunner {
	return &DBBackupExecRunner{cfg: cfg}
}

func (r *DBBackupExecRunner) Export(ctx context.Context, mode string) (string, []byte, error) {
	args := []string{"--format=custom", "--no-owner", "--no-privileges"}
	switch mode {
	case "", "full":
	case "excludeLogs":
		args = append(args, "--exclude-table=message_request")
	case "ledgerOnly":
		args = append(args, "--table=message_request", "--data-only")
	default:
		return "", nil, appErrors.NewInvalidRequest("不支持的导出模式")
	}
	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	cmd.Env = append(os.Environ(), buildPostgresEnv(r.cfg)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", nil, appErrors.NewInternalError("导出数据库失败: " + strings.TrimSpace(stderr.String()))
	}
	return backupFilename(mode), stdout.Bytes(), nil
}

func (r *DBBackupExecRunner) Import(ctx context.Context, dumpPath string, cleanFirst bool) ([]importProgressEvent, error) {
	args := []string{"--no-owner", "--no-privileges"}
	if cleanFirst {
		args = append(args, "--clean", "--if-exists")
	}
	args = append(args, normalizeDumpPath(dumpPath))
	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	cmd.Env = append(os.Environ(), buildPostgresEnv(r.cfg)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	events := []importProgressEvent{{Type: "progress", Message: "Starting database import..."}}
	if err := cmd.Run(); err != nil {
		events = append(events, importProgressEvent{Type: "error", Message: strings.TrimSpace(stderr.String())})
		return events, nil
	}
	events = append(events, importProgressEvent{Type: "complete", Message: "Database import completed"})
	return events, nil
}

func buildPostgresEnv(cfg config.DatabaseConfig) []string {
	if strings.TrimSpace(cfg.DSN) != "" {
		return []string{"DATABASE_URL=" + cfg.DSN, "PGDATABASE="}
	}
	env := []string{
		"PGHOST=" + cfg.Host,
		"PGPORT=" + strconv.Itoa(cfg.Port),
		"PGUSER=" + cfg.User,
		"PGPASSWORD=" + cfg.Password,
		"PGDATABASE=" + cfg.DBName,
	}
	if strings.TrimSpace(cfg.SSLMode) != "" {
		env = append(env, fmt.Sprintf("PGSSLMODE=%s", cfg.SSLMode))
	}
	return env
}
