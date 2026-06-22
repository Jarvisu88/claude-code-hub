import { describe, expect, it } from "vitest";
import net from "node:net";
import {
  devToolsCall,
  devToolsCookieHeader,
  extractChromeAuthFromStorage,
  readChromeAuthForUpstream,
} from "@/lib/upstream-rate-sync/browser-auth";

function serverTextFrame(payload: Buffer): Buffer {
  if (payload.length < 126) {
    return Buffer.concat([Buffer.from([0x81, payload.length]), payload]);
  }
  return Buffer.concat([
    Buffer.from([0x81, 126, payload.length >> 8, payload.length & 0xff]),
    payload,
  ]);
}

describe("upstream browser auth extraction", () => {
  it("extracts Sub2API auth from nested persisted storage", () => {
    const result = extractChromeAuthFromStorage("sub2api", {
      "persist:auth": JSON.stringify({
        auth_token: "access-token",
        refresh_token: "refresh-token",
        token_expires_at: 1_710_000_000,
      }),
    });

    expect(result).toEqual({
      authToken: "access-token",
      refreshToken: "refresh-token",
      tokenExpiresAt: 1_710_000_000_000,
    });
  });

  it("extracts new-api token, user id, and cookie from browser state", () => {
    const result = extractChromeAuthFromStorage(
      "newapi",
      {
        state: JSON.stringify({
          user: {
            token: "new-token",
            id: 42,
          },
        }),
      },
      "session=abc"
    );

    expect(result).toEqual({
      token: "new-token",
      userId: 42,
      cookie: "session=abc",
    });
  });

  it("builds a cookie header from DevTools cookies including HttpOnly rows", () => {
    const header = devToolsCookieHeader(
      JSON.stringify({
        result: {
          cookies: [
            { name: "session", value: "abc", domain: ".vip.lyclaude.site", httpOnly: true },
            { name: "csrf", value: "def", domain: "vip.lyclaude.site" },
            { name: "other", value: "skip", domain: ".example.com" },
          ],
        },
      }),
      "https://vip.lyclaude.site/console/token"
    );

    expect(header).toBe("session=abc; csrf=def");
  });

  it("reads a DevTools response frame that arrives with the websocket handshake", async () => {
    const server = net.createServer((socket) => {
      socket.once("data", () => {
        const payload = Buffer.from(JSON.stringify({ id: 7, result: { ok: true } }));
        socket.write(
          Buffer.concat([
            Buffer.from(
              [
                "HTTP/1.1 101 Switching Protocols",
                "Upgrade: websocket",
                "Connection: Upgrade",
                "",
                "",
              ].join("\r\n")
            ),
            serverTextFrame(payload),
          ])
        );
      });
      socket.on("data", () => undefined);
    });

    await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
    const address = server.address();
    if (!address || typeof address === "string") {
      server.close();
      throw new Error("test server did not expose a TCP port");
    }

    try {
      const response = await devToolsCall(
        `ws://127.0.0.1:${address.port}/devtools/page/test`,
        7,
        "Network.getCookies",
        {}
      );
      expect(JSON.parse(response.toString("utf8"))).toEqual({
        id: 7,
        result: { ok: true },
      });
    } finally {
      server.close();
    }
  });

  it("finds the matching Chrome page on a scanned DevTools port", async () => {
    const fetchImpl = (async (url: string) => {
      if (url === "http://127.0.0.1:49521/json") {
        return new Response(JSON.stringify([{ type: "page", url: "https://other.example" }]));
      }
      if (url === "http://127.0.0.1:49522/json") {
        return new Response(
          JSON.stringify([
            {
              type: "page",
              url: "https://sub.nightyu.com/dashboard",
              webSocketDebuggerUrl: "ws://127.0.0.1:1/devtools/page/test",
            },
          ])
        );
      }
      throw new Error(`unexpected request ${url}`);
    }) as typeof fetch;

    await expect(
      readChromeAuthForUpstream("sub2api", "https://sub.nightyu.com", { fetchImpl })
    ).rejects.not.toThrow(/没有找到/);
  });
});
