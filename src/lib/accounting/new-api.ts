export interface NewApiLogsUrlParams {
  startTimestamp: number;
  endTimestamp: number;
  page: number;
  pageSize: number;
}

export function buildNewApiLogsUrl(baseUrl: string, params: NewApiLogsUrlParams): string {
  const url = new URL("/api/log/", baseUrl);
  url.searchParams.set("type", "2");
  url.searchParams.set("start_timestamp", String(params.startTimestamp));
  url.searchParams.set("end_timestamp", String(params.endTimestamp));
  url.searchParams.set("p", String(params.page));
  url.searchParams.set("page_size", String(params.pageSize));
  return url.toString();
}

export function buildNewApiPricingUrl(baseUrl: string): string {
  return new URL("/api/pricing", baseUrl).toString();
}

export function buildNewApiStatusUrl(baseUrl: string): string {
  return new URL("/api/status", baseUrl).toString();
}

export interface NewApiConsumeLog {
  id?: number;
  user_id?: number;
  username?: string;
  token_name?: string;
  model_name?: string;
  quota?: number | string | null;
  prompt_tokens?: number | string | null;
  completion_tokens?: number | string | null;
  group?: string | null;
  channel?: number | string | null;
  channel_name?: string | null;
  created_at?: number;
  request_id?: string;
  upstream_request_id?: string;
  content?: string;
}

export interface NewApiLogsPayload {
  success?: boolean;
  message?: string;
  data?:
    | {
        items?: NewApiConsumeLog[];
        total?: number;
        page?: number;
        page_size?: number;
      }
    | NewApiConsumeLog[];
}

export function parseNewApiLogsPayload(payload: NewApiLogsPayload): {
  logs: NewApiConsumeLog[];
  total: number | null;
} {
  if (!payload.success) {
    throw new Error(payload.message || "new-api rejected the log request");
  }

  if (Array.isArray(payload.data)) {
    return { logs: payload.data, total: payload.data.length };
  }

  const data = payload.data ?? {};
  const logs = Array.isArray(data.items) ? data.items : [];
  const total = Number(data.total);
  return {
    logs,
    total: Number.isFinite(total) ? total : null,
  };
}

export interface NewApiPricingItem {
  model_name?: string;
  quota_type?: number;
  model_ratio?: number | string | null;
  model_price?: number | string | null;
  completion_ratio?: number | string | null;
  enable_groups?: string[];
}

export interface NewApiPricingPayload {
  success?: boolean;
  message?: string;
  data?: NewApiPricingItem[];
  group_ratio?: Record<string, number | string | null>;
}

export function parseNewApiPricingPayload(payload: NewApiPricingPayload): {
  pricing: NewApiPricingItem[];
  groupRatios: Map<string, number>;
} {
  if (!payload.success) {
    throw new Error(payload.message || "new-api rejected the pricing request");
  }

  const groupRatios = new Map<string, number>();
  for (const [group, rawRatio] of Object.entries(payload.group_ratio ?? {})) {
    const ratio = Number(rawRatio);
    if (Number.isFinite(ratio)) {
      groupRatios.set(group, ratio);
    }
  }

  return {
    pricing: Array.isArray(payload.data) ? payload.data : [],
    groupRatios,
  };
}

export interface NewApiStatusPayload {
  success?: boolean;
  message?: string;
  data?: {
    quota_per_unit?: number | string | null;
  } | null;
}

export function parseNewApiQuotaPerUnit(payload: NewApiStatusPayload): number {
  if (!payload.success) {
    throw new Error(payload.message || "new-api rejected the status request");
  }

  const quotaPerUnit = Number(payload.data?.quota_per_unit ?? 500000);
  return Number.isFinite(quotaPerUnit) && quotaPerUnit > 0 ? quotaPerUnit : 500000;
}
