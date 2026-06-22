import "server-only";

import {
  fetchNewApiProviderRate,
  fetchSub2ApiProviderRate,
  type NewApiRateConfig,
  type Sub2ApiRateConfig,
  type UpstreamRateResult,
} from "@/lib/upstream-rate-sync/clients";
import { findProviderById, updateProviderCostMultiplier } from "@/repository/provider";
import {
  findProviderUpstreamRateSyncConfig,
  recordUpstreamRateSyncFailure,
  recordUpstreamRateSyncSuccess,
  shareUpstreamRateSyncAuthBySourceAndBaseUrl,
  type UpstreamRateSyncConfig,
  type UpstreamRateSyncAuthShareInput,
  type UpstreamRateSyncFailureUpdate,
  type UpstreamRateSyncSuccessUpdate,
} from "@/repository/upstream-rate-sync";
import type { Provider } from "@/types/provider";

export type UpstreamRateSyncServiceResult =
  | {
      ok: true;
      configId: number;
      providerId: number;
      rate: number;
      refreshed: boolean;
      upstreamGroupName: string | null;
    }
  | {
      ok: false;
      configId: number;
      providerId: number;
      error: string;
    };

interface UpstreamRateSyncClients {
  fetchSub2ApiProviderRate: typeof fetchSub2ApiProviderRate;
  fetchNewApiProviderRate: typeof fetchNewApiProviderRate;
}

interface UpstreamRateSyncRepository {
  updateProviderCostMultiplier: (providerId: number, costMultiplier: number) => Promise<boolean>;
  recordSyncSuccess: (configId: number, update: UpstreamRateSyncSuccessUpdate) => Promise<void>;
  recordSyncFailure: (configId: number, update: UpstreamRateSyncFailureUpdate) => Promise<void>;
  shareAuthBySourceAndBaseUrl: (
    input: UpstreamRateSyncAuthShareInput,
    excludeConfigId?: number
  ) => Promise<number>;
}

export interface SyncProviderUpstreamRateOptions {
  clients?: UpstreamRateSyncClients;
  repository?: UpstreamRateSyncRepository;
  provider?: Provider;
  now?: () => Date;
}

const defaultClients: UpstreamRateSyncClients = {
  fetchSub2ApiProviderRate,
  fetchNewApiProviderRate,
};

const defaultRepository: UpstreamRateSyncRepository = {
  updateProviderCostMultiplier,
  recordSyncSuccess: recordUpstreamRateSyncSuccess,
  recordSyncFailure: recordUpstreamRateSyncFailure,
  shareAuthBySourceAndBaseUrl: shareUpstreamRateSyncAuthBySourceAndBaseUrl,
};

export async function syncProviderUpstreamRateByProviderId(
  providerId: number,
  options: SyncProviderUpstreamRateOptions = {}
): Promise<UpstreamRateSyncServiceResult | null> {
  const config = await findProviderUpstreamRateSyncConfig(providerId);
  if (!config) {
    return null;
  }
  return syncProviderUpstreamRate(config, options);
}

