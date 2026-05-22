import { useTranslation } from "react-i18next";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/Card";
import { BarChart3, DollarSign, Users, Server } from "lucide-react";

function Dashboard() {
  const { t } = useTranslation();

  const stats = [
    {
      labelKey: "dashboard.totalRequests",
      value: "--",
      icon: BarChart3,
      color: "text-blue-500",
    },
    {
      labelKey: "dashboard.totalCost",
      value: "--",
      icon: DollarSign,
      color: "text-green-500",
    },
    {
      labelKey: "dashboard.activeUsers",
      value: "--",
      icon: Users,
      color: "text-purple-500",
    },
    {
      labelKey: "dashboard.activeProviders",
      value: "--",
      icon: Server,
      color: "text-orange-500",
    },
  ];

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
              <p className="text-3xl font-bold text-[var(--color-text)]">
                {stat.value}
              </p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Placeholder area for charts */}
      <div className="mt-8 grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardTitle>{t("dashboard.totalRequests")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex h-64 items-center justify-center text-[var(--color-text-tertiary)]">
              {t("common.noData")}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>{t("dashboard.totalCost")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="flex h-64 items-center justify-center text-[var(--color-text-tertiary)]">
              {t("common.noData")}
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export { Dashboard };
