import { PROVIDER_GROUP } from "@/lib/constants/provider.constants";
import type { SortStrategy } from "./types";

interface GroupOwner {
  providerGroup?: string | null;
  priceProviderGroup?: string | null;
  latencyProviderGroup?: string | null;
}

interface ResolveEffectiveProviderGroupInput {
  sortStrategy: SortStrategy;
  key?: GroupOwner | null;
  user?: GroupOwner | null;
}

function normalizeGroup(group: string | null | undefined): string | null {
  const trimmed = typeof group === "string" ? group.trim() : "";
  return trimmed ? trimmed : null;
}

function firstConfiguredGroup(...groups: Array<string | null | undefined>): string | null {
  for (const group of groups) {
    const normalized = normalizeGroup(group);
    if (normalized) return normalized;
  }
  return null;
}

/**
 * Resolve the provider group used by the active routing strategy.
 *
 * `providerGroup` remains the compatibility/default group. Price and latency
 * groups are strategy-specific overlays so configuring one does not affect
 * the other.
 */
export function resolveEffectiveProviderGroupForStrategy({
  sortStrategy,
  key,
  user,
}: ResolveEffectiveProviderGroupInput): string | null {
  if (!key && !user) {
    return null;
  }

  const strategyGroup =
    sortStrategy === "price"
      ? firstConfiguredGroup(key?.priceProviderGroup, user?.priceProviderGroup)
      : sortStrategy === "latency"
        ? firstConfiguredGroup(key?.latencyProviderGroup, user?.latencyProviderGroup)
        : null;

  return (
    strategyGroup ??
    firstConfiguredGroup(key?.providerGroup, user?.providerGroup) ??
    PROVIDER_GROUP.DEFAULT
  );
}
