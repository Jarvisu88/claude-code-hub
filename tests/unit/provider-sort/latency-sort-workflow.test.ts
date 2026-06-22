import { describe, expect, it, vi } from "vitest";
import {
  buildLatencyPriorityPlan,
  runLatencyPriorityWorkflow,
} from "@/lib/provider-sort/latency-sort-workflow";
import type { Provider } from "@/types/provider";

function provider(overrides: Partial<Provider>): Provider {
  return {
    id: 1,
    name: "provider",
    url: "https://api.example.com",
    key: "sk-test",
    providerVendorId: null,
    isEnabled: true,
    weight: 100,
    priority: 9,
    groupPriorities: null,
    costMultiplier: 1,
    groupTag: "speed",
    providerType: "openai-compatible",
    preserveClientIp: false,
    disableSessionReuse: false,
    modelRedirects: null,
    activeTimeStart: null,
    activeTimeEnd: null,
    allowedModels: null,
    allowedClients: [],
    blockedClients: [],
    latencyProbeEnabled: true,
    latencyProbeModel: null,
    latencyProbeIntervalMs: null,
    latencyProbeTimeStart: null,
    latencyProbeTimeEnd: null,
    latencyProbeLastAvgMs: null,
    latencyProbeLastStatus: null,
    latencyProbeLastError: null,
    latencyProbeLastRunAt: null,
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

describe("latency sort workflow", () => {
  it("does not rank unmeasured providers as fastest", () => {
    const plan = buildLatencyPriorityPlan({
      providers: [
        provider({ id: 1, name: "unknown", latencyProbeLastAvgMs: null }),
        provider({ id: 2, name: "zero", latencyProbeLastAvgMs: 0 }),
        provider({ id: 3, name: "fast", latencyProbeLastAvgMs: 180 }),
        provider({ id: 4, name: "slow", latencyProbeLastAvgMs: 420 }),
      ],
      targetGroup: "speed",
    });

    expect(plan.groups).toEqual([
      {
        avgLatencyMs: 180,
        priority: 0,
        providers: [{ id: 3, name: "fast" }],
      },
      {
        avgLatencyMs: 420,
        priority: 1,
        providers: [{ id: 4, name: "slow" }],
      },
    ]);
    expect(plan.pending.map((item) => item.providerId)).toEqual([1, 2]);
    expect(plan.changes.map((change) => change.providerId)).toEqual([3, 4]);
  });

  it("writes priority results to the target group without changing global priority", async () => {
    const updateProviderGroupPriorities = vi.fn(async () => 2);
    const result = await runLatencyPriorityWorkflow({
      providers: [
        provider({
          id: 1,
          name: "fast",
          priority: 7,
          groupPriorities: { price: 3 },
        }),
        provider({
          id: 2,
          name: "slow",
          priority: 8,
          groupPriorities: { speed: 5 },
        }),
      ],
      targetGroup: "speed",
      confirm: true,
      probeProviderLatency: vi
        .fn()
        .mockResolvedValueOnce(120)
        .mockResolvedValueOnce(360),
      updateProviderGroupPriorities,
      now: () => 1_800_000,
    });

    expect(result.applied).toBe(true);
    expect(updateProviderGroupPriorities).toHaveBeenCalledWith([
      { id: 1, groupPriorities: { price: 3, speed: 0 } },
      { id: 2, groupPriorities: { speed: 1 } },
    ]);
  });
});
