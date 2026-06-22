import { beforeEach, describe, expect, it, vi } from "vitest";
import { syncProviderUpstreamRate } from "@/lib/upstream-rate-sync/service";
import type { UpstreamRateSyncConfig } from "@/repository/upstream-rate-sync";
import type { Provider } from "@/types/provider";

function baseConfig(overrides: Partial<UpstreamRateSyncConfig> = {}): UpstreamRateSyncConfig {
  return {
    id: 11,
    providerId: 42,
    source: "newapi",
    isEnabled: true,
    baseUrl: "https://upstream.example",
    apiKey: "sk-target",
    keyName: null,
    accessToken: "access",
    refreshToken: null,
    tokenExpiresAt: null,
    userId: "1001",
    syncIntervalMinutes: 60,
    lastSyncedAt: null,
    lastSyncOk: null,
    lastSyncRate: null,
    lastSyncError: null,
    lastUpstreamGroupName: null,
    createdAt: new Date("2026-01-01T00:00:00.000Z"),
    updatedAt: new Date("2026-01-01T00:00:00.000Z"),
    ...overrides,
  };
}

function provider(overrides: Partial<Provider> = {}): Provider {
  return {
    id: 42,
    name: "configured-provider",
    url: "https://current.example/v1",
    key: "sk-current",
    providerVendorId: null,
    isEnabled: true,
    weight: 100,
    priority: 50,
    groupPriorities: null,
    costMultiplier: 1,
    groupTag: null,
    providerType: "openai-compatible",
    preserveClientIp: false,
    disableSessionReuse: false,
    modelRedirects: null,
    activeTimeStart: null,
    activeTimeEnd: null,
    allowedModels: null,
    allowedClients: [],
    blockedClients: [],
    mcpPassthroughType: "none",
    mcpPassthroughUrl: null,
    limit5hUsd: null,
    limit5hResetMode: "rolling",
    limitDailyUsd: null,
    dailyResetMode: "fixed",
    dailyResetTime: "00:00",
    limitWeeklyUsd: null,
    limitMonthlyUsd: null,
    limitTotalUsd: null,
    totalCostResetAt: null,
    limitConcurrentSessions: 0,
    maxRetryAttempts: null,
    circuitBreakerFailureThreshold: 5,
    circuitBreakerOpenDuration: 1_800_000,
    circuitBreakerHalfOpenSuccessThreshold: 2,
    proxyUrl: null,
    proxyFallbackToDirect: false,
    customHeaders: null,
    firstByteTimeoutStreamingMs: 30_000,
    streamingIdleTimeoutMs: 120_000,
    requestTimeoutNonStreamingMs: 120_000,
    websiteUrl: null,
    faviconUrl: null,
    cacheTtlPreference: null,
    swapCacheTtlBilling: false,
    context1mPreference: null,
    codexReasoningEffortPreference: null,
    codexReasoningSummaryPreference: null,
    codexTextVerbosityPreference: null,
    codexParallelToolCallsPreference: null,
    codexServiceTierPreference: null,
    anthropicMaxTokensPreference: null,
    anthropicThinkingBudgetPreference: null,
    anthropicAdaptiveThinking: null,
    geminiGoogleSearchPreference: null,
    tpm: null,
    rpm: null,
    rpd: null,
    cc: null,
    createdAt: new Date("2026-01-01T00:00:00.000Z"),
    updatedAt: new Date("2026-01-01T00:00:00.000Z"),
    deletedAt: null,
    ...overrides,
  };
}

