import { beforeEach, describe, expect, it, vi } from "vitest";

// Mutable redis stub so individual tests can simulate "redis unavailable" (null).
let currentRedis: { mget: ReturnType<typeof vi.fn>; set: ReturnType<typeof vi.fn>; status: string } | null = {
  mget: vi.fn(),
  set: vi.fn(),
  status: "ready",
};

vi.mock("@/lib/redis", () => ({ getRedisClient: () => currentRedis }));

import { readLatencyCache, writeLatencySample } from "./latency-cache";

describe("latency-cache", () => {
  beforeEach(() => {
    currentRedis = { mget: vi.fn(), set: vi.fn(), status: "ready" };
  });

  it("readLatencyCache parses samples into a map", async () => {
    currentRedis?.mget.mockResolvedValue([
      JSON.stringify({ providerId: 1, avgLatencyMs: 120, sampledAt: 1, source: "probe" }),
      null, // miss
      JSON.stringify({ providerId: 3, avgLatencyMs: null, sampledAt: 1, source: "probe" }),
    ]);
    const map = await readLatencyCache([1, 2, 3]);
    expect(map.get(1)).toBe(120);
    expect(map.has(2)).toBe(false);
    expect(map.get(3)).toBe(null);
  });

  it("readLatencyCache returns empty map for empty input without calling redis", async () => {
    const map = await readLatencyCache([]);
    expect(map.size).toBe(0);
    expect(currentRedis?.mget).not.toHaveBeenCalled();
  });

  it("writeLatencySample sets key with TTL", async () => {
    await writeLatencySample({ providerId: 5, avgLatencyMs: 80, sampledAt: 1, source: "probe" });
    expect(currentRedis?.set).toHaveBeenCalledWith(
      "provider:latency:5",
      expect.any(String),
      "EX",
      expect.any(Number)
    );
  });

  describe("when redis is unavailable", () => {
    beforeEach(() => {
      currentRedis = null;
    });

    it("readLatencyCache returns empty map and does not throw", async () => {
      const map = await readLatencyCache([1, 2]);
      expect(map.size).toBe(0);
    });

    it("writeLatencySample silently skips and does not throw", async () => {
      await expect(
        writeLatencySample({ providerId: 9, avgLatencyMs: 50, sampledAt: 1, source: "probe" })
      ).resolves.toBeUndefined();
    });
  });
});
