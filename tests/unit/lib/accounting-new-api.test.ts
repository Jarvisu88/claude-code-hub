import { describe, expect, test } from "vitest";
import {
  buildNewApiLogsUrl,
  buildNewApiPricingUrl,
  buildNewApiStatusUrl,
  parseNewApiLogsPayload,
  parseNewApiPricingPayload,
  parseNewApiQuotaPerUnit,
} from "@/lib/accounting/new-api";

describe("new-api accounting helpers", () => {
  test("builds the consume log list URL", () => {
    const url = new URL(
      buildNewApiLogsUrl("https://new-api.example.com/base", {
        startTimestamp: 1778688000,
        endTimestamp: 1778774399,
        page: 2,
        pageSize: 100,
      })
    );

    expect(url.origin).toBe("https://new-api.example.com");
    expect(url.pathname).toBe("/api/log/");
    expect(url.searchParams.get("type")).toBe("2");
    expect(url.searchParams.get("start_timestamp")).toBe("1778688000");
    expect(url.searchParams.get("end_timestamp")).toBe("1778774399");
    expect(url.searchParams.get("p")).toBe("2");
    expect(url.searchParams.get("page_size")).toBe("100");
  });

  test("reads paged consume logs and total count", () => {
    const result = parseNewApiLogsPayload({
      success: true,
      data: {
        total: 12,
        items: [
          {
            username: "alice",
            group: "vip",
            quota: 250000,
            channel: 3,
            channel_name: "provider-a",
          },
        ],
      },
    });

    expect(result.total).toBe(12);
    expect(result.logs).toEqual([
      {
        username: "alice",
        group: "vip",
        quota: 250000,
        channel: 3,
        channel_name: "provider-a",
      },
    ]);
  });

  test("rejects failed new-api responses", () => {
    expect(() => parseNewApiLogsPayload({ success: false, message: "access denied" })).toThrow(
      "access denied"
    );
  });

  test("builds pricing and status URLs", () => {
    expect(buildNewApiPricingUrl("https://new-api.example.com/base")).toBe(
      "https://new-api.example.com/api/pricing"
    );
    expect(buildNewApiStatusUrl("https://new-api.example.com/base")).toBe(
      "https://new-api.example.com/api/status"
    );
  });

  test("reads model pricing and group ratios", () => {
    const result = parseNewApiPricingPayload({
      success: true,
      data: [
        {
          model_name: "gpt-4o",
          quota_type: 0,
          model_ratio: 2,
          completion_ratio: 3,
          enable_groups: ["vip"],
        },
      ],
      group_ratio: {
        default: 1,
        vip: "1.5",
      },
    });

    expect(result.pricing).toHaveLength(1);
    expect(result.groupRatios.get("default")).toBe(1);
    expect(result.groupRatios.get("vip")).toBe(1.5);
  });

  test("reads quota per USD unit", () => {
    expect(parseNewApiQuotaPerUnit({ success: true, data: { quota_per_unit: 500000 } })).toBe(
      500000
    );
  });

  test("uses a conservative default page size in URL builder inputs", () => {
    const url = new URL(
      buildNewApiLogsUrl("https://new-api.example.com", {
        startTimestamp: 1778688000,
        endTimestamp: 1778774399,
        page: 1,
        pageSize: 100,
      })
    );

    expect(url.searchParams.get("page_size")).toBe("100");
  });
});
