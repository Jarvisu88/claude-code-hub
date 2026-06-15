import { describe, expect, it } from "vitest";
import type { Provider } from "@/types/provider";
import { selectProviderByStrategy } from "@/app/v1/_lib/proxy/provider-selector";

function p(over: Partial<Provider> = {}): Provider {
  return {
    id: 1,
    name: "P",
    isEnabled: true,
    providerType: "claude",
    groupTag: null,
    weight: 1,
    priority: 0,
    costMultiplier: 1,
    allowedModels: null,
    ...over,
  } as Provider;
}

describe("selectProviderByStrategy", () => {
  it("price: picks the cheapest (lowest costMultiplier) in the tier", () => {
    const chosen = selectProviderByStrategy(
      [p({ id: 1, costMultiplier: 3 }), p({ id: 2, costMultiplier: 1 }), p({ id: 3, costMultiplier: 2 })],
      "price",
      new Map()
    );
    expect(chosen.id).toBe(2);
  });

  it("latency: picks the fastest (lowest avg latency) in the tier", () => {
    const lat = new Map<number, number | null>([
      [1, 300],
      [2, 80],
      [3, 150],
    ]);
    const chosen = selectProviderByStrategy([p({ id: 1 }), p({ id: 2 }), p({ id: 3 })], "latency", lat);
    expect(chosen.id).toBe(2);
  });

  it("latency: provider with no latency data sorts last", () => {
    const lat = new Map<number, number | null>([[2, 500]]);
    const chosen = selectProviderByStrategy([p({ id: 1 }), p({ id: 2 })], "latency", lat);
    // id=2 has data (500), id=1 has none (Infinity) -> id=2 chosen
    expect(chosen.id).toBe(2);
  });

  it("price tie broken by id (stable, deterministic)", () => {
    const chosen = selectProviderByStrategy(
      [p({ id: 5, costMultiplier: 1 }), p({ id: 2, costMultiplier: 1 })],
      "price",
      new Map()
    );
    expect(chosen.id).toBe(2);
  });
});
