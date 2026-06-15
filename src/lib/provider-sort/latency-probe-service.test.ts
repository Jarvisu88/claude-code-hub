import { describe, expect, it, vi } from "vitest";
import { computeAvgLatency, getLatencyMap, type LatencyDeps } from "./latency-probe-service";

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
