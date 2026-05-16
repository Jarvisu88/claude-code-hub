"use server";

import { formatInTimeZone, fromZonedTime } from "date-fns-tz";
import { revalidatePath } from "next/cache";
import { z } from "zod";
import { getSession } from "@/lib/auth";
import { invalidateSystemSettingsCache } from "@/lib/config";
import { logger } from "@/lib/logger";
import { resolveSystemTimezone } from "@/lib/utils/timezone";
import {
  findAllProviderSellConfigs,
  findModelSellMultipliers,
  findProviderProfitSummary,
  findRevenueTimeline,
  type ModelSellMultiplierRow,
  type ProviderProfitSummaryRow,
  type ProviderSellConfig,
  updateProviderSellMultiplier,
  upsertModelSellMultiplier,
} from "@/repository/accounting";
import { getSystemSettings, updateSystemSettings } from "@/repository/system-config";
import type { ActionResult } from "./types";

export interface RevenueTimelineRow {
  timeBucket: string;
  userName: string;
  userId: number;
  providerGroup: string;
  modelName: string;
  requestCount: number;
  inputTokens: number;
  outputTokens: number;
  supplierCostUsd: number;
  revenueUsd: number;
}

export interface AccountingSummary {
  startTime: string;
  endTime: string;
  timezone: string;
  globalSellMultiplier: number;
  providers: ProviderProfitSummaryRow[];
  allProviders: ProviderSellConfig[];
  modelSellMultipliers: ModelSellMultiplierRow[];
}

export interface RevenueTimelineData {
  startTime: string;
  endTime: string;
  timezone: string;
  rows: RevenueTimelineRow[];
}

const SaveGlobalSellMultiplierSchema = z.object({
  multiplier: z.coerce.number().min(0).max(1000000),
});

const UpsertModelSellMultiplierSchema = z.object({
  modelName: z.string().trim().min(1).max(128),
  multiplier: z.coerce.number().min(0).max(1000000),
  note: z.string().trim().max(200).optional(),
});

const SaveProviderSellMultiplierSchema = z.object({
  providerId: z.coerce.number().int().positive(),
  sellMultiplier: z.coerce.number().min(0).max(1000000).nullable(),
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
    const [providers, allProviders, modelSellMultipliers] = await Promise.all([
      findProviderProfitSummary({
        startTime: range.startTime,
        endTime: range.endTime,
        globalSellMultiplier: settings.globalSellMultiplier,
      }),
      findAllProviderSellConfigs(),
      findModelSellMultipliers(),
    ]);

    return {
      ok: true,
      data: {
        startTime: range.startTime.toISOString(),
        endTime: range.endTime.toISOString(),
        timezone: range.timezone,
        globalSellMultiplier: settings.globalSellMultiplier,
        providers,
        allProviders,
        modelSellMultipliers,
      },
    };
  } catch (error) {
    logger.error("[Accounting] Failed to load summary", error);
    return { ok: false, error: "Failed to load accounting summary" };
  }
}

export async function getRevenueTimeline(input: {
  days?: number;
}): Promise<ActionResult<RevenueTimelineData>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const days = Math.min(Math.max(input.days || 1, 1), 30);
    const [settings, range] = await Promise.all([getSystemSettings(), resolveTimeRange(days)]);
    const bucketInterval = days <= 7 ? "hour" : "day";

    const rows = await findRevenueTimeline({
      startTime: range.startTime,
      endTime: range.endTime,
      globalSellMultiplier: settings.globalSellMultiplier,
      bucketInterval,
    });

    return {
      ok: true,
      data: {
        startTime: range.startTime.toISOString(),
        endTime: range.endTime.toISOString(),
        timezone: range.timezone,
        rows,
      },
    };
  } catch (error) {
    logger.error("[Accounting] Failed to load revenue timeline", error);
    return { ok: false, error: "Failed to load revenue timeline" };
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

export async function saveProviderSellMultiplier(input: {
  providerId: number;
  sellMultiplier: number | null;
}): Promise<ActionResult<{ providerId: number; sellMultiplier: number | null }>> {
  try {
    const session = await requireAdmin();
    if (!session) {
      return { ok: false, error: "Unauthorized" };
    }

    const validated = SaveProviderSellMultiplierSchema.parse(input);
    await updateProviderSellMultiplier(validated.providerId, validated.sellMultiplier);
    revalidatePath("/dashboard/accounting");
    return {
      ok: true,
      data: { providerId: validated.providerId, sellMultiplier: validated.sellMultiplier },
    };
  } catch (error) {
    logger.error("[Accounting] Failed to save provider sell multiplier", error);
    return { ok: false, error: "Failed to save provider sell multiplier" };
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
