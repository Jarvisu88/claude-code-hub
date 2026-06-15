import { describe, expect, it } from "vitest";
import { resolveEffectiveSortStrategy } from "./get-effective-sort-strategy";

describe("resolveEffectiveSortStrategy", () => {
  it("uses key strategy when key is not none", () => {
    expect(resolveEffectiveSortStrategy("price", "latency")).toBe("price");
  });
  it("falls back to user when key is none", () => {
    expect(resolveEffectiveSortStrategy("none", "latency")).toBe("latency");
  });
  it("returns none when both none", () => {
    expect(resolveEffectiveSortStrategy("none", "none")).toBe("none");
  });
  it("treats null/undefined as none", () => {
    expect(resolveEffectiveSortStrategy(null, undefined)).toBe("none");
    expect(resolveEffectiveSortStrategy(undefined, "price")).toBe("price");
  });
});
