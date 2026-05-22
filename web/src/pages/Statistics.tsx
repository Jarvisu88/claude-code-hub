import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Spinner } from "@/components/ui/Spinner";
import { useStatistics } from "@/api/hooks";
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  PieChart,
  Pie,
  Cell,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";

const CHART_COLORS = [
  "var(--color-primary)",
  "#10b981",
  "#f59e0b",
  "#ef4444",
  "#8b5cf6",
  "#06b6d4",
  "#ec4899",
  "#f97316",
];

const PIE_COLORS = [
  "#6366f1",
  "#10b981",
  "#f59e0b",
  "#ef4444",
  "#8b5cf6",
  "#06b6d4",
  "#ec4899",
  "#f97316",
  "#14b8a6",
  "#a855f7",
];

type TimeRange = "24h" | "7d" | "30d";

function StatisticsPage() {
  const { t } = useTranslation();
  const [range, setRange] = useState<TimeRange>("7d");

  const { data, isLoading, error } = useStatistics(range);

  const ranges: { value: TimeRange; label: string }[] = [
    { value: "24h", label: t("statistics.range24h") },
    { value: "7d", label: t("statistics.range7d") },
    { value: "30d", label: t("statistics.range30d") },
  ];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("statistics.title")}
        </h1>
        <div className="flex gap-2">
          {ranges.map((r) => (
            <Button
              key={r.value}
              variant={range === r.value ? "primary" : "secondary"}
              size="sm"
              onClick={() => setRange(r.value)}
            >
              {r.label}
            </Button>
          ))}
        </div>
      </div>

      {error ? (
        <Card>
          <CardContent>
            <div className="flex h-64 items-center justify-center text-[var(--color-danger)]">
              {t("common.loadError")}
            </div>
          </CardContent>
        </Card>
      ) : isLoading ? (
        <div className="flex h-64 items-center justify-center">
          <Spinner size="lg" />
        </div>
      ) : (
        <div className="grid gap-6 lg:grid-cols-2">
          {/* Requests over time */}
          <Card>
            <CardHeader>
              <CardTitle>{t("statistics.requestsOverTime")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart data={data?.requestsOverTime ?? []}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                    <XAxis
                      dataKey="time"
                      stroke="var(--color-text-secondary)"
                      fontSize={12}
                    />
                    <YAxis stroke="var(--color-text-secondary)" fontSize={12} />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "var(--color-bg)",
                        border: "1px solid var(--color-border)",
                        borderRadius: "8px",
                        color: "var(--color-text)",
                      }}
                    />
                    <Bar
                      dataKey="count"
                      name={t("statistics.requests")}
                      fill={CHART_COLORS[0]}
                      radius={[4, 4, 0, 0]}
                    />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Cost over time */}
          <Card>
            <CardHeader>
              <CardTitle>{t("statistics.costOverTime")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={data?.costOverTime ?? []}>
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                    <XAxis
                      dataKey="time"
                      stroke="var(--color-text-secondary)"
                      fontSize={12}
                    />
                    <YAxis stroke="var(--color-text-secondary)" fontSize={12} />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "var(--color-bg)",
                        border: "1px solid var(--color-border)",
                        borderRadius: "8px",
                        color: "var(--color-text)",
                      }}
                      formatter={(value: number) => [`$${value.toFixed(4)}`, t("statistics.cost")]}
                    />
                    <Line
                      type="monotone"
                      dataKey="cost"
                      name={t("statistics.cost")}
                      stroke={CHART_COLORS[1]}
                      strokeWidth={2}
                      dot={{ fill: CHART_COLORS[1], r: 3 }}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Top models */}
          <Card>
            <CardHeader>
              <CardTitle>{t("statistics.topModels")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <PieChart>
                    <Pie
                      data={data?.topModels ?? []}
                      dataKey="count"
                      nameKey="model"
                      cx="50%"
                      cy="50%"
                      outerRadius={100}
                      label={({ model, percent }) =>
                        `${model} (${(percent * 100).toFixed(0)}%)`
                      }
                      labelLine={false}
                    >
                      {(data?.topModels ?? []).map((_, index) => (
                        <Cell
                          key={`cell-${index}`}
                          fill={PIE_COLORS[index % PIE_COLORS.length]}
                        />
                      ))}
                    </Pie>
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "var(--color-bg)",
                        border: "1px solid var(--color-border)",
                        borderRadius: "8px",
                        color: "var(--color-text)",
                      }}
                    />
                  </PieChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>

          {/* Top users */}
          <Card>
            <CardHeader>
              <CardTitle>{t("statistics.topUsers")}</CardTitle>
            </CardHeader>
            <CardContent>
              <div className="h-72">
                <ResponsiveContainer width="100%" height="100%">
                  <BarChart
                    data={data?.topUsers ?? []}
                    layout="vertical"
                    margin={{ left: 80 }}
                  >
                    <CartesianGrid strokeDasharray="3 3" stroke="var(--color-border)" />
                    <XAxis
                      type="number"
                      stroke="var(--color-text-secondary)"
                      fontSize={12}
                    />
                    <YAxis
                      type="category"
                      dataKey="userName"
                      stroke="var(--color-text-secondary)"
                      fontSize={12}
                      width={70}
                    />
                    <Tooltip
                      contentStyle={{
                        backgroundColor: "var(--color-bg)",
                        border: "1px solid var(--color-border)",
                        borderRadius: "8px",
                        color: "var(--color-text)",
                      }}
                    />
                    <Bar
                      dataKey="count"
                      name={t("statistics.requests")}
                      fill={CHART_COLORS[4]}
                      radius={[0, 4, 4, 0]}
                    />
                  </BarChart>
                </ResponsiveContainer>
              </div>
            </CardContent>
          </Card>
        </div>
      )}
    </div>
  );
}

export { StatisticsPage };
