import { logger } from "@/lib/logger";
import {
  acquireLeaderLock,
  type LeaderLock,
  releaseLeaderLock,
  startLeaderLockKeepAlive,
} from "@/lib/provider-endpoints/leader-lock";
import { syncProviderUpstreamRate } from "@/lib/upstream-rate-sync/service";
import { findDueProviderUpstreamRateSyncConfigs } from "@/repository/upstream-rate-sync";

const LOCK_KEY = "locks:upstream-rate-sync-scheduler";

function parseIntWithDefault(value: string | undefined, fallback: number): number {
  const n = value ? Number.parseInt(value, 10) : Number.NaN;
  return Number.isFinite(n) ? n : fallback;
}

function isEnabled(): boolean {
  return process.env.ENABLE_UPSTREAM_RATE_SYNC !== "false";
}

const TICK_INTERVAL_MS = Math.max(
  10_000,
  parseIntWithDefault(process.env.UPSTREAM_RATE_SYNC_TICK_MS, 60_000)
);
const LOCK_TTL_MS = Math.max(
  TICK_INTERVAL_MS,
  parseIntWithDefault(process.env.UPSTREAM_RATE_SYNC_LOCK_TTL_MS, 120_000)
);
const BATCH_LIMIT = Math.max(
  1,
  parseIntWithDefault(process.env.UPSTREAM_RATE_SYNC_BATCH_LIMIT, 25)
);

const state = globalThis as unknown as {
  __CCH_UPSTREAM_RATE_SYNC_STARTED__?: boolean;
  __CCH_UPSTREAM_RATE_SYNC_INTERVAL_ID__?: ReturnType<typeof setInterval>;
  __CCH_UPSTREAM_RATE_SYNC_RUNNING__?: boolean;
  __CCH_UPSTREAM_RATE_SYNC_LOCK__?: LeaderLock;
};

async function runCycle(): Promise<void> {
  if (state.__CCH_UPSTREAM_RATE_SYNC_RUNNING__) return;

  const lock = await acquireLeaderLock(LOCK_KEY, LOCK_TTL_MS);
  if (!lock) return;

  state.__CCH_UPSTREAM_RATE_SYNC_LOCK__ = lock;
  state.__CCH_UPSTREAM_RATE_SYNC_RUNNING__ = true;
  let lockLost = false;
  const keepAlive = startLeaderLockKeepAlive({
    getLock: () => state.__CCH_UPSTREAM_RATE_SYNC_LOCK__,
    clearLock: () => {
      state.__CCH_UPSTREAM_RATE_SYNC_LOCK__ = undefined;
    },
    ttlMs: LOCK_TTL_MS,
    logTag: "upstream-rate-sync-scheduler",
    onLost: () => {
      lockLost = true;
    },
  });

  try {
    const configs = await findDueProviderUpstreamRateSyncConfigs(new Date(), BATCH_LIMIT);
    for (const config of configs) {
      if (lockLost) break;
      const result = await syncProviderUpstreamRate(config);
      if (!result.ok) {
        logger.warn("[upstream-rate-sync-scheduler] sync failed", {
          configId: result.configId,
          providerId: result.providerId,
          error: result.error,
        });
      }
    }
  } catch (error) {
    logger.warn("[upstream-rate-sync-scheduler] cycle failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  } finally {
    keepAlive.stop();
    state.__CCH_UPSTREAM_RATE_SYNC_RUNNING__ = false;
    const heldLock = state.__CCH_UPSTREAM_RATE_SYNC_LOCK__;
    state.__CCH_UPSTREAM_RATE_SYNC_LOCK__ = undefined;
    if (heldLock) await releaseLeaderLock(heldLock);
  }
}

export function startUpstreamRateSyncScheduler(): void {
  if (!isEnabled() || state.__CCH_UPSTREAM_RATE_SYNC_STARTED__) return;

  state.__CCH_UPSTREAM_RATE_SYNC_STARTED__ = true;
  void runCycle();
  const intervalId = setInterval(() => {
    void runCycle();
  }, TICK_INTERVAL_MS);
  (intervalId as unknown as { unref?: () => void }).unref?.();
  state.__CCH_UPSTREAM_RATE_SYNC_INTERVAL_ID__ = intervalId;
  logger.info("[upstream-rate-sync-scheduler] started", {
    tickIntervalMs: TICK_INTERVAL_MS,
    lockTtlMs: LOCK_TTL_MS,
    batchLimit: BATCH_LIMIT,
  });
}

export function stopUpstreamRateSyncScheduler(): void {
  if (state.__CCH_UPSTREAM_RATE_SYNC_INTERVAL_ID__) {
    clearInterval(state.__CCH_UPSTREAM_RATE_SYNC_INTERVAL_ID__);
    state.__CCH_UPSTREAM_RATE_SYNC_INTERVAL_ID__ = undefined;
  }

  state.__CCH_UPSTREAM_RATE_SYNC_STARTED__ = false;
  const lock = state.__CCH_UPSTREAM_RATE_SYNC_LOCK__;
  state.__CCH_UPSTREAM_RATE_SYNC_LOCK__ = undefined;
  if (lock) {
    void releaseLeaderLock(lock);
  }
}
