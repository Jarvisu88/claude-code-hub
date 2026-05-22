package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type SmokeDebugHandler struct{}

func NewSmokeDebugHandler() *SmokeDebugHandler {
	return &SmokeDebugHandler{}
}

func (h *SmokeDebugHandler) RegisterRoutes(router gin.IRouter) {
	if router == nil {
		return
	}
	router.GET("/__debug__/smoke", h.page)
}

func (h *SmokeDebugHandler) page(c *gin.Context) {
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, smokeDebugHTML)
}

const smokeDebugHTML = `<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>CCH Go Rewrite Smoke Debug</title>
  <style>
    :root { color-scheme: dark; }
    body { font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; margin: 0; background: #0b1020; color: #e5e7eb; }
    .wrap { max-width: 1200px; margin: 0 auto; padding: 24px; }
    h1 { margin: 0 0 8px; font-size: 28px; }
    p { color: #9ca3af; }
    .grid { display: grid; grid-template-columns: 360px 1fr; gap: 20px; }
    .card { background: #111827; border: 1px solid #1f2937; border-radius: 14px; padding: 16px; box-shadow: 0 8px 30px rgba(0,0,0,.25); }
    .card h2 { margin: 0 0 12px; font-size: 18px; }
    label { display: block; font-size: 12px; color: #93c5fd; margin: 10px 0 6px; text-transform: uppercase; letter-spacing: .06em; }
    input, textarea, select, button { width: 100%; box-sizing: border-box; border-radius: 10px; border: 1px solid #374151; background: #0f172a; color: #f9fafb; padding: 10px 12px; font: inherit; }
    textarea { min-height: 140px; resize: vertical; }
    button { cursor: pointer; background: linear-gradient(135deg,#2563eb,#1d4ed8); border: none; font-weight: 600; }
    button.secondary { background: #1f2937; border: 1px solid #374151; }
    .row { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
    .actions { display: grid; grid-template-columns: repeat(2, 1fr); gap: 10px; margin-top: 14px; }
    .status { margin-top: 12px; padding: 10px 12px; border-radius: 10px; background: #0f172a; border: 1px solid #1f2937; color: #cbd5e1; min-height: 22px; }
    .results { display: grid; gap: 14px; }
    pre { margin: 0; white-space: pre-wrap; word-break: break-word; background: #020617; border: 1px solid #1e293b; border-radius: 12px; padding: 12px; min-height: 160px; overflow: auto; }
    .muted { color: #94a3b8; font-size: 12px; }
    .quick-list button { text-align: left; }
    @media (max-width: 960px) { .grid { grid-template-columns: 1fr; } .actions { grid-template-columns: 1fr; } }
  </style>
</head>
<body>
  <div class="wrap">
    <h1>Smoke Debug Console</h1>
    <p>用于本地联调 Claude Code Hub Go Rewrite。可直接测试登录、health、admin actions、system settings、/v1/responses。</p>
    <div class="grid">
      <section class="card">
        <h2>请求设置</h2>
        <label for="baseUrl">Base URL</label>
        <input id="baseUrl" value="" placeholder="默认当前站点" />

        <label for="authKey">Admin Token / API Key</label>
        <input id="authKey" placeholder="例如 dev-admin-token 或 proxy-key" />

        <label for="requestPath">自定义路径</label>
        <input id="requestPath" value="/v1/responses" />

        <div class="row">
          <div>
            <label for="method">Method</label>
            <select id="method">
              <option>GET</option>
              <option selected>POST</option>
              <option>PUT</option>
              <option>DELETE</option>
            </select>
          </div>
          <div>
            <label for="contentType">Content-Type</label>
            <select id="contentType">
              <option selected>application/json</option>
              <option>text/plain</option>
            </select>
          </div>
        </div>

        <label for="requestBody">Request Body</label>
        <textarea id="requestBody">{
  "input": [
    {
      "role": "user",
      "content": "hello from smoke debug"
    }
  ],
  "model": "gpt-5.4"
}</textarea>

        <div class="actions">
          <button id="loginBtn">测试登录</button>
          <button id="healthBtn" class="secondary">/api/health</button>
          <button id="readyBtn" class="secondary">/api/health/ready</button>
          <button id="usersBtn" class="secondary">/api/actions/users</button>
          <button id="settingsBtn" class="secondary">/api/system-settings</button>
          <button id="responsesBtn">/v1/responses</button>
        </div>

        <label for="quickChecks" style="margin-top:16px;">更多快捷测试</label>
        <div class="quick-list actions" id="quickChecks">
          <button class="secondary" data-path="/api/health/live" data-method="GET">/api/health/live</button>
          <button class="secondary" data-path="/api/actions/openapi.json" data-method="GET">/api/actions/openapi.json</button>
          <button class="secondary" data-path="/v1/models" data-method="GET">/v1/models</button>
          <button class="secondary" data-path="/v1/chat/completions/models" data-method="GET">/v1/chat/completions/models</button>
        </div>

        <div class="status" id="status">等待执行…</div>
      </section>

      <section class="results">
        <div class="card">
          <h2>响应摘要</h2>
          <pre id="summary"></pre>
        </div>
        <div class="card">
          <h2>响应头</h2>
          <pre id="headers"></pre>
        </div>
        <div class="card">
          <h2>响应体</h2>
          <pre id="body"></pre>
        </div>
      </section>
    </div>
    <p class="muted">提示：同源下会自动带 cookie；如果要测 admin/actions，直接在上方填 admin token 即可。</p>
  </div>

  <script>
    const $ = (id) => document.getElementById(id);
    const baseUrlInput = $("baseUrl");
    const authKeyInput = $("authKey");
    const requestPathInput = $("requestPath");
    const methodInput = $("method");
    const contentTypeInput = $("contentType");
    const requestBodyInput = $("requestBody");
    const statusEl = $("status");
    const summaryEl = $("summary");
    const headersEl = $("headers");
    const bodyEl = $("body");

    baseUrlInput.value = window.location.origin;

    function getBaseUrl() {
      const value = baseUrlInput.value.trim();
      return value || window.location.origin;
    }

    function buildHeaders(contentType) {
      const headers = {};
      if (contentType) headers["Content-Type"] = contentType;
      const authKey = authKeyInput.value.trim();
      if (authKey) {
        headers["Authorization"] = "Bearer " + authKey;
        headers["x-api-key"] = authKey;
      }
      return headers;
    }

    async function runRequest({ path, method = "GET", body = null, contentType = "application/json", useLogin = false }) {
      const url = getBaseUrl().replace(/\/$/, "") + path;
      const headers = useLogin ? { "Content-Type": "application/json" } : buildHeaders(contentType);
      const startedAt = performance.now();
      statusEl.textContent = "请求中: " + method + " " + path;

      const response = await fetch(url, {
        method,
        headers,
        body,
        credentials: "include"
      });

      const elapsed = Math.round(performance.now() - startedAt);
      const responseText = await response.text();
      const responseHeaders = {};
      response.headers.forEach((value, key) => { responseHeaders[key] = value; });

      summaryEl.textContent = JSON.stringify({
        url,
        status: response.status,
        ok: response.ok,
        elapsedMs: elapsed
      }, null, 2);
      headersEl.textContent = JSON.stringify(responseHeaders, null, 2);
      bodyEl.textContent = responseText || "(empty)";
      statusEl.textContent = "完成: " + response.status + " " + method + " " + path + " (" + elapsed + "ms)";
      return { response, responseText, responseHeaders };
    }

    $("loginBtn").addEventListener("click", async () => {
      const key = authKeyInput.value.trim();
      if (!key) {
        statusEl.textContent = "请先填写 Admin Token / API Key";
        return;
      }
      await runRequest({
        path: "/api/auth/login",
        method: "POST",
        body: JSON.stringify({ key }),
        useLogin: true
      });
    });

    $("healthBtn").addEventListener("click", () => runRequest({ path: "/api/health" }));
    $("readyBtn").addEventListener("click", () => runRequest({ path: "/api/health/ready" }));
    $("usersBtn").addEventListener("click", () => runRequest({ path: "/api/actions/users" }));
    $("settingsBtn").addEventListener("click", () => runRequest({ path: "/api/system-settings" }));
    $("responsesBtn").addEventListener("click", () => runRequest({
      path: "/v1/responses",
      method: "POST",
      body: requestBodyInput.value,
      contentType: contentTypeInput.value
    }));

    $("quickChecks").addEventListener("click", (event) => {
      const button = event.target.closest("button[data-path]");
      if (!button) return;
      requestPathInput.value = button.dataset.path || "";
      methodInput.value = button.dataset.method || "GET";
      runRequest({
        path: button.dataset.path,
        method: button.dataset.method || "GET",
        body: button.dataset.method === "GET" ? null : requestBodyInput.value,
        contentType: contentTypeInput.value
      });
    });

    requestPathInput.addEventListener("change", () => {
      statusEl.textContent = "已更新自定义路径: " + requestPathInput.value.trim();
    });

    methodInput.addEventListener("change", () => {
      statusEl.textContent = "已更新 Method: " + methodInput.value;
    });

    document.addEventListener("keydown", (event) => {
      if ((event.ctrlKey || event.metaKey) && event.key === "Enter") {
        runRequest({
          path: requestPathInput.value.trim() || "/v1/responses",
          method: methodInput.value,
          body: methodInput.value === "GET" ? null : requestBodyInput.value,
          contentType: contentTypeInput.value
        });
      }
    });
  </script>
</body>
</html>`
