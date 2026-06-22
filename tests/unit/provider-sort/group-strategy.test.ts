import { describe, expect, test } from "vitest";
import { PROVIDER_GROUP } from "@/lib/constants/provider.constants";
import { resolveEffectiveProviderGroupForStrategy } from "@/lib/provider-sort/group-strategy";

describe("resolveEffectiveProviderGroupForStrategy", () => {
  test("uses price provider group only for price strategy", () => {
    const group = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "price",
      key: {
        providerGroup: "default",
        priceProviderGroup: "cheap-key",
        latencyProviderGroup: "fast-key",
      },
      user: {
        providerGroup: "user-default",
        priceProviderGroup: "cheap-user",
        latencyProviderGroup: "fast-user",
      },
    });

    expect(group).toBe("cheap-key");
  });

  test("uses latency provider group only for latency strategy", () => {
    const group = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "latency",
      key: {
        providerGroup: "default",
        priceProviderGroup: "cheap-key",
        latencyProviderGroup: "fast-key",
      },
      user: {
        providerGroup: "user-default",
        priceProviderGroup: "cheap-user",
        latencyProviderGroup: "fast-user",
      },
    });

    expect(group).toBe("fast-key");
  });

  test("falls back from key strategy group to user strategy group", () => {
    const priceGroup = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "price",
      key: { providerGroup: "legacy-key", priceProviderGroup: "", latencyProviderGroup: null },
      user: {
        providerGroup: "legacy-user",
        priceProviderGroup: "cheap-user",
        latencyProviderGroup: "fast-user",
      },
    });

    const latencyGroup = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "latency",
      key: { providerGroup: "", priceProviderGroup: "cheap-key", latencyProviderGroup: "" },
      user: {
        providerGroup: "legacy-user",
        priceProviderGroup: "cheap-user",
        latencyProviderGroup: "fast-user",
      },
    });

    expect(priceGroup).toBe("cheap-user");
    expect(latencyGroup).toBe("fast-user");
  });

  test("falls back to legacy key and user provider groups when strategy groups are empty", () => {
    const priceGroup = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "price",
      key: { providerGroup: "legacy-key", priceProviderGroup: "", latencyProviderGroup: null },
      user: { providerGroup: "legacy-user", priceProviderGroup: "", latencyProviderGroup: "" },
    });

    const latencyGroup = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "latency",
      key: { providerGroup: "", priceProviderGroup: "", latencyProviderGroup: "" },
      user: { providerGroup: "legacy-user", priceProviderGroup: "", latencyProviderGroup: "" },
    });

    expect(priceGroup).toBe("legacy-key");
    expect(latencyGroup).toBe("legacy-user");
  });

  test("uses legacy provider group for none strategy", () => {
    const group = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "none",
      key: {
        providerGroup: null,
        priceProviderGroup: "cheap-key",
        latencyProviderGroup: "fast-key",
      },
      user: {
        providerGroup: "legacy-user",
        priceProviderGroup: "cheap-user",
        latencyProviderGroup: "fast-user",
      },
    });

    expect(group).toBe("legacy-user");
  });

  test("returns default when auth exists but all groups are empty", () => {
    const group = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "latency",
      key: { providerGroup: null, priceProviderGroup: null, latencyProviderGroup: null },
      user: { providerGroup: "", priceProviderGroup: "", latencyProviderGroup: "" },
    });

    expect(group).toBe(PROVIDER_GROUP.DEFAULT);
  });

  test("returns null when there is no auth state", () => {
    const group = resolveEffectiveProviderGroupForStrategy({
      sortStrategy: "price",
      key: null,
      user: null,
    });

    expect(group).toBeNull();
  });
});
