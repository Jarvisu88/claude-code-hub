import "server-only";

import { eq, isNull, sql } from "drizzle-orm";
import { db } from "@/drizzle/db";
import {
  accountingNewApiConfigs,
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
  missingSellMultiplierCount: number;
}

export interface ModelSellMultiplierRow {
  id: number;
  modelName: string;
  multiplier: number;
  note: string | null;
  createdAt: Date | null;
  updatedAt: Date | null;
}

export interface AccountingNewApiConfigRow {
  id: number;
  baseUrl: string;
  accessToken: string;
  adminUserId: number;
  pageSize: number;
  createdAt: Date | null;
  updatedAt: Date | null;
}

export interface AccountingNewApiConfigPreview {
  configured: boolean;
  baseUrl: string;
  adminUserId: number | null;
  pageSize: number;
}

function toNumber(value: unknown): number {
  if (typeof value === "number") return Number.isFinite(value) ? value : 0;
  if (typeof value === "string") {
    const parsed = Number.parseFloat(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

function toConfig(row: typeof accountingNewApiConfigs.$inferSelect): AccountingNewApiConfigRow {
  return {
    id: row.id,
    baseUrl: row.baseUrl,
    accessToken: row.accessToken,
    adminUserId: row.adminUserId,
    pageSize: row.pageSize,
    createdAt: row.createdAt,
    updatedAt: row.updatedAt,
  };
}

export async function findAccountingNewApiConfig(): Promise<AccountingNewApiConfigRow | null> {
  const [row] = await db
    .select()
    .from(accountingNewApiConfigs)
    .where(eq(accountingNewApiConfigs.name, "default"))
    .limit(1);

  return row ? toConfig(row) : null;
}

export async function findAccountingNewApiConfigPreview(): Promise<AccountingNewApiConfigPreview> {
  const row = await findAccountingNewApiConfig();
  return {
    configured: Boolean(row),
    baseUrl: row?.baseUrl ?? "",
    adminUserId: row?.adminUserId ?? null,
    pageSize: row?.pageSize ?? 100,
  };
}

export async function upsertAccountingNewApiConfig(input: {
  baseUrl: string;
  accessToken?: string | null;
  adminUserId: number;
  pageSize?: number;
}): Promise<AccountingNewApiConfigPreview> {
  const existing = await findAccountingNewApiConfig();
  const accessToken = input.accessToken?.trim() || existing?.accessToken;
  if (!accessToken) {
    throw new Error("new-api access token is required");
  }

  const pageSize = Math.min(Math.max(input.pageSize ?? existing?.pageSize ?? 100, 20), 500);
  await db
    .insert(accountingNewApiConfigs)
    .values({
      name: "default",
      baseUrl: input.baseUrl,
      accessToken,
      adminUserId: input.adminUserId,
      pageSize,
      updatedAt: new Date(),
    })
    .onConflictDoUpdate({
      target: accountingNewApiConfigs.name,
      set: {
        baseUrl: input.baseUrl,
        accessToken,
        adminUserId: input.adminUserId,
        pageSize,
        updatedAt: new Date(),
      },
    });

  return findAccountingNewApiConfigPreview();
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

export async function findProviderProfitSummary(params: {
  startTime: Date;
  endTime: Date;
  globalSellMultiplier: number;
}): Promise<ProviderProfitSummaryRow[]> {
  const globalSellMultiplier = Number.isFinite(params.globalSellMultiplier)
    ? Math.max(params.globalSellMultiplier, 0)
    : 1;
  const startTime = params.startTime.toISOString();
  const endTime = params.endTime.toISOString();

  const rows = await db.execute(sql`
    WITH ledger_base AS (
      SELECT
        ${usageLedger.finalProviderId} AS provider_id,
        ${providers.name} AS provider_name,
        COALESCE(NULLIF(${usageLedger.model}, ''), NULLIF(${usageLedger.originalModel}, ''), '') AS model_name,
        COALESCE(${usageLedger.costUsd}, 0)::numeric AS charged_cost_usd,
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
        model_name,
        provider_multiplier * group_multiplier AS revenue_multiplier,
        CASE
          WHEN provider_multiplier * group_multiplier > 0
            THEN charged_cost_usd / (provider_multiplier * group_multiplier)
          ELSE 0
        END AS base_cost_usd,
        CASE
          WHEN provider_multiplier * group_multiplier > 0
            THEN (charged_cost_usd / (provider_multiplier * group_multiplier)) * provider_multiplier
          ELSE 0
        END AS supplier_cost_usd,
        CASE
          WHEN provider_multiplier * group_multiplier > 0
            THEN charged_cost_usd
          ELSE 0
        END AS estimated_revenue_usd
      FROM ledger_base
    )
    SELECT
      provider_id AS "providerId",
      provider_name AS "providerName",
      COUNT(*)::int AS "requestCount",
      COUNT(DISTINCT NULLIF(model_name, ''))::int AS "modelCount",
      COALESCE(SUM(base_cost_usd), 0)::text AS "baseCostUsd",
      COALESCE(SUM(supplier_cost_usd), 0)::text AS "supplierCostUsd",
      COALESCE(SUM(estimated_revenue_usd), 0)::text AS "estimatedRevenueUsd",
      COALESCE(SUM(estimated_revenue_usd - supplier_cost_usd), 0)::text AS "estimatedProfitUsd",
      COALESCE(
        SUM(base_cost_usd * revenue_multiplier) / NULLIF(SUM(base_cost_usd), 0),
        0
      )::text AS "revenueMultiplier",
      0::int AS "missingSellMultiplierCount"
    FROM normalized
    GROUP BY provider_id, provider_name
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
    missingSellMultiplierCount: Number(row.missingSellMultiplierCount ?? 0),
  }));
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
