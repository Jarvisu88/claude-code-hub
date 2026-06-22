/**
 * @vitest-environment happy-dom
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NextIntlClientProvider } from "next-intl";
import { type ReactNode, act } from "react";
import { createRoot } from "react-dom/client";
import { beforeEach, describe, expect, test, vi } from "vitest";
import { ProviderRichListItem } from "@/app/[locale]/settings/providers/_components/provider-rich-list-item";
import type { ProviderDisplay } from "@/types/provider";
import type { User } from "@/types/user";
import enMessages from "../../../../messages/en";

// Mock dependencies
vi.mock("next/navigation", () => ({
  useRouter: () => ({ refresh: vi.fn() }),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

// Mock actions
const providerEndpointsActionMocks = vi.hoisted(() => ({
  getProviderVendors: vi.fn(async () => [
    {
      id: 101,
      displayName: "Anthropic",
      websiteDomain: "anthropic.com",
      websiteUrl: "https://anthropic.com",
      faviconUrl: null,
      createdAt: "2026-01-01",
      updatedAt: "2026-01-01",
    },
  ]),
  getProviderEndpointsByVendor: vi.fn(async () => []),
}));
vi.mock("@/actions/provider-endpoints", () => providerEndpointsActionMocks);

const providersActionMocks = vi.hoisted(() => ({
  editProvider: vi.fn(async () => ({ ok: true })),
  removeProvider: vi.fn(async () => ({ ok: true })),
  getUnmaskedProviderKey: vi.fn(async () => ({ ok: true, data: { key: "sk-test" } })),
  resetProviderCircuit: vi.fn(async () => ({ ok: true })),
  resetProviderTotalUsage: vi.fn(async () => ({ ok: true })),
  getProviderUpstreamRateSyncConfig: vi.fn(async () => ({ ok: true, data: null })),
  runProviderLatencyProbe: vi.fn(async () => ({
    ok: true,
    data: { avgLatencyMs: 123, sampledAt: 1, status: "success" },
  })),
  syncProviderUpstreamRateNow: vi.fn(async () => ({ ok: true, data: {} })),
}));
vi.mock("@/lib/api-client/v1/actions/providers", () => providersActionMocks);

// Mock tooltip to simplify testing
vi.mock("@/components/ui/tooltip", () => ({
  Tooltip: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipTrigger: ({ children }: { children: ReactNode }) => <>{children}</>,
  TooltipContent: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  TooltipProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

// Mock ProviderEndpointHover to avoid complex children rendering if needed,
// but we want to check if it's rendered.
// Actually, let's NOT mock it fully, or mock it to render a simple test id.
vi.mock("@/app/[locale]/settings/providers/_components/provider-endpoint-hover", () => ({
  ProviderEndpointHover: ({ vendorId }: { vendorId: number }) => (
    <div data-testid="mock-endpoint-hover">Endpoints for Vendor {vendorId}</div>
  ),
}));

const ADMIN_USER: User = {
  id: 1,
  name: "admin",
  description: "",
  role: "admin",
  rpm: null,
  dailyQuota: null,
  providerGroup: null,
  tags: [],
  createdAt: new Date("2026-01-01"),
  updatedAt: new Date("2026-01-01"),
  dailyResetMode: "fixed",
  dailyResetTime: "00:00",
  isEnabled: true,
};

function makeProviderDisplay(overrides: Partial<ProviderDisplay> = {}): ProviderDisplay {
  return {
    id: 1,
    name: "Claude 3.5 Sonnet",
    url: "https://api.anthropic.com",
    maskedKey: "sk-***",
    isEnabled: true,
    weight: 1,
    priority: 1,
    costMultiplier: 1,
    groupTag: null,
    providerType: "claude",
    providerVendorId: null, // Default to null for legacy check
    preserveClientIp: false,
    modelRedirects: null,
    allowedModels: null,
    mcpPassthroughType: "none",
    mcpPassthroughUrl: null,
    limit5hUsd: null,
    limitDailyUsd: null,
    dailyResetMode: "fixed",
    dailyResetTime: "00:00",
    limitWeeklyUsd: null,
    limitMonthlyUsd: null,
    limitTotalUsd: null,
    limitConcurrentSessions: 1,
    maxRetryAttempts: null,
    circuitBreakerFailureThreshold: 1,
    circuitBreakerOpenDuration: 60,
    circuitBreakerHalfOpenSuccessThreshold: 1,
    proxyUrl: null,
    proxyFallbackToDirect: false,
    firstByteTimeoutStreamingMs: 0,
    streamingIdleTimeoutMs: 0,
    requestTimeoutNonStreamingMs: 0,
    websiteUrl: null,
    faviconUrl: null,
    cacheTtlPreference: null,
    context1mPreference: null,
    codexReasoningEffortPreference: null,
    codexReasoningSummaryPreference: null,
    codexTextVerbosityPreference: null,
    codexParallelToolCallsPreference: null,
    anthropicMaxTokensPreference: null,
    anthropicThinkingBudgetPreference: null,
    activeTimeStart: null,
    activeTimeEnd: null,
    allowedClients: [],
    blockedClients: [],
    groupPriorities: null,
    isFailoverOnly: false,
    latencyProbeEnabled: null,
    latencyProbeIntervalMs: null,
    latencyProbeLastAvgMs: null,
    latencyProbeLastError: null,
    latencyProbeLastRunAt: null,
    latencyProbeLastStatus: null,
    latencyProbeModel: null,
    latencyProbeTimeEnd: null,
    latencyProbeTimeStart: null,
    upstreamRateSync: null,
    tpm: null,
    rpm: null,
    rpd: null,
    cc: null,
    createdAt: "2026-01-01",
    updatedAt: "2026-01-01",
    ...overrides,
  };
}

let queryClient: QueryClient;

function renderWithProviders(node: ReactNode) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);

  act(() => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
          {node}
        </NextIntlClientProvider>
      </QueryClientProvider>
    );
  });

  return {
    unmount: () => {
      act(() => root.unmount());
      container.remove();
    },
    container,
  };
}

async function flushTicks(times = 3) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await new Promise((r) => setTimeout(r, 0));
    });
  }
}

async function waitForClickDelay() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 280));
  });
  await flushTicks(2);
}

describe("ProviderRichListItem Endpoint Display", () => {
  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
      },
    });
    vi.clearAllMocks();
    while (document.body.firstChild) {
      document.body.removeChild(document.body.firstChild);
    }
  });

  test("renders legacy URL when providerVendorId is null", async () => {
    const provider = makeProviderDisplay({
      providerVendorId: null,
      url: "https://api.legacy.com",
    });

    const { unmount } = renderWithProviders(
      <ProviderRichListItem
        provider={provider}
        currentUser={ADMIN_USER}
        enableMultiProviderTypes={true}
      />
    );

    await flushTicks(5);

    expect(document.body.textContent).toContain("https://api.legacy.com");
    expect(document.body.textContent).not.toContain("Anthropic");
    expect(document.querySelector('[data-testid="mock-endpoint-hover"]')).toBeNull();

    unmount();
  });

  test("renders vendor name and endpoint hover when providerVendorId exists", async () => {
    const provider = makeProviderDisplay({
      providerVendorId: 101,
      url: "https://api.anthropic.com",
    });

    const { unmount } = renderWithProviders(
      <ProviderRichListItem
        provider={provider}
        vendor={{
          id: 101,
          websiteDomain: "anthropic.com",
          displayName: "Anthropic",
          websiteUrl: "https://anthropic.com",
          faviconUrl: null,
          createdAt: new Date("2026-02-01T00:00:00Z"),
          updatedAt: new Date("2026-02-01T00:00:00Z"),
        }}
        currentUser={ADMIN_USER}
        enableMultiProviderTypes={true}
      />
    );

    await flushTicks(5); // Wait for query to resolve

    // Should show vendor name (mocked as "Anthropic")
    expect(document.body.textContent).toContain("Anthropic");

    // Should NOT show the raw URL in the main label position (though it might be in tooltip, but here we check main text replacement)
    // The implementation replaces the URL span with the vendor/hover block

    // Should render the mock endpoint hover
    expect(document.querySelector('[data-testid="mock-endpoint-hover"]')).not.toBeNull();
    expect(document.body.textContent).toContain("Endpoints for Vendor 101");

    unmount();
  });

  test("renders timeout summary as 0s when provider timeouts are disabled", async () => {
    const provider = makeProviderDisplay({
      firstByteTimeoutStreamingMs: 0,
      streamingIdleTimeoutMs: 0,
      requestTimeoutNonStreamingMs: 0,
    });

    const { unmount } = renderWithProviders(
      <ProviderRichListItem
        provider={provider}
        currentUser={ADMIN_USER}
        enableMultiProviderTypes={true}
      />
    );

    await flushTicks(5);

    expect(document.body.textContent).toContain("First byte: 0s");
    expect(document.body.textContent).toContain("Stream interval: 0s");
    expect(document.body.textContent).toContain("Non-streaming: 0s");

    unmount();
  });

  test("single click runs upstream rate sync and double click opens its editor", async () => {
    const provider = makeProviderDisplay();

    const { unmount } = renderWithProviders(
      <ProviderRichListItem
        provider={provider}
        currentUser={ADMIN_USER}
        enableMultiProviderTypes={true}
      />
    );

    await flushTicks(5);

    const rateSyncButton = Array.from(document.querySelectorAll("button")).find(
      (button) => button.getAttribute("title") === "Upstream Rate Sync"
    );
    expect(rateSyncButton).toBeTruthy();

    await act(async () => {
      rateSyncButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    await waitForClickDelay();

    expect(providersActionMocks.syncProviderUpstreamRateNow).toHaveBeenCalledWith(provider.id);

    providersActionMocks.syncProviderUpstreamRateNow.mockClear();

    await act(async () => {
      rateSyncButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      rateSyncButton?.dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    });
    await waitForClickDelay();

    expect(providersActionMocks.syncProviderUpstreamRateNow).not.toHaveBeenCalled();
    expect(providersActionMocks.getProviderUpstreamRateSyncConfig).toHaveBeenCalledWith(
      provider.id
    );
    expect(document.body.textContent).toContain("Upstream Rate Sync");

    unmount();
  }, 30000);

  test("single click runs Mini probe and double click opens its editor", async () => {
    const provider = makeProviderDisplay();

    const { unmount } = renderWithProviders(
      <ProviderRichListItem
        provider={provider}
        currentUser={ADMIN_USER}
        enableMultiProviderTypes={true}
      />
    );

    await flushTicks(5);

    const probeButton = Array.from(document.querySelectorAll("button")).find(
      (button) => button.getAttribute("aria-label") === "Mini probe"
    );
    expect(probeButton).toBeTruthy();

    await act(async () => {
      probeButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    await waitForClickDelay();

    expect(providersActionMocks.runProviderLatencyProbe).toHaveBeenCalledWith(provider.id);

    providersActionMocks.runProviderLatencyProbe.mockClear();

    await act(async () => {
      probeButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      probeButton?.dispatchEvent(new MouseEvent("dblclick", { bubbles: true }));
    });
    await waitForClickDelay();

    expect(providersActionMocks.runProviderLatencyProbe).not.toHaveBeenCalled();
    expect(document.body.textContent).toContain("Mini probe");
    expect(document.body.textContent).toContain("Enable background probe");

    unmount();
  }, 30000);

  test("opens Mini probe failure details from the failed status chip", async () => {
    const provider = makeProviderDisplay({
      latencyProbeLastStatus: "failed",
      latencyProbeLastError: "HTTP 401 - Unauthorized",
      latencyProbeLastRunAt: "2026-06-22T00:00:00.000Z",
    });

    const { unmount } = renderWithProviders(
      <ProviderRichListItem
        provider={provider}
        currentUser={ADMIN_USER}
        enableMultiProviderTypes={true}
      />
    );

    await flushTicks(5);

    const failedProbeButton = Array.from(document.querySelectorAll("button")).find(
      (button) => button.textContent?.includes("Probe failed")
    );
    expect(failedProbeButton).toBeTruthy();

    await act(async () => {
      failedProbeButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    await flushTicks(2);

    expect(document.body.textContent).toContain("Probe failure details");
    expect(document.body.textContent).toContain("HTTP 401 - Unauthorized");

    unmount();
  }, 30000);
});
