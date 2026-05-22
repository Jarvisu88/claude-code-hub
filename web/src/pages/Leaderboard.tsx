import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Table, type Column } from "@/components/ui/Table";
import { Spinner } from "@/components/ui/Spinner";
import { useLeaderboard } from "@/api/hooks";
import type { LeaderboardEntry } from "@/api/types";

type Period = "today" | "7d" | "30d";

function LeaderboardPage() {
  const { t } = useTranslation();
  const [period, setPeriod] = useState<Period>("7d");

  const { data, isLoading, error } = useLeaderboard(period);

  const periods: { value: Period; label: string }[] = [
    { value: "today", label: t("leaderboard.periodToday") },
    { value: "7d", label: t("leaderboard.period7d") },
    { value: "30d", label: t("leaderboard.period30d") },
  ];

  const columns: Column<LeaderboardEntry>[] = [
    {
      key: "rank",
      header: t("leaderboard.rank"),
      className: "w-16",
      render: (row) => (
        <span className="font-bold text-[var(--color-primary)]">
          #{row.rank}
        </span>
      ),
    },
    {
      key: "userName",
      header: t("leaderboard.user"),
      render: (row) => <span className="font-medium">{row.userName}</span>,
    },
    {
      key: "totalRequests",
      header: t("leaderboard.totalRequests"),
      render: (row) => (
        <span className="text-sm">{row.totalRequests.toLocaleString()}</span>
      ),
    },
    {
      key: "totalCost",
      header: t("leaderboard.totalCost"),
      render: (row) => (
        <span className="text-sm font-medium">${row.totalCost.toFixed(4)}</span>
      ),
    },
    {
      key: "avgCost",
      header: t("leaderboard.avgCost"),
      render: (row) => (
        <span className="text-sm text-[var(--color-text-secondary)]">
          ${row.avgCost.toFixed(6)}
        </span>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("leaderboard.title")}
        </h1>
        <div className="flex gap-2">
          {periods.map((p) => (
            <Button
              key={p.value}
              variant={period === p.value ? "primary" : "secondary"}
              size="sm"
              onClick={() => setPeriod(p.value)}
            >
              {p.label}
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
        <Table<LeaderboardEntry>
          columns={columns}
          data={data ?? []}
          keyExtractor={(row) => row.userId}
        />
      )}
    </div>
  );
}

export { LeaderboardPage };
