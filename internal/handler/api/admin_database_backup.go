package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/gin-gonic/gin"
)

type databaseExportRunner interface {
	Export(ctx context.Context, mode string) (filename string, data []byte, err error)
}

type importProgressEvent struct {
	Type     string `json:"type"`
	Message  string `json:"message"`
	ExitCode int    `json:"exitCode,omitempty"`
}

type databaseImportRunner interface {
	Import(ctx context.Context, dumpPath string, cleanFirst bool) ([]importProgressEvent, error)
}

type AdminDatabaseExportHandler struct {
	auth   adminAuthenticator
	runner databaseExportRunner
}

type AdminDatabaseImportHandler struct {
	auth   adminAuthenticator
	runner databaseImportRunner
}

func NewAdminDatabaseExportHandler(auth adminAuthenticator, runner databaseExportRunner) *AdminDatabaseExportHandler {
	return &AdminDatabaseExportHandler{auth: auth, runner: runner}
}

func NewAdminDatabaseImportHandler(auth adminAuthenticator, runner databaseImportRunner) *AdminDatabaseImportHandler {
	return &AdminDatabaseImportHandler{auth: auth, runner: runner}
}

func (h *AdminDatabaseExportHandler) RegisterRoutes(router gin.IRouter) {
	router.GET("/api/admin/database/export", h.export)
}

func (h *AdminDatabaseImportHandler) RegisterRoutes(router gin.IRouter) {
	router.POST("/api/admin/database/import", h.importDump)
}

func ensureAdminToken(auth adminAuthenticator, c *gin.Context, serviceName string) bool {
	if auth == nil {
		writeAdminError(c, appErrors.NewInternalError(serviceName+"鉴权服务未初始化"))
		return false
	}
	authResult, err := auth.AuthenticateAdminToken(resolveAdminToken(c))
	if err != nil {
		writeAdminError(c, err)
		return false
	}
	if authResult == nil || !authResult.IsAdmin {
		writeAdminError(c, appErrors.NewPermissionDenied("权限不足", appErrors.CodePermissionDenied))
		return false
	}
	return true
}

func (h *AdminDatabaseExportHandler) export(c *gin.Context) {
	if h == nil || h.runner == nil {
		writeAdminError(c, appErrors.NewInternalError("数据库导出服务未初始化"))
		return
	}
	if !ensureAdminToken(h.auth, c, "数据库导出") {
		return
	}
	mode := strings.TrimSpace(c.DefaultQuery("mode", "full"))
	filename, data, err := h.runner.Export(c.Request.Context(), mode)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, "application/octet-stream", data)
}

func (h *AdminDatabaseImportHandler) importDump(c *gin.Context) {
	if h == nil || h.runner == nil {
		writeAdminError(c, appErrors.NewInternalError("数据库导入服务未初始化"))
		return
	}
	if !ensureAdminToken(h.auth, c, "数据库导入") {
		return
	}
	fileHeader, err := c.FormFile("file")
	if err != nil || fileHeader == nil {
		writeAdminError(c, appErrors.NewInvalidRequest("缺少备份文件"))
		return
	}
	if !strings.HasSuffix(strings.ToLower(fileHeader.Filename), ".dump") {
		writeAdminError(c, appErrors.NewInvalidRequest("文件格式错误，仅支持 .dump 格式的备份文件"))
		return
	}
	cleanFirst := c.PostForm("cleanFirst") == "true"

	tmpFile, err := os.CreateTemp("", "cch-import-*.dump")
	if err != nil {
		writeAdminError(c, appErrors.NewInternalError("创建临时文件失败"))
		return
	}
	tmpPath := tmpFile.Name()
	src, err := fileHeader.Open()
	if err != nil {
		_ = tmpFile.Close()
		writeAdminError(c, appErrors.NewInternalError("读取上传文件失败"))
		return
	}
	if _, err := io.Copy(tmpFile, src); err != nil {
		_ = src.Close()
		_ = tmpFile.Close()
		writeAdminError(c, appErrors.NewInternalError("保存上传文件失败"))
		return
	}
	_ = src.Close()
	_ = tmpFile.Close()
	defer os.Remove(tmpPath)

	events, err := h.runner.Import(c.Request.Context(), tmpPath, cleanFirst)
	if err != nil {
		writeAdminError(c, err)
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Status(http.StatusOK)
	for _, event := range events {
		payload, _ := json.Marshal(event)
		_, _ = c.Writer.WriteString("data: " + string(payload) + "\n\n")
		c.Writer.Flush()
	}
}

func backupFilename(mode string) string {
	suffix := ""
	switch mode {
	case "excludeLogs":
		suffix = "_no-logs"
	case "ledgerOnly":
		suffix = "_ledger-only"
	}
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05")
	return "backup_" + timestamp + suffix + ".dump"
}

func normalizeDumpPath(path string) string {
	return filepath.Clean(path)
}
