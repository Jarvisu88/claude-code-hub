# /v1 models minimal catalog endpoints

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /v1/models`
- `GET /v1/responses/models`
- `GET /v1/chat/completions/models`
- `GET /v1/chat/models`

## 最小语义

- 读取 active providers
- 从 `AllowedModels` 聚合唯一模型列表
- `/v1/models` 返回全部 provider 的聚合模型
- `/v1/responses/models` 只返回 `codex` / `openai-compatible`
- `/v1/chat/completions/models` 与 `/v1/chat/models` 只返回 `openai-compatible`
- 若 provider service 未注入，则保持旧行为，继续返回 `notImplemented`

## 验证记录

- PASS `go test ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`

## 边界

本轮只提供最小模型目录，不补 Node 的完整 available-models 选择逻辑、用户维度过滤、供应商探测或价格联动。
