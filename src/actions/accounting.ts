"use server";

import { formatInTimeZone, fromZonedTime } from "date-fns-tz";
import { revalidatePath } from "next/cache";
import { z } from "zod";
import {
  buildNewApiLogsUrl,
  buildNewApiPricingUrl,
  buildNewApiStatusUrl,
  type NewApiConsumeLog,
  type NewApiPricingItem,
  parseNewApiLogsPayload,
  parseNewApiPricingPayload,
  parseNewApiQuotaPerUnit,
} from "@/lib/accounting/new-api";
import { getSession } from "@/lib/auth";
import { invalidateSystemSettingsCache } from "@/lib/config";
import { logger } from "@/lib/logger";
import { resolveSystemTimezone } from "@/lib/utils/timezone";
import {
  type AccountingNewApiConfigPreview,
  findAccountingNewApiConfig,
  findAccountingNewApiConfigPreview,
  findModelSellMultipliers,
  findProviderProfitSummary,
  type ModelSellMultiplierRow,
  type ProviderProfitSummaryRow,
  upsertAccountingNewApiConfig,
  upsertModelSellMultiplier,
} from "@/repository/accounting";
import { getSystemSettings, updateSystemSettings } from "@/repository/system-config";
import type { ActionResult } from "./types";

export interface AccountingSummary {
  startTime: string;
  endTime: string;
  timezone: string;
  globalSellMultiplier: number;
  providers: ProviderProfitSummaryRow[];
  modelSellMultipliers: ModelSellMultiplierRow[];
  newApiConfig: AccountingNewApiConfigPreview;
}

export interface NewApiRevenueRow {
  username: string;
  group: string;
  modelName: string;
  quota: number;
  inputTokens: number;
  outputTokens: number;
  multiplier: number;
  modelRatio: number;
  completionRatio: number;
  groupRatio: number;
  revenueUsd: number;
  requestCount: number;
}

export interface NewApiUsageDetailRow {
  id: number | null;
  createdAt: number | null;
  username: string;
  group: string;
  modelName: string;
  tokenName: string;
  channelName: string;
  inputTokens: number;
  outputTokens: number;
  quota: number;
  modelRatio: number;
  completionRatio: number;
  groupRatio: number;
  quotaType: number;
  quotaPerUnit: number;
  revenueUsd: number;
  requestId: string;
}

export interface NewApiRevenueResult {
  startTimestamp: number;
  endTimestamp: number;
  timezone: string;
  totalLogs: number;
  quotaPerUnit: number;
  rows: NewApiRevenueRow[];
  details: NewApiUsageDetailRow[];
}

const SaveGlobalSellMultiplierSchema = z.object({
  multiplier: z.coerce.number().min(0).max(1000000),
});

const UpsertModelSellMultiplierSchema = z.object({
  modelName: z.string().trim().min(1).max(128),
  multiplier: z.coerce.number().min(0).max(1000000),
  note: z.string().trim().max(200).optional(),
});

const NewApiRevenueSchema = z.object({
  poll: z.boolean().optional(),
  days: z.number().int().min(1).max(30).optional().default(1),
});

const SaveNewApiConfigSchema = z.object({
  baseUrl: z.string().trim().url(),
  accessToken: z.string().trim().optional(),
  userId: z.coerce.number().int().positive(),
  pageSize: z.coerce.number().int().min(20).max(500).optional(),
});

const NEW_API_MAX_LOG_PAGE_SIZE = 100;
const NEW_API_LOG_SLICE_SECONDS = 3 * 60 * 60;
const NEW_API_FETCH_ATTEMPTS = 5;
const NEW_API_FETCH_RETRY_BASE_DELAY_MS = 350;
const NEW_API_RATE_LIMIT_RETRY_DELAY_MS = 2_000;

function addIsoDays(dateStr: string, days: number): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateStr);
  if (!match) return dateStr;
  const date = new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])));
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

async function requireAdmin() {
  const session = await getSession();
  if (!session || session.user.role !== "admin") {
    return null;
  }
  return session;
}

