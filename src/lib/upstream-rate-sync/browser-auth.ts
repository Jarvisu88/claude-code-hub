import "server-only";

import { randomBytes } from "node:crypto";
import { mkdtemp, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import net from "node:net";

import type { UpstreamRateSource } from "./clients";

const DEFAULT_CHROME_DEBUG_PORT = 49521;
const CHROME_DEBUG_PORT_SCAN_LIMIT = 20;
const CHROME_PAGE_DISCOVERY_ATTEMPTS = 20;

export type ChromeAuthFields =
  | {
      authToken: string;
      refreshToken: string;
      tokenExpiresAt: number;
      token?: never;
      userId?: never;
      cookie?: never;
    }
  | {
      token: string;
      userId: number;
      cookie: string;
      authToken?: never;
      refreshToken?: never;
      tokenExpiresAt?: never;
    };

interface ChromePage {
  type: string;
  url: string;
  webSocketDebuggerUrl?: string;
}

interface ChromeBrowserState {
  storage: Record<string, string>;
  cookie: string;
}

export async function openChromeForUpstreamAuth(
  baseUrl: string,
  options: { port?: number; chromePath?: string } = {}
): Promise<{ port: number; profileDir: string }> {
  const requestedPort = options.port ?? DEFAULT_CHROME_DEBUG_PORT;
  const port = await findAvailableChromeDebugPort(requestedPort);
  const chromePath = options.chromePath ?? (await findChromePath());
  const profileDir = await mkdtemp(path.join(tmpdir(), "cch-upstream-chrome-profile-"));

  const child = spawn(
    chromePath,
    [
      "--remote-debugging-address=127.0.0.1",
      `--remote-debugging-port=${port}`,
      "--remote-allow-origins=*",
      `--user-data-dir=${profileDir}`,
      "--no-first-run",
      "--no-default-browser-check",
      normalizeBrowserUrl(baseUrl),
    ],
    { detached: true, stdio: "ignore", windowsHide: false }
  );
  child.unref();

  return { port, profileDir };
}

export async function readChromeAuthForUpstream(
  source: UpstreamRateSource,
  baseUrl: string,
  options: { port?: number; fetchImpl?: typeof fetch } = {}
): Promise<ChromeAuthFields> {
  const state = await readChromeState(baseUrl, options);
  return extractChromeAuthFromStorage(source, state.storage, state.cookie);
}

export function extractChromeAuthFromStorage(
  source: UpstreamRateSource,
  storage: Record<string, string>,
  cookie = ""
): ChromeAuthFields {
  const flattened = flattenChromeStorage(storage);
  if (source === "sub2api") {
    const authToken = firstStorageValue(flattened, [
      "auth_token",
      "authtoken",
      "access_token",
      "accesstoken",
    ]);
    const refreshToken = firstStorageValue(flattened, ["refresh_token", "refreshtoken"]);
    if (!authToken && !refreshToken) {
      throw new Error("没有读取到 sub2api 登录态，请确认已在 CCH 打开的 Chrome 窗口完成登录");
    }
    return {
      authToken,
      refreshToken,
      tokenExpiresAt: storageInt(flattened, [
        "token_expires_at",
        "tokenexpiresat",
        "expires_at",
        "expiresat",
      ]),
    };
  }

  const token = firstStorageValue(flattened, [
    "auth_token",
    "authtoken",
    "access_token",
    "accesstoken",
    "token",
    "user.token",
    "state.user.token",
  ]);
  const userId = storageNumber(flattened, [
    "uid",
    "user_id",
    "userid",
    "id",
    "user.id",
    "state.user.id",
  ]);
  const trimmedCookie = cookie.trim();
  if (!token && !trimmedCookie) {
    throw new Error("没有读取到 new-api 登录态");
  }
  return { token, userId, cookie: trimmedCookie };
}

export function flattenChromeStorage(storage: Record<string, string>): Record<string, string> {
  const result: Record<string, string> = {};
  for (const [key, rawValue] of Object.entries(storage)) {
    const value = String(rawValue ?? "").trim();
    if (!value) continue;
    result[key] = value;

    try {
      const decoded = JSON.parse(value) as unknown;
      flattenChromeJson(result, key, decoded);
      const lowerKey = key.toLowerCase();
      if (
        lowerKey.includes("auth") ||
        lowerKey.includes("user") ||
        lowerKey.includes("state")
      ) {
        flattenChromeJson(result, "", decoded);
      }
    } catch {
      // Non-JSON storage values are still available by their original key.
    }
  }
  return result;
}

function flattenChromeJson(out: Record<string, string>, prefix: string, value: unknown): void {
  if (Array.isArray(value)) {
    value.forEach((child, index) => {
      flattenChromeJson(out, joinKey(prefix, String(index)), child);
    });
    return;
  }
  if (value && typeof value === "object") {
    for (const [key, child] of Object.entries(value)) {
      flattenChromeJson(out, joinKey(prefix, key), child);
    }
    return;
  }
  if (prefix && value != null) {
    const text = String(value).trim();
    if (text) out[prefix] = text;
  }
}

function joinKey(prefix: string, key: string): string {
  return prefix ? `${prefix}.${key}` : key;
}

function firstStorageValue(values: Record<string, string>, keys: string[]): string {
  for (const key of keys) {
    if (values[key]) return values[key];
  }
  for (const key of keys) {
    for (const [actual, value] of Object.entries(values)) {
      if (!value) continue;
      const lowerActual = actual.toLowerCase();
      if (lowerActual === key || lowerActual.endsWith(`.${key}`)) {
        return value;
      }
    }
  }
  return "";
}

function storageInt(values: Record<string, string>, keys: string[]): number {
  const parsed = storageNumber(values, keys);
  return parsed < 10_000_000_000 ? parsed * 1000 : parsed;
}

function storageNumber(values: Record<string, string>, keys: string[]): number {
  const raw = firstStorageValue(values, keys);
  const parsed = Number.parseInt(raw, 10);
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
}

async function readChromeState(
  baseUrl: string,
  options: { port?: number; fetchImpl?: typeof fetch }
): Promise<ChromeBrowserState> {
  const page = await findChromePage(baseUrl, options);
  if (!page.webSocketDebuggerUrl) {
    throw new Error("Chrome DevTools 页面缺少 websocket 地址");
  }
  const storage = await readChromeStorage(page.webSocketDebuggerUrl);
  const cookie = await readChromeCookie(page.webSocketDebuggerUrl, page.url).catch(() => "");
  return { storage, cookie };
}

async function findChromePage(
  baseUrl: string,
  options: { port?: number; fetchImpl?: typeof fetch }
): Promise<ChromePage> {
  const ports =
    options.port != null
      ? [options.port]
      : Array.from(
          { length: CHROME_DEBUG_PORT_SCAN_LIMIT },
          (_, index) => DEFAULT_CHROME_DEBUG_PORT + index
        );
  const fetchImpl = options.fetchImpl ?? globalThis.fetch;
  const targetHost = upstreamHost(baseUrl);
  for (let i = 0; i < CHROME_PAGE_DISCOVERY_ATTEMPTS; i += 1) {
    for (const port of ports) {
      try {
        const response = await fetchImpl(`http://127.0.0.1:${port}/json`);
        const pages = (await response.json()) as ChromePage[];
        for (const page of pages) {
          if (page.type !== "page") continue;
          const pageHost = upstreamHost(page.url);
          if (pageHost === targetHost || page.url.toLowerCase().includes(targetHost)) {
            return page;
          }
        }
      } catch {
        // Chrome may still be starting, or this port may not host DevTools.
      }
    }
    await new Promise((resolve) => setTimeout(resolve, 500));
  }
  throw new Error(`没有找到 ${targetHost} 的 Chrome 登录页`);
}

async function findAvailableChromeDebugPort(startPort: number): Promise<number> {
  for (let offset = 0; offset < CHROME_DEBUG_PORT_SCAN_LIMIT; offset += 1) {
    const port = startPort + offset;
    if (await canListenOnPort(port)) {
      return port;
    }
  }
  throw new Error(`没有可用的 Chrome DevTools 端口: ${startPort}`);
}

function canListenOnPort(port: number): Promise<boolean> {
  return new Promise((resolve) => {
    const server = net.createServer();
    server.once("error", () => resolve(false));
    server.once("listening", () => {
      server.close(() => resolve(true));
    });
    server.listen(port, "127.0.0.1");
  });
}

async function readChromeStorage(webSocketUrl: string): Promise<Record<string, string>> {
  const expression = `(() => {
const copy = (storage) => {
  const out = {};
  for (let i = 0; i < storage.length; i += 1) {
    const key = storage.key(i);
    out[key] = storage.getItem(key);
  }
  return out;
};
return {localStorage: copy(localStorage), sessionStorage: copy(sessionStorage)};
})()`;
  const out = await devToolsEvaluate<{
    localStorage: Record<string, string>;
    sessionStorage: Record<string, string>;
  }>(webSocketUrl, 1, expression);
  const storage: Record<string, string> = {};
  for (const [key, value] of Object.entries(out.localStorage ?? {})) {
    storage[key] = value;
    storage[`localStorage.${key}`] = value;
  }
  for (const [key, value] of Object.entries(out.sessionStorage ?? {})) {
    if (!storage[key]) storage[key] = value;
    storage[`sessionStorage.${key}`] = value;
  }
  return storage;
}

async function readChromeCookie(webSocketUrl: string, pageUrl: string): Promise<string> {
  const networkCookies = await devToolsCall(
    webSocketUrl,
    2,
    "Network.getCookies",
    { urls: [pageUrl] }
  ).catch(() => null);
  if (networkCookies) {
    const header = devToolsCookieHeader(networkCookies, pageUrl);
    if (header) return header;
  }

  const storageCookies = await devToolsCall(webSocketUrl, 3, "Storage.getCookies", {}).catch(
    () => null
  );
  if (storageCookies) {
    const header = devToolsCookieHeader(storageCookies, pageUrl);
    if (header) return header;
  }

  const expression = `document.cookie || (location.href === ${JSON.stringify(pageUrl)} ? "" : "")`;
  return devToolsEvaluate<string>(webSocketUrl, 4, expression);
}

export function devToolsCookieHeader(data: string | Buffer, pageUrl: string): string {
  const parsed = JSON.parse(data.toString()) as {
    result?: {
      cookies?: Array<{
        name?: unknown;
        value?: unknown;
        domain?: unknown;
      }>;
    };
  };
  const cookies = parsed.result?.cookies ?? [];
  const host = upstreamHost(pageUrl);
  const parts: string[] = [];
  for (const cookie of cookies) {
    const name = String(cookie.name ?? "").trim();
    if (!name) continue;
    if (!cookieDomainMatchesHost(cookie.domain, host)) continue;
    parts.push(`${name}=${String(cookie.value ?? "")}`);
  }
  return parts.join("; ");
}

function cookieDomainMatchesHost(domainValue: unknown, host: string): boolean {
  if (!host) return true;
  const domain = String(domainValue ?? "")
    .trim()
    .toLowerCase()
    .replace(/^\.+/, "");
  if (!domain) return true;
  const normalizedHost = host.toLowerCase();
  return normalizedHost === domain || normalizedHost.endsWith(`.${domain}`);
}

async function devToolsEvaluate<T>(
  webSocketUrl: string,
  id: number,
  expression: string
): Promise<T> {
  const response = await devToolsCall(webSocketUrl, id, "Runtime.evaluate", {
    expression,
    returnByValue: true,
    awaitPromise: true,
  });
  const parsed = JSON.parse(response.toString("utf8")) as {
    result?: { result?: { value?: T } };
  };
  if (!parsed.result?.result || !("value" in parsed.result.result)) {
    throw new Error("Chrome DevTools evaluate 响应无效");
  }
  return parsed.result.result.value as T;
}

export function devToolsCall(
  webSocketUrl: string,
  id: number,
  method: string,
  params: Record<string, unknown>
): Promise<Buffer> {
  const payload = JSON.stringify({ id, method, params });
  return sendDevToolsWebSocket(webSocketUrl, id, Buffer.from(payload));
}

function sendDevToolsWebSocket(webSocketUrl: string, id: number, payload: Buffer): Promise<Buffer> {
  const url = new URL(webSocketUrl);
  if (url.protocol !== "ws:") {
    return Promise.reject(new Error(`unsupported DevTools websocket scheme ${url.protocol}`));
  }

  return new Promise((resolve, reject) => {
    const socket = net.createConnection({ host: url.hostname, port: Number(url.port) || 80 });
    const chunks: Buffer[] = [];
    let settled = false;
    let frameBuffer = Buffer.alloc(0);

    const fail = (error: Error) => {
      if (settled) return;
      settled = true;
      reject(error);
      socket.destroy();
    };
    const succeed = (frame: Buffer) => {
      if (settled) return;
      settled = true;
      resolve(frame);
      socket.end();
    };
    const readResponseFrames = () => {
      try {
        while (frameBuffer.length > 0) {
          const frame = parseServerTextFrame(frameBuffer);
          if (!frame) return;
          frameBuffer = frameBuffer.subarray(frame.bytesRead);
          try {
            const envelope = JSON.parse(frame.payload.toString("utf8")) as { id?: number };
            if (envelope.id === id) {
              succeed(frame.payload);
              return;
            }
          } catch {
            // Ignore non-JSON frames and continue waiting for the matching CDP response.
          }
        }
      } catch (error) {
        fail(error instanceof Error ? error : new Error(String(error)));
      }
    };

    socket.setTimeout(10_000);
    socket.once("error", fail);
    socket.once("timeout", () => fail(new Error("Chrome DevTools websocket timeout")));
    socket.once("connect", () => {
      const secKey = randomBytes(16).toString("base64");
      socket.write(
        [
          `GET ${url.pathname}${url.search} HTTP/1.1`,
          `Host: ${url.host}`,
          "Upgrade: websocket",
          "Connection: Upgrade",
          "Sec-WebSocket-Version: 13",
          `Sec-WebSocket-Key: ${secKey}`,
          "Origin: http://127.0.0.1",
          "",
          "",
        ].join("\r\n")
      );
    });
    socket.on("data", (chunk) => {
      chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
      const data = Buffer.concat(chunks);
      const headerEnd = data.indexOf("\r\n\r\n");
      if (headerEnd < 0) return;
      const header = data.subarray(0, headerEnd).toString("utf8");
      if (!header.includes("101")) {
        fail(new Error(`Chrome DevTools websocket handshake failed: ${header.split("\r\n")[0]}`));
        return;
      }
      const remaining = data.subarray(headerEnd + 4);
      chunks.length = 0;
      socket.removeAllListeners("data");
      socket.write(buildClientTextFrame(payload));
      frameBuffer = remaining;
      readResponseFrames();
      socket.on("data", (frameChunk) => {
        const nextChunk = Buffer.isBuffer(frameChunk) ? frameChunk : Buffer.from(frameChunk);
        frameBuffer = Buffer.concat([frameBuffer, nextChunk]);
        readResponseFrames();
      });
    });
  });
}

function buildClientTextFrame(payload: Buffer): Buffer {
  const mask = randomBytes(4);
  const length = payload.length;
  const header =
    length < 126
      ? Buffer.from([0x81, length | 0x80])
      : Buffer.from([0x81, 126 | 0x80, length >> 8, length & 0xff]);
  const masked = Buffer.alloc(payload.length);
  for (let i = 0; i < payload.length; i += 1) {
    masked[i] = payload[i] ^ mask[i % 4];
  }
  return Buffer.concat([header, mask, masked]);
}

function parseServerTextFrame(buffer: Buffer): { payload: Buffer; bytesRead: number } | null {
  if (buffer.length < 2) return null;
  const opcode = buffer[0] & 0x0f;
  let offset = 2;
  let length = buffer[1] & 0x7f;
  if (length === 126) {
    if (buffer.length < 4) return null;
    length = buffer.readUInt16BE(2);
    offset = 4;
  } else if (length === 127) {
    throw new Error("Chrome DevTools websocket frame too large");
  }
  if (buffer.length < offset + length) return null;
  if (opcode === 0x8) throw new Error("Chrome DevTools websocket closed");
  return {
    payload: buffer.subarray(offset, offset + length),
    bytesRead: offset + length,
  };
}

async function findChromePath(): Promise<string> {
  const candidates = [
    path.join(process.env.ProgramFiles ?? "", "Google", "Chrome", "Application", "chrome.exe"),
    path.join(
      process.env["ProgramFiles(x86)"] ?? "",
      "Google",
      "Chrome",
      "Application",
      "chrome.exe"
    ),
    path.join(
      process.env.LocalAppData ?? "",
      "Google",
      "Chrome",
      "Application",
      "chrome.exe"
    ),
  ];
  for (const candidate of candidates) {
    if (!candidate) continue;
    try {
      await stat(candidate);
      return candidate;
    } catch {
      // Try next candidate.
    }
  }
  throw new Error("未找到 Google Chrome");
}

function normalizeBrowserUrl(value: string): string {
  const trimmed = value.trim().replace(/\/+$/, "");
  return trimmed.includes("://") ? trimmed : `https://${trimmed}`;
}

function upstreamHost(rawUrl: string): string {
  const normalized = normalizeBrowserUrl(rawUrl);
  try {
    return new URL(normalized).hostname.toLowerCase();
  } catch {
    return "";
  }
}
