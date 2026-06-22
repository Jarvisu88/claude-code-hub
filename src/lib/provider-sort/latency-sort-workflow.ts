import type { Provider } from "@/types/provider";

export interface LatencyPriorityChange {
  providerId: number;
  name: string;
  oldPriority: number | null;
  newPriority: number;
  avgLatencyMs: number;
}

export interface LatencyPriorityPending {
  providerId: number;
  name: string;
  reason: "unmeasured" | "failed";
}

export interface LatencyPriorityPlan {
  groups: Array<{
    avgLatencyMs: number;
    priority: number;
    providers: Array<{ id: number; name: string }>;
  }>;
  changes: LatencyPriorityChange[];
  pending: LatencyPriorityPending[];
  summary: {
    totalProviders: number;
    measuredCount: number;
    pendingCount: number;
    changedCount: number;
    groupCount: number;
  };
}

export interface LatencyPriorityWorkflowResult extends LatencyPriorityPlan {
  applied: boolean;
  targetGroup: string;
}

export interface LatencyPriorityWorkflowInput {
  providers: Provider[];
  targetGroup: string;
  confirm: boolean;
  probeProviderLatency?: (provider: Provider, now: number) => Promise<number | null>;
  updateProviderGroupPriorities?: (
    updates: Array<{ id: number; groupPriorities: Record<string, number> }>
  ) => Promise<number>;
  now?: () => number;
}

export function buildLatencyPriorityPlan(input: {
  providers: Provider[];
  targetGroup: string;
}): LatencyPriorityPlan {
  const targetGroup = normalizeTargetGroup(input.targetGroup);
  const measured: Array<{ provider: Provider; avgLatencyMs: number }> = [];
  const pending: LatencyPriorityPending[] = [];

  for (const provider of input.providers) {
    const avgLatencyMs = Number(provider.latencyProbeLastAvgMs);
    if (Number.isFinite(avgLatencyMs) && avgLatencyMs > 0) {
      measured.push({ provider, avgLatencyMs });
    } else {
      pending.push({
        providerId: provider.id,
        name: provider.name,
        reason: provider.latencyProbeLastStatus === "failed" ? "failed" : "unmeasured",
      });
    }
  }

  const groupsByLatency = new Map<number, Provider[]>();
  for (const item of measured) {
    const bucket = groupsByLatency.get(item.avgLatencyMs);
    if (bucket) {
      bucket.push(item.provider);
    } else {
      groupsByLatency.set(item.avgLatencyMs, [item.provider]);
    }
  }

  const groups: LatencyPriorityPlan["groups"] = [];
  const changes: LatencyPriorityChange[] = [];
  const sortedLatencies = Array.from(groupsByLatency.keys()).sort((a, b) => a - b);

  for (const [priority, avgLatencyMs] of sortedLatencies.entries()) {
    const providers = groupsByLatency.get(avgLatencyMs) ?? [];
    const sortedProviders = providers.slice().sort((a, b) => a.id - b.id);
    groups.push({
      avgLatencyMs,
      priority,
      providers: sortedProviders.map((provider) => ({ id: provider.id, name: provider.name })),
    });

    for (const provider of sortedProviders) {
      const oldPriority = provider.groupPriorities?.[targetGroup] ?? null;
      if (oldPriority !== priority) {
        changes.push({
          providerId: provider.id,
          name: provider.name,
          oldPriority,
          newPriority: priority,
          avgLatencyMs,
        });
      }
    }
  }

  return {
    groups,
    changes,
    pending,
    summary: {
      totalProviders: input.providers.length,
      measuredCount: measured.length,
      pendingCount: pending.length,
      changedCount: changes.length,
      groupCount: groups.length,
    },
  };
}

export async function runLatencyPriorityWorkflow(
  input: LatencyPriorityWorkflowInput
): Promise<LatencyPriorityWorkflowResult> {
  const targetGroup = normalizeTargetGroup(input.targetGroup);
  const now = input.now?.() ?? Date.now();
  const providers = await collectProbeResults(input.providers, input.probeProviderLatency, now);
  const plan = buildLatencyPriorityPlan({ providers, targetGroup });

  if (input.confirm && plan.changes.length > 0) {
    if (!input.updateProviderGroupPriorities) {
      throw new Error("updateProviderGroupPriorities is required when confirm is true");
    }
    await input.updateProviderGroupPriorities(
      plan.changes.map((change) => {
        const provider = providers.find((item) => item.id === change.providerId);
        return {
          id: change.providerId,
          groupPriorities: {
            ...(provider?.groupPriorities ?? {}),
            [targetGroup]: change.newPriority,
          },
        };
      })
    );
  }

  return {
    ...plan,
    applied: input.confirm,
    targetGroup,
  };
}

async function collectProbeResults(
  providers: Provider[],
  probeProviderLatency: LatencyPriorityWorkflowInput["probeProviderLatency"],
  now: number
): Promise<Provider[]> {
  if (!probeProviderLatency) {
    return providers;
  }

  const result: Provider[] = [];
  for (const provider of providers) {
    const avgLatencyMs = await probeProviderLatency(provider, now);
    result.push({
      ...provider,
      latencyProbeLastAvgMs: avgLatencyMs,
      latencyProbeLastStatus: avgLatencyMs === null ? "failed" : "success",
      latencyProbeLastRunAt: new Date(now),
    });
  }
  return result;
}

function normalizeTargetGroup(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) {
    throw new Error("targetGroup is required");
  }
  return trimmed;
}
