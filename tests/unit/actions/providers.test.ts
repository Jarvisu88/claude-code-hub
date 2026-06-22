import { beforeEach, describe, expect, it, vi } from "vitest";

const getSessionMock = vi.fn();

const findProviderByIdMock = vi.fn();
const findAllProvidersFreshMock = vi.fn();
const getProviderStatisticsMock = vi.fn();
const createProviderMock = vi.fn();
const updateProviderMock = vi.fn();
const deleteProviderMock = vi.fn();
const updateProviderPrioritiesBatchMock = vi.fn();
const updateProviderGroupPrioritiesBatchMock = vi.fn();
const findProviderUpstreamRateSyncConfigsByProviderIdsMock = vi.fn();
const setProviderUpstreamRateSyncEnabledByProviderIdsMock = vi.fn();
const findProviderUpstreamRateSyncConfigMock = vi.fn(async () => null);
const shareUpstreamRateSyncAuthBySourceAndBaseUrlMock = vi.fn(async () => 0);
const upsertProviderUpstreamRateSyncConfigMock = vi.fn(async () => ({}));
const readChromeAuthForUpstreamMock = vi.fn();
const probeProviderLatencyMock = vi.fn();

const publishProviderCacheInvalidationMock = vi.fn();
const saveProviderCircuitConfigMock = vi.fn();
const deleteProviderCircuitConfigMock = vi.fn();
const clearConfigCacheMock = vi.fn();
const clearProviderStateMock = vi.fn();
const terminateProviderSessionsBatchMock = vi.fn();

const revalidatePathMock = vi.fn();
const emitActionAuditMock = vi.fn();
const loggerTraceMock = vi.fn();
const loggerDebugMock = vi.fn();
const loggerWarnMock = vi.fn();

vi.mock("@/lib/auth", () => ({
  getSession: getSessionMock,
}));

vi.mock("@/repository/provider", () => ({
  createProvider: createProviderMock,
  deleteProvider: deleteProviderMock,
  findAllProviders: vi.fn(async () => []),
  findAllProvidersFresh: findAllProvidersFreshMock,
  findProviderById: findProviderByIdMock,
  getProviderStatistics: getProviderStatisticsMock,
  resetProviderTotalCostResetAt: vi.fn(async () => {}),
  updateProvider: updateProviderMock,
  updateProviderGroupPrioritiesBatch: updateProviderGroupPrioritiesBatchMock,
  updateProviderCostMultiplier: vi.fn(async () => true),
  updateProviderPrioritiesBatch: updateProviderPrioritiesBatchMock,
  updateProvidersBatch: vi.fn(async () => []),
}));

vi.mock("@/repository/upstream-rate-sync", () => ({
  deleteProviderUpstreamRateSyncConfig: vi.fn(async () => true),
  findProviderUpstreamRateSyncConfig: findProviderUpstreamRateSyncConfigMock,
  findProviderUpstreamRateSyncConfigsByProviderIds:
    findProviderUpstreamRateSyncConfigsByProviderIdsMock,
  recordUpstreamRateSyncFailure: vi.fn(async () => {}),
  recordUpstreamRateSyncSuccess: vi.fn(async () => {}),
  setProviderUpstreamRateSyncEnabledByProviderIds:
    setProviderUpstreamRateSyncEnabledByProviderIdsMock,
  shareUpstreamRateSyncAuthBySourceAndBaseUrl:
    shareUpstreamRateSyncAuthBySourceAndBaseUrlMock,
  upsertProviderUpstreamRateSyncConfig: upsertProviderUpstreamRateSyncConfigMock,
}));

vi.mock("@/lib/upstream-rate-sync/browser-auth", () => ({
  openChromeForUpstreamAuth: vi.fn(async () => undefined),
  readChromeAuthForUpstream: readChromeAuthForUpstreamMock,
}));

vi.mock("@/lib/provider-sort/latency-probe-service", () => ({
  getLatencyMap: vi.fn(async () => new Map()),
  probeProviderLatency: probeProviderLatencyMock,
}));

