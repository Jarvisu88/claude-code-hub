# Auth parity scaffold

更新时间：2026-04-21
负责人：worker-2

## Node 基线

- `../claude-code-hub plus/src/app/v1/_lib/proxy/auth-guard.ts`
- `../claude-code-hub plus/src/app/v1/_lib/proxy/responses.ts`

## 本次新增

- `internal/service/auth/proxy_parity_test.go`
- `tests/go-parity/fixtures/auth/proxy_auth_inputs.json`

## 覆盖点

1. 代理鉴权输入归一化
   - `Authorization: Bearer <key>`
   - `x-api-key`
   - `x-goog-api-key`
   - Gemini `?key=` 查询参数回退
   - 同 key 多头部共存
   - 冲突 key 拒绝
2. `auth.Service` 当前可验证行为
   - Gemini query key 成功鉴权
   - 相同凭据跨头部复用成功
   - disabled key / expired key 错误码
   - admin token 缺失 / 未配置错误

## 当前已识别但未在 scaffold 中强制断言的差异

1. Node `/v1` 401 响应的 `error.type`/`error.code` 往往使用同一字符串；Go 当前 `AppError` 采用 `type=authentication_error + 细粒度 code`。
2. Node 过期用户提示包含具体日期（`YYYY-MM-DD`）；Go 当前文案只返回通用过期提示。

## 目的

该 scaffold 先把 Node->Go 的鉴权输入语义与核心错误分支固定下来，便于后续把 auth service 接到 `/v1` handler 时继续补 HTTP contract parity 断言，而不直接改共享 handler / proxy 实现文件。