describe("upstream rate sync service", () => {
  const now = new Date("2026-06-20T10:00:00.000Z");

  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("syncs the upstream rate into the provider cost multiplier", async () => {
    const updateProviderCostMultiplier = vi.fn(async () => true);
    const recordSyncSuccess = vi.fn(async () => undefined);
    const recordSyncFailure = vi.fn(async () => undefined);
    const fetchNewApiProviderRate = vi.fn(async () => ({
      rate: 0.72,
      refreshed: false,
      key: { id: 5, name: "target", keyPrefix: "sk-t...rget", group: "vip" },
      group: { id: null, name: "vip", rate: 0.72, raw: { ratio: 0.72 } },
      raw: {},
    }));

    const result = await syncProviderUpstreamRate(baseConfig(), {
      provider: provider(),
      now: () => now,
      clients: {
        fetchNewApiProviderRate,
        fetchSub2ApiProviderRate: vi.fn(),
      },
      repository: {
        updateProviderCostMultiplier,
        recordSyncSuccess,
        recordSyncFailure,
      },
    });

    expect(result.ok).toBe(true);
    expect(updateProviderCostMultiplier).toHaveBeenCalledWith(42, 0.72);
    expect(recordSyncSuccess).toHaveBeenCalledWith(
      11,
      expect.objectContaining({
        syncedAt: now,
        rate: 0.72,
        error: null,
      })
    );
  });

  it("uses the configured upstream url and key before provider fallbacks", async () => {
    const fetchNewApiProviderRate = vi.fn(async () => ({
      rate: 0.66,
      refreshed: false,
      key: { id: 5, name: "target", keyPrefix: "curr...rent", group: "fast" },
      group: { id: null, name: "fast", rate: 0.66, raw: { ratio: 0.66 } },
      raw: {},
    }));
    const recordSyncSuccess = vi.fn(async () => undefined);

    const result = await syncProviderUpstreamRate(
      baseConfig({
        baseUrl: "https://stale.example",
        apiKey: "sk-stale",
        keyName: "stale-name",
      }),
      {
        provider: provider(),
        now: () => now,
        clients: {
          fetchNewApiProviderRate,
          fetchSub2ApiProviderRate: vi.fn(),
        },
        repository: {
          updateProviderCostMultiplier: vi.fn(async () => true),
          recordSyncSuccess,
          recordSyncFailure: vi.fn(async () => undefined),
        },
      }
    );

    expect(result.ok).toBe(true);
    expect(fetchNewApiProviderRate).toHaveBeenCalledWith({
      baseUrl: "https://stale.example",
      apiKey: "sk-stale",
      keyName: "stale-name",
      accessToken: "access",
      cookie: undefined,
      userId: "1001",
    });
    expect(recordSyncSuccess).toHaveBeenCalledWith(
      11,
      expect.objectContaining({
        upstreamGroupName: "fast",
      })
    );
  });

  it("persists refreshed Sub2API auth returned by the client", async () => {
    const recordSyncSuccess = vi.fn(async () => undefined);
    const shareAuthBySourceAndBaseUrl = vi.fn(async () => 2);
    const fetchSub2ApiProviderRate = vi.fn(async () => ({
      rate: 1.25,
      refreshed: true,
      auth: {
        accessToken: "access-next",
        refreshToken: "refresh-next",
        tokenExpiresAt: 1_800_000,
      },
      key: { id: 7, name: "target", keyPrefix: "sk-t...rget", group: 3 },
      group: { id: 3, name: "pro", rate: 1.25, raw: { balance_charge_rate: 1.25 } },
      raw: {},
    }));

    await syncProviderUpstreamRate(
      baseConfig({
        source: "sub2api",
        accessToken: "access-old",
        refreshToken: "refresh-old",
        tokenExpiresAt: 1_000,
        userId: null,
      }),
      {
        provider: provider(),
        now: () => now,
        clients: {
          fetchNewApiProviderRate: vi.fn(),
          fetchSub2ApiProviderRate,
        },
        repository: {
          updateProviderCostMultiplier: vi.fn(async () => true),
          recordSyncSuccess,
          recordSyncFailure: vi.fn(async () => undefined),
          shareAuthBySourceAndBaseUrl,
        },
      }
    );

    expect(recordSyncSuccess).toHaveBeenCalledWith(
      11,
      expect.objectContaining({
        accessToken: "access-next",
        refreshToken: "refresh-next",
        tokenExpiresAt: 1_800_000,
      })
    );
    expect(shareAuthBySourceAndBaseUrl).toHaveBeenCalledWith(
      {
        source: "sub2api",
        baseUrl: "https://upstream.example",
        accessToken: "access-next",
        refreshToken: "refresh-next",
        tokenExpiresAt: 1_800_000,
      },
      11
    );
  });

  it("records failure without updating provider multiplier", async () => {
    const updateProviderCostMultiplier = vi.fn(async () => true);
    const recordSyncFailure = vi.fn(async () => undefined);
    const error = new Error("New API token not found");

    const result = await syncProviderUpstreamRate(baseConfig(), {
      provider: provider(),
      now: () => now,
      clients: {
        fetchNewApiProviderRate: vi.fn(async () => {
          throw error;
        }),
        fetchSub2ApiProviderRate: vi.fn(),
      },
      repository: {
        updateProviderCostMultiplier,
        recordSyncSuccess: vi.fn(async () => undefined),
        recordSyncFailure,
      },
    });

    expect(result).toEqual({
      ok: false,
      configId: 11,
      providerId: 42,
      error: "New API token not found",
    });
    expect(updateProviderCostMultiplier).not.toHaveBeenCalled();
    expect(recordSyncFailure).toHaveBeenCalledWith(11, {
      syncedAt: now,
      error: "New API token not found",
    });
  });
});
