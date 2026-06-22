import { logger } from "@/lib/logger";
import {
  acquireLeaderLock,
  type LeaderLock,
  releaseLeaderLock,
  startLeaderLockKeepAlive,
} from "@/lib/provider-endpoints/leader-lock";
import { publishProviderCacheInvalidation } from "@/lib/cache/provider-cache";
import { findAllProvidersFresh, updateProviderGroupPrioritiesBatch } from "@/repository/provider";
import { parseProviderGroups } from "@/lib/utils/provider-group";
import { probeProviderLatency } from "./latency-probe-service";
import { runLatencyPriorityWorkflow } from "./latency-sort-workflow";

const LOCK_KEY = "locks:latency-priority-scheduler";
const DEFAULT_TICK_MS = 60_000;
const LOCK_TTL_MS = 120_000;

const state = globalThis as unknown as {
  __CCH_LATENCY_PRIORITY_STARTED__?: boolean;
  __CCH_LATENCY_PRIORITY_INTERVAL_ID__?: ReturnType<typeof setInterval>;
  __CCH_LATENCY_PRIORITY_RUNNING__?: boolean;
  __CCH_LATENCY_PRIORITY_LAST_RUN_DAY__?: string;
  __CCH_LATENCY_PRIORITY_LOCK__?: LeaderLock;
};

function isEnabled(): boolean {
  return process.env.ENABLE_LATENCY_PRIORITY_WORKFLOW === "true";
}

function configuredTime(): string | null {
  const value = process.env.LATENCY_PRIORITY_WORKFLOW_TIME?.trim();
  return value && /^\d{2}:\d{2}$/.test(value) ? value : null;
}

function configuredTargetGroup(): string | null {
  return process.env.LATENCY_PRIORITY_WORKFLOW_TARGET_GROUP?.trim() || null;
}

function configuredProviderGroup(): string | null {
  return process.env.LATENCY_PRIORITY_WORKFLOW_PROVIDER_GROUP?.trim() || null;
}

function shouldRun(now: Date): boolean {
  const time = configuredTime();
  const targetGroup = configuredTargetGroup();
  if (!time || !targetGroup) return false;

  const hhmm = now.toTimeString().slice(0, 5);
  if (hhmm !== time) return false;

  const day = now.toISOString().slice(0, 10);
  if (state.__CCH_LATENCY_PRIORITY_LAST_RUN_DAY__ === day) return false;
  state.__CCH_LATENCY_PRIORITY_LAST_RUN_DAY__ = day;
  return true;
}

async function runCycle(nowFn: () => number): Promise<void> {
  if (state.__CCH_LATENCY_PRIORITY_RUNNING__) return;
  const now = new Date(nowFn());
  if (!shouldRun(now)) return;

  const lock = await acquireLeaderLock(LOCK_KEY, LOCK_TTL_MS);
  if (!lock) return;
  state.__CCH_LATENCY_PRIORITY_LOCK__ = lock;
  state.__CCH_LATENCY_PRIORITY_RUNNING__ = true;

  let lockLost = false;
  const keepAlive = startLeaderLockKeepAlive({
    getLock: () => state.__CCH_LATENCY_PRIORITY_LOCK__,
    clearLock: () => {
      state.__CCH_LATENCY_PRIORITY_LOCK__ = undefined;
    },
    ttlMs: LOCK_TTL_MS,
    logTag: "latency-priority-scheduler",
    onLost: () => {
      lockLost = true;
    },
  });

  try {
    const providerGroup = configuredProviderGroup();
    const targetGroup = configuredTargetGroup();
    if (!targetGroup) return;
    if (lockLost) return;

    const providers = (await findAllProvidersFresh())
      .filter((provider) => provider.isEnabled)
      .filter((provider) =>
        providerGroup ? parseProviderGroups(provider.groupTag).includes(providerGroup) : true
      );

    const result = await runLatencyPriorityWorkflow({
      providers,
      targetGroup,
      confirm: true,
      probeProviderLatency,
      updateProviderGroupPriorities: updateProviderGroupPrioritiesBatch,
      now: () => now.getTime(),
    });

    if (result.changes.length > 0) {
      await publishProviderCacheInvalidation();
    }

    logger.info("[latency-priority-scheduler] workflow complete", {
      providerGroup,
      targetGroup,
      changedCount: result.summary.changedCount,
      pendingCount: result.summary.pendingCount,
    });
  } catch (error) {
    logger.warn("[latency-priority-scheduler] cycle failed", {
      error: error instanceof Error ? error.message : String(error),
    });
  } finally {
    keepAlive.stop();
    state.__CCH_LATENCY_PRIORITY_RUNNING__ = false;
    const heldLock = state.__CCH_LATENCY_PRIORITY_LOCK__;
    state.__CCH_LATENCY_PRIORITY_LOCK__ = undefined;
    if (heldLock) await releaseLeaderLock(heldLock);
  }
}

export function startLatencyPriorityScheduler(): void {
  if (!isEnabled() || state.__CCH_LATENCY_PRIORITY_STARTED__) return;
  state.__CCH_LATENCY_PRIORITY_STARTED__ = true;
  const intervalId = setInterval(() => {
    void runCycle(() => Date.now());
  }, DEFAULT_TICK_MS);
  (intervalId as unknown as { unref?: () => void }).unref?.();
  state.__CCH_LATENCY_PRIORITY_INTERVAL_ID__ = intervalId;
  logger.info("[latency-priority-scheduler] started", {
    time: configuredTime(),
    providerGroup: configuredProviderGroup(),
    targetGroup: configuredTargetGroup(),
  });
}

export function stopLatencyPriorityScheduler(): void {
  if (state.__CCH_LATENCY_PRIORITY_INTERVAL_ID__) {
    clearInterval(state.__CCH_LATENCY_PRIORITY_INTERVAL_ID__);
    state.__CCH_LATENCY_PRIORITY_INTERVAL_ID__ = undefined;
  }
  state.__CCH_LATENCY_PRIORITY_STARTED__ = false;
}
