import { useTranslation } from "react-i18next";
import {
  BarChart,
  Bar,
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from "recharts";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { useOverview } from "@/api/hooks";
import { BarChart3, DollarSign, Users, Server, Clock } from "lucide-react";

function Dashboard() {
  const { t } = useTranslation();
  const { data, isLoading, error } = useOverview();

  const stats = [
    {
      labelKey: "dashboard.totalRequests",
      value: data?.stats.totalRequests ?? "--",
      icon: BarChart3,
      color: "text-blue-500",
    },
    {
      labelKey: "dashboard.totalCost",
      value: data?.stats.totalCost !== undefined
        ? `$${data.stats.totalCost.toFixed(2)}`
        : "--",
      icon: DollarSign,
      color: "text-green-500",
    },
    {
      labelKey: "dashboard.activeUsers",
      value: data?.stats.activeUsers ?? "--",
      icon: Users,
      color: "text-purple-500",
    },
    {
      labelKey: "dashboard.activeProviders",
      value: data?.stats.activeProviders ?? "--",
      icon: Server,
      color: "text-orange-500",
    },
  ];

  if (error) {
    return (
      <div>
        <h1 className="mb-6 text-2xl font-bold text-[var(--color-text)]">
          {t("dashboard.title")}
        </h1>
        <Card>
          <CardContent>
            <div className="flex h-64 items-center justify-center text-[var(--color-danger)]">
              {t("common.loadError")}
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div>
      <h1 className="mb-6 text-2xl font-bold text-[var(--color-text)]">
        {t("dashboard.title")}
      </h1>

      {/* Stats grid */}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {stats.map((stat) => (
          <Card key={stat.labelKey}>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle className="text-sm font-medium text-[var(--color-text-secondary)]">
                  {t(stat.labelKey)}
                </CardTitle>
                <stat.icon className={`h-5 w-5 ${stat.color}`} />
              </div>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <Spinner size="sm" />
              ) : (
                <p className="text-3xl font-bold text-[var(--color-text)]">
                  {stat.value}
                </p>
              )}
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Charts row */}
      <div className="mt-8 grid gap-4 lg:grid-cols-2">
        {/* Requests per hour chart */}
        <Card>
          <CardHeader>
            <CardTitle>{t("dashboard.requestsPerHour")}</CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex h-64 items-center justify-center">
                <Spinner />
              </div>
            ) : data?.requestsPerHour && data.requestsPerHour.length > 0 ? (
              <ResponsiveContainer width="100%" height={260}>
                <BarChart data={data.requestsPerHour}>
                  <CartesianGrid
                    strokeDasharray="3 3"
                    stroke="var(--color-border)"
                  />
                  <XAxis
                    dataKey="hour"
                    tick={{ fill: "var(--color-text-secondary)", fontSize: 12 }}
                    tickLine={false}
                  />
                  <YAxis
                    tick={{ fill: "var(--color-text-secondary)", fontSize: 12 }}
                    tickLine={false}
                    axisLine={false}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "var(--color-bg)",
                      border: "1px solid var(--color-border)",
                      borderRadius: "8px",
                      color: "var(--color-text)",
                    }}
                    labelFormatter={(label) => `${label}:00`}
                  />
                  <Bar
                    dataKey="count"
                    fill="var(--color-primary)"
                    radius={[4, 4, 0, 0]}
                    name={t("dashboard.requests")}
                  />
                </BarChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-64 items-center justify-center text-[var(--color-text-tertiary)]">
                {t("common.noData")}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Cost per day chart */}
        <Card>
          <CardHeader>
            <CardTitle>{t("dashboard.costPerDay")}</CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex h-64 items-center justify-center">
                <Spinner />
              </div>
            ) : data?.costPerDay && data.costPerDay.length > 0 ? (
              <ResponsiveContainer width="100%" height={260}>
                <LineChart data={data.costPerDay}>
                  <CartesianGrid
                    strokeDasharray="3 3"
                    stroke="var(--color-border)"
                  />
                  <XAxis
                    dataKey="date"
                    tick={{ fill: "var(--color-text-secondary)", fontSize: 12 }}
                    tickLine={false}
                  />
                  <YAxis
                    tick={{ fill: "var(--color-text-secondary)", fontSize: 12 }}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(value) => `$${value}`}
                  />
                  <Tooltip
                    contentStyle={{
                      backgroundColor: "var(--color-bg)",
                      border: "1px solid var(--color-border)",
                      borderRadius: "8px",
                      color: "var(--color-text)",
                    }}
                    formatter={(value: number) => [`$${value.toFixed(4)}`, t("dashboard.cost")]}
                  />
                  <Line
                    type="monotone"
                    dataKey="cost"
                    stroke="var(--color-primary)"
                    strokeWidth={2}
                    dot={{ fill: "var(--color-primary)", r: 4 }}
                    activeDot={{ r: 6 }}
                  />
                </LineChart>
              </ResponsiveContainer>
            ) : (
              <div className="flex h-64 items-center justify-center text-[var(--color-text-tertiary)]">
                {t("common.noData")}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Recent activity */}
      <div className="mt-8">
        <Card>
          <CardHeader>
            <CardTitle>{t("dashboard.recentActivity")}</CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="flex h-40 items-center justify-center">
                <Spinner />
              </div>
            ) : data?.recentActivity && data.recentActivity.length > 0 ? (
              <div className="space-y-3">
                {data.recentActivity.map((activity) => (
                  <div
                    key={activity.id}
                    className="flex items-center justify-between rounded-md border border-[var(--color-border)] p-3"
                  >
                    <div className="flex items-center gap-3">
                      <Clock className="h-4 w-4 text-[var(--color-text-tertiary)]" />
                      <div>
                        <span className="font-medium text-[var(--color-text)]">
                          {activity.userName}
                        </span>
                        <span className="ml-2 text-sm text-[var(--color-text-secondary)]">
                          {activity.model}
                        </span>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-sm text-[var(--color-text-secondary)]">
                        ${activity.cost.toFixed(4)}
                      </span>
                      <Badge
                        variant={activity.status === 200 ? "success" : "danger"}
                      >
                        {activity.status}
                      </Badge>
                      <span className="text-xs text-[var(--color-text-tertiary)]">
                        {new Date(activity.createdAt).toLocaleTimeString()}
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="flex h-40 items-center justify-center text-[var(--color-text-tertiary)]">
                {t("common.noData")}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export { Dashboard };
