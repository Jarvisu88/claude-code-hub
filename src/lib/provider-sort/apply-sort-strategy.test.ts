import { describe, expect, it } from "vitest";
import type { Provider } from "@/types/provider";
import { applySortStrategy } from "./apply-sort-strategy";

function p(over: Partial<Provider> = {}): Provider {
  // Only fill fields used by sort; cast the rest since the sort function does not read them
  return {
    id: 1,
    name: "P",
    costMultiplier: 1,
    weight: 1,
    priority: 0,
    ...over,
  } as Provider;
}

describe("applySortStrategy", () => {
  it("price: sorts by costMultiplier ascending, id tie-break", () => {
    const out = applySortStrategy(
      [
        p({ id: 1, costMultiplier: 2 }),
        p({ id: 2, costMultiplier: 1 }),
        p({ id: 3, costMultiplier: 1 }),
      ],
      "price",
      new Map()
    );
    expect(out.map((x) => x.id)).toEqual([2, 3, 1]);
  });

  it("latency: sorts by latencyMap ascending, null last, id tie-break", () => {
    const lat = new Map<number, number | null>([
      [1, 300],
      [2, 100],
      [3, null],
    ]);
    const out = applySortStrategy([p({ id: 1 }), p({ id: 2 }), p({ id: 3 })], "latency", lat);
    expect(out.map((x) => x.id)).toEqual([2, 1, 3]);
  });

  it("latency: missing from map treated as Infinity (last)", () => {
    const lat = new Map<number, number | null>([[2, 50]]);
    const out = applySortStrategy([p({ id: 1 }), p({ id: 2 })], "latency", lat);
    expect(out.map((x) => x.id)).toEqual([2, 1]);
  });

  it("does not mutate input array", () => {
    const input = [p({ id: 1, costMultiplier: 2 }), p({ id: 2, costMultiplier: 1 })];
    const copy = [...input];
    applySortStrategy(input, "price", new Map());
    expect(input.map((x) => x.id)).toEqual(copy.map((x) => x.id));
  });

  it("handles empty and single candidate", () => {
    expect(applySortStrategy([], "price", new Map())).toEqual([]);
    const one = applySortStrategy([p({ id: 9 })], "latency", new Map());
    expect(one.map((x) => x.id)).toEqual([9]);
  });
});
