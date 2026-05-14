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
});

const SaveNewApiConfigSchema = z.object({
  baseUrl: z.string().trim().url(),
  accessToken: z.string().trim().optional(),
  userId: z.coerce.number().int().positive(),
  pageSize: z.coerce.number().int().min(20).max(500).optional(),
});

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

async function resolveTodayRange(): Promise<{ startTime: Date; endTime: Date; timezone: string }> {
  const timezone = await resolveSystemTimezone();
  const today = formatInTimeZone(new Date(), timezone, "yyyy-MM-dd");
  const tomorrow = addIsoDays(today, 1);
  return {
    startTime: fromZonedTime(`${today}T00:00:00`, timezone),
    endTime: fromZonedTime(`${tomorrow}T00:00:00`, timezone),
    timezone,
  };
}

export async function getAccountingSummary(): Promise<ActionResult<AccountingSummary>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const [settings, range] = await Promise.all([getSystemSettings(), resolveTodayRange()]);
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
  let page = 1;
  let total = 0;

  for (;;) {
    const url = buildNewApiLogsUrl(params.baseUrl, {
      startTimestamp: params.startTimestamp,
      endTimestamp: params.endTimestamp,
      page,
      pageSize: params.pageSize,
    });

    const response = await fetch(url, {
      method: "GET",
      cache: "no-store",
      headers: {
        Authorization: params.accessToken,
        "New-Api-User": String(params.adminUserId),
      },
    });

    await assertNewApiOk(response, "日志");

    const parsed = parseNewApiLogsPayload(await response.json());
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
  input: { poll?: boolean } = {}
): Promise<ActionResult<NewApiRevenueResult>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    NewApiRevenueSchema.parse(input);
    const range = await resolveTodayRange();
    const startTimestamp = Math.floor(range.startTime.getTime() / 1000);
    const endTimestamp = Math.max(startTimestamp, Math.floor(range.endTime.getTime() / 1000) - 1);
    const config = await findAccountingNewApiConfig();

    if (!config) {
      return { ok: false, error: "Please configure new-api first" };
    }

    const billingConfig = await fetchNewApiBillingConfig({
      baseUrl: config.baseUrl,
      accessToken: config.accessToken,
      adminUserId: config.adminUserId,
    });
    const { logs, total } = await fetchAllNewApiLogs({
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
    for (const detail of details) {
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
        totalLogs: total,
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
