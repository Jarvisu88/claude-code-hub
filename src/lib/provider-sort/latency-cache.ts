import { logger } from "@/lib/logger";
import { getRedisClient } from "@/lib/redis";
import type { ProviderLatencySample } from "./types";

const KEY_PREFIX = "provider:latency:";
const TTL_SECONDS = Math.max(
  1,
  Number.parseInt(process.env.LATENCY_PROBE_TTL_MS ?? "300000", 10) / 1000 || 300
);

function cacheKey(providerId: number): string {
  return `${KEY_PREFIX}${providerId}`;
}

/** 批量读延迟缓存 -> providerId -> avgLatencyMs (null 表示探测过但全失败)。Redis 不可用返回空 map。 */
export async function readLatencyCache(
  providerIds: number[]
): Promise<Map<number, number | null>> {
  const map = new Map<number, number | null>();
  if (providerIds.length === 0) return map;
  // allowWhenRateLimitDisabled: 探测/缓存应在 ENABLE_RATE_LIMIT=false 时仍可用 (与 leader-lock 对齐)
  const redis = getRedisClient({ allowWhenRateLimitDisabled: true });
  if (!redis || redis.status !== "ready") return map;
  try {
    const raw = await redis.mget(...providerIds.map(cacheKey));
    raw.forEach((value, idx) => {
      if (!value) return;
      try {
        const parsed = JSON.parse(value) as ProviderLatencySample;
        map.set(providerIds[idx], parsed.avgLatencyMs);
      } catch {
        // 跳过损坏条目
      }
    });
  } catch (error) {
    logger.warn("[latency-cache] read failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  }
  return map;
}

/** 写一个延迟样本 (带 TTL)。Redis 不可用静默跳过。 */
export async function writeLatencySample(sample: ProviderLatencySample): Promise<void> {
  const redis = getRedisClient({ allowWhenRateLimitDisabled: true });
  if (!redis || redis.status !== "ready") return;
  try {
    await redis.set(cacheKey(sample.providerId), JSON.stringify(sample), "EX", TTL_SECONDS);
  } catch (error) {
    logger.warn("[latency-cache] write failed", {
      providerId: sample.providerId,
      error: error instanceof Error ? error.message : String(error),
    });
  }
}
