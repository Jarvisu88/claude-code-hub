export type SortStrategy = "none" | "price" | "latency";

export interface ProviderLatencySample {
  providerId: number;
  /** 3 successful probes average; null when all probes failed (treated as slowest during sort) */
  avgLatencyMs: number | null;
  /** epoch ms */
  sampledAt: number;
  source: "probe" | "passive";
}
