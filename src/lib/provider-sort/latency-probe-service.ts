import { queryProviderAvailability } from "@/lib/availability/availability-service";
import { logger } from "@/lib/logger";
import { DEFAULT_TEST_MODELS } from "@/lib/provider-testing/default-test-models";
import { executeProviderTest } from "@/lib/provider-testing/test-service";
import type { ProviderTestResult } from "@/lib/provider-testing/types";
import { updateProviderLatencyProbeSnapshot } from "@/repository/provider";
import type { Provider } from "@/types/provider";
import { readLatencyCache, writeLatencySample } from "./latency-cache";

const PROBE_RUNS = 3;

export interface LatencyDeps {
  readCache: (ids: number[]) => Promise<Map<number, number | null>>;
  readPassive: (ids: number[]) => Promise<Map<number, number | null>>;
}

/** 仅对 success 的运行取平均; 无成功返回 null。导出供单测。 */
export function computeAvgLatency(
  runs: Array<{ success: boolean; latencyMs: number; firstByteMs?: number }>
): number | null {
  const ok = runs.filter((r) => r.success);
  if (ok.length === 0) return null;
  return (
    ok.reduce((s, r) => {
      const firstByteMs = Number(r.firstByteMs);
      return s + (Number.isFinite(firstByteMs) && firstByteMs > 0 ? firstByteMs : r.latencyMs);
    }, 0) / ok.length
  );
}

/** 被动: 从 availability-service 取近 24h avgLatencyMs, 并回填缓存避免冷启动期反复打 DB。 */
async function readPassiveLatency(
  providerIds: number[],
  now: number
): Promise<Map<number, number | null>> {
  const map = new Map<number, number | null>();
  if (providerIds.length === 0) return map;
  try {
    const result = await queryProviderAvailability({ providerIds, includeDisabled: true });
    for (const prov of result.providers) {
      // 重要: availability-service 无样本时返回 avgLatencyMs=0 (非 null), 故必须用 > 0 过滤,
      //       不可简化为 != null, 否则会把"无样本"误当成"0ms 最快"排到最前。
      const avg = prov.avgLatencyMs;
      if (typeof avg === "number" && avg > 0) {
        map.set(prov.providerId, avg);
        // 回填缓存 (source=passive), 让冷启动期后续请求命中缓存而非重复聚合查询。
        await writeLatencySample({
          providerId: prov.providerId,
          avgLatencyMs: avg,
          sampledAt: now,
          source: "passive",
        });
      }
    }
  } catch (error) {
    logger.warn("[latency-probe] passive query failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  }
  return map;
}

/** 合并缓存命中与被动回退。导出供单测 (依赖注入)。 */
export async function getLatencyMap(
  providerIds: number[],
  deps: LatencyDeps = {
    readCache: readLatencyCache,
    // 默认 passive 注入当前时间用于回填的 sampledAt; 单测时可传不回填的 stub
    readPassive: (ids) => readPassiveLatency(ids, Date.now()),
  }
): Promise<Map<number, number | null>> {
  const map = await deps.readCache(providerIds);
  const missing = providerIds.filter((id) => !map.has(id));
  if (missing.length > 0) {
    const passive = await deps.readPassive(missing);
    for (const [id, v] of passive) map.set(id, v);
  }
  return map;
}

function resolveProbeModel(provider: Provider): string {
  const configured = provider.latencyProbeModel?.trim();
  return configured || DEFAULT_TEST_MODELS[provider.providerType];
}

/** 对单个供应商跑 PROBE_RUNS 次测试取平均, 写缓存和最近状态快照。 */
export async function probeProviderLatency(provider: Provider, now: number): Promise<number | null> {
  const model = resolveProbeModel(provider);
  const timeoutMs =
    provider.providerType === "gemini" || provider.providerType === "gemini-cli" ? 60000 : 15000;
  const runs: Array<{ success: boolean; latencyMs: number; firstByteMs?: number }> = [];
  let lastErrorMessage: string | null = null;
  for (let i = 0; i < PROBE_RUNS; i++) {
    try {
      const r = await executeProviderTest({
        providerUrl: provider.url,
        apiKey: provider.key,
        providerType: provider.providerType,
        model,
        proxyUrl: provider.proxyUrl ?? undefined,
        proxyFallbackToDirect: provider.proxyFallbackToDirect ?? false,
        timeoutMs,
      });
      runs.push({ success: r.success, latencyMs: r.latencyMs, firstByteMs: r.firstByteMs });
      if (!r.success) {
        lastErrorMessage = formatProbeFailure(r);
      }
    } catch (error) {
      runs.push({ success: false, latencyMs: 0 });
      lastErrorMessage = error instanceof Error ? error.message : String(error);
    }
  }
  const avgLatencyMs = computeAvgLatency(runs);
  await writeLatencySample({
    providerId: provider.id,
    avgLatencyMs,
    sampledAt: now,
    source: "probe",
  });
  await updateProviderLatencyProbeSnapshot(provider.id, {
    avgLatencyMs,
    sampledAt: now,
    status: avgLatencyMs === null ? "failed" : "success",
    errorMessage: avgLatencyMs === null ? lastErrorMessage : null,
  });
  return avgLatencyMs;
}

function formatProbeFailure(result: ProviderTestResult): string {
  if (result.errorMessage?.trim()) {
    return result.errorMessage.trim();
  }

  const parts = [
    result.httpStatusCode ? `HTTP ${result.httpStatusCode}` : null,
    result.httpStatusText?.trim() || null,
    result.subStatus,
    result.content?.trim() || null,
  ].filter(Boolean);
  return parts.join(" - ");
}
