# Test Spec: Go Rewrite — Codex Session Extraction Slice

## Scope

验证 Go 版本的 Codex session extractor 与 Node 基线在以下方面保持一致：
1. 提取优先级
2. 长度与字符校验
3. previous_response_id fallback 规则
4. source 标识输出

## Baseline Evidence

- Node extractor: `src/app/v1/_lib/codex/session-extractor.ts`
- Node tests: `src/app/v1/_lib/codex/__tests__/session-extractor.test.ts`
- Go target field consumers: `internal/model/message_request.go`

## Acceptance Matrix

- [ ] header `session_id` 优先级最高
- [ ] header `x-session-id` 次优先
- [ ] `prompt_cache_key` 优先于 `metadata.session_id`
- [ ] `metadata.session_id` 在前序无值时生效
- [ ] `previous_response_id` 回退时带 `codex_prev_` 前缀
- [ ] 小于 21 长度被拒绝
- [ ] 大于 256 长度被拒绝
- [ ] 非法字符被拒绝
- [ ] 合法特殊字符被接受
- [ ] 无合法值时返回 null source

## Verification Commands

- `go test ./internal/service/... ./tests/...`
- `go test ./...`
- `go build ./cmd/server`
