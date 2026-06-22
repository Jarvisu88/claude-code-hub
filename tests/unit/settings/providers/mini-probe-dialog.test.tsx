/**
 * @vitest-environment happy-dom
 */

import { NextIntlClientProvider } from "next-intl";
import { type ReactNode, act } from "react";
import { createRoot } from "react-dom/client";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { ProviderDisplay } from "@/types/provider";
import enMessages from "../../../../messages/en";
import { MiniProbeDialog } from "@/app/[locale]/settings/providers/_components/mini-probe-dialog";

vi.mock("@/lib/api-client/v1/actions/providers", () => ({
  editProvider: vi.fn(),
  runProviderLatencyProbe: vi.fn(),
}));

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

function makeProvider(overrides: Partial<ProviderDisplay> = {}): ProviderDisplay {
  return {
    id: 1,
    name: "Provider A",
    url: "https://api.example.com",
    maskedKey: "sk-***",
    isEnabled: true,
    weight: 1,
    priority: 1,
    costMultiplier: 1,
    groupTag: null,
    groupPriorities: null,
    providerType: "claude",
    providerVendorId: null,
    preserveClientIp: false,
    modelRedirects: null,
    activeTimeStart: null,
    activeTimeEnd: null,
    allowedModels: null,
    allowedClients: [],
    blockedClients: [],
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
    geminiGoogleSearchPreference: null,
    isFailoverOnly: false,
    latencyProbeEnabled: null,
    latencyProbeModel: null,
    latencyProbeIntervalMs: null,
    latencyProbeTimeStart: null,
    latencyProbeTimeEnd: null,
    latencyProbeLastAvgMs: null,
    latencyProbeLastError: null,
    latencyProbeLastStatus: null,
    latencyProbeLastRunAt: null,
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

function renderWithIntl(node: ReactNode) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);

  act(() => {
    root.render(
      <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
        {node}
      </NextIntlClientProvider>
    );
  });

  return {
    container,
    rerender: (next: ReactNode) => {
      act(() => {
        root.render(
          <NextIntlClientProvider locale="en" messages={enMessages} timeZone="UTC">
            {next}
          </NextIntlClientProvider>
        );
      });
    },
    unmount: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

describe("MiniProbeDialog", () => {
  beforeEach(() => {
    while (document.body.firstChild) {
      document.body.removeChild(document.body.firstChild);
    }
  });

  test("refreshes the enabled switch from updated provider props while open", () => {
    const { rerender, unmount } = renderWithIntl(
      <MiniProbeDialog provider={makeProvider({ latencyProbeEnabled: false })} open={true} />
    );

    const switchInput = document.body.querySelector(
      "#latency-probe-enabled-1"
    ) as HTMLButtonElement | null;
    expect(switchInput?.getAttribute("data-state")).toBe("unchecked");

    rerender(
      <MiniProbeDialog provider={makeProvider({ latencyProbeEnabled: true })} open={true} />
    );

    const updatedSwitchInput = document.body.querySelector(
      "#latency-probe-enabled-1"
    ) as HTMLButtonElement | null;
    expect(updatedSwitchInput?.getAttribute("data-state")).toBe("checked");

    unmount();
  });
});
