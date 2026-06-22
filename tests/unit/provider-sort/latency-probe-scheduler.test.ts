import { afterEach, describe, expect, test, vi } from "vitest";
import type { Provider } from "@/types/provider";

const mocks = vi.hoisted(() => ({
  acquireLeaderLock: vi.fn(),
  releaseLeaderLock: vi.fn(),
  findAllProvidersFresh: vi.fn(),
  probeProviderLatency: vi.fn(),
}));

vi.mock("@/lib/provider-endpoints/leader-lock", () => ({
  acquireLeaderLock: mocks.acquireLeaderLock,
  releaseLeaderLock: mocks.releaseLeaderLock,
  startLeaderLockKeepAlive: () => ({ stop: vi.fn() }),
}));

vi.mock("@/repository/provider", () => ({
  findAllProvidersFresh: mocks.findAllProvidersFresh,
}));

vi.mock("@/lib/provider-sort/latency-probe-service", () => ({
  probeProviderLatency: mocks.probeProviderLatency,
}));

function makeProvider(
  id: number,
  overrides: Partial<Provider> = {}
): Provider {
  return {
    id,
    name: `provider-${id}`,
    url: "https://example.com",
    key: "sk-test",
    isEnabled: true,
    weight: 1,
    priority: 0,
    groupPriorities: null,
    costMultiplier: 1,
    groupTag: null,
    providerType: "claude",
    preserveClientIp: false,
    disableSessionReuse: false,
    latencyProbeEnabled: null,
    latencyProbeModel: null,
    latencyProbeIntervalMs: null,
    latencyProbeTimeStart: null,
    latencyProbeTimeEnd: null,
    latencyProbeLastAvgMs: null,
    latencyProbeLastStatus: null,
    latencyProbeLastError: null,
    latencyProbeLastRunAt: null,
    createdAt: new Date("2026-01-01T00:00:00.000Z"),
    updatedAt: new Date("2026-01-01T00:00:00.000Z"),
    ...overrides,
  } as Provider;
}

async function flushMicrotasks(times = 6): Promise<void> {
  for (let i = 0; i < times; i += 1) {
    await Promise.resolve();
  }
}

describe("latency probe scheduler", () => {
  afterEach(async () => {
    const { stopLatencyProbeScheduler } = await import(
      "@/lib/provider-sort/latency-probe-scheduler"
    );
    stopLatencyProbeScheduler();
    vi.useRealTimers();
    vi.unstubAllEnvs();
    vi.clearAllMocks();
    vi.resetModules();
  });

  test("starts by default and immediately probes only explicitly enabled providers", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-01-01T00:00:30.000Z"));
    vi.stubEnv("LATENCY_PROBE_INTERVAL_MS", "10000");
    vi.stubEnv("ENABLE_LATENCY_PROBE", undefined);

    mocks.acquireLeaderLock.mockResolvedValue({
      key: "locks:latency-probe-scheduler",
      lockId: "test-lock",
      lockType: "memory",
    });
    mocks.releaseLeaderLock.mockResolvedValue(undefined);
    mocks.probeProviderLatency.mockResolvedValue(120);
    mocks.findAllProvidersFresh.mockResolvedValue([
      makeProvider(1, { latencyProbeEnabled: true }),
      makeProvider(2, { latencyProbeEnabled: false }),
      makeProvider(3, { latencyProbeEnabled: null }),
    ]);

    const { startLatencyProbeScheduler } = await import(
      "@/lib/provider-sort/latency-probe-scheduler"
    );

    startLatencyProbeScheduler();
    await flushMicrotasks();

    expect(mocks.findAllProvidersFresh).toHaveBeenCalledTimes(1);
    expect(mocks.probeProviderLatency).toHaveBeenCalledTimes(1);
    expect(mocks.probeProviderLatency.mock.calls[0][0].id).toBe(1);
  });
});
