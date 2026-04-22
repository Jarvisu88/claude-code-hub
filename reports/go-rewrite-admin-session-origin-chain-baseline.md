# admin session-origin-chain baseline

更新时间：2026-04-22  
负责人：Codex

## 本次新增

- `GET /api/actions/session-origin-chain?sessionId=...`

## 最小语义

- 复用 admin token 鉴权
- 通过 `MessageRequestRepository.FindLatestBySessionID` 查找最新日志
- 返回该日志上的 `providerChain`
- 缺少 `sessionId` 返回 `400`

## 边界

本轮只补最小 session-origin-chain 查询，不做权限细分、跨 key/user 归属校验、live chain、深层错误详情或 session debug artifacts。

## 验证记录

- PASS `go test ./internal/handler/api`
- PASS `go test ./internal/handler/api ./internal/handler/v1 ./tests/go_parity`
- PASS `go test ./...`
- PASS `go build ./cmd/server`
- PASS `go vet ./...`
