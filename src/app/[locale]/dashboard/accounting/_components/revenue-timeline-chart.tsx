"use client";

import { formatInTimeZone } from "date-fns-tz";
import { useTranslations } from "next-intl";
import * as React from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import type { RevenueTimelineData } from "@/actions/accounting";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig, ChartContainer, ChartTooltip } from "@/components/ui/chart";
import { formatCurrency } from "@/lib/utils/currency";

export interface RevenueTimelineChartProps {
  title: string;
  data: RevenueTimelineData;
  globalMultiplier?: string;
  isLoading?: boolean;
}

const USER_COLOR_PALETTE = [
  "hsl(var(--chart-1))",
  "hsl(var(--chart-2))",
  "hsl(var(--chart-3))",
  "hsl(var(--chart-4))",
  "hsl(var(--chart-5))",
] as const;

const getUserColor = (index: number) => USER_COLOR_PALETTE[index % USER_COLOR_PALETTE.length];

export function RevenueTimelineChart({
  title,
  data,
  isLoading = false,
}: RevenueTimelineChartProps) {
  const t = useTranslations("settings.accounting.charts");
  const chartId = React.useId().replace(/:/g, "");

  const { chartData, users } = React.useMemo(() => {
    const tz = data.timezone || "UTC";
    const buckets: Record<string, Record<string, number>> = {};
    const userSet = new Set<string>();

    data.rows.forEach((row) => {
      const key = formatInTimeZone(new Date(row.timeBucket), tz, "MM-dd HH:mm");
      const userName = row.userName || "unknown";
      userSet.add(userName);

      if (!buckets[key]) buckets[key] = {};
      buckets[key][userName] = (buckets[key][userName] || 0) + row.revenueUsd;
    });

    const sortedKeys = Object.keys(buckets).sort();
    const chartData = sortedKeys.map((key) => ({
      time: key,
      ...buckets[key],
    }));

    return { chartData, users: Array.from(userSet).sort() };
  }, [data]);

  const chartConfig: ChartConfig = React.useMemo(() => {
    return users.reduce((acc, user, idx) => {
      acc[user] = {
        label: user,
        color: getUserColor(idx),
      };
      return acc;
    }, {} as ChartConfig);
  }, [users]);

  const totals = React.useMemo(() => {
    return users.reduce(
      (acc, user) => {
        acc[user] = chartData.reduce((sum, item) => sum + ((item as any)[user] || 0), 0);
        return acc;
      },
      {} as Record<string, number>
    );
  }, [chartData, users]);

  if (isLoading) {
    return (
      <Card className="flex h-full flex-col rounded-lg">
        <CardHeader className="border-b border-border/50 pb-3 pt-4">
          <CardTitle className="text-sm font-semibold">{title}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-1 items-center justify-center p-4">
          <div className="text-sm text-muted-foreground">{t("loading")}</div>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className="flex h-full flex-col rounded-lg">
      <CardHeader className="flex flex-row items-center justify-between border-b border-border/50 pb-3 pt-4">
        <CardTitle className="text-sm font-semibold">{title}</CardTitle>
        <div className="flex flex-wrap items-center gap-3 text-xs">
          {users.slice(0, 5).map((user) => (
            <div key={user} className="flex items-center gap-1.5">
              <div
                className="h-2 w-2 rounded-full"
                style={{ backgroundColor: chartConfig[user]?.color }}
              />
              <span className="text-muted-foreground">{user}:</span>
              <span className="font-mono font-medium">
                {formatCurrency(totals[user], "USD", 2)}
              </span>
            </div>
          ))}
        </div>
      </CardHeader>
      <CardContent className="flex-1 p-4">
        <ChartContainer config={chartConfig} className="aspect-auto h-[240px] w-full">
          <AreaChart
            data={chartData}
            margin={{ left: 0, right: 0, top: 12, bottom: 0 }}
            stackOffset="none"
          >
            <defs>
              {users.map((user) => (
                <linearGradient
                  key={user}
                  id={`fill-${user}-${chartId}`}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop offset="5%" stopColor={chartConfig[user]?.color} stopOpacity={0.8} />
                  <stop offset="95%" stopColor={chartConfig[user]?.color} stopOpacity={0.1} />
                </linearGradient>
              ))}
            </defs>
            <CartesianGrid
              vertical={false}
              strokeDasharray="3 3"
              className="stroke-border/30 dark:stroke-white/[0.06]"
            />
            <XAxis
              dataKey="time"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={32}
              className="text-[10px] fill-muted-foreground"
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              width={70}
              tickFormatter={(value) => formatCurrency(value, "USD", 0)}
              className="text-[10px] fill-muted-foreground"
            />
            <ChartTooltip
              cursor={{
                stroke: "hsl(var(--muted-foreground))",
                strokeWidth: 1,
                strokeDasharray: "4 4",
                opacity: 0.5,
              }}
              wrapperStyle={{ zIndex: 1000 }}
              content={({ active, payload, label }) => {
                if (!active || !payload?.length) return null;

                return (
                  <div className="min-w-[180px] rounded-lg border bg-background p-3 shadow-sm">
                    <div className="mb-2 font-medium">{label}</div>
                    <div className="grid gap-2 text-sm">
                      {payload.map((entry, idx) => (
                        <div
                          key={`${entry.dataKey}-${idx}`}
                          className="flex items-center justify-between gap-4"
                        >
                          <div className="flex items-center gap-2">
                            <div
                              className="h-2 w-2 rounded-full"
                              style={{ backgroundColor: entry.color }}
                            />
                            <span className="text-muted-foreground">{String(entry.dataKey)}:</span>
                          </div>
                          <span className="font-mono font-semibold">
                            {formatCurrency((entry.value as number) || 0, "USD", 2)}
                          </span>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              }}
            />
            {users.map((user) => (
              <Area
                key={user}
                dataKey={user}
                type="monotone"
                fill={`url(#fill-${user}-${chartId})`}
                stroke={chartConfig[user]?.color}
                strokeWidth={2}
                stackId="stack"
                activeDot={{ r: 4, strokeWidth: 0 }}
              />
            ))}
          </AreaChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
