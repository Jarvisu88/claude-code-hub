import { useEffect, useCallback, useState, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from "recharts";
import { Spinner } from "@/components/ui/Spinner";

// ---------------------------------------------------------------------------
// Types for the real-time dashboard API response
// ---------------------------------------------------------------------------

interface RealtimeMetrics {
  concurrentSessions: number;
  todayRequests: number;
  todayCost: number;
  avgResponseTime: number;
  todayErrorRate: number;
  recentMinuteRequests: number;
}

interface ActivityItem {
  id: string;
  user: string;
  model: string;
  provider: string;
  latency: number;
  status: number;
  cost: number;
  startTime: number;
}

interface ProviderSlot {
  providerId: number;
  name: string;
  usedSlots: number;
  totalSlots: number;
}

interface ModelDistItem {
  model: string;
  totalRequests: number;
  totalCost: number;
}

interface ProviderRanking {
  providerId: number;
  providerName: string;
  totalRequests: number;
  successRate: number;
}

interface RealtimeData {
  metrics: RealtimeMetrics;
  activityStream: ActivityItem[];
  providerSlots: ProviderSlot[];
  providerRankings: ProviderRanking[];
  modelDistribution: ModelDistItem[];
  trendData: { hour: number; value: number }[];
}

// ---------------------------------------------------------------------------
// Data fetch helper (bypasses react-query for simplicity -- standalone page)
// ---------------------------------------------------------------------------

function getAuthToken(): string | null {
  try {
    const stored = localStorage.getItem("auth-storage");
    if (stored) {
      const parsed = JSON.parse(stored);
      return parsed?.state?.token ?? null;
    }
  } catch {
    // ignore
  }
  return null;
}

async function fetchRealtimeData(): Promise<RealtimeData | null> {
  try {
    const token = getAuthToken();
    const headers: Record<string, string> = {
      "Content-Type": "application/json",
    };
    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }
    const res = await fetch("/api/actions/dashboard-realtime/getDashboardRealtimeData", {
      method: "POST",
      headers,
    });
    if (!res.ok) return null;
    const body = await res.json();
    return body?.data ?? null;
  } catch {
    return null;
  }
}

// ---------------------------------------------------------------------------
// Color palette
// ---------------------------------------------------------------------------

const COLORS = {
  bg: "#0f172a",
  card: "#1e293b",
  cardBorder: "#334155",
  text: "#f1f5f9",
  textSecondary: "#94a3b8",
  textMuted: "#64748b",
  primary: "#3b82f6",
  primaryLight: "#60a5fa",
  success: "#22c55e",
  warning: "#eab308",
  danger: "#ef4444",
  accent1: "#8b5cf6",
  accent2: "#06b6d4",
};

const BAR_PALETTE = [
  "#3b82f6",
  "#8b5cf6",
  "#06b6d4",
  "#22c55e",
  "#eab308",
  "#f97316",
  "#ec4899",
  "#14b8a6",
  "#f43e5e",
  "#a78bfa",
];

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function MetricCard({
  label,
  value,
  sub,
  color,
}: {
  label: string;
  value: string;
  sub?: string;
  color?: string;
}) {
  return (
    <div
      className="flex flex-col items-center justify-center rounded-xl border p-4 lg:p-6"
      style={{
        backgroundColor: COLORS.card,
        borderColor: COLORS.cardBorder,
      }}
    >
      <span
        className="text-xs font-medium uppercase tracking-wider lg:text-sm"
        style={{ color: COLORS.textSecondary }}
      >
        {label}
      </span>
      <span
        className="mt-2 text-3xl font-bold lg:text-5xl"
        style={{ color: color ?? COLORS.text }}
      >
        {value}
      </span>
      {sub && (
        <span
          className="mt-1 text-xs lg:text-sm"
          style={{ color: COLORS.textMuted }}
        >
          {sub}
        </span>
      )}
    </div>
  );
}

