> 注意：本报告对应较早的 team worktree 审查结论，当前主线状态已由 `reports/go-rewrite-responses-minimal-loop.md` 更新。

# /v1/responses regression/contracts review

更新时间：2026-04-22  
负责人：worker-3

## Review scope

审查本轮 **`/v1/responses` first runnable loop** 的 regression/contracts 面，只核对：

- 现有 auth/session 语义是否被保持
- 当前测试是否已经锁住最小边界
- 还有哪些 contract 缺口阻止我们把该切片判定为“runnable”

本轮**不改共享 handler / provider / repository**，避免覆盖 worker-1 / worker-2 责任面。

## Current code evidence

### 1. `/v1/responses` 仍未进入真正 runnable handler

`internal/handler/v1/proxy.go:38-45` 目前仍把 `/v1/responses` 注册到统一的 `notImplemented` 占位；`internal/handler/v1/proxy.go:130-137` 直接返回 `501` + `not_implemented` 错误体。

这意味着：

- auth middleware 已接入
- session lifecycle middleware 已接入
- 但 **happy-path `/v1/responses` 仍未形成真正的 request -> handler -> controlled response loop**

因此按 `.omx/plans/execution-plan-go-rewrite-fastlane-20260422.md` 的 sprint gate 来看，当前 worktree 里的 `/v1/responses` 还不能算“第一个真正可运行的最小代理闭环”。

### 2. auth/session 边界目前是稳定的

`internal/handler/v1/proxy.go:48-127` 已经把以下边界接到 `/v1` 代理入口：

- `AuthMiddleware` 会把 `AuthResult` 写入 Gin context
- `SessionLifecycleMiddleware` 只在 POST 代理路由上生效（`/v1/messages`、`/v1/chat/completions`、`/v1/responses`）
- 请求体会先 decode，再提取 session，再做并发计数增减
- `GET /v1/models` 不会误触发 session lifecycle

这与当前最小切片的目标一致：**先保 auth + session lifecycle 对称，再等真实 `/v1/responses` handler 接上**。

### 3. 已有回归测试覆盖了 auth/session preservation，但没有覆盖 `/v1/responses` happy path contract

现有单测已经锁住以下行为：

- `internal/handler/v1/proxy_test.go:91-156`：auth result 写入 context、无效 key 返回 401
- `internal/handler/v1/proxy_test.go:158-217`：`/v1/responses` 在鉴权通过后会触发 session lifecycle，并把生成后的 session id 写入 context
- `internal/handler/v1/proxy_test.go:219-308`：`/v1/models` 不追踪 session；`/v1/messages` 与 `/v1/chat/completions` 也会沿用同一 lifecycle
- `internal/handler/v1/proxy_test.go:310-430`：缺失 auth、缺失 key、非法 JSON、空 session id 时都能 fail-safe 跳过 lifecycle

现有 contract harness 只锁了 **auth contract**：

- `tests/go_parity/proxy_auth_contract_test.go:57-91`
- `tests/go-parity/fixtures/proxy-auth-cases.json`

其中 fixture 仍只要求受保护路由在鉴权成功后可以进入当前占位行为；它**没有**定义 `/v1/responses` 成功请求的 response shape、session extraction carrier、或“已不再返回 501”的新 contract。

## Regression verdict

### Preserved now

1. `/v1` auth gating 仍在
2. `/v1/responses` 的 session lifecycle 仍在 auth 之后、handler 之前执行
3. 非代理路由（如 `GET /v1/models`）不会被并发 session 计数误伤
4. 非法 body / 缺失 key 等错误路径仍是 fail-safe，不会误写 session state

### Missing before sprint can be called complete

1. `/v1/responses` 需要脱离 `notImplemented`，不再返回 501
2. 需要新增 **happy-path contract**，至少固定：
   - 鉴权成功后不再 501
   - request body decode 成功
   - Codex/Responses session extraction 结果可被 handler 层消费
   - 返回受控最小响应，而不是再次回落到 stub
3. 需要把上述 happy-path 行为写入新的 contract test / fixture，而不是只留在 unit-test middleware 级别

## Recommended next contract slice

等 worker-1 的 handler wiring 落地后，回归/contracts lane 下一刀建议只补下面 3 个 case：

1. **authorized minimal responses request**
   - 输入：有效 key + `input` body
   - 断言：非 501；返回受控最小占位响应
2. **codex session extraction visibility**
   - 输入：`session_id` / `x-session-id` / `metadata.session_id` / `previous_response_id` 中至少两组 fixture
   - 断言：handler 层能读到提取结果，而不是只在 service 内消费掉
3. **boundary guard**
   - 断言：当前切片仍不触发 provider selection、upstream forwarding、usage/cost writeback

## Verification record

- PASS `go test ./internal/handler/v1 ./internal/service/session ./tests/go_parity`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## Summary

当前 worktree 的 regression 证据说明：**auth/session 语义已稳定，但 `/v1/responses` 仍是 501 stub，尚未达到 runnable-loop 完成线。** 本报告先把这个事实与下一步 contract 缺口固定下来，避免在 worker-1/worker-2 未完成前误报闭环完成。