async function resolveTimeRange(
  days: number = 1
): Promise<{ startTime: Date; endTime: Date; timezone: string; todayStart: Date }> {
  const timezone = await resolveSystemTimezone();
  const today = formatInTimeZone(new Date(), timezone, "yyyy-MM-dd");
  const tomorrow = addIsoDays(today, 1);
  const startDay = addIsoDays(today, -days + 1);
  return {
    startTime: fromZonedTime(`${startDay}T00:00:00`, timezone),
    endTime: fromZonedTime(`${tomorrow}T00:00:00`, timezone),
    todayStart: fromZonedTime(`${today}T00:00:00`, timezone),
    timezone,
  };
}

export async function getAccountingSummary(): Promise<ActionResult<AccountingSummary>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const [settings, range] = await Promise.all([getSystemSettings(), resolveTimeRange(1)]);
    const [providers, modelSellMultipliers, newApiConfig] = await Promise.all([
      findProviderProfitSummary({
        startTime: range.startTime,
        endTime: range.endTime,
        globalSellMultiplier: settings.globalSellMultiplier,
      }),
      findModelSellMultipliers(),
      findAccountingNewApiConfigPreview(),
    ]);

    return {
      ok: true,
      data: {
        startTime: range.startTime.toISOString(),
        endTime: range.endTime.toISOString(),
        timezone: range.timezone,
        globalSellMultiplier: settings.globalSellMultiplier,
        providers,
        modelSellMultipliers,
        newApiConfig,
      },
    };
  } catch (error) {
    logger.error("[Accounting] Failed to load summary", error);
    return { ok: false, error: "Failed to load accounting summary" };
  }
}

export async function saveGlobalSellMultiplier(input: {
  multiplier: number;
}): Promise<ActionResult<{ globalSellMultiplier: number }>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const validated = SaveGlobalSellMultiplierSchema.parse(input);
    const updated = await updateSystemSettings({ globalSellMultiplier: validated.multiplier });
    invalidateSystemSettingsCache();
    revalidatePath("/dashboard/accounting");
    return { ok: true, data: { globalSellMultiplier: updated.globalSellMultiplier } };
  } catch (error) {
    logger.error("[Accounting] Failed to save global sell multiplier", error);
    return { ok: false, error: "Failed to save sell multiplier" };
  }
}

export async function saveModelSellMultiplier(input: {
  modelName: string;
  multiplier: number;
  note?: string | null;
}): Promise<ActionResult<ModelSellMultiplierRow>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const validated = UpsertModelSellMultiplierSchema.parse(input);
    const row = await upsertModelSellMultiplier(validated);
    revalidatePath("/dashboard/accounting");
    return { ok: true, data: row };
  } catch (error) {
    logger.error("[Accounting] Failed to save model sell multiplier", error);
    return { ok: false, error: "Failed to save model sell multiplier" };
  }
}

export async function saveNewApiConfig(input: {
  baseUrl: string;
  userId: number;
  accessToken?: string;
  pageSize?: number;
}): Promise<ActionResult<AccountingNewApiConfigPreview>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const validated = SaveNewApiConfigSchema.parse(input);
    const config = await upsertAccountingNewApiConfig({
      baseUrl: validated.baseUrl,
      accessToken: validated.accessToken,
      adminUserId: validated.userId,
      pageSize: validated.pageSize,
    });
    revalidatePath("/dashboard/accounting");
    return { ok: true, data: config };
  } catch (error) {
    logger.error("[Accounting] Failed to save new-api config", error);
    const message = error instanceof Error ? error.message : "Failed to save new-api config";
    return { ok: false, error: message };
  }
}

function normalizeName(value: string | null | undefined): string {
  return value?.trim() || "unknown";
}