function ProviderHealthGrid({
  providers,
  rankings,
  label,
}: {
  providers: ProviderSlot[];
  rankings: ProviderRanking[];
  label: string;
}) {
  // Merge slot info + success rate from rankings
  const merged = (rankings ?? []).map((r) => {
    const slot = (providers ?? []).find((s) => s.providerId === r.providerId);
    return {
      name: r.providerName,
      requests: r.totalRequests,
      successRate: r.successRate,
      usedSlots: slot?.usedSlots ?? 0,
      totalSlots: slot?.totalSlots ?? 0,
    };
  });

  function healthColor(successRate: number): string {
    if (successRate >= 0.95) return COLORS.success;
    if (successRate >= 0.8) return COLORS.warning;
    return COLORS.danger;
  }

  return (
    <div
      className="flex flex-col rounded-xl border p-4"
      style={{ backgroundColor: COLORS.card, borderColor: COLORS.cardBorder }}
    >
      <h3
        className="mb-3 text-sm font-semibold uppercase tracking-wider"
        style={{ color: COLORS.textSecondary }}
      >
        {label}
      </h3>
      {merged.length === 0 ? (
        <div
          className="flex flex-1 items-center justify-center text-sm"
          style={{ color: COLORS.textMuted }}
        >
          --
        </div>
      ) : (
        <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          {merged.map((p) => (
            <div
              key={p.name}
              className="flex items-center gap-3 rounded-lg border px-3 py-2"
              style={{ borderColor: COLORS.cardBorder }}
            >
              <div
                className="h-3 w-3 shrink-0 rounded-full"
                style={{ backgroundColor: healthColor(p.successRate) }}
              />
              <div className="min-w-0 flex-1">
                <div
                  className="truncate text-sm font-medium"
                  style={{ color: COLORS.text }}
                >
                  {p.name}
                </div>
                <div className="text-xs" style={{ color: COLORS.textMuted }}>
                  {p.requests} req | {(p.successRate * 100).toFixed(0)}%
                  {p.totalSlots > 0 && ` | ${p.usedSlots}/${p.totalSlots}`}
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function RequestFlowBar({
  trendData,
  label,
}: {
  trendData: { hour: number; value: number }[];
  label: string;
}) {
  const data = (trendData ?? []).map((d) => ({
    hour: `${String(d.hour).padStart(2, "0")}`,
    value: d.value,
  }));

  return (
    <div
      className="flex flex-col rounded-xl border p-4"
      style={{ backgroundColor: COLORS.card, borderColor: COLORS.cardBorder }}
    >
      <h3
        className="mb-3 text-sm font-semibold uppercase tracking-wider"
        style={{ color: COLORS.textSecondary }}
      >
        {label}
      </h3>
      {data.length === 0 ? (
        <div
          className="flex flex-1 items-center justify-center text-sm"
          style={{ color: COLORS.textMuted }}
        >
          --
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={180}>
          <BarChart data={data}>
            <CartesianGrid
              strokeDasharray="3 3"
              stroke={COLORS.cardBorder}
              vertical={false}
            />
            <XAxis
              dataKey="hour"
              tick={{ fill: COLORS.textMuted, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              interval={2}
            />
            <YAxis
              tick={{ fill: COLORS.textMuted, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              width={30}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: COLORS.card,
                border: `1px solid ${COLORS.cardBorder}`,
                borderRadius: "8px",
                color: COLORS.text,
                fontSize: 12,
              }}
              labelFormatter={(v) => `${v}:00`}
            />
            <Bar dataKey="value" radius={[3, 3, 0, 0]}>
              {data.map((_, idx) => (
                <Cell
                  key={idx}
                  fill={
                    idx === data.length - 1
                      ? COLORS.primaryLight
                      : COLORS.primary
                  }
                />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}

function TopModelsChart({
  models,
  label,
}: {
  models: ModelDistItem[];
  label: string;
}) {
  const data = (models ?? []).slice(0, 8).map((m) => ({
    name: m.model.length > 20 ? m.model.slice(0, 18) + ".." : m.model,
    requests: m.totalRequests,
  }));

  return (
    <div
      className="flex flex-col rounded-xl border p-4"
      style={{ backgroundColor: COLORS.card, borderColor: COLORS.cardBorder }}
    >
      <h3
        className="mb-3 text-sm font-semibold uppercase tracking-wider"
        style={{ color: COLORS.textSecondary }}
      >
        {label}
      </h3>
      {data.length === 0 ? (
        <div
          className="flex flex-1 items-center justify-center text-sm"
          style={{ color: COLORS.textMuted }}
        >
          --
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={180}>
          <BarChart data={data} layout="vertical" margin={{ left: 10 }}>
            <CartesianGrid
              strokeDasharray="3 3"
              stroke={COLORS.cardBorder}
              horizontal={false}
            />
            <XAxis
              type="number"
              tick={{ fill: COLORS.textMuted, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
            />
            <YAxis
              type="category"
              dataKey="name"
              tick={{ fill: COLORS.textSecondary, fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              width={100}
            />
            <Tooltip
              contentStyle={{
                backgroundColor: COLORS.card,
                border: `1px solid ${COLORS.cardBorder}`,
                borderRadius: "8px",
                color: COLORS.text,
                fontSize: 12,
              }}
            />
            <Bar dataKey="requests" radius={[0, 4, 4, 0]}>
              {data.map((_, idx) => (
                <Cell
                  key={idx}
                  fill={BAR_PALETTE[idx % BAR_PALETTE.length]}
                />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      )}
    </div>
  );
}

function ActivityStream({
  items,
  label,
}: {
  items: ActivityItem[];
  label: string;
}) {
  return (
    <div
      className="flex flex-col rounded-xl border p-4"
      style={{ backgroundColor: COLORS.card, borderColor: COLORS.cardBorder }}
    >
      <h3
        className="mb-3 text-sm font-semibold uppercase tracking-wider"
        style={{ color: COLORS.textSecondary }}
      >
        {label}
      </h3>
      <div className="flex-1 space-y-1 overflow-y-auto" style={{ maxHeight: 200 }}>
        {(items ?? []).length === 0 ? (
          <div
            className="flex h-full items-center justify-center text-sm"
            style={{ color: COLORS.textMuted }}
          >
            --
          </div>
        ) : (
          (items ?? []).slice(0, 10).map((item) => (
            <div
              key={item.id + item.startTime}
              className="flex items-center justify-between rounded px-2 py-1 text-xs"
              style={{
                backgroundColor: "rgba(255,255,255,0.02)",
                color: COLORS.textSecondary,
              }}
            >
              <span className="w-24 truncate font-medium" style={{ color: COLORS.text }}>
                {item.user}
              </span>
              <span className="w-36 truncate" style={{ color: COLORS.accent2 }}>
                {item.model}
              </span>
              <span className="w-20 truncate">{item.provider || "--"}</span>
              <span
                className="w-12 text-center font-mono"
                style={{
                  color:
                    item.status === 0
                      ? COLORS.warning
                      : item.status < 400
                        ? COLORS.success
                        : COLORS.danger,
                }}
              >
                {item.status === 0 ? "..." : item.status}
              </span>
              <span className="w-16 text-right font-mono">
                ${item.cost.toFixed(4)}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main page component
// ---------------------------------------------------------------------------

function BigScreen() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [data, setData] = useState<RealtimeData | null>(null);
  const [loading, setLoading] = useState(true);
  const [lastUpdate, setLastUpdate] = useState<Date | null>(null);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const refresh = useCallback(async () => {
    const result = await fetchRealtimeData();
    if (result) {
      setData(result);
      setLastUpdate(new Date());
    }
    setLoading(false);
  }, []);

  // ESC to navigate back
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        navigate("/dashboard");
      }
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [navigate]);

  // Auto-refresh every 5 seconds
  useEffect(() => {
    refresh();
    intervalRef.current = setInterval(refresh, 5000);
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, [refresh]);

  const metrics = data?.metrics;

  return (
    <div
      className="relative flex min-h-screen flex-col gap-4 p-4 lg:p-6"
      style={{ backgroundColor: COLORS.bg, color: COLORS.text }}
    >
      {/* Header row */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold tracking-wide lg:text-2xl">
          {t("bigScreen.title")}
        </h1>
        <div className="flex items-center gap-4">
          {lastUpdate && (
            <span className="text-xs" style={{ color: COLORS.textMuted }}>
              {t("bigScreen.lastUpdate")}: {lastUpdate.toLocaleTimeString()}
            </span>
          )}
          <span
            className="cursor-pointer rounded border px-2 py-1 text-xs transition-colors hover:border-blue-500"
            style={{ borderColor: COLORS.cardBorder, color: COLORS.textSecondary }}
            onClick={() => navigate("/dashboard")}
          >
            ESC {t("bigScreen.exitToDashboard")}
          </span>
        </div>
      </div>

      {loading && !data ? (
        <div className="flex flex-1 items-center justify-center">
          <Spinner />
        </div>
      ) : (
        <>
          {/* Key metrics row */}
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
            <MetricCard
              label={t("bigScreen.todayRequests")}
              value={String(metrics?.todayRequests ?? 0)}
              color={COLORS.primary}
            />
            <MetricCard
              label={t("bigScreen.activeSessions")}
              value={String(metrics?.concurrentSessions ?? 0)}
              color={COLORS.accent1}
            />
            <MetricCard
              label={t("bigScreen.totalCost")}
              value={`$${(metrics?.todayCost ?? 0).toFixed(2)}`}
              color={COLORS.success}
            />
            <MetricCard
              label={t("bigScreen.requestsPerMin")}
              value={String(metrics?.recentMinuteRequests ?? 0)}
              sub={
                metrics?.todayErrorRate !== undefined
                  ? `${t("bigScreen.errorRate")}: ${(metrics.todayErrorRate * 100).toFixed(1)}%`
                  : undefined
              }
              color={COLORS.accent2}
            />
          </div>

          {/* Charts row */}
          <div className="grid gap-3 lg:grid-cols-2">
            <RequestFlowBar
              trendData={data?.trendData ?? []}
              label={t("bigScreen.requestFlow")}
            />
            <TopModelsChart
              models={data?.modelDistribution ?? []}
              label={t("bigScreen.topModels")}
            />
          </div>

          {/* Bottom row */}
          <div className="grid gap-3 lg:grid-cols-2">
            <ProviderHealthGrid
              providers={data?.providerSlots ?? []}
              rankings={data?.providerRankings ?? []}
              label={t("bigScreen.providerHealth")}
            />
            <ActivityStream
              items={data?.activityStream ?? []}
              label={t("bigScreen.recentActivity")}
            />
          </div>
        </>
      )}
    </div>
  );
}

export { BigScreen };
