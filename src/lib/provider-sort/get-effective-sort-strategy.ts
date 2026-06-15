import type { SortStrategy } from "./types";

/**
 * Resolve effective sort strategy: key takes priority when not none,
 * otherwise falls back to user; returns none when both are missing/none.
 * (Consistent with getEffectiveProviderGroup key||user semantics)
 */
export function resolveEffectiveSortStrategy(
  keyStrategy: SortStrategy | null | undefined,
  userStrategy: SortStrategy | null | undefined
): SortStrategy {
  const k = keyStrategy ?? "none";
  if (k !== "none") return k;
  return userStrategy ?? "none";
}