function toFiniteNumber(value: unknown, fallback = 0): number {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function normalizeNewApiPageSize(value: number): number {
  return Math.min(Math.max(Math.trunc(value), 20), NEW_API_MAX_LOG_PAGE_SIZE);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function isRetryableNewApiFetchError(error: unknown): boolean {
  if (!(error instanceof Error)) return false;
  const message = `${error.name} ${error.message}`.toLowerCase();
  return (
    message.includes("bodytimeout") ||
    message.includes("econnreset") ||
    message.includes("fetch failed") ||
    message.includes("network") ||
    message.includes("terminated") ||
    message.includes("und_err") ||
    message.includes("unexpected end")
  );
}

function getRetryAfterDelayMs(response: Response): number | null {
  const retryAfter = response.headers.get("retry-after");
  if (!retryAfter) return null;

  const seconds = Number(retryAfter);
  if (Number.isFinite(seconds) && seconds > 0) {
    return Math.min(seconds * 1000, 30_000);
  }

  const retryAt = Date.parse(retryAfter);
  if (Number.isFinite(retryAt)) {
    return Math.min(Math.max(retryAt - Date.now(), 0), 30_000);
  }

  return null;
}

async function readNewApiErrorMessage(response: Response): Promise<string> {
  try {
    const payload = (await response.json()) as { message?: unknown; error?: unknown };
    const message = typeof payload.message === "string" ? payload.message : payload.error;
    return typeof message === "string" && message.trim() ? message.trim() : "";
  } catch {
    return "";
  }
}

async function assertNewApiOk(response: Response, target: string): Promise<void> {
  if (response.ok) return;

  const upstreamMessage = await readNewApiErrorMessage(response);
  if (response.status === 429) {
    throw new Error(
      upstreamMessage
        ? `new-api 请求过于频繁，请稍后再试：${upstreamMessage}`
        : "new-api 请求过于频繁，请稍后再试，或调大每页日志数后减少分页请求"
    );
  }

  throw new Error(
    upstreamMessage
      ? `new-api ${target} 请求失败（HTTP ${response.status}）：${upstreamMessage}`
      : `new-api ${target} 请求失败（HTTP ${response.status}）`
  );
}

async function fetchAllNewApiLogs(params: {
  baseUrl: string;
  accessToken: string;
  adminUserId: number;
  pageSize: number;
  startTimestamp: number;
  endTimestamp: number;
}): Promise<{ logs: NewApiConsumeLog[]; total: number }> {
  const allLogs: NewApiConsumeLog[] = [];
  const pageSize = normalizeNewApiPageSize(params.pageSize);
  const slices: Array<{ startTimestamp: number; endTimestamp: number }> = [];
  let total = 0;

  for (
    let sliceStart = params.startTimestamp;
    sliceStart <= params.endTimestamp;
    sliceStart += NEW_API_LOG_SLICE_SECONDS
  ) {
    slices.push({
      startTimestamp: sliceStart,
      endTimestamp: Math.min(sliceStart + NEW_API_LOG_SLICE_SECONDS - 1, params.endTimestamp),
    });
  }

  for (const slice of slices) {
    const result = await fetchNewApiLogsInWindow({
      ...params,
      pageSize,
      startTimestamp: slice.startTimestamp,
      endTimestamp: slice.endTimestamp,
    });
    allLogs.push(...result.logs);
    total += result.total;
  }

  return { logs: allLogs, total };
}

async function fetchNewApiLogsInWindow(params: {
  baseUrl: string;
  accessToken: string;
  adminUserId: number;
  pageSize: number;
  startTimestamp: number;
  endTimestamp: number;
}): Promise<{ logs: NewApiConsumeLog[]; total: number }> {
  const allLogs: NewApiConsumeLog[] = [];
  let page = 1;
  let total = 0;

  for (;;) {
    let parsed: ReturnType<typeof parseNewApiLogsPayload> | null = null;
    let lastError: unknown;
    const url = buildNewApiLogsUrl(params.baseUrl, {
      startTimestamp: params.startTimestamp,
      endTimestamp: params.endTimestamp,
      page,
      pageSize: params.pageSize,
    });

    for (let attempt = 1; attempt <= NEW_API_FETCH_ATTEMPTS; attempt += 1) {
      try {
        const response = await fetch(url, {
          method: "GET",
          cache: "no-store",
          headers: {
            Authorization: params.accessToken,
            "New-Api-User": String(params.adminUserId),
          },
        });

        if (
          (response.status === 429 || response.status >= 500) &&
          attempt < NEW_API_FETCH_ATTEMPTS
        ) {
          await sleep(
            getRetryAfterDelayMs(response) ??
              (response.status === 429
                ? NEW_API_RATE_LIMIT_RETRY_DELAY_MS * attempt
                : NEW_API_FETCH_RETRY_BASE_DELAY_MS * attempt)
          );
          continue;
        }

        await assertNewApiOk(response, "logs");
        parsed = parseNewApiLogsPayload(await response.json());
        break;
      } catch (error) {
        lastError = error;
        if (attempt >= NEW_API_FETCH_ATTEMPTS || !isRetryableNewApiFetchError(error)) {
          throw error;
        }
        await sleep(NEW_API_FETCH_RETRY_BASE_DELAY_MS * attempt);
      }
    }

    if (!parsed) {
      throw lastError instanceof Error ? lastError : new Error("new-api logs request failed");
    }

    allLogs.push(...parsed.logs);
    total = parsed.total ?? allLogs.length;

    if (parsed.logs.length < params.pageSize || allLogs.length >= total || page >= 100) {
      break;
    }
    page += 1;
  }

  return { logs: allLogs, total };
}

async function fetchNewApiBillingConfig(params: {
  baseUrl: string;
  accessToken: string;
  adminUserId: number;
}): Promise<{
  pricingByModel: Map<string, NewApiPricingItem>;
  groupRatios: Map<string, number>;
  quotaPerUnit: number;
}> {
  const headers = {
    Authorization: params.accessToken,
    "New-Api-User": String(params.adminUserId),
  };
  const pricingResponse = await fetch(buildNewApiPricingUrl(params.baseUrl), {
    method: "GET",
    cache: "no-store",
    headers,
  });
  await assertNewApiOk(pricingResponse, "价格");

  const statusResponse = await fetch(buildNewApiStatusUrl(params.baseUrl), {
    method: "GET",
    cache: "no-store",
    headers,
  });
  await assertNewApiOk(statusResponse, "状态");

  const pricing = parseNewApiPricingPayload(await pricingResponse.json());
  const quotaPerUnit = parseNewApiQuotaPerUnit(await statusResponse.json());

  return {
    pricingByModel: new Map(
      pricing.pricing
        .filter((item) => item.model_name)
        .map((item) => [String(item.model_name), item])
    ),
    groupRatios: pricing.groupRatios,
    quotaPerUnit,
  };
}

function calculateNewApiUsageDetail(params: {
  log: NewApiConsumeLog;
  pricing: NewApiPricingItem | undefined;
  groupRatio: number;
  quotaPerUnit: number;
}): NewApiUsageDetailRow {
  const log = params.log;
  const quota = toFiniteNumber(log.quota);
  const modelRatio = toFiniteNumber(params.pricing?.model_ratio, 0);
  const modelPrice = toFiniteNumber(params.pricing?.model_price, 0);
  const completionRatio = toFiniteNumber(params.pricing?.completion_ratio, 1);
  const quotaType = Number(params.pricing?.quota_type ?? 0);
  const inputTokens = toFiniteNumber(log.prompt_tokens);
  const outputTokens = toFiniteNumber(log.completion_tokens);
  const calculatedQuota =
    quotaType === 1
      ? modelPrice * params.quotaPerUnit * params.groupRatio
      : ((inputTokens + outputTokens * completionRatio) * modelRatio * params.groupRatio) / 500000;
  const billableQuota = quota > 0 ? quota : calculatedQuota;

  return {
    id: typeof log.id === "number" ? log.id : null,
    createdAt: typeof log.created_at === "number" ? log.created_at : null,
    username: normalizeName(log.username),
    group: normalizeName(log.group),
    modelName: normalizeName(log.model_name),
    tokenName: normalizeName(log.token_name),
    channelName: normalizeName(log.channel_name),
    inputTokens,
    outputTokens,
    quota: billableQuota,
    modelRatio: quotaType === 1 ? modelPrice : modelRatio,
    completionRatio,
    groupRatio: params.groupRatio,
    quotaType,
    quotaPerUnit: params.quotaPerUnit,
    revenueUsd: billableQuota / params.quotaPerUnit,
    requestId: log.request_id ?? log.upstream_request_id ?? "",
  };
}

export async function fetchNewApiRevenue(
  input: { poll?: boolean; days?: number } = {}
): Promise<ActionResult<NewApiRevenueResult>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const validatedInput = NewApiRevenueSchema.parse(input);
    const range = await resolveTimeRange(validatedInput.days);
    const startTimestamp = Math.floor(range.startTime.getTime() / 1000);
    const endTimestamp = Math.max(startTimestamp, Math.floor(range.endTime.getTime() / 1000) - 1);
    const todayStartTimestamp = Math.floor(range.todayStart.getTime() / 1000);
    const config = await findAccountingNewApiConfig();

    if (!config) {
      return { ok: false, error: "Please configure new-api first" };
    }

    const billingConfig = await fetchNewApiBillingConfig({
      baseUrl: config.baseUrl,
      accessToken: config.accessToken,
      adminUserId: config.adminUserId,
    });
    const { logs } = await fetchAllNewApiLogs({
      baseUrl: config.baseUrl,
      accessToken: config.accessToken,
      adminUserId: config.adminUserId,
      pageSize: config.pageSize,
      startTimestamp,
      endTimestamp,
    });

    const details = logs.map((log) => {
      const modelName = normalizeName(log.model_name);
      const group = normalizeName(log.group);
      return calculateNewApiUsageDetail({
        log,
        pricing: billingConfig.pricingByModel.get(modelName),
        groupRatio: billingConfig.groupRatios.get(group) ?? 1,
        quotaPerUnit: billingConfig.quotaPerUnit,
      });
    });

    const rowByKey = new Map<string, NewApiRevenueRow>();
    let todayLogCount = 0;
    for (const detail of details) {
      if (detail.createdAt !== null && detail.createdAt < todayStartTimestamp) {
        continue;
      }
      todayLogCount += 1;
      const multiplier = detail.modelRatio * detail.groupRatio;
      const key = `${detail.username}\u0000${detail.group}\u0000${detail.modelName}\u0000${multiplier}`;
      const current =
        rowByKey.get(key) ??
        ({
          username: detail.username,
          group: detail.group,
          modelName: detail.modelName,
          quota: 0,
          inputTokens: 0,
          outputTokens: 0,
          multiplier,
          modelRatio: detail.modelRatio,
          completionRatio: detail.completionRatio,
          groupRatio: detail.groupRatio,
          revenueUsd: 0,
          requestCount: 0,
        } satisfies NewApiRevenueRow);
      current.quota += detail.quota;
      current.inputTokens += detail.inputTokens;
      current.outputTokens += detail.outputTokens;
      current.revenueUsd += detail.revenueUsd;
      current.requestCount += 1;
      rowByKey.set(key, current);
    }

    const rows = Array.from(rowByKey.values()).sort(
      (a, b) => b.revenueUsd - a.revenueUsd || a.username.localeCompare(b.username)
    );

    return {
      ok: true,
      data: {
        startTimestamp,
        endTimestamp,
        timezone: range.timezone,
        totalLogs: todayLogCount,
        quotaPerUnit: billingConfig.quotaPerUnit,
        rows,
        details,
      },
    };
  } catch (error) {
    logger.error("[Accounting] Failed to fetch new-api revenue", error);
    const message = error instanceof Error ? error.message : "Failed to fetch new-api revenue";
    return { ok: false, error: message };
  }
}
