# Codex session extraction parity slice

更新时间：2026-04-22
负责人：worker-2

## Repo evidence used

- `.omx/plans/prd-go-rewrite-codex-session-extraction.md`
- `.omx/plans/test-spec-go-rewrite-codex-session-extraction.md`
- `/home/vlyxo/work/claude-code-hub-plus/src/app/v1/_lib/codex/session-extractor.ts`
- `/home/vlyxo/work/claude-code-hub-plus/src/app/v1/_lib/codex/__tests__/session-extractor.test.ts`
- `internal/service/session/codex_extractor.go`

## Added verification coverage

1. 提取优先级 contract
   - `session_id` header 优先于所有 body fallback
   - `x-session-id` 在 `session_id` 缺失时生效
   - `prompt_cache_key` 优先于 `metadata.session_id`
   - `metadata.session_id` 会在前序无效时接管
2. 校验边界 contract
   - trim 后恰好 21 长度可通过
   - 小于 21 长度拒绝
   - 大于 256 长度拒绝
   - 非法字符拒绝
   - `- _ . :` 组合字符可通过
   - `metadata` 非对象、以及非字符串值会被忽略并继续按回退链处理
3. previous_response_id fallback contract
   - 成功时统一加 `codex_prev_` 前缀
   - 前缀拼接后超过 256 时整体拒绝
4. 空结果 contract
   - 无合法值时返回空 session id 与空 source

## Files added for this verification lane

- `tests/go-parity/fixtures/codex-session-extractor-cases.json`
- `tests/go_parity/codex_session_extractor_contract_test.go`
- `reports/go-rewrite/codex-session-extraction-parity.md`


## Verification record

- PASS `/home/vlyxo/.local/sdk/go/bin/gofmt -l tests/go_parity/codex_session_extractor_contract_test.go`
- PASS `/home/vlyxo/.local/sdk/go/bin/go test ./tests/go_parity -run TestCodexSessionExtractorContractCases -v`
- PASS `/home/vlyxo/.local/sdk/go/bin/go test ./internal/service/... ./tests/...`
- PASS `/home/vlyxo/.local/sdk/go/bin/go test ./...`
- PASS `/home/vlyxo/.local/sdk/go/bin/go build ./cmd/server`

## Current slice boundary

- 本轮只锁定 pure extractor 语义，不把 Redis session manager、request_sequence、或 `/v1/responses` handler wiring 纳入 contract。
- verification lane 依赖 implementation lane 的 `internal/service/session/codex_extractor.go`；本报告不扩展 implementation scope。

## Intent

该说明把 Go 版本 Codex session extractor 的 parity 证据固定到 fixtures + contract tests 上，确保后续 `/v1/responses` 或 session manager 接线时，有一个独立、可回归、可追溯的 Node->Go 语义基线。
