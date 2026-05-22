package notification

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// Template 通知模板
type Template struct {
	Name    string
	Subject string
	Body    string
	tmpl    *template.Template
}

// TemplateData 模板数据
type TemplateData struct {
	// 用户信息
	UserID       int    `json:"user_id"`
	UserName     string `json:"user_name"`
	UserEmail    string `json:"user_email"`
	APIKeyID     int    `json:"api_key_id"`
	APIKeyName   string `json:"api_key_name"`
	APIKeyString string `json:"api_key_string"`

	// 请求信息
	RequestID     string `json:"request_id"`
	Model         string `json:"model"`
	Provider      string `json:"provider"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
	TotalTokens   int    `json:"total_tokens"`
	Cost          string `json:"cost"`
	Duration      int64  `json:"duration_ms"`
	StatusCode    int    `json:"status_code"`
	ErrorMessage  string `json:"error_message,omitempty"`
	StopReason    string `json:"stop_reason,omitempty"`

	// 限流信息
	RateLimitType   string `json:"rate_limit_type,omitempty"`
	CurrentUsage    int    `json:"current_usage,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	ResetTime       string `json:"reset_time,omitempty"`

	// 自定义字段
	Custom map[string]interface{} `json:"custom,omitempty"`
}

// TemplateRenderer 模板渲染器
type TemplateRenderer struct {
	templates map[string]*Template
}

// NewTemplateRenderer 创建模板渲染器
func NewTemplateRenderer() *TemplateRenderer {
	renderer := &TemplateRenderer{
		templates: make(map[string]*Template),
	}

	// 注册默认模板
	renderer.registerDefaultTemplates()

	return renderer
}

// registerDefaultTemplates 注册默认模板
func (r *TemplateRenderer) registerDefaultTemplates() {
	// 请求成功模板
	r.RegisterTemplate(&Template{
		Name:    "request_success",
		Subject: "API Request Successful",
		Body: `API Request Completed Successfully

User: {{.UserName}} (ID: {{.UserID}})
API Key: {{.APIKeyName}}
Request ID: {{.RequestID}}

Model: {{.Model}}
Provider: {{.Provider}}

Tokens:
- Input: {{.InputTokens}}
- Output: {{.OutputTokens}}
- Total: {{.TotalTokens}}

Cost: {{.Cost}}
Duration: {{.Duration}}ms
Status: {{.StatusCode}}
{{if .StopReason}}Stop Reason: {{.StopReason}}{{end}}
`,
	})

	// 请求失败模板
	r.RegisterTemplate(&Template{
		Name:    "request_error",
		Subject: "API Request Failed",
		Body: `API Request Failed

User: {{.UserName}} (ID: {{.UserID}})
API Key: {{.APIKeyName}}
Request ID: {{.RequestID}}

Model: {{.Model}}
Provider: {{.Provider}}

Error: {{.ErrorMessage}}
Status: {{.StatusCode}}
Duration: {{.Duration}}ms
`,
	})

	// 限流触发模板
	r.RegisterTemplate(&Template{
		Name:    "rate_limit_exceeded",
		Subject: "Rate Limit Exceeded",
		Body: `Rate Limit Exceeded

User: {{.UserName}} (ID: {{.UserID}})
API Key: {{.APIKeyName}}

Limit Type: {{.RateLimitType}}
Current Usage: {{.CurrentUsage}}
Limit: {{.Limit}}
{{if .ResetTime}}Reset Time: {{.ResetTime}}{{end}}

Please reduce your request rate or upgrade your plan.
`,
	})

	// 成本预警模板
	r.RegisterTemplate(&Template{
		Name:    "cost_warning",
		Subject: "Cost Warning",
		Body: `Cost Warning

User: {{.UserName}} (ID: {{.UserID}})
API Key: {{.APIKeyName}}

Current Cost: {{.Cost}}
Limit: {{.Custom.cost_limit}}

You have used {{.Custom.usage_percentage}}% of your cost limit.
Please monitor your usage or upgrade your plan.
`,
	})

	// Webhook 通用模板
	r.RegisterTemplate(&Template{
		Name:    "webhook_generic",
		Subject: "Webhook Notification",
		Body: `{
  "event": "{{.Custom.event_type}}",
  "user_id": {{.UserID}},
  "user_name": "{{.UserName}}",
  "api_key_id": {{.APIKeyID}},
  "api_key_name": "{{.APIKeyName}}",
  "request_id": "{{.RequestID}}",
  "model": "{{.Model}}",
  "provider": "{{.Provider}}",
  "tokens": {
    "input": {{.InputTokens}},
    "output": {{.OutputTokens}},
    "total": {{.TotalTokens}}
  },
  "cost": "{{.Cost}}",
  "duration_ms": {{.Duration}},
  "status_code": {{.StatusCode}}
  {{if .ErrorMessage}},"error": "{{.ErrorMessage}}"{{end}}
  {{if .StopReason}},"stop_reason": "{{.StopReason}}"{{end}}
}`,
	})
}

// RegisterTemplate 注册模板
func (r *TemplateRenderer) RegisterTemplate(tmpl *Template) error {
	// 编译模板
	t, err := template.New(tmpl.Name).Parse(tmpl.Body)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	tmpl.tmpl = t
	r.templates[tmpl.Name] = tmpl

	return nil
}

// Render 渲染模板
func (r *TemplateRenderer) Render(templateName string, data *TemplateData) (string, error) {
	tmpl, ok := r.templates[templateName]
	if !ok {
		return "", fmt.Errorf("template not found: %s", templateName)
	}

	var buf bytes.Buffer
	if err := tmpl.tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}

	return buf.String(), nil
}

// RenderToMap 渲染模板为 map（用于 JSON）
func (r *TemplateRenderer) RenderToMap(templateName string, data *TemplateData) (map[string]interface{}, error) {
	rendered, err := r.Render(templateName, data)
	if err != nil {
		return nil, err
	}

	// 如果渲染结果是 JSON，解析为 map
	if strings.HasPrefix(strings.TrimSpace(rendered), "{") {
		result := make(map[string]interface{})
		// 简单的键值对解析（实际应该使用 JSON 解析）
		result["body"] = rendered
		return result, nil
	}

	// 否则返回文本格式
	return map[string]interface{}{
		"subject": r.templates[templateName].Subject,
		"body":    rendered,
	}, nil
}

// GetTemplate 获取模板
func (r *TemplateRenderer) GetTemplate(name string) (*Template, bool) {
	tmpl, ok := r.templates[name]
	return tmpl, ok
}

// ListTemplates 列出所有模板
func (r *TemplateRenderer) ListTemplates() []string {
	names := make([]string, 0, len(r.templates))
	for name := range r.templates {
		names = append(names, name)
	}
	return names
}

// RemoveTemplate 移除模板
func (r *TemplateRenderer) RemoveTemplate(name string) {
	delete(r.templates, name)
}
