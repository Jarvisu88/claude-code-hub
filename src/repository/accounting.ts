import "server-only";

import { eq, isNull, sql } from "drizzle-orm";
import { db } from "@/drizzle/db";
import {
  modelSellMultipliers,
  providerGroups,
  providers,
  usageLedger,
  users,
} from "@/drizzle/schema";
import { parseProviderGroups } from "@/lib/utils/provider-group";
import { LEDGER_BILLING_CONDITION } from "./_shared/ledger-conditions";

export interface ProviderProfitSummaryRow {
  providerId: number;
  providerName: string;
  requestCount: number;
  modelCount: number;
  baseCostUsd: number;
  supplierCostUsd: number;
  estimatedRevenueUsd: number;
  estimatedProfitUsd: number;
  revenueMultiplier: number;
  sellMultiplier: number | null;
}

export interface ModelSellMultiplierRow {
  id: number;
  modelName: string;
  multiplier: number;
  note: string | null;
  createdAt: Date | null;
  updatedAt: Date | null;
}

function toNumber(value: unknown): number {
  if (typeof value === "number") return Number.isFinite(value) ? value : 0;
  if (typeof value === "string") {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  if (value !== null && value !== undefined) {
    console.warn("[Accounting] Unexpected value type in toNumber:", typeof value, value);
  }
  return 0;
}

export async function findAccountingGroupMultiplierMap(): Promise<Map<string, number>> {
  const rows = await db
    .select({
      name: providerGroups.name,
      costMultiplier: providerGroups.costMultiplier,
    })
    .from(providerGroups);

  return new Map(rows.map((row) => [row.name, toNumber(row.costMultiplier)]));
}

export async function findAccountingUserGroupMap(): Promise<Map<string, string>> {
  const rows = await db
    .select({
      name: users.name,
      providerGroup: users.providerGroup,
    })
    .from(users)
    .where(isNull(users.deletedAt));

  const result = new Map<string, string>();
  for (const row of rows) {
    if (!row.name) continue;
    const groups = parseProviderGroups(row.providerGroup);
    result.set(row.name, groups[0] ?? "default");
  }
  return result;
}

export interface ProviderSellConfig {
  providerId: number;
  providerName: string;
  sellMultiplier: number | null;
}

export async function findAllProviderSellConfigs(): Promise<ProviderSellConfig[]> {
  const rows = await db
    .select({
      id: providers.id,
      name: providers.name,
      sellMultiplier: providers.sellMultiplier,
    })
    .from(providers)
    .where(isNull(providers.deletedAt))
    .orderBy(providers.name);

  return rows.map((row) => ({
    providerId: row.id,
    providerName: row.name,
    sellMultiplier: row.sellMultiplier ? toNumber(row.sellMultiplier) : null,
  }));
}

export async function findProviderProfitSummary(params: {
  startTime: Date;
  endTime: Date;
  globalSellMultiplier: number;
}): Promise<ProviderProfitSummaryRow[]> {
  const globalSellMultiplier = Number.isFinite(params.globalSellMultiplier)
    ? params.globalSellMultiplier > 0
      ? params.globalSellMultiplier
      : 1
    : 1;
  const startTime = params.startTime.toISOString();
  const endTime = params.endTime.toISOString();

  const rows = await db.execute(sql`
    WITH ledger_base AS (
      SELECT
        ${usageLedger.finalProviderId} AS provider_id,
        ${providers.name} AS provider_name,
        ${providers.sellMultiplier}::numeric AS sell_multiplier,
        COALESCE(NULLIF(${usageLedger.model}, ''), NULLIF(${usageLedger.originalModel}, ''), '') AS model_name,
        COALESCE(${usageLedger.costUsd}, 0)::numeric AS cost_usd,
        COALESCE(${usageLedger.costMultiplier}, ${providers.costMultiplier}, '1')::numeric AS provider_multiplier,
        COALESCE(${usageLedger.groupCostMultiplier}, ${globalSellMultiplier})::numeric AS group_multiplier
      FROM ${usageLedger}
      INNER JOIN ${providers} ON ${providers.id} = ${usageLedger.finalProviderId}
      WHERE
        ${usageLedger.createdAt} >= ${startTime}::timestamptz
        AND ${usageLedger.createdAt} < ${endTime}::timestamptz
        AND ${LEDGER_BILLING_CONDITION}
    ),
    normalized AS (
      SELECT
        provider_id,
        provider_name,
        sell_multiplier,
        model_name,
        cost_usd AS supplier_cost_usd,
        CASE
          WHEN provider_multiplier * group_multiplier > 0
            THEN cost_usd / (provider_multiplier * group_multiplier)
          ELSE 0
        END AS base_cost_usd,
        CASE
          WHEN sell_multiplier IS NOT NULL AND provider_multiplier * group_multiplier > 0
            THEN (cost_usd / (provider_multiplier * group_multiplier)) * sell_multiplier
          ELSE cost_usd
        END AS estimated_revenue_usd
      FROM ledger_base
    )
    SELECT
      provider_id AS "providerId",
      provider_name AS "providerName",
      sell_multiplier::text AS "sellMultiplier",
      COUNT(*)::int AS "requestCount",
      COUNT(DISTINCT NULLIF(model_name, ''))::int AS "modelCount",
      COALESCE(SUM(base_cost_usd), 0)::text AS "baseCostUsd",
      COALESCE(SUM(supplier_cost_usd), 0)::text AS "supplierCostUsd",
      COALESCE(SUM(estimated_revenue_usd), 0)::text AS "estimatedRevenueUsd",
      COALESCE(SUM(estimated_revenue_usd - supplier_cost_usd), 0)::text AS "estimatedProfitUsd",
      COALESCE(sell_multiplier, 0)::text AS "revenueMultiplier"
    FROM normalized
    GROUP BY provider_id, provider_name, sell_multiplier
    ORDER BY COALESCE(SUM(estimated_revenue_usd - supplier_cost_usd), 0) DESC, provider_name ASC
  `);

  return Array.from(rows).map((row) => ({
    providerId: Number(row.providerId),
    providerName: String(row.providerName ?? ""),
    requestCount: Number(row.requestCount ?? 0),
    modelCount: Number(row.modelCount ?? 0),
    baseCostUsd: toNumber(row.baseCostUsd),
    supplierCostUsd: toNumber(row.supplierCostUsd),
    estimatedRevenueUsd: toNumber(row.estimatedRevenueUsd),
    estimatedProfitUsd: toNumber(row.estimatedProfitUsd),
    revenueMultiplier: toNumber(row.revenueMultiplier),
    sellMultiplier: row.sellMultiplier ? toNumber(row.sellMultiplier) : null,
  }));
}

export async function updateProviderSellMultiplier(
  providerId: number,
  sellMultiplier: number | null
): Promise<void> {
  await db
    .update(providers)
    .set({
      sellMultiplier: sellMultiplier !== null ? String(sellMultiplier) : null,
      updatedAt: new Date(),
    })
    .where(eq(providers.id, providerId));
}

export async function findModelSellMultipliers(): Promise<ModelSellMultiplierRow[]> {
  const rows = await db
    .select({
      id: modelSellMultipliers.id,
      modelName: modelSellMultipliers.modelName,
      multiplier: modelSellMultipliers.multiplier,
      note: modelSellMultipliers.note,
      createdAt: modelSellMultipliers.createdAt,
      updatedAt: modelSellMultipliers.updatedAt,
    })
    .from(modelSellMultipliers)
    .orderBy(modelSellMultipliers.modelName);

  return rows.map((row) => ({
    ...row,
    multiplier: toNumber(row.multiplier),
  }));
}

export async function upsertModelSellMultiplier(input: {
  modelName: string;
  multiplier: number;
  note?: string | null;
}): Promise<ModelSellMultiplierRow> {
  const [row] = await db
    .insert(modelSellMultipliers)
    .values({
      modelName: input.modelName,
      multiplier: String(input.multiplier),
      note: input.note?.trim() || null,
      updatedAt: new Date(),
    })
    .onConflictDoUpdate({
      target: modelSellMultipliers.modelName,
      set: {
        multiplier: String(input.multiplier),
        note: input.note?.trim() || null,
        updatedAt: new Date(),
      },
    })
    .returning({
      id: modelSellMultipliers.id,
      modelName: modelSellMultipliers.modelName,
      multiplier: modelSellMultipliers.multiplier,
      note: modelSellMultipliers.note,
      createdAt: modelSellMultipliers.createdAt,
      updatedAt: modelSellMultipliers.updatedAt,
    });

  return {
    ...row,
    multiplier: toNumber(row.multiplier),
  };
}

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

export async function findRevenueTimeline(params: {
  startTime: Date;
  endTime: Date;
  globalSellMultiplier: number;
  bucketInterval: "hour" | "day";
}): Promise<RevenueTimelineRow[]> {
  const globalSellMultiplier = Number.isFinite(params.globalSellMultiplier)
    ? params.globalSellMultiplier > 0
      ? params.globalSellMultiplier
      : 1
    : 1;
  const startTime = params.startTime.toISOString();
  const endTime = params.endTime.toISOString();
  const truncInterval = params.bucketInterval === "hour" ? "hour" : "day";

  const rows = await db.execute(sql`
    WITH ledger_base AS (
      SELECT
        date_trunc(${truncInterval}, ${usageLedger.createdAt}) AS time_bucket,
        ${users.name} AS user_name,
        ${users.id} AS user_id,
        ${users.providerGroup} AS provider_group,
        COALESCE(NULLIF(${usageLedger.model}, ''), NULLIF(${usageLedger.originalModel}, ''), 'unknown') AS model_name,
        COALESCE(${usageLedger.costUsd}, 0)::numeric AS charged_cost_usd,
        COALESCE(${usageLedger.costMultiplier}, ${providers.costMultiplier}, '1')::numeric AS provider_multiplier,
        COALESCE(${usageLedger.groupCostMultiplier}, ${globalSellMultiplier})::numeric AS group_multiplier,
        ${providers.sellMultiplier}::numeric AS sell_multiplier,
        COALESCE(${usageLedger.inputTokens}, 0) AS input_tokens,
        COALESCE(${usageLedger.outputTokens}, 0) AS output_tokens
      FROM ${usageLedger}
      INNER JOIN ${users} ON ${users.id} = ${usageLedger.userId}
      INNER JOIN ${providers} ON ${providers.id} = ${usageLedger.finalProviderId}
      WHERE
        ${usageLedger.createdAt} >= ${startTime}::timestamptz
        AND ${usageLedger.createdAt} < ${endTime}::timestamptz
        AND ${LEDGER_BILLING_CONDITION}
    )
    SELECT
      time_bucket,
      user_name,
      user_id,
      provider_group,
      model_name,
      COUNT(*)::int AS request_count,
      SUM(input_tokens)::bigint AS input_tokens,
      SUM(output_tokens)::bigint AS output_tokens,
      SUM(charged_cost_usd)::numeric AS supplier_cost_usd,
      SUM(
        CASE
          WHEN sell_multiplier IS NOT NULL AND provider_multiplier * group_multiplier > 0
            THEN (charged_cost_usd / (provider_multiplier * group_multiplier)) * sell_multiplier
          ELSE charged_cost_usd
        END
      )::numeric AS revenue_usd
    FROM ledger_base
    GROUP BY time_bucket, user_name, user_id, provider_group, model_name
    ORDER BY time_bucket ASC, revenue_usd DESC
  `);

  return rows.map((row: any) => {
    const tb = row.time_bucket;
    const timeBucket = tb instanceof Date ? tb.toISOString() : typeof tb === "string" ? tb : "";
    return {
      timeBucket,
      userName: String(row.user_name || "unknown"),
      userId: Number(row.user_id || 0),
      providerGroup: String(row.provider_group || "default"),
      modelName: String(row.model_name || "unknown"),
      requestCount: Number(row.request_count || 0),
      inputTokens: Number(row.input_tokens || 0),
      outputTokens: Number(row.output_tokens || 0),
      supplierCostUsd: toNumber(row.supplier_cost_usd),
      revenueUsd: toNumber(row.revenue_usd),
    };
  });
}
