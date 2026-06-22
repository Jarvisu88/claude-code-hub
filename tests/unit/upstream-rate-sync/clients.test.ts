import { describe, expect, it, vi } from "vitest";
import {
  fetchNewApiProviderRate,
  fetchSub2ApiProviderRate,
} from "@/lib/upstream-rate-sync/clients";

function jsonResponse(body: unknown, init?: ResponseInit): Response {
  return new Response(JSON.stringify(body), {
    status: init?.status ?? 200,
    headers: { "content-type": "application/json" },
  });
}

describe("upstream rate sync clients", () => {
  it("refreshes Sub2API auth and reads the configured key group rate", async () => {
    const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
      if (url === "https://sub.example/api/v1/auth/refresh") {
        expect(init?.method).toBe("POST");
        return jsonResponse({
          code: 0,
          data: {
            access_token: "access-next",
            refresh_token: "refresh-next",
            expires_in: 3600,
          },
        });
      }
      if (url === "https://sub.example/api/v1/keys?page=1&page_size=100") {
        expect(init?.headers).toMatchObject({ Authorization: "Bearer access-next" });
        return jsonResponse({
          code: 0,
          data: {
            items: [
              {
                id: 7,
                key: "sk-target",
                name: "target",
                group_id: 3,
                group: { id: 3, name: "vip", balance_charge_rate: 0.42 },
              },
            ],
            total: 1,
          },
        });
      }
      if (url === "https://sub.example/api/v1/groups/rates") {
        return jsonResponse({ code: 0, data: {} });
      }
      throw new Error(`unexpected request ${url}`);
    });

    const result = await fetchSub2ApiProviderRate(
      {
        baseUrl: "https://sub.example/",
        apiKey: "sk-target",
        refreshToken: "refresh-old",
      },
      { fetchImpl, now: () => 1_000 }
    );

    expect(result.rate).toBe(0.42);
    expect(result.auth?.accessToken).toBe("access-next");
    expect(result.auth?.refreshToken).toBe("refresh-next");
    expect(result.auth?.tokenExpiresAt).toBe(3_601_000);
  });

  it("refreshes Sub2API auth once when the saved access token is unauthorized", async () => {
    const fetchImpl = vi.fn(async (url: string) => {
      if (url === "https://sub.example/api/v1/keys?page=1&page_size=100") {
        if (fetchImpl.mock.calls.length === 1) {
          return jsonResponse({ message: "unauthorized" }, { status: 401 });
        }
        return jsonResponse({
          code: 0,
          data: {
            items: [
              {
                id: 9,
                key: "sk-target",
                group: { id: 4, name: "pro", balance_charge_rate: 1.25 },
              },
            ],
            total: 1,
          },
        });
      }
      if (url === "https://sub.example/api/v1/groups/rates") {
        return jsonResponse({ code: 0, data: {} });
      }
      if (url === "https://sub.example/api/v1/auth/refresh") {
        return jsonResponse({
          code: 0,
          data: {
            access_token: "access-refreshed",
            refresh_token: "refresh-refreshed",
            expires_in: 60,
          },
        });
      }
      throw new Error(`unexpected request ${url}`);
    });

    const result = await fetchSub2ApiProviderRate(
      {
        baseUrl: "https://sub.example",
        apiKey: "sk-target",
        accessToken: "access-stale",
        refreshToken: "refresh-current",
        tokenExpiresAt: 999_999,
      },
      { fetchImpl, now: () => 2_000 }
    );

    expect(result.rate).toBe(1.25);
    expect(result.refreshed).toBe(true);
    expect(fetchImpl).toHaveBeenCalledTimes(4);
  });

  it("fetches the full Sub2API group when the key embeds only group metadata", async () => {
    const fetchImpl = vi.fn(async (url: string) => {
      if (url === "https://sub.example/api/v1/keys?page=1&page_size=100") {
        return jsonResponse({
          code: 0,
          data: {
            items: [
              {
                id: 9,
                key: "sk-target",
                group_id: 4,
                group: { id: 4, name: "pro" },
              },
            ],
            total: 1,
          },
        });
      }
      if (url === "https://sub.example/api/v1/groups/available") {
        return jsonResponse({
          code: 0,
          data: [{ id: 4, name: "pro", balance_charge_rate: 1.5 }],
        });
      }
      if (url === "https://sub.example/api/v1/groups/rates") {
        return jsonResponse({ code: 0, data: {} });
      }
      throw new Error(`unexpected request ${url}`);
    });

    const result = await fetchSub2ApiProviderRate(
      {
        baseUrl: "https://sub.example",
        apiKey: "sk-target",
        accessToken: "access",
        tokenExpiresAt: 999_999,
      },
      { fetchImpl, now: () => 2_000 }
    );

    expect(result.rate).toBe(1.5);
    expect(result.group?.name).toBe("pro");
    expect(fetchImpl).toHaveBeenCalledWith(
      "https://sub.example/api/v1/groups/available",
      expect.any(Object)
    );
  });

  it("falls back to Sub2API rate_multiplier when balance_charge_rate is absent", async () => {
    const fetchImpl = vi.fn(async (url: string) => {
      if (url === "https://sub.example/api/v1/keys?page=1&page_size=100") {
        return jsonResponse({
          code: 0,
          data: {
            items: [
              {
                id: 9,
                key: "sk-target",
                group_id: 4,
                group: { id: 4, name: "pro", rate_multiplier: 0.9 },
              },
            ],
            total: 1,
          },
        });
      }
      if (url === "https://sub.example/api/v1/groups/rates") {
        return jsonResponse({ code: 0, data: {} });
      }
      throw new Error(`unexpected request ${url}`);
    });

    const result = await fetchSub2ApiProviderRate(
      {
        baseUrl: "https://sub.example",
        apiKey: "sk-target",
        accessToken: "access",
        tokenExpiresAt: 999_999,
      },
      { fetchImpl, now: () => 2_000 }
    );

    expect(result.rate).toBe(0.9);
    expect(result.group?.raw).toMatchObject({ rate_multiplier: 0.9 });
  });

  it("uses Sub2API user group rates before the public group multiplier", async () => {
    const fetchImpl = vi.fn(async (url: string) => {
      if (url === "https://sub.example/api/v1/keys?page=1&page_size=100") {
        return jsonResponse({
          code: 0,
          data: {
            items: [
              {
                id: 9,
                key: "sk-target",
                group_id: 2,
                group: { id: 2, name: "gpt-走量", rate_multiplier: 0.1 },
              },
            ],
            total: 1,
          },
        });
      }
      if (url === "https://sub.example/api/v1/groups/rates") {
        return jsonResponse({ code: 0, data: { "2": 0.05 } });
      }
      throw new Error(`unexpected request ${url}`);
    });

    const result = await fetchSub2ApiProviderRate(
      {
        baseUrl: "https://sub.example",
        apiKey: "sk-target",
        accessToken: "access",
        tokenExpiresAt: 999_999,
      },
      { fetchImpl, now: () => 2_000 }
    );

    expect(result.rate).toBe(0.05);
    expect(result.group?.raw).toMatchObject({ rate_multiplier: 0.1 });
  });

  it("reads New API token group ratio as the provider rate", async () => {
    const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
      if (url === "https://new.example/api/token/search?p=0&size=100&token=target") {
        expect(init?.headers).toMatchObject({
          Authorization: "Bearer access",
          "New-Api-User": "1001",
        });
        return jsonResponse({
          success: true,
          data: {
            items: [{ id: 5, key: "sk-target", name: "target", group: "vip" }],
            total: 1,
          },
        });
      }
      if (url === "https://new.example/api/user/self/groups") {
        return jsonResponse({
          success: true,
          data: {
            vip: { ratio: 0.8, desc: "VIP" },
          },
        });
      }
      throw new Error(`unexpected request ${url}`);
    });

    const result = await fetchNewApiProviderRate(
      {
        baseUrl: "https://new.example",
        apiKey: "sk-target",
        accessToken: "access",
        userId: "1001",
      },
      { fetchImpl }
    );

    expect(result.rate).toBe(0.8);
    expect(result.key.name).toBe("target");
    expect(result.group?.name).toBe("vip");
  });

  it("sends new-api browser cookie and matches masked token by key suffix", async () => {
    const fetchImpl = vi.fn(async (url: string, init?: RequestInit) => {
      if (url === "https://new.example/api/token/search?p=0&size=100&token=targetsecret") {
        expect(init?.headers).toMatchObject({
          Authorization: "Bearer browser-access",
          Cookie: "session=abc",
          "New-Api-User": "42",
        });
        return jsonResponse({
          success: true,
          data: {
            items: [{ id: 5, key: "targ****cret", name: "target", group: "vip" }],
            total: 1,
          },
        });
      }
      if (url === "https://new.example/api/user/self/groups") {
        expect(init?.headers).toMatchObject({ Cookie: "session=abc" });
        return jsonResponse({
          success: true,
          data: {
            vip: { ratio: 0.8 },
          },
        });
      }
      throw new Error(`unexpected request ${url}`);
    });

    const result = await fetchNewApiProviderRate(
      {
        baseUrl: "https://new.example",
        apiKey: "sk-targetsecret",
        accessToken: "browser-access",
        cookie: "session=abc",
        userId: "42",
      },
      { fetchImpl }
    );

    expect(result.rate).toBe(0.8);
    expect(result.key.keyPrefix).toBe("targ****...cret");
  });
});
