const DEFAULT_REFRESH_WINDOW_MS = 5 * 60 * 1000;

export type UpstreamRateSource = "sub2api" | "newapi";

export interface UpstreamRateAuthUpdate {
  accessToken: string;
  refreshToken?: string;
  tokenExpiresAt?: number;
}

export interface UpstreamRateResult {
  rate: number;
  refreshed: boolean;
  auth?: UpstreamRateAuthUpdate;
  key: {
    id: number | null;
    name: string;
    keyPrefix: string;
    group: string | number | null;
  };
  group: {
    id: number | null;
    name: string;
    rate: number;
    raw: unknown;
  } | null;
  raw: unknown;
}

export interface Sub2ApiRateConfig {
  baseUrl: string;
  apiKey?: string | null;
  keyName?: string | null;
  accessToken?: string | null;
  refreshToken?: string | null;
  tokenExpiresAt?: number | null;
  pageSize?: number | null;
  maxPages?: number | null;
}

export interface NewApiRateConfig {
  baseUrl: string;
  apiKey?: string | null;
  keyName?: string | null;
  accessToken?: string | null;
  cookie?: string | null;
  userId?: string | number | null;
  pageSize?: number | null;
  maxPages?: number | null;
}

interface ClientOptions {
  fetchImpl?: typeof fetch;
  now?: () => number;
  refreshWindowMs?: number;
}

class HttpError extends Error {
  constructor(
    readonly status: number,
    readonly url: string,
    readonly body: unknown
  ) {
    super(`HTTP ${status} from ${url}`);
  }
}

function getFetch(fetchImpl?: typeof fetch): typeof fetch {
  const impl = fetchImpl ?? globalThis.fetch;
  if (typeof impl !== "function") {
    throw new Error("fetch is not available");
  }
  return impl;
}

function normalizeBaseUrl(baseUrl: string): string {
  const normalized = String(baseUrl || "")
    .trim()
    .replace(/\/+$/, "");
  if (!normalized) {
    throw new Error("baseUrl is required");
  }
  return normalized;
}

async function requestJson(fetchImpl: typeof fetch, url: string, init?: RequestInit) {
  const response = await fetchImpl(url, init);
  const text = await response.text();
  let body: unknown = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    throw new Error(`Non-JSON response from ${url}`);
  }
  if (!response.ok) {
    throw new HttpError(response.status, url, body);
  }
  return unwrapApiResponse(body);
}

function unwrapApiResponse(body: unknown): unknown {
  if (!body || typeof body !== "object") {
    return body;
  }
  const record = body as Record<string, unknown>;
  if ("code" in record && "data" in record) {
    if (record.code !== 0) {
      throw new Error(`API error ${String(record.code)}: ${String(record.message ?? "unknown")}`);
    }
    return record.data;
  }
  if ("success" in record) {
    if (record.success === false) {
      throw new Error(`API error: ${String(record.message ?? "unknown")}`);
    }
    return record.data;
  }
  return body;
}

function normalizeApiKey(apiKey: string | null | undefined): string {
  return String(apiKey ?? "")
    .trim()
    .replace(/^sk-/i, "");
}

function keyPrefix(apiKey: string | null | undefined): string {
  const normalized = normalizeApiKey(apiKey);
  if (!normalized) return "";
  return `${normalized.slice(0, 8)}...${normalized.slice(-4)}`;
}

function tokenKeyMatches(candidate: unknown, target: string | null | undefined): boolean {
  const left = normalizeApiKey(String(candidate ?? ""));
  const right = normalizeApiKey(target);
  return Boolean(left && right && left === right);
}

function maskedTokenMayMatch(candidate: unknown, target: string | null | undefined): boolean {
  const left = normalizeApiKey(String(candidate ?? ""));
  const right = normalizeApiKey(target);
  if (!left || !right || !left.includes("*") || left.length < 8 || right.length < 8) {
    return false;
  }
  return right.startsWith(left.slice(0, 4)) && right.endsWith(left.slice(-4));
}

