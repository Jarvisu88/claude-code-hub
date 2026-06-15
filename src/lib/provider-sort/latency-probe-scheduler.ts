import { logger } from "@/lib/logger";
import {
  acquireLeaderLock,
  type LeaderLock,
  releaseLeaderLock,
  startLeaderLockKeepAlive,
} from "@/lib/provider-endpoints/leader-lock";
import { findAllProvidersFresh } from "@/repository/provider";
import { probeProviderLatency } from "./latency-probe-service";

const LOCK_KEY = "locks:latency-probe-scheduler";

function parseIntWithDefault(value: string | undefined, fallback: number): number {
  const n = value ? Number.parseInt(value, 10) : Number.NaN;
  return Number.isFinite(n) ? n : fallback;
}

// 周期间隔 (默认 90s, 最低 10s)
const INTERVAL_MS = Math.max(
  10_000,
  parseIntWithDefault(process.env.LATENCY_PROBE_INTERVAL_MS, 90_000)
);
// 锁 TTL: 至少覆盖一个周期, 由 keep-alive 续租; 探测可能比周期长, 故续租是必须的
const LOCK_TTL_MS = Math.max(INTERVAL_MS, 120_000);

function isEnabled(): boolean {
  return process.env.ENABLE_LATENCY_PROBE === "true";
}

const state = globalThis as unknown as {
  __CCH_LATENCY_PROBE_STARTED__?: boolean;
  __CCH_LATENCY_PROBE_INTERVAL_ID__?: ReturnType<typeof setInterval>;
  __CCH_LATENCY_PROBE_RUNNING__?: boolean;
  __CCH_LATENCY_PROBE_LOCK__?: LeaderLock;
};

async function runCycle(nowFn: () => number): Promise<void> {
  if (state.__CCH_LATENCY_PROBE_RUNNING__) return; // 防重入
  const lock = await acquireLeaderLock(LOCK_KEY, LOCK_TTL_MS);
  if (!lock) return; // 别的实例在跑
  state.__CCH_LATENCY_PROBE_LOCK__ = lock;
  state.__CCH_LATENCY_PROBE_RUNNING__ = true;

  // 续租: 探测一轮 (供应商数 x 3 次 x 超时) 可能超过 LOCK_TTL_MS, 必须续租防止锁过期被另一实例并发探测
  let lockLost = false;
  const keepAlive = startLeaderLockKeepAlive({
    getLock: () => state.__CCH_LATENCY_PROBE_LOCK__,
    clearLock: () => {
      state.__CCH_LATENCY_PROBE_LOCK__ = undefined;
    },
    ttlMs: LOCK_TTL_MS,
    logTag: "latency-probe-scheduler",
    onLost: () => {
      lockLost = true;
    },
  });

  try {
    const providers = (await findAllProvidersFresh()).filter((p) => p.isEnabled);
    // 低并发: 串行探测以避免打爆上游
    for (const p of providers) {
      if (lockLost) break; // 锁丢了就停, 让接管的实例去探测
      await probeProviderLatency(p, nowFn());
    }
  } catch (error) {
    logger.warn("[latency-probe-scheduler] cycle failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  } finally {
    keepAlive.stop();
    state.__CCH_LATENCY_PROBE_RUNNING__ = false;
    const heldLock = state.__CCH_LATENCY_PROBE_LOCK__;
    state.__CCH_LATENCY_PROBE_LOCK__ = undefined;
    if (heldLock) await releaseLeaderLock(heldLock);
  }
}

export function startLatencyProbeScheduler(): void {
  if (!isEnabled() || state.__CCH_LATENCY_PROBE_STARTED__) return;
  state.__CCH_LATENCY_PROBE_STARTED__ = true;
  const intervalId = setInterval(() => {
    void runCycle(() => Date.now());
  }, INTERVAL_MS);
  (intervalId as unknown as { unref?: () => void }).unref?.();
  state.__CCH_LATENCY_PROBE_INTERVAL_ID__ = intervalId;
  logger.info("[latency-probe-scheduler] started", { intervalMs: INTERVAL_MS });
}

export function stopLatencyProbeScheduler(): void {
  if (state.__CCH_LATENCY_PROBE_INTERVAL_ID__) {
    clearInterval(state.__CCH_LATENCY_PROBE_INTERVAL_ID__);
    state.__CCH_LATENCY_PROBE_INTERVAL_ID__ = undefined;
  }
  state.__CCH_LATENCY_PROBE_STARTED__ = false;
}
