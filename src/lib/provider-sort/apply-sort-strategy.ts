import type { Provider } from "@/types/provider";

/**
 * Deterministic sort within candidate list by strategy (pure function, does not mutate input).
 * - price: costMultiplier ascending (cheaper first)
 * - latency: average latency ascending (faster first); missing data (null / absent) treated as +Infinity (last)
 * Tie-break by id ascending for stable ordering.
 */
export function applySortStrategy(
  candidates: Provider[],
  strategy: "price" | "latency",
  latencyMap: Map<number, number | null>
): Provider[] {
  return [...candidates].sort((a, b) => {
    if (strategy === "price") {
      if (a.costMultiplier !== b.costMultiplier) return a.costMultiplier - b.costMultiplier;
      return a.id - b.id;
    }
    const la = latencyMap.get(a.id) ?? Number.POSITIVE_INFINITY;
    const lb = latencyMap.get(b.id) ?? Number.POSITIVE_INFINITY;
    if (la !== lb) return la - lb;
    return a.id - b.id;
  });
}
