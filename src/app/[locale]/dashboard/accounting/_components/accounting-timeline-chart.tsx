"use client";

import { formatInTimeZone } from "date-fns-tz";
import { useTranslations } from "next-intl";
import * as React from "react";
import { Area, AreaChart, CartesianGrid, XAxis, YAxis } from "recharts";
import type { NewApiUsageDetailRow } from "@/actions/accounting";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig, ChartContainer, ChartTooltip } from "@/components/ui/chart";
import { formatCurrency } from "@/lib/utils/currency";

export interface AccountingTimelineChartProps {
  title: string;
  details: NewApiUsageDetailRow[];
  timezone: string;
  globalMultiplier: string;
}

function calculateDisplayedRevenueDetail(
  detail: NewApiUsageDetailRow,
  configuredMultiplier: string
): number {
  const manualMultiplier = Number(configuredMultiplier);
  if (!Number.isFinite(manualMultiplier) || manualMultiplier <= 0) {
    return detail.revenueUsd;
  }
  const sourceMultiplier =
    Number.isFinite(detail.groupRatio * detail.modelRatio) &&
    detail.groupRatio * detail.modelRatio > 0
      ? detail.groupRatio * detail.modelRatio
      : 1;
  return (detail.revenueUsd / sourceMultiplier) * manualMultiplier;
}

export function AccountingTimelineChart({
  title,
  details,
  timezone,
  globalMultiplier,
}: AccountingTimelineChartProps) {
  const t = useTranslations("settings.accounting.charts");
  const chartId = React.useId().replace(/:/g, "");

  const chartData = React.useMemo(() => {
    const today = new Date();
    const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000);

    // We use the timezone returned by the server to ensure consistency
    const tz = timezone || "UTC";
    const todayStr = formatInTimeZone(today, tz, "yyyy-MM-dd");
    const yesterdayStr = formatInTimeZone(yesterday, tz, "yyyy-MM-dd");

    const buckets = Array.from({ length: 24 }).map((_, i) => ({
      hour: `${i.toString().padStart(2, "0")}:00`,
      today: 0,
      yesterday: 0,
    }));

    details.forEach((detail) => {
      if (!detail.createdAt) return;
      const date = new Date(detail.createdAt * 1000);
      const dateStr = formatInTimeZone(date, tz, "yyyy-MM-dd");
      const hourStr = formatInTimeZone(date, tz, "HH");
      const hour = parseInt(hourStr, 10);

      const revenue = calculateDisplayedRevenueDetail(detail, globalMultiplier);

      if (dateStr === todayStr) {
        buckets[hour].today += revenue;
      } else if (dateStr === yesterdayStr) {
        buckets[hour].yesterday += revenue;
      }
    });

    return buckets;
  }, [details, timezone, globalMultiplier]);

  const chartConfig: ChartConfig = {
    today: {
      label: t("todayRevenue"),
      color: "hsl(var(--primary))",
    },
    yesterday: {
      label: t("yesterdayRevenue"),
      color: "hsl(var(--muted-foreground))",
    },
  };

  const totalToday = chartData.reduce((sum, item) => sum + item.today, 0);
  const totalYesterday = chartData.reduce((sum, item) => sum + item.yesterday, 0);

  return (
    <Card className="flex h-full flex-col rounded-lg">
      <CardHeader className="flex flex-row items-center justify-between border-b border-border/50 pb-3 pt-4">
        <CardTitle className="text-sm font-semibold">{title}</CardTitle>
        <div className="flex items-center gap-4 text-xs">
          <div className="flex items-center gap-1.5">
            <div className="h-2 w-2 rounded-full bg-primary" />
            <span className="text-muted-foreground">{t("todayRevenue")}:</span>
            <span className="font-mono font-medium">{formatCurrency(totalToday, "USD", 2)}</span>
          </div>
          <div className="flex items-center gap-1.5">
            <div className="h-2 w-2 rounded-full bg-muted-foreground/50" />
            <span className="text-muted-foreground">{t("yesterdayRevenue")}:</span>
            <span className="font-mono font-medium">
              {formatCurrency(totalYesterday, "USD", 2)}
            </span>
          </div>
        </div>
      </CardHeader>
      <CardContent className="flex-1 p-4">
        <ChartContainer config={chartConfig} className="aspect-auto h-[240px] w-full">
          <AreaChart data={chartData} margin={{ left: 0, right: 0, top: 12, bottom: 0 }}>
            <defs>
              <linearGradient id={`fillToday-${chartId}`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--color-today)" stopOpacity={0.8} />
                <stop offset="95%" stopColor="var(--color-today)" stopOpacity={0.1} />
              </linearGradient>
              <linearGradient id={`fillYesterday-${chartId}`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%" stopColor="var(--color-yesterday)" stopOpacity={0.3} />
                <stop offset="95%" stopColor="var(--color-yesterday)" stopOpacity={0.05} />
              </linearGradient>
            </defs>
            <CartesianGrid
              vertical={false}
              strokeDasharray="3 3"
              className="stroke-border/30 dark:stroke-white/[0.06]"
            />
            <XAxis
              dataKey="hour"
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
                const todayPayload = payload.find((p) => p.dataKey === "today");
                const yesterdayPayload = payload.find((p) => p.dataKey === "yesterday");

                return (
                  <div className="min-w-[160px] rounded-lg border bg-background p-3 shadow-sm">
                    <div className="mb-2 font-medium">{label}</div>
                    <div className="grid gap-2 text-sm">
                      <div className="flex items-center justify-between gap-4">
                        <div className="flex items-center gap-2">
                          <div className="h-2 w-2 rounded-full bg-primary" />
                          <span className="text-muted-foreground">{t("todayRevenue")}:</span>
                        </div>
                        <span className="font-mono font-semibold">
                          {formatCurrency((todayPayload?.value as number) || 0, "USD", 2)}
                        </span>
                      </div>
                      <div className="flex items-center justify-between gap-4">
                        <div className="flex items-center gap-2">
                          <div className="h-2 w-2 rounded-full bg-muted-foreground" />
                          <span className="text-muted-foreground">{t("yesterdayRevenue")}:</span>
                        </div>
                        <span className="font-mono font-semibold">
                          {formatCurrency((yesterdayPayload?.value as number) || 0, "USD", 2)}
                        </span>
                      </div>
                    </div>
                  </div>
                );
              }}
            />
            <Area
              dataKey="yesterday"
              type="monotone"
              fill={`url(#fillYesterday-${chartId})`}
              stroke="var(--color-yesterday)"
              strokeWidth={2}
              strokeDasharray="4 4"
              activeDot={{ r: 4, strokeWidth: 0 }}
            />
            <Area
              dataKey="today"
              type="monotone"
              fill={`url(#fillToday-${chartId})`}
              stroke="var(--color-today)"
              strokeWidth={2}
              activeDot={{ r: 4, strokeWidth: 0 }}
            />
          </AreaChart>
        </ChartContainer>
      </CardContent>
    </Card>
  );
}