vi.mock("@/lib/cache/provider-cache", () => ({
  publishProviderCacheInvalidation: publishProviderCacheInvalidationMock,
}));

vi.mock("@/lib/redis/circuit-breaker-config", () => ({
  deleteProviderCircuitConfig: deleteProviderCircuitConfigMock,
  saveProviderCircuitConfig: saveProviderCircuitConfigMock,
}));

vi.mock("@/lib/circuit-breaker", () => ({
  clearConfigCache: clearConfigCacheMock,
  clearProviderState: clearProviderStateMock,
  getAllHealthStatusAsync: vi.fn(async () => ({})),
  resetCircuit: vi.fn(),
}));

vi.mock("@/lib/session-manager", () => ({
  SessionManager: {
    terminateProviderSessionsBatch: terminateProviderSessionsBatchMock,
    terminateStickySessionsForProviders: terminateProviderSessionsBatchMock,
  },
}));

vi.mock("@/lib/logger", () => ({
  logger: {
    trace: loggerTraceMock,
    debug: loggerDebugMock,
    info: vi.fn(),
    warn: loggerWarnMock,
    error: vi.fn(),
  },
}));

vi.mock("@/lib/audit/emit", () => ({
  emitActionAudit: emitActionAuditMock,
}));

vi.mock("next/cache", () => ({
  revalidatePath: revalidatePathMock,
}));

function nowMs(): number {
  if (typeof performance !== "undefined" && typeof performance.now === "function") {
    return performance.now();
  }
  return Date.now();
}

async function withTimeout<T>(promise: Promise<T>, ms: number): Promise<T> {
  let timeoutId: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<never>((_, reject) => {
    timeoutId = setTimeout(() => reject(new Error(`超时：${ms}ms`)), ms);
  });

  try {
    return await Promise.race([promise, timeout]);
  } finally {
    if (timeoutId) clearTimeout(timeoutId);
  }
}

