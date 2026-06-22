import "server-only";

import { and, eq, inArray, isNull, or, sql } from "drizzle-orm";
import { db } from "@/drizzle/db";
import { providerUpstreamRateSyncConfigs } from "@/drizzle/schema";
import type { UpstreamRateSource } from "@/lib/upstream-rate-sync/clients";

export type UpstreamRateSyncConfig = typeof providerUpstreamRateSyncConfigs.$inferSelect;

export interface UpstreamRateSyncConfigInput {
  source: UpstreamRateSource;
  isEnabled?: boolean;
  baseUrl?: string | null;
  apiKey?: string | null;
  keyName?: string | null;
  accessToken?: string | null;
  refreshToken?: string | null;
  tokenExpiresAt?: number | null;
  cookie?: string | null;
  userId?: string | null;
  syncIntervalMinutes?: number;
}

export interface UpstreamRateSyncSuccessUpdate {
  syncedAt: Date;
  rate: number;
  error: null;
  upstreamGroupName?: string | null;
  accessToken?: string;
  refreshToken?: string;
  tokenExpiresAt?: number;
  cookie?: string;
}

export interface UpstreamRateSyncFailureUpdate {
  syncedAt: Date;
  error: string;
}

export interface UpstreamRateSyncAuthShareInput {
  source: UpstreamRateSource;
  baseUrl: string;
  accessToken?: string | null;
  refreshToken?: string | null;
  tokenExpiresAt?: number | null;
  cookie?: string | null;
  userId?: string | null;
}

export async function findProviderUpstreamRateSyncConfig(
  providerId: number
): Promise<UpstreamRateSyncConfig | null> {
  const [config] = await db
    .select()
    .from(providerUpstreamRateSyncConfigs)
    .where(eq(providerUpstreamRateSyncConfigs.providerId, providerId))
    .limit(1);
  return config ?? null;
}

export async function findProviderUpstreamRateSyncConfigsByProviderIds(
  providerIds: number[]
): Promise<Map<number, UpstreamRateSyncConfig>> {
  if (providerIds.length === 0) return new Map();
  const configs = await db
    .select()
    .from(providerUpstreamRateSyncConfigs)
    .where(inArray(providerUpstreamRateSyncConfigs.providerId, providerIds));
  return new Map(configs.map((config) => [config.providerId, config]));
}

export async function upsertProviderUpstreamRateSyncConfig(
  providerId: number,
  input: UpstreamRateSyncConfigInput
): Promise<UpstreamRateSyncConfig> {
  const now = new Date();
  const values = normalizeConfigInput(providerId, input, now);
  const [config] = await db
    .insert(providerUpstreamRateSyncConfigs)
    .values(values)
    .onConflictDoUpdate({
      target: providerUpstreamRateSyncConfigs.providerId,
      set: {
        source: values.source,
        isEnabled: values.isEnabled,
        baseUrl: values.baseUrl,
        apiKey: values.apiKey,
        keyName: values.keyName,
        accessToken: values.accessToken,
        refreshToken: values.refreshToken,
        tokenExpiresAt: values.tokenExpiresAt,
        cookie: values.cookie,
        userId: values.userId,
        syncIntervalMinutes: values.syncIntervalMinutes,
        updatedAt: now,
      },
    })
    .returning();

  return config;
}

export async function deleteProviderUpstreamRateSyncConfig(providerId: number): Promise<boolean> {
  const deleted = await db
    .delete(providerUpstreamRateSyncConfigs)
    .where(eq(providerUpstreamRateSyncConfigs.providerId, providerId))
    .returning({ id: providerUpstreamRateSyncConfigs.id });
  return deleted.length > 0;
}

export async function setProviderUpstreamRateSyncEnabledByProviderIds(
  providerIds: number[],
  isEnabled: boolean
): Promise<number> {
  if (providerIds.length === 0) return 0;
  const updated = await db
    .update(providerUpstreamRateSyncConfigs)
    .set({ isEnabled, updatedAt: new Date() })
    .where(inArray(providerUpstreamRateSyncConfigs.providerId, providerIds))
    .returning({ id: providerUpstreamRateSyncConfigs.id });
  return updated.length;
}

export async function findDueProviderUpstreamRateSyncConfigs(
  now: Date,
  limit = 25
): Promise<UpstreamRateSyncConfig[]> {
  return db
    .select()
    .from(providerUpstreamRateSyncConfigs)
    .where(
      and(
        eq(providerUpstreamRateSyncConfigs.isEnabled, true),
        or(
          isNull(providerUpstreamRateSyncConfigs.lastSyncedAt),
          sql`${providerUpstreamRateSyncConfigs.lastSyncedAt} +
            (${providerUpstreamRateSyncConfigs.syncIntervalMinutes} * interval '1 minute')
            <= CAST(${now.toISOString()} AS timestamptz)`
        )
      )
    )
    .limit(limit);
}

