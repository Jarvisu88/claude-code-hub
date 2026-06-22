import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ProviderTestResult } from "@/lib/provider-testing/types";
import type { Provider } from "@/types/provider";
import {
  computeAvgLatency,
  getLatencyMap,
  probeProviderLatency,
  type LatencyDeps,
} from "./latency-probe-service";

const latencyProbeMocks = vi.hoisted(() => ({
  executeProviderTest: vi.fn(),
  updateProviderLatencyProbeSnapshot: vi.fn(),
  writeLatencySample: vi.fn(),
}));

vi.mock("@/lib/provider-testing/test-service", () => ({
  executeProviderTest: latencyProbeMocks.executeProviderTest,
}));

vi.mock("@/repository/provider", () => ({
  updateProviderLatencyProbeSnapshot: latencyProbeMocks.updateProviderLatencyProbeSnapshot,
}));

vi.mock("./latency-cache", () => ({
  readLatencyCache: vi.fn(async () => new Map<number, number | null>()),
  writeLatencySample: latencyProbeMocks.writeLatencySample,
}));

describe("computeAvgLatency", () => {
  it("averages only successful runs", () => {
    expect(
      computeAvgLatency([
        { success: true, latencyMs: 100 },
        { success: false, latencyMs: 9999 },
        { success: true, latencyMs: 200 },
      ])
    ).toBe(150);
  });
  it("returns null when all failed", () => {
    expect(computeAvgLatency([{ success: false, latencyMs: 1 }])).toBe(null);
  });
  it("returns null for empty", () => {
    expect(computeAvgLatency([])).toBe(null);
  });

  it("uses successful first-byte timings before total latency", () => {
    expect(
      computeAvgLatency([
        { success: true, latencyMs: 900, firstByteMs: 120 },
        { success: true, latencyMs: 700, firstByteMs: 80 },
        { success: false, latencyMs: 1, firstByteMs: 1 },
      ])
    ).toBe(100);
  });
});

describe("getLatencyMap", () => {
  it("uses cache hits and falls back to passive for misses", async () => {
    const deps: LatencyDeps = {
      readCache: async () => new Map<number, number | null>([[1, 120]]),
      readPassive: async () => new Map<number, number | null>([[2, 300]]),
    };
    const map = await getLatencyMap([1, 2, 3], deps);
    expect(map.get(1)).toBe(120); // cache
    expect(map.get(2)).toBe(300); // passive fallback
    expect(map.has(3)).toBe(false); // no data anywhere -> absent (sorts last)
  });

  it("does not query passive when all cached", async () => {
    const readPassive = vi.fn();
    const deps: LatencyDeps = {
      readCache: async () =>
        new Map<number, number | null>([
          [1, 1],
          [2, 2],
        ]),
      readPassive,
    };
    await getLatencyMap([1, 2], deps);
    expect(readPassive).not.toHaveBeenCalled();
  });

  it("treats a cached null as present (does not re-query passive)", async () => {
    const readPassive = vi.fn();
    const deps: LatencyDeps = {
      readCache: async () => new Map<number, number | null>([[1, null]]),
      readPassive,
    };
    const map = await getLatencyMap([1], deps);
    expect(map.get(1)).toBe(null);
    expect(readPassive).not.toHaveBeenCalled();
  });
});

describe("probeProviderLatency", () => {
  const now = Date.parse("2026-06-22T00:00:00.000Z");
  const provider = {
    id: 7,
    url: "https://api.example.com",
    key: "sk-test",
    providerType: "claude",
    proxyUrl: null,
    proxyFallbackToDirect: false,
    latencyProbeModel: null,
  } as Provider;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("persists the reused provider test error when every probe run fails", async () => {
    latencyProbeMocks.executeProviderTest.mockResolvedValue(
      providerTestResult({
        success: false,
        status: "red",
        subStatus: "auth_error",
        errorMessage: "HTTP 401 from upstream",
        latencyMs: 0,
      })
    );

    const avgLatencyMs = await probeProviderLatency(provider, now);

    expect(avgLatencyMs).toBeNull();
    expect(latencyProbeMocks.updateProviderLatencyProbeSnapshot).toHaveBeenCalledWith(7, {
      avgLatencyMs: null,
      sampledAt: now,
      status: "failed",
      errorMessage: "HTTP 401 from upstream",
    });
  });

  it("clears the persisted probe error after a successful average", async () => {
    latencyProbeMocks.executeProviderTest.mockResolvedValue(
      providerTestResult({
        success: true,
        status: "green",
        subStatus: "success",
        latencyMs: 300,
        firstByteMs: 120,
      })
    );

    const avgLatencyMs = await probeProviderLatency(provider, now);

    expect(avgLatencyMs).toBe(120);
    expect(latencyProbeMocks.updateProviderLatencyProbeSnapshot).toHaveBeenCalledWith(7, {
      avgLatencyMs: 120,
      sampledAt: now,
      status: "success",
      errorMessage: null,
    });
  });
});

function providerTestResult(
  overrides: Partial<ProviderTestResult> = {}
): ProviderTestResult {
  return {
    success: true,
    status: "green",
    subStatus: "success",
    latencyMs: 100,
    testedAt: new Date("2026-06-22T00:00:00.000Z"),
    validationDetails: {
      httpPassed: true,
      latencyPassed: true,
      contentPassed: true,
    },
    ...overrides,
  };
}
