/**
 * @vitest-environment happy-dom
 */

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { NextIntlClientProvider } from "next-intl";
import { type ReactNode, act } from "react";
import { createRoot } from "react-dom/client";
import { beforeEach, describe, expect, test, vi } from "vitest";
import type { ProviderDisplay } from "@/types/provider";
import enMessages from "../../../../messages/en";

vi.mock("@/lib/hooks/use-debounce", () => ({
  useDebounce: (value: string) => value,
}));

const providerActionMocks = vi.hoisted(() => ({
  batchSetProviderUpstreamRateSyncEnabled: vi.fn(async () => ({
    ok: true,
    data: { updatedCount: 2, skippedCount: 0 },
  })),
  batchUpdateProviders: vi.fn(async () => ({ ok: true, data: {} })),
  runProviderLatencyProbe: vi.fn(async () => ({
    ok: true,
    data: { avgLatencyMs: 100, sampledAt: 1, status: "success" },
  })),
  syncProviderUpstreamRateNow: vi.fn(async () => ({ ok: true, data: {} })),
}));

vi.mock("@/lib/api-client/v1/actions/providers", () => providerActionMocks);

vi.mock("sonner", () => ({
  toast: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock("@/app/[locale]/settings/providers/_components/provider-list", () => ({
  ProviderList: ({
    providers,
    isMultiSelectMode,
    selectedProviderIds,
    onSelectProvider,
  }: {
    providers: ProviderDisplay[];
    isMultiSelectMode: boolean;
    selectedProviderIds: Set<number>;
    onSelectProvider: (providerId: number, checked: boolean) => void;
  }) => (
    <div data-testid="provider-list">
      {providers.map((provider) => (
        <label key={provider.id}>
          <input
            type="checkbox"
            disabled={!isMultiSelectMode}
            checked={selectedProviderIds.has(provider.id)}
            onChange={(event) => onSelectProvider(provider.id, event.currentTarget.checked)}
          />
          {provider.name}
        </label>
      ))}
    </div>
  ),
}));

vi.mock("@/app/[locale]/settings/providers/_components/batch-edit", () => ({
  ProviderBatchToolbar: ({
    onEnterMode,
    onSelectAll,
  }: {
    onEnterMode: () => void;
    onSelectAll: (checked: boolean) => void;
  }) => (
    <>
      <button onClick={onEnterMode}>enter-batch</button>
      <button onClick={() => onSelectAll(true)}>select-all</button>
    </>
  ),
  ProviderBatchActions: ({
    isVisible,
    onAction,
  }: {
    isVisible: boolean;
    onAction: (
      mode:
        | "rateSyncAutoOn"
        | "rateSyncAutoOff"
        | "miniProbeAutoOn"
        | "miniProbeAutoOff"
    ) => void;
  }) =>
    isVisible ? (
      <>
        <button onClick={() => onAction("rateSyncAutoOn")}>batch-rate-sync-auto-on</button>
        <button onClick={() => onAction("rateSyncAutoOff")}>batch-rate-sync-auto-off</button>
        <button onClick={() => onAction("miniProbeAutoOn")}>batch-mini-probe-auto-on</button>
        <button onClick={() => onAction("miniProbeAutoOff")}>batch-mini-probe-auto-off</button>
      </>
    ) : null,
  ProviderBatchDialog: () => null,
}));

vi.mock("@/app/[locale]/settings/providers/_components/batch-test", () => ({
  BatchTestDialog: () => null,
}));

vi.mock("@/app/[locale]/settings/providers/_components/provider-group-tab", () => ({
  ProviderGroupTab: () => null,
}));

vi.mock("@/app/[locale]/settings/providers/_components/provider-vendor-view", () => ({
  ProviderVendorView: () => null,
}));

vi.mock("@/app/[locale]/settings/providers/_components/provider-type-filter", () => ({
  ProviderTypeFilter: () => null,
}));

vi.mock("@/app/[locale]/settings/providers/_components/provider-sort-dropdown", () => ({
  ProviderSortDropdown: () => null,
}));

vi.mock("@/components/ui/dialog", () => ({
  Dialog: ({ children }: { children: ReactNode }) => <>{children}</>,
  DialogTitle: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@radix-ui/react-visually-hidden", () => ({
  VisuallyHidden: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("@/app/[locale]/settings/providers/_components/forms/provider-form", () => ({
  ProviderForm: () => null,
}));

vi.mock("@/app/[locale]/settings/providers/_components/provider-form-dialog-content", () => ({
  ProviderFormDialogContent: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

import { ProviderManager } from "@/app/[locale]/settings/providers/_components/provider-manager";

function makeProvider(id: number, name: string): ProviderDisplay {
  return {
    id,
    name,
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
  };
}

function renderWithProviders(node: ReactNode, queryClient: QueryClient) {
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
    container,
    unmount: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

async function flushTicks(times = 3) {
  for (let i = 0; i < times; i++) {
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
  }
}

describe("ProviderManager batch automatic switches", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.clearAllMocks();
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    while (document.body.firstChild) {
      document.body.removeChild(document.body.firstChild);
    }
  });

  test("enables upstream rate auto sync for selected providers", async () => {
    const invalidateSpy = vi.spyOn(queryClient, "invalidateQueries");
    const providers = [makeProvider(1, "A"), makeProvider(2, "B")];
    providers[0].upstreamRateSync = {
      isConfigured: true,
      isEnabled: false,
      source: "newapi",
      lastSyncedAt: null,
      lastSyncOk: null,
      lastSyncRate: null,
      lastSyncError: null,
      lastUpstreamGroupName: null,
    };
    queryClient.setQueryData(["providers"], providers);
    const { container, unmount } = renderWithProviders(
      <ProviderManager
        providers={providers}
        healthStatus={{}}
        enableMultiProviderTypes={true}
      />,
      queryClient
    );

    const enterButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("enter-batch")
    );
    act(() => enterButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const selectAllButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("select-all")
    );
    act(() => selectAllButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const rateSyncButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("batch-rate-sync-auto-on")
    );
    act(() => rateSyncButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));
    await flushTicks(5);

    expect(providerActionMocks.batchSetProviderUpstreamRateSyncEnabled).toHaveBeenCalledWith({
      providerIds: [1, 2],
      enabled: true,
    });
    expect(providerActionMocks.syncProviderUpstreamRateNow).not.toHaveBeenCalled();
    expect(providerActionMocks.runProviderLatencyProbe).not.toHaveBeenCalled();
    expect(invalidateSpy).toHaveBeenCalledWith({ predicate: expect.any(Function) });
    expect(queryClient.getQueryData<ProviderDisplay[]>(["providers"])?.[0]?.upstreamRateSync).toMatchObject({
      isEnabled: true,
    });

    unmount();
  });

  test("enables Mini probe background switch for selected providers", async () => {
    const providers = [makeProvider(1, "A"), makeProvider(2, "B")];
    queryClient.setQueryData(["providers"], providers);
    const { container, unmount } = renderWithProviders(
      <ProviderManager
        providers={providers}
        healthStatus={{}}
        enableMultiProviderTypes={true}
      />,
      queryClient
    );

    const enterButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("enter-batch")
    );
    act(() => enterButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const selectAllButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("select-all")
    );
    act(() => selectAllButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const miniProbeButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("batch-mini-probe-auto-on")
    );
    act(() => miniProbeButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));
    await flushTicks(5);

    expect(providerActionMocks.batchUpdateProviders).toHaveBeenCalledWith({
      providerIds: [1, 2],
      updates: { latency_probe_enabled: true },
    });
    expect(providerActionMocks.runProviderLatencyProbe).not.toHaveBeenCalled();
    expect(queryClient.getQueryData<ProviderDisplay[]>(["providers"])?.[0]?.latencyProbeEnabled).toBe(
      true
    );

    unmount();
  });

  test("disables upstream rate auto sync for selected providers", async () => {
    const providers = [makeProvider(1, "A"), makeProvider(2, "B")];
    providers[0].upstreamRateSync = {
      isConfigured: true,
      isEnabled: true,
      source: "newapi",
      lastSyncedAt: null,
      lastSyncOk: null,
      lastSyncRate: null,
      lastSyncError: null,
      lastUpstreamGroupName: null,
    };
    queryClient.setQueryData(["providers"], providers);
    const { container, unmount } = renderWithProviders(
      <ProviderManager
        providers={providers}
        healthStatus={{}}
        enableMultiProviderTypes={true}
      />,
      queryClient
    );

    const enterButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("enter-batch")
    );
    act(() => enterButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const selectAllButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("select-all")
    );
    act(() => selectAllButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const rateSyncButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("batch-rate-sync-auto-off")
    );
    act(() => rateSyncButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));
    await flushTicks(5);

    expect(providerActionMocks.batchSetProviderUpstreamRateSyncEnabled).toHaveBeenCalledWith({
      providerIds: [1, 2],
      enabled: false,
    });
    expect(providerActionMocks.syncProviderUpstreamRateNow).not.toHaveBeenCalled();
    expect(providerActionMocks.runProviderLatencyProbe).not.toHaveBeenCalled();
    expect(queryClient.getQueryData<ProviderDisplay[]>(["providers"])?.[0]?.upstreamRateSync).toMatchObject({
      isEnabled: false,
    });

    unmount();
  });

  test("disables Mini probe background switch for selected providers", async () => {
    const providers = [makeProvider(1, "A"), makeProvider(2, "B")];
    providers[0].latencyProbeEnabled = true;
    queryClient.setQueryData(["providers"], providers);
    const { container, unmount } = renderWithProviders(
      <ProviderManager
        providers={providers}
        healthStatus={{}}
        enableMultiProviderTypes={true}
      />,
      queryClient
    );

    const enterButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("enter-batch")
    );
    act(() => enterButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const selectAllButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("select-all")
    );
    act(() => selectAllButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));

    const miniProbeButton = Array.from(container.querySelectorAll("button")).find((button) =>
      button.textContent?.includes("batch-mini-probe-auto-off")
    );
    act(() => miniProbeButton?.dispatchEvent(new MouseEvent("click", { bubbles: true })));
    await flushTicks(5);

    expect(providerActionMocks.batchUpdateProviders).toHaveBeenCalledWith({
      providerIds: [1, 2],
      updates: { latency_probe_enabled: false },
    });
    expect(providerActionMocks.runProviderLatencyProbe).not.toHaveBeenCalled();
    expect(queryClient.getQueryData<ProviderDisplay[]>(["providers"])?.[0]?.latencyProbeEnabled).toBe(
      false
    );

    unmount();
  });
});