function nameMatches(candidate: unknown, target: string | null | undefined): boolean {
  const left = String(candidate ?? "")
    .trim()
    .toLowerCase();
  const right = String(target ?? "")
    .trim()
    .toLowerCase();
  return Boolean(right && (left === right || left.includes(right)));
}

function pageItems(data: unknown): Record<string, unknown>[] {
  if (Array.isArray(data)) return data as Record<string, unknown>[];
  if (!data || typeof data !== "object") return [];
  const record = data as Record<string, unknown>;
  const nested = record.data && typeof record.data === "object" ? record.data : null;
  const items = record.items ?? (nested as Record<string, unknown> | null)?.items;
  return Array.isArray(items) ? (items as Record<string, unknown>[]) : [];
}

function pageTotal(data: unknown, items: unknown[]): number {
  if (!data || typeof data !== "object") return items.length;
  const record = data as Record<string, unknown>;
  const nested = record.data && typeof record.data === "object" ? record.data : null;
  return Number(record.total ?? (nested as Record<string, unknown> | null)?.total ?? items.length);
}

function readNumericRate(value: unknown, label: string): number {
  if (value === "自动") {
    throw new Error(`${label} is auto and cannot be synced as a numeric multiplier`);
  }
  const rate = Number(value);
  if (!Number.isFinite(rate) || rate < 0) {
    throw new Error(`${label} is not a valid non-negative number`);
  }
  return rate;
}

function readFirstNumericRate(
  values: Array<{ value: unknown; label: string }>,
  context: string
): number {
  const firstPresent = values.find(({ value }) => value !== undefined && value !== null);
  if (!firstPresent) {
    throw new Error(`${context} has no numeric multiplier field`);
  }
  return readNumericRate(firstPresent.value, firstPresent.label);
}

function isUnauthorized(error: unknown): boolean {
  return error instanceof HttpError && error.status === 401;
}

function shouldRefresh(config: Sub2ApiRateConfig, now: number, refreshWindowMs: number): boolean {
  const expiresAt = Number(config.tokenExpiresAt);
  return !config.accessToken || !Number.isFinite(expiresAt) || expiresAt <= now + refreshWindowMs;
}

async function refreshSub2ApiAuth(
  config: Sub2ApiRateConfig,
  fetchImpl: typeof fetch,
  now: number
): Promise<Required<Pick<UpstreamRateAuthUpdate, "accessToken">> & UpstreamRateAuthUpdate> {
  if (!config.refreshToken) {
    throw new Error("refreshToken is required for Sub2API auth refresh");
  }
  const baseUrl = normalizeBaseUrl(config.baseUrl);
  const data = (await requestJson(fetchImpl, `${baseUrl}/api/v1/auth/refresh`, {
    method: "POST",
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
    },
    body: JSON.stringify({ refresh_token: config.refreshToken }),
  })) as Record<string, unknown>;

  const accessToken = String(data.access_token ?? "");
  const refreshToken = String(data.refresh_token ?? "");
  if (!accessToken || !refreshToken) {
    throw new Error("Sub2API refresh response missing token fields");
  }
  const expiresIn = Number(data.expires_in ?? 0);
  return {
    accessToken,
    refreshToken,
    tokenExpiresAt: Number.isFinite(expiresIn) ? now + expiresIn * 1000 : undefined,
  };
}

async function findSub2ApiKey(
  config: Sub2ApiRateConfig,
  accessToken: string,
  fetchImpl: typeof fetch
): Promise<Record<string, unknown>> {
  const baseUrl = normalizeBaseUrl(config.baseUrl);
  const pageSize = Number(config.pageSize ?? 100);
  const maxPages = Number(config.maxPages ?? 20);
  if (!config.apiKey && !config.keyName) {
    throw new Error("apiKey or keyName is required");
  }

  for (let page = 1; page <= maxPages; page += 1) {
    const params = new URLSearchParams({
      page: String(page),
      page_size: String(pageSize),
    });
    if (!config.apiKey && config.keyName) {
      params.set("search", config.keyName);
    }
    const data = await requestJson(fetchImpl, `${baseUrl}/api/v1/keys?${params}`, {
      headers: {
        Accept: "application/json",
        Authorization: `Bearer ${accessToken}`,
      },
    });
    const items = pageItems(data);
    const key =
      items.find((item) => tokenKeyMatches(item.key, config.apiKey)) ??
      items.find((item) => nameMatches(item.name, config.keyName));
    if (key) return key;
    if (items.length < pageSize || page * pageSize >= pageTotal(data, items)) break;
  }

  throw new Error("Sub2API key not found");
}