describe("Provider Actions - Async Optimization", () => {
  beforeEach(() => {
    vi.clearAllMocks();

    getSessionMock.mockResolvedValue({ user: { id: 1, role: "admin" } });

    findAllProvidersFreshMock.mockResolvedValue([
      {
        id: 1,
        name: "p1",
        url: "https://api.example.com",
        key: "sk-test-1234567890",
        isEnabled: true,
        weight: 1,
        priority: 0,
        costMultiplier: 1,
        groupTag: "default",
        providerType: "claude",
        preserveClientIp: false,
        modelRedirects: null,
        allowedModels: null,
        mcpPassthroughType: "none",
        mcpPassthroughUrl: null,
        limit5hUsd: null,
        limitDailyUsd: null,
        dailyResetMode: "fixed",
        dailyResetTime: "00:00",
        limitWeeklyUsd: null,
        limitMonthlyUsd: null,
        limitTotalUsd: null,
        limitConcurrentSessions: 0,
        maxRetryAttempts: null,
        circuitBreakerFailureThreshold: 5,
        circuitBreakerOpenDuration: 1800000,
        circuitBreakerHalfOpenSuccessThreshold: 2,
        proxyUrl: null,
        proxyFallbackToDirect: false,
        firstByteTimeoutStreamingMs: null,
        streamingIdleTimeoutMs: null,
        requestTimeoutNonStreamingMs: null,
        websiteUrl: null,
        faviconUrl: null,
        cacheTtlPreference: "inherit",
        context1mPreference: "inherit",
        codexReasoningEffortPreference: "inherit",
        codexReasoningSummaryPreference: "inherit",
        codexTextVerbosityPreference: "inherit",
        codexParallelToolCallsPreference: "inherit",
        tpm: null,
        rpm: null,
        rpd: null,
        cc: null,
        createdAt: new Date("2026-01-01T00:00:00.000Z"),
        updatedAt: new Date("2026-01-01T00:00:00.000Z"),
      },
    ]);

    getProviderStatisticsMock.mockResolvedValue([]);

    findProviderByIdMock.mockImplementation(async (id: number) => {
      const providers = await findAllProvidersFreshMock();
      return providers.find((p: { id: number }) => p.id === id) ?? null;
    });

    createProviderMock.mockResolvedValue({
      id: 123,
      circuitBreakerFailureThreshold: 5,
      circuitBreakerOpenDuration: 1800000,
      circuitBreakerHalfOpenSuccessThreshold: 2,
    });

    updateProviderMock.mockResolvedValue({
      id: 1,
      circuitBreakerFailureThreshold: 5,
      circuitBreakerOpenDuration: 1800000,
      circuitBreakerHalfOpenSuccessThreshold: 2,
    });

    deleteProviderMock.mockResolvedValue(undefined);
    publishProviderCacheInvalidationMock.mockResolvedValue(undefined);
    saveProviderCircuitConfigMock.mockResolvedValue(undefined);
    deleteProviderCircuitConfigMock.mockResolvedValue(undefined);
    clearProviderStateMock.mockResolvedValue(undefined);
    terminateProviderSessionsBatchMock.mockResolvedValue(0);
    updateProviderPrioritiesBatchMock.mockResolvedValue(0);
    updateProviderGroupPrioritiesBatchMock.mockResolvedValue(0);
    findProviderUpstreamRateSyncConfigsByProviderIdsMock.mockResolvedValue(new Map());
    setProviderUpstreamRateSyncEnabledByProviderIdsMock.mockResolvedValue(0);
    findProviderUpstreamRateSyncConfigMock.mockResolvedValue(null);
    shareUpstreamRateSyncAuthBySourceAndBaseUrlMock.mockResolvedValue(0);
    upsertProviderUpstreamRateSyncConfigMock.mockResolvedValue({});
    readChromeAuthForUpstreamMock.mockResolvedValue({
      authToken: "access-next",
      refreshToken: "refresh-next",
      tokenExpiresAt: 1_800_000,
      cookie: "session=next",
      userId: "1001",
    });
    probeProviderLatencyMock.mockResolvedValue(250);
  });

  describe("getProviders", () => {
    it("should return providers without blocking on statistics", async () => {
      getProviderStatisticsMock.mockImplementation(() => new Promise(() => {}));

      const { getProviders } = await import("@/actions/providers");
      const result = await withTimeout(getProviders(), 200);

      expect(result).toHaveLength(1);
      expect(result[0]?.id).toBe(1);
      expect(getProviderStatisticsMock).not.toHaveBeenCalled();
    });

    it("should complete within 500ms", async () => {
      getProviderStatisticsMock.mockImplementation(() => new Promise(() => {}));

      const { getProviders } = await import("@/actions/providers");
      const start = nowMs();
      const result = await withTimeout(getProviders(), 500);
      const elapsed = nowMs() - start;

      expect(result).toHaveLength(1);
      expect(elapsed).toBeLessThan(500);
    });

    it("should include upstream rate sync summary for list display", async () => {
      findProviderUpstreamRateSyncConfigsByProviderIdsMock.mockResolvedValue(
        new Map([
          [
            1,
            {
              id: 10,
              providerId: 1,
              source: "newapi",
              isEnabled: true,
              lastSyncedAt: new Date("2026-06-22T06:00:00.000Z"),
              lastSyncOk: true,
              lastSyncRate: "0.75",
              lastSyncError: null,
              lastUpstreamGroupName: "codex-pro",
            },
          ],
        ])
      );

      const { getProviders } = await import("@/actions/providers");
      const result = await getProviders();

      expect(result[0]?.upstreamRateSync).toEqual({
        isConfigured: true,
        isEnabled: true,
        source: "newapi",
        lastSyncedAt: "2026-06-22T06:00:00.000Z",
        lastSyncOk: true,
        lastSyncRate: 0.75,
        lastSyncError: null,
        lastUpstreamGroupName: "codex-pro",
      });
    });
  });

  describe("autoSortProviderPriority", () => {
    it("should return preview only when confirm is false", async () => {
      findAllProvidersFreshMock.mockResolvedValue([
        { id: 1, name: "a", costMultiplier: "2.0", priority: 0 } as any,
        { id: 2, name: "b", costMultiplier: "1.0", priority: 1 } as any,
        { id: 3, name: "c", costMultiplier: "1.0", priority: 9 } as any,
      ]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: false });

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.data.applied).toBe(false);
      expect(result.data.summary.groupCount).toBe(2);
      expect(result.data.summary.totalProviders).toBe(3);
      expect(result.data.summary.changedCount).toBe(3);
      expect(result.data.groups).toEqual([
        {
          costMultiplier: 1,
          priority: 0,
          providers: [
            { id: 2, name: "b" },
            { id: 3, name: "c" },
          ],
        },
        {
          costMultiplier: 2,
          priority: 1,
          providers: [{ id: 1, name: "a" }],
        },
      ]);

      expect(updateProviderPrioritiesBatchMock).not.toHaveBeenCalled();
      expect(publishProviderCacheInvalidationMock).not.toHaveBeenCalled();
    });

    it("should handle invalid costMultiplier values gracefully", async () => {
      findAllProvidersFreshMock.mockResolvedValue([
        { id: 1, name: "bad", costMultiplier: undefined, priority: 5 } as any,
        { id: 2, name: "good", costMultiplier: "1.0", priority: 0 } as any,
      ]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: false });

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.data.summary.groupCount).toBe(2);
      expect(result.data.groups).toEqual([
        {
          costMultiplier: 0,
          priority: 0,
          providers: [{ id: 1, name: "bad" }],
        },
        {
          costMultiplier: 1,
          priority: 1,
          providers: [{ id: 2, name: "good" }],
        },
      ]);
    });

    it("should apply changes when confirm is true", async () => {
      findAllProvidersFreshMock.mockResolvedValue([
        { id: 10, name: "x", costMultiplier: "2.0", priority: 0 } as any,
        { id: 20, name: "y", costMultiplier: "1.0", priority: 0 } as any,
      ]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: true });

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.data.applied).toBe(true);
      expect(result.data.summary.changedCount).toBe(1);

      expect(updateProviderPrioritiesBatchMock).toHaveBeenCalledTimes(1);
      expect(updateProviderPrioritiesBatchMock).toHaveBeenCalledWith([{ id: 10, priority: 1 }]);
      expect(publishProviderCacheInvalidationMock).toHaveBeenCalledTimes(1);
    });

    it("should work with a single provider", async () => {
      findAllProvidersFreshMock.mockResolvedValue([
        { id: 1, name: "only", costMultiplier: "1.0", priority: 9 } as any,
      ]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: true });

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.data.applied).toBe(true);
      expect(result.data.summary.groupCount).toBe(1);
      expect(result.data.summary.changedCount).toBe(1);
      expect(updateProviderPrioritiesBatchMock).toHaveBeenCalledWith([{ id: 1, priority: 0 }]);
      expect(publishProviderCacheInvalidationMock).toHaveBeenCalledTimes(1);
    });

    it("should set priority 0 for all providers when costMultiplier is the same", async () => {
      findAllProvidersFreshMock.mockResolvedValue([
        { id: 1, name: "a", costMultiplier: "1.0", priority: 5 } as any,
        { id: 2, name: "b", costMultiplier: "1.0", priority: 6 } as any,
        { id: 3, name: "c", costMultiplier: "1.0", priority: 7 } as any,
      ]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: true });

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.data.groups).toEqual([
        {
          costMultiplier: 1,
          priority: 0,
          providers: [
            { id: 1, name: "a" },
            { id: 2, name: "b" },
            { id: 3, name: "c" },
          ],
        },
      ]);
      expect(updateProviderPrioritiesBatchMock).toHaveBeenCalledWith([
        { id: 1, priority: 0 },
        { id: 2, priority: 0 },
        { id: 3, priority: 0 },
      ]);
      expect(publishProviderCacheInvalidationMock).toHaveBeenCalledTimes(1);
    });

    it("should reject non-admin users", async () => {
      getSessionMock.mockResolvedValueOnce({ user: { id: 2, role: "user" } });

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: true });

      expect(result.ok).toBe(false);
      if (result.ok) return;

      expect(result.error).toBe("无权限执行此操作");
      expect(updateProviderPrioritiesBatchMock).not.toHaveBeenCalled();
      expect(publishProviderCacheInvalidationMock).not.toHaveBeenCalled();
    });

    it("should not fail when cache invalidation publish throws", async () => {
      publishProviderCacheInvalidationMock.mockRejectedValueOnce(new Error("boom"));
      findAllProvidersFreshMock.mockResolvedValue([
        { id: 10, name: "x", costMultiplier: "2.0", priority: 0 } as any,
        { id: 20, name: "y", costMultiplier: "1.0", priority: 0 } as any,
      ]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: true });

      expect(result.ok).toBe(true);
      expect(updateProviderPrioritiesBatchMock).toHaveBeenCalledTimes(1);
      expect(publishProviderCacheInvalidationMock).toHaveBeenCalledTimes(1);
    });

    it("should not write or invalidate cache when already sorted", async () => {
      findAllProvidersFreshMock.mockResolvedValue([
        { id: 10, name: "x", costMultiplier: "1.0", priority: 0 } as any,
        { id: 20, name: "y", costMultiplier: "2.0", priority: 1 } as any,
      ]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const result = await autoSortProviderPriority({ confirm: true });

      expect(result.ok).toBe(true);
      if (!result.ok) return;

      expect(result.data.applied).toBe(true);
      expect(result.data.changes).toEqual([]);
      expect(updateProviderPrioritiesBatchMock).not.toHaveBeenCalled();
      expect(publishProviderCacheInvalidationMock).not.toHaveBeenCalled();
    });

    it("should handle empty providers list", async () => {
      findAllProvidersFreshMock.mockResolvedValue([]);

      const { autoSortProviderPriority } = await import("@/actions/providers");
      const preview = await autoSortProviderPriority({ confirm: false });
      const applied = await autoSortProviderPriority({ confirm: true });

      expect(preview.ok).toBe(true);
      if (preview.ok) {
        expect(preview.data.summary.totalProviders).toBe(0);
        expect(preview.data.applied).toBe(false);
      }

      expect(applied.ok).toBe(true);
      if (applied.ok) {
        expect(applied.data.summary.totalProviders).toBe(0);
        expect(applied.data.applied).toBe(true);
      }
    });
  });

  describe("getProviderStatisticsAsync", () => {
    it("should return statistics map by provider id", async () => {
      getProviderStatisticsMock.mockResolvedValue([
        {
          id: 1,
          today_cost: "1.23",
          today_calls: 10,
          last_call_time: new Date("2026-01-01T00:00:00.000Z"),
          last_call_model: "model-a",
        },
        {
          id: 2,
          today_cost: "0",
          today_calls: 0,
          last_call_time: "2026-01-02T00:00:00.000Z",
          last_call_model: null,
        },
      ]);

      const { getProviderStatisticsAsync } = await import("@/actions/providers");
      const result = await getProviderStatisticsAsync();

      expect(result[1]).toEqual({
        todayCost: "1.23",
        todayCalls: 10,
        lastCallTime: "2026-01-01T00:00:00.000Z",
        lastCallModel: "model-a",
      });
      expect(result[2]).toEqual({
        todayCost: "0",
        todayCalls: 0,
        lastCallTime: "2026-01-02T00:00:00.000Z",
        lastCallModel: null,
      });
    });

    it("should return empty object for non-admin", async () => {
      getSessionMock.mockResolvedValueOnce({ user: { id: 2, role: "user" } });

      const { getProviderStatisticsAsync } = await import("@/actions/providers");
      const result = await getProviderStatisticsAsync();

      expect(result).toEqual({});
      expect(getProviderStatisticsMock).not.toHaveBeenCalled();
    });

    it("should handle errors gracefully and return empty object", async () => {
      getProviderStatisticsMock.mockRejectedValueOnce(new Error("boom"));

      const { getProviderStatisticsAsync } = await import("@/actions/providers");
      const result = await getProviderStatisticsAsync();

      expect(result).toEqual({});
    });
  });

  describe("batchSetProviderUpstreamRateSyncEnabled", () => {
    it("updates existing upstream sync configs and reports skipped providers", async () => {
      setProviderUpstreamRateSyncEnabledByProviderIdsMock.mockResolvedValueOnce(2);

      const { batchSetProviderUpstreamRateSyncEnabled } = await import("@/actions/providers");
      const result = await batchSetProviderUpstreamRateSyncEnabled({
        providerIds: [1, 2, 2, 3],
        enabled: true,
      });

      expect(result).toEqual({
        ok: true,
        data: { updatedCount: 2, skippedCount: 1 },
      });
      expect(setProviderUpstreamRateSyncEnabledByProviderIdsMock).toHaveBeenCalledWith(
        [1, 2, 3],
        true
      );
      expect(publishProviderCacheInvalidationMock).toHaveBeenCalledTimes(1);
    });

    it("rejects non-admin users", async () => {
      getSessionMock.mockResolvedValueOnce({ user: { id: 2, role: "user" } });

      const { batchSetProviderUpstreamRateSyncEnabled } = await import("@/actions/providers");
      const result = await batchSetProviderUpstreamRateSyncEnabled({
        providerIds: [1],
        enabled: false,
      });

      expect(result.ok).toBe(false);
      expect(setProviderUpstreamRateSyncEnabledByProviderIdsMock).not.toHaveBeenCalled();
    });
  });

  describe("runProviderLatencyPriorityWorkflowNow", () => {
    it("probes providers and writes latency priorities to the target group", async () => {
      updateProviderGroupPrioritiesBatchMock.mockResolvedValueOnce(2);
      findAllProvidersFreshMock.mockResolvedValueOnce([
        {
          id: 1,
          name: "fast",
          url: "https://fast.example.com",
          key: "sk-fast",
          isEnabled: true,
          priority: 9,
          groupPriorities: { price: 2 },
          groupTag: "claude",
          providerType: "openai-compatible",
        },
        {
          id: 2,
          name: "slow",
          url: "https://slow.example.com",
          key: "sk-slow",
          isEnabled: true,
          priority: 9,
          groupPriorities: null,
          groupTag: "claude",
          providerType: "openai-compatible",
        },
      ]);
      probeProviderLatencyMock.mockResolvedValueOnce(120).mockResolvedValueOnce(360);

      const { runProviderLatencyPriorityWorkflowNow } = await import("@/actions/providers");
      const result = await runProviderLatencyPriorityWorkflowNow({
        confirm: true,
        providerGroup: "claude",
        targetGroup: "claude_speed",
      });

      expect(result.ok).toBe(true);
      expect(updateProviderGroupPrioritiesBatchMock).toHaveBeenCalled();
      expect(updateProviderPrioritiesBatchMock).not.toHaveBeenCalled();
    });
  });

  describe("importProviderUpstreamRateAuthFromBrowser", () => {
    it("shares imported auth across configs on the same source and upstream site", async () => {
      const existingConfig = {
        id: 10,
        providerId: 1,
        source: "newapi",
        isEnabled: true,
        baseUrl: "https://same.example/",
        apiKey: "sk-target",
        keyName: "p1",
        accessToken: "access-old",
        refreshToken: null,
        tokenExpiresAt: null,
        cookie: "session=old",
        userId: null,
        syncIntervalMinutes: 60,
        lastSyncedAt: null,
        lastSyncOk: null,
        lastSyncRate: null,
        lastSyncError: null,
        lastUpstreamGroupName: null,
        createdAt: new Date("2026-01-01T00:00:00.000Z"),
        updatedAt: new Date("2026-01-01T00:00:00.000Z"),
      };
      const updatedConfig = {
        ...existingConfig,
        accessToken: "access-next",
        refreshToken: "refresh-next",
        tokenExpiresAt: 1_800_000,
        cookie: "session=next",
        userId: "1001",
      };
      findProviderUpstreamRateSyncConfigMock.mockResolvedValueOnce(existingConfig);
      upsertProviderUpstreamRateSyncConfigMock.mockResolvedValueOnce(updatedConfig);
      shareUpstreamRateSyncAuthBySourceAndBaseUrlMock.mockResolvedValueOnce(3);

      const { importProviderUpstreamRateAuthFromBrowser } = await import("@/actions/providers");
      const result = await importProviderUpstreamRateAuthFromBrowser(1);

      expect(result.ok).toBe(true);
      expect(shareUpstreamRateSyncAuthBySourceAndBaseUrlMock).toHaveBeenCalledWith(
        {
          source: "newapi",
          baseUrl: "https://same.example/",
          accessToken: "access-next",
          refreshToken: "refresh-next",
          tokenExpiresAt: 1_800_000,
          cookie: "session=next",
          userId: "1001",
        },
        10
      );
    });
  });

  describe("addProvider", () => {
    it("redacts URL credentials from provider action logs and audit metadata", async () => {
      const { addProvider } = await import("@/actions/providers");

      const result = await addProvider({
        name: "p2",
        url: "https://main-user:main-pass@api.example.com",
        key: "sk-test-2",
        proxy_url: "https://proxy-user:proxy-pass@proxy.example.com",
        website_url: "https://web-user:web-pass@example.com",
        tpm: null,
        rpm: null,
        rpd: null,
        cc: null,
      });

      expect(result.ok).toBe(true);
      const traceCalls = JSON.stringify(loggerTraceMock.mock.calls);
      const warnCalls = JSON.stringify(loggerWarnMock.mock.calls);
      const auditCalls = JSON.stringify(emitActionAuditMock.mock.calls);
      expect(traceCalls).toContain("https://REDACTED:REDACTED@api.example.com/");
      expect(traceCalls).toContain("https://REDACTED:REDACTED@proxy.example.com/");
      expect(warnCalls).not.toContain("web-pass");
      expect(auditCalls).not.toContain("main-pass");
      expect(traceCalls).not.toContain("main-pass");
      expect(traceCalls).not.toContain("proxy-pass");
    });

    it("should not call revalidatePath", async () => {
      const { addProvider } = await import("@/actions/providers");
      const result = await addProvider({
        name: "p2",
        url: "https://api.example.com",
        key: "sk-test-2",
        tpm: null,
        rpm: null,
        rpd: null,
        cc: null,
      });

      expect(result.ok).toBe(true);
      expect(revalidatePathMock).not.toHaveBeenCalled();
    });

    it("should complete quickly without blocking", async () => {
      const { addProvider } = await import("@/actions/providers");
      const start = nowMs();
      await withTimeout(
        addProvider({
          name: "p2",
          url: "https://api.example.com",
          key: "sk-test-2",
          tpm: null,
          rpm: null,
          rpd: null,
          cc: null,
        }),
        500
      );
      const elapsed = nowMs() - start;

      expect(elapsed).toBeLessThan(500);
      expect(revalidatePathMock).not.toHaveBeenCalled();
    });
  });

  // 说明：当前代码实现的函数名为 editProvider/removeProvider。
  // 这里按需求用例命名 describe，但实际调用对应实现以确保测试可编译、可运行。
  describe("updateProvider", () => {
    it("should not call revalidatePath", async () => {
      const { editProvider } = await import("@/actions/providers");
      const result = await editProvider(1, { name: "p1-updated" });

      expect(result.ok).toBe(true);
      expect(revalidatePathMock).not.toHaveBeenCalled();
      expect(terminateProviderSessionsBatchMock).not.toHaveBeenCalled();
    });

    it("editProvider endpoint sync: should forward url/provider_type edits to repository", async () => {
      const nextUrl = "https://new.example.com/v1/responses";
      const { editProvider } = await import("@/actions/providers");

      const result = await editProvider(1, {
        url: nextUrl,
        provider_type: "codex",
      });

      expect(result.ok).toBe(true);
      expect(updateProviderMock).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          url: nextUrl,
          provider_type: "codex",
        })
      );
      expect(publishProviderCacheInvalidationMock).toHaveBeenCalledTimes(1);
      expect(terminateProviderSessionsBatchMock).toHaveBeenCalledWith([1], "editProvider");
    });

    it("editProvider endpoint sync: should generate favicon_url when website_url is updated", async () => {
      const nextUrl = "https://new.example.com/v1/messages";
      const nextWebsiteUrl = "https://vendor.example.com/home";
      const { editProvider } = await import("@/actions/providers");

      const result = await editProvider(1, {
        url: nextUrl,
        website_url: nextWebsiteUrl,
      });

      expect(result.ok).toBe(true);
      expect(updateProviderMock).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          url: nextUrl,
          website_url: nextWebsiteUrl,
          favicon_url: "https://www.google.com/s2/favicons?domain=vendor.example.com&sz=32",
        })
      );
      expect(terminateProviderSessionsBatchMock).toHaveBeenCalledWith([1], "editProvider");
    });

    it("editProvider endpoint sync: should clear favicon_url when website_url is cleared", async () => {
      const { editProvider } = await import("@/actions/providers");

      const result = await editProvider(1, {
        url: "https://new.example.com/v1/messages",
        website_url: null,
      });

      expect(result.ok).toBe(true);
      expect(updateProviderMock).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          website_url: null,
          favicon_url: null,
        })
      );
      expect(terminateProviderSessionsBatchMock).toHaveBeenCalledWith([1], "editProvider");
    });

    it("editProvider: group or allowlist changes should also terminate sticky sessions", async () => {
      const { editProvider } = await import("@/actions/providers");

      const result = await editProvider(1, {
        group_tag: "gpt-load",
        allowed_models: ["gpt-4.1"],
      });

      expect(result.ok).toBe(true);
      expect(updateProviderMock).toHaveBeenCalledWith(
        1,
        expect.objectContaining({
          group_tag: "gpt-load",
          allowed_models: [{ matchType: "exact", pattern: "gpt-4.1" }],
        })
      );
      expect(terminateProviderSessionsBatchMock).toHaveBeenCalledWith([1], "editProvider");
    });
  });

  describe("deleteProvider", () => {
    it("should not call revalidatePath", async () => {
      const { removeProvider } = await import("@/actions/providers");
      const result = await removeProvider(1);

      expect(result.ok).toBe(true);
      expect(revalidatePathMock).not.toHaveBeenCalled();
      expect(terminateProviderSessionsBatchMock).toHaveBeenCalledWith([1], "removeProvider");
    });
  });

  describe("getUnmaskedProviderKey", () => {
    it("emits a durable audit event without persisting the raw provider key", async () => {
      findProviderByIdMock.mockResolvedValueOnce({
        id: 7,
        name: "secure-provider",
        key: "sk-real-provider-secret",
      });

      const { getUnmaskedProviderKey } = await import("@/actions/providers");
      const result = await getUnmaskedProviderKey(7);

      expect(result).toEqual({ ok: true, data: { key: "sk-real-provider-secret" } });
      expect(emitActionAuditMock).toHaveBeenCalledWith(
        expect.objectContaining({
          category: "provider",
          action: "provider.key_reveal",
          targetType: "provider",
          targetId: "7",
          targetName: "secure-provider",
          after: { id: 7, name: "secure-provider" },
          success: true,
          redactExtraKeys: ["key"],
        })
      );
      expect(JSON.stringify(emitActionAuditMock.mock.calls)).not.toContain(
        "sk-real-provider-secret"
      );
    });
  });
});
