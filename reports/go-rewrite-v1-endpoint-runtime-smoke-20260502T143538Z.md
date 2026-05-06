# Go Rewrite /v1 Endpoint Runtime Smoke — 20260502T143538Z

## 结论

通过。该 smoke 使用反伪阳性夹具证明：

- `providers.url` 被故意改为不可用 control URL：`http://127.0.0.1:1/provider-url-should-not-be-used`
- `provider_endpoints.url` 指向本地 mock upstream：`http://127.0.0.1:23001/__mock__/v1/responses`
- `POST /v1/responses` 返回 HTTP 200，并且最新 `message_request.provider_chain` 记录的 `endpointUrl` 是 provider endpoint URL，不包含 control URL。

## Runtime 环境

- Go server: `127.0.0.1:23001`
- Postgres container: `cch-go-smoke-postgres` / host port `55432`
- Redis container: `cch-go-smoke-redis` / host port `56379`
- curl 均使用 `--noproxy '*'`，避免本机代理污染 smoke 结果。

## HTTP 验证

- `GET /health` -> HTTP 200
- `GET /v1/models` -> HTTP 200
- `POST /v1/responses` -> HTTP 200

### /v1/models response body

```json
{"data":[{"id":"claude-sonnet-4","object":"model"},{"id":"gpt-4o-mini","object":"model"},{"id":"gpt-5.4","object":"model"}],"object":"list"}
```

### /v1/responses response body

```json
{"id":"resp_local_mock","prompt_cache_key":"019b82ff-08ff-75a3-a203-7e10274fdbd8","status":"completed","usage":{"input_tokens":10,"output_tokens":5}}
```

## Anti-false-positive fixture

### Provider row

```
2|local-codex-mock|http://127.0.0.1:1/provider-url-should-not-be-used|https://local.mock|codex|t
```

### Provider endpoint rows

```
2|1|codex|http://127.0.0.1:23001/__mock__/v1/responses|t|
```

## DB evidence: latest message_request

```
1|200|[{"id": 2, "name": "local-codex-mock", "reason": "initial_selection", "weight": 1, "priority": 1, "timestamp": 1777732410772, "statusCode": 200, "endpointUrl": "http://127.0.0.1:23001/__mock__/v1/responses", "providerType": "codex", "costMultiplier": 1, "selectionMethod": "weighted_random"}]|10|5
```

## 判定

`/v1/responses` runtime 主链成功走 endpoint-aware routing：provider 通过 `website_url=local.mock` 关联 vendor，再按 `provider_type=codex` 选择 active provider endpoint。请求没有落回不可用的 `providers.url`。