export async function syncProviderUpstreamRate(
  config: UpstreamRateSyncConfig,
  options: SyncProviderUpstreamRateOptions = {}
): Promise<UpstreamRateSyncServiceResult> {
  const now = options.now?.() ?? new Date();
  const clients = options.clients ?? defaultClients;
  const repository = options.repository ?? defaultRepository;
  const provider = options.provider ?? (await findProviderById(config.providerId));

  if (!config.isEnabled) {
    return {
      ok: false,
      configId: config.id,
      providerId: config.providerId,
      error: "Upstream rate sync is disabled",
    };
  }
  if (!provider) {
    return {
      ok: false,
      configId: config.id,
      providerId: config.providerId,
      error: "Provider was not found",
    };
  }

  try {
    const target = resolveConfiguredSyncTarget(config, provider);
    const upstream = await fetchRate(config, target, clients);
    const updated = await repository.updateProviderCostMultiplier(config.providerId, upstream.rate);
    if (!updated) {
      throw new Error("Provider was not found");
    }

    await repository.recordSyncSuccess(config.id, {
      syncedAt: now,
      rate: upstream.rate,
      error: null,
      upstreamGroupName: upstream.group?.name ?? null,
      ...(upstream.auth?.accessToken ? { accessToken: upstream.auth.accessToken } : {}),
      ...(upstream.auth?.refreshToken ? { refreshToken: upstream.auth.refreshToken } : {}),
      ...(upstream.auth?.tokenExpiresAt != null
        ? { tokenExpiresAt: upstream.auth.tokenExpiresAt }
        : {}),
    });
    if (upstream.auth) {
      await repository.shareAuthBySourceAndBaseUrl(
        {
          source: config.source,
          baseUrl: target.baseUrl,
          accessToken: upstream.auth.accessToken,
          refreshToken: upstream.auth.refreshToken,
          tokenExpiresAt: upstream.auth.tokenExpiresAt,
        },
        config.id
      );
    }

    return {
      ok: true,
      configId: config.id,
      providerId: config.providerId,
      rate: upstream.rate,
      refreshed: upstream.refreshed,
      upstreamGroupName: upstream.group?.name ?? null,
    };
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    await repository.recordSyncFailure(config.id, {
      syncedAt: now,
      error: message,
    });
    return {
      ok: false,
      configId: config.id,
      providerId: config.providerId,
      error: message,
    };
  }
}

async function fetchRate(
  config: UpstreamRateSyncConfig,
  target: ProviderSyncTarget,
  clients: UpstreamRateSyncClients
): Promise<UpstreamRateResult> {
  if (config.source === "sub2api") {
    return clients.fetchSub2ApiProviderRate(toSub2ApiConfig(config, target));
  }
  return clients.fetchNewApiProviderRate(toNewApiConfig(config, target));
}

interface ProviderSyncTarget {
  baseUrl: string;
  apiKey: string;
  keyName: string;
}

function resolveProviderSyncTarget(provider: Provider): ProviderSyncTarget {
  return {
    baseUrl: resolveUpstreamSiteBaseUrl(provider.url),
    apiKey: provider.key,
    keyName: provider.name,
  };
}

function resolveConfiguredSyncTarget(
  config: UpstreamRateSyncConfig,
  provider: Provider
): ProviderSyncTarget {
  const fallback = resolveProviderSyncTarget(provider);
  return {
    baseUrl: nullableString(config.baseUrl) ?? fallback.baseUrl,
    apiKey: nullableString(config.apiKey) ?? fallback.apiKey,
    keyName: nullableString(config.keyName) ?? fallback.keyName,
  };
}

function resolveUpstreamSiteBaseUrl(providerUrl: string): string {
  const trimmed = providerUrl.trim();
  if (!trimmed) {
    throw new Error("Provider URL is required");
  }
  const url = new URL(trimmed);
  return url.origin;
}

function toSub2ApiConfig(
  config: UpstreamRateSyncConfig,
  target: ProviderSyncTarget
): Sub2ApiRateConfig {
  return {
    baseUrl: target.baseUrl,
    apiKey: target.apiKey,
    keyName: target.keyName,
    accessToken: config.accessToken,
    refreshToken: config.refreshToken,
    tokenExpiresAt: config.tokenExpiresAt,
  };
}

function toNewApiConfig(
  config: UpstreamRateSyncConfig,
  target: ProviderSyncTarget
): NewApiRateConfig {
  return {
    baseUrl: target.baseUrl,
    apiKey: target.apiKey,
    keyName: target.keyName,
    accessToken: config.accessToken,
    cookie: config.cookie,
    userId: config.userId,
  };
}

function nullableString(value: string | null | undefined): string | null {
  const trimmed = String(value ?? "").trim();
  return trimmed ? trimmed : null;
}
