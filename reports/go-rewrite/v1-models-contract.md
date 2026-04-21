# /v1/models 最小兼容切片验证说明

更新时间：2026-04-21
负责人：worker-2

## Repo evidence used

- `.omx/specs/deep-interview-go-rewrite-functional-parity-auth-sprint.md`
- `internal/handler/v1/proxy.go`
- `tests/go_parity/proxy_auth_contract_test.go`
- `tests/go-parity/fixtures/proxy-auth-cases.json`

## Added coverage

1. `/v1/models` 鉴权失败 contract
   - 缺失凭据 -> `401 token_required`
   - 冲突凭据 -> `401 invalid_credentials`
2. `/v1/models` 鉴权通过 contract
   - `Authorization: Bearer <key>`
   - `x-api-key`
   - `x-goog-api-key`
   - Gemini `?key=` 查询参数
3. `/v1/models` 最小响应形状
   - OpenAI 默认格式：`200 { object: "list", data: [...] }`
   - Gemini 凭据格式：`200 { models: [...] }`
   - 仅使用 Gemini `?key=` 查询参数时，当前按 Node `detectResponseFormat` 语义仍保持 OpenAI 兼容 shape，而不是自动切到 Gemini shape
   - 该 contract 仍保持“顶层必须是 JSON 且模型集合字段必须为数组”的约束，便于后续继续补齐 Anthropic / upstream fetch 细节

## 当前实现边界

- 当前 Go 切片只聚合已配置 provider 的 `allowedModels`，**不会**像 Node 完整实现那样回源请求上游 `/models`
- provider 过滤已覆盖：
  - auth 后的有效 provider group
  - response/client format 对应的 provider type
  - 去重与稳定排序
- handler 若未注入 provider repository 会返回 internal error，而不是静默 `200 []`，避免 wiring 失效被测试误判
- 这保证了 `/v1/models` 从“auth 后 501 占位”前进到“auth 后返回可消费的最小模型目录”，但仍不是完整 parity 终点

## Intent

该说明用于固定本轮最小兼容切片的行为边界：`/v1/models` 在 Go 中已具备 auth 穿透和基础模型目录输出能力，同时把“未实现 upstream fetch / schedule / provider health 细节”明确标记为后续工作。