async function resolveSub2ApiGroup(
  config: Sub2ApiRateConfig,
  key: Record<string, unknown>,
  accessToken: string,
  fetchImpl: typeof fetch
): Promise<Record<string, unknown> | null> {
  const embeddedGroup =
    key.group && typeof key.group === "object" ? (key.group as Record<string, unknown>) : null;
  if (
    embeddedGroup?.balance_charge_rate !== undefined ||
    embeddedGroup?.rate_multiplier !== undefined
  ) {
    return embeddedGroup;
  }
  if (key.group_id == null) return embeddedGroup;

  const baseUrl = normalizeBaseUrl(config.baseUrl);
  const data = await requestJson(fetchImpl, `${baseUrl}/api/v1/groups/available`, {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
  });
  const groups = Array.isArray(data) ? (data as Record<string, unknown>[]) : [];
  return groups.find((group) => Number(group.id) === Number(key.group_id)) ?? embeddedGroup;
}

async function readSub2ApiUserGroupRates(
  config: Sub2ApiRateConfig,
  accessToken: string,
  fetchImpl: typeof fetch
): Promise<Record<string, unknown>> {
  const baseUrl = normalizeBaseUrl(config.baseUrl);
  const data = await requestJson(fetchImpl, `${baseUrl}/api/v1/groups/rates`, {
    headers: {
      Accept: "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
  });
  return data && typeof data === "object" && !Array.isArray(data)
    ? (data as Record<string, unknown>)
    : {};
}

function readSub2ApiUserGroupRate(
  rates: Record<string, unknown>,
  key: Record<string, unknown>,
  group: Record<string, unknown> | null
): number | null {
  const candidates = [
    key.group_id == null ? null : String(key.group_id),
    group?.id == null ? null : String(group.id),
    group?.name == null ? null : String(group.name),
  ].filter(Boolean) as string[];
  for (const candidate of candidates) {
    if (rates[candidate] === undefined || rates[candidate] === null) continue;
    return readNumericRate(rates[candidate], `groups/rates.${candidate}`);
  }
  return null;
}

export async function fetchSub2ApiProviderRate(
  config: Sub2ApiRateConfig,
  options: ClientOptions = {}
): Promise<UpstreamRateResult> {
  const fetchImpl = getFetch(options.fetchImpl);
  const now = options.now?.() ?? Date.now();
  const refreshWindowMs = options.refreshWindowMs ?? DEFAULT_REFRESH_WINDOW_MS;
  let auth: UpstreamRateAuthUpdate = {
    accessToken: config.accessToken ?? "",
    refreshToken: config.refreshToken ?? undefined,
    tokenExpiresAt: config.tokenExpiresAt ?? undefined,
  };
  let refreshed = false;
  if (shouldRefresh(config, now, refreshWindowMs)) {
    auth = await refreshSub2ApiAuth(config, fetchImpl, now);
    refreshed = true;
  }

  let key: Record<string, unknown>;
  let group: Record<string, unknown> | null;
  let userGroupRates: Record<string, unknown>;
  try {
    key = await findSub2ApiKey(config, auth.accessToken, fetchImpl);
    group = await resolveSub2ApiGroup(config, key, auth.accessToken, fetchImpl);
    userGroupRates = await readSub2ApiUserGroupRates(config, auth.accessToken, fetchImpl);
  } catch (error) {
    if (!isUnauthorized(error) || refreshed) throw error;
    auth = await refreshSub2ApiAuth(config, fetchImpl, now);
    refreshed = true;
    key = await findSub2ApiKey(config, auth.accessToken, fetchImpl);
    group = await resolveSub2ApiGroup(config, key, auth.accessToken, fetchImpl);
    userGroupRates = await readSub2ApiUserGroupRates(config, auth.accessToken, fetchImpl);
  }

  const rate =
    readSub2ApiUserGroupRate(userGroupRates, key, group) ??
    readFirstNumericRate(
      [
        { value: group?.subscription_charge_rate, label: "subscription_charge_rate" },
        { value: group?.balance_charge_rate, label: "balance_charge_rate" },
        { value: group?.rate_multiplier, label: "rate_multiplier" },
      ],
      "Sub2API group"
    );
  return {
    rate,
    refreshed,
    auth: refreshed ? auth : undefined,
    key: {
      id: key.id == null ? null : Number(key.id),
      name: String(key.name ?? ""),
      keyPrefix: keyPrefix(String(key.key ?? config.apiKey ?? "")),
      group: key.group_id == null ? null : Number(key.group_id),
    },
    group: group
      ? {
          id: group.id == null ? null : Number(group.id),
          name: String(group.name ?? ""),
          rate,
          raw: group,
        }
      : null,
    raw: { key, group },
  };
}

async function findNewApiToken(
  config: NewApiRateConfig,
  fetchImpl: typeof fetch
): Promise<Record<string, unknown>> {
  const baseUrl = normalizeBaseUrl(config.baseUrl);
  const pageSize = Number(config.pageSize ?? 100);
  const maxPages = Number(config.maxPages ?? 20);
  if (!config.apiKey && !config.keyName) {
    throw new Error("apiKey or keyName is required");
  }

  for (let page = 1; page <= maxPages; page += 1) {
    const params = new URLSearchParams({
      p: String(page - 1),
      size: String(pageSize),
    });
    if (config.apiKey) {
      params.set("token", normalizeApiKey(config.apiKey));
    } else if (config.keyName) {
      params.set("keyword", config.keyName);
    }
    const data = await requestJson(fetchImpl, `${baseUrl}/api/token/search?${params}`, {
      headers: newApiHeaders(config),
    });
    const items = pageItems(data);
    const token =
      items.find((item) => tokenKeyMatches(item.key, config.apiKey)) ??
      items.find((item) => maskedTokenMayMatch(item.key, config.apiKey)) ??
      items.find((item) => nameMatches(item.name, config.keyName));
    if (token) return token;
    if (items.length < pageSize || page * pageSize >= pageTotal(data, items)) break;
  }

  throw new Error("New API token not found");
}

function newApiHeaders(config: NewApiRateConfig): HeadersInit {
  const headers: Record<string, string> = {
    Accept: "application/json",
    "Content-Type": "application/json",
  };
  if (config.accessToken) {
    headers.Authorization = `Bearer ${config.accessToken}`;
  }
  if (config.cookie) {
    headers.Cookie = config.cookie;
  }
  if (config.userId != null && String(config.userId).trim()) {
    headers["New-Api-User"] = String(config.userId);
  }
  return headers;
}

export async function fetchNewApiProviderRate(
  config: NewApiRateConfig,
  options: ClientOptions = {}
): Promise<UpstreamRateResult> {
  const fetchImpl = getFetch(options.fetchImpl);
  const baseUrl = normalizeBaseUrl(config.baseUrl);
  const token = await findNewApiToken(config, fetchImpl);
  const groups = (await requestJson(fetchImpl, `${baseUrl}/api/user/self/groups`, {
    headers: newApiHeaders(config),
  })) as Record<string, unknown>;
  const groupName = String(token.group ?? "");
  const group = groups[groupName] as Record<string, unknown> | undefined;
  const rate = readNumericRate(group?.ratio, "ratio");

  return {
    rate,
    refreshed: false,
    key: {
      id: token.id == null ? null : Number(token.id),
      name: String(token.name ?? ""),
      keyPrefix: keyPrefix(String(token.key ?? config.apiKey ?? "")),
      group: groupName || null,
    },
    group: {
      id: null,
      name: groupName,
      rate,
      raw: group,
    },
    raw: { token, group },
  };
}