export async function recordUpstreamRateSyncSuccess(
  configId: number,
  update: UpstreamRateSyncSuccessUpdate
): Promise<void> {
  await db
    .update(providerUpstreamRateSyncConfigs)
    .set({
      lastSyncedAt: update.syncedAt,
      lastSyncOk: true,
      lastSyncRate: update.rate.toString(),
      lastSyncError: null,
      ...(update.upstreamGroupName !== undefined
        ? { lastUpstreamGroupName: nullableTrim(update.upstreamGroupName) }
        : {}),
      ...(update.accessToken !== undefined ? { accessToken: update.accessToken } : {}),
      ...(update.refreshToken !== undefined ? { refreshToken: update.refreshToken } : {}),
      ...(update.tokenExpiresAt !== undefined ? { tokenExpiresAt: update.tokenExpiresAt } : {}),
      ...(update.cookie !== undefined ? { cookie: update.cookie } : {}),
      updatedAt: update.syncedAt,
    })
    .where(eq(providerUpstreamRateSyncConfigs.id, configId));
}

export async function recordUpstreamRateSyncFailure(
  configId: number,
  update: UpstreamRateSyncFailureUpdate
): Promise<void> {
  await db
    .update(providerUpstreamRateSyncConfigs)
    .set({
      lastSyncedAt: update.syncedAt,
      lastSyncOk: false,
      lastSyncError: update.error,
      updatedAt: update.syncedAt,
    })
    .where(eq(providerUpstreamRateSyncConfigs.id, configId));
}

export async function shareUpstreamRateSyncAuthBySourceAndBaseUrl(
  input: UpstreamRateSyncAuthShareInput,
  excludeConfigId?: number
): Promise<number> {
  const baseUrl = normalizeComparableBaseUrl(input.baseUrl);
  if (!baseUrl) return 0;

  const setValues: Partial<typeof providerUpstreamRateSyncConfigs.$inferInsert> = {
    updatedAt: new Date(),
  };
  if (input.accessToken !== undefined) {
    setValues.accessToken = nullableTrim(input.accessToken);
  }
  if (input.refreshToken !== undefined) {
    setValues.refreshToken = nullableTrim(input.refreshToken);
  }
  if (input.tokenExpiresAt !== undefined) {
    setValues.tokenExpiresAt = input.tokenExpiresAt ?? null;
  }
  if (input.cookie !== undefined) {
    setValues.cookie = nullableTrim(input.cookie);
  }
  if (input.userId !== undefined) {
    setValues.userId = nullableTrim(input.userId);
  }

  const sameSiteCondition = and(
    eq(providerUpstreamRateSyncConfigs.source, input.source),
    sql`regexp_replace(trim(${providerUpstreamRateSyncConfigs.baseUrl}), '/+$', '') = ${baseUrl}`,
    excludeConfigId == null
      ? undefined
      : sql`${providerUpstreamRateSyncConfigs.id} <> ${excludeConfigId}`
  );

  const updated = await db
    .update(providerUpstreamRateSyncConfigs)
    .set(setValues)
    .where(sameSiteCondition)
    .returning({ id: providerUpstreamRateSyncConfigs.id });

  return updated.length;
}

function normalizeConfigInput(
  providerId: number,
  input: UpstreamRateSyncConfigInput,
  now: Date
): typeof providerUpstreamRateSyncConfigs.$inferInsert {
  return {
    providerId,
    source: input.source,
    isEnabled: input.isEnabled ?? true,
    // Deprecated compatibility fields. Runtime sync derives these from the
    // current provider row so provider edits cannot leave stale sync targets.
    baseUrl: nullableTrim(input.baseUrl) ?? "",
    apiKey: nullableTrim(input.apiKey),
    keyName: nullableTrim(input.keyName),
    accessToken: nullableTrim(input.accessToken),
    refreshToken: nullableTrim(input.refreshToken),
    tokenExpiresAt: input.tokenExpiresAt ?? null,
    cookie: nullableTrim(input.cookie),
    userId: nullableTrim(input.userId),
    syncIntervalMinutes: input.syncIntervalMinutes ?? 60,
    createdAt: now,
    updatedAt: now,
  };
}

function nullableTrim(value: string | null | undefined): string | null {
  const trimmed = String(value ?? "").trim();
  return trimmed ? trimmed : null;
}

function normalizeComparableBaseUrl(value: string | null | undefined): string {
  return String(value ?? "")
    .trim()
    .replace(/\/+$/, "");
}
