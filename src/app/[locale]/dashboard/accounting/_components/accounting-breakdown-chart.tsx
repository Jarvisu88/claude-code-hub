"use client";

import { useTranslations } from "next-intl";
import * as React from "react";
import { Bar, BarChart, CartesianGrid, Cell, XAxis, YAxis } from "recharts";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { type ChartConfig, ChartContainer, ChartTooltip } from "@/components/ui/chart";
import { formatCurrency } from "@/lib/utils/currency";

export interface AccountingBreakdownData {
  name: string;
  value: number;
}

export interface AccountingBreakdownChartProps {
  title: string;
  data: AccountingBreakdownData[];
}

const NEW_API_PALETTE = [
  "#5470c6",
  "#91cc75",
  "#fac858",
  "#ee6666",
  "#73c0de",
  "#3ba272",
  "#fc8452",
  "#9a60b4",
  "#ea7ccc",
  "#1677ff",
  "#52c41a",
  "#faad14",
  "#f5222d",
  "#13c2c2",
  "#eb2f96",
  "#722ed1",
  "#fa8c16",
  "#a0d911",
  "#2f54eb",
  "#0088FE",
  "#00C49F",
  "#FFBB28",
  "#FF8042",
  "#8884d8",
];

function getStringColor(str: string) {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    hash = str.charCodeAt(i) + ((hash << 5) - hash);
  }
  return NEW_API_PALETTE[Math.abs(hash) % NEW_API_PALETTE.length];
}

export function AccountingBreakdownChart({ title, data }: AccountingBreakdownChartProps) {
  const t = useTranslations("settings.accounting.charts");
  const chartId = React.useId().replace(/:/g, "");

  const chartData = React.useMemo(() => {
    const sorted = [...data].filter((item) => item.value > 0).sort((a, b) => b.value - a.value);

    // 取前 11 条数据，其余归为“其他”
    if (sorted.length > 11) {
      const topData = sorted.slice(0, 11);
      const othersValue = sorted.slice(11).reduce((sum, item) => sum + item.value, 0);
      if (othersValue > 0) {
        topData.push({ name: t("others"), value: othersValue });
      }
      return topData;
    }

    return sorted;
  }, [data, t]);

  const chartConfig: ChartConfig = React.useMemo(() => {
    const config: ChartConfig = {};
    chartData.forEach((item) => {
      const isOthers = item.name === t("others");
      config[item.name] = {
        label: item.name,
        color: isOthers ? "hsl(var(--muted-foreground))" : getStringColor(item.name),
      };
    });
    return config;
  }, [chartData, t]);

  const totalValue = React.useMemo(() => {
    return chartData.reduce((sum, item) => sum + item.value, 0);
  }, [chartData]);

  return (
    <Card className="flex h-full flex-col rounded-lg">
      <CardHeader className="border-b border-border/50 pb-3 pt-4">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="flex-1 p-4">
        {chartData.length === 0 ? (
          <div className="flex h-[240px] items-center justify-center text-sm text-muted-foreground">
            {t("noData")}
          </div>
        ) : (
          <ChartContainer config={chartConfig} className="aspect-auto h-[280px] w-full">
            <BarChart
              data={chartData}
              layout="vertical"
              margin={{ left: 0, right: 16, top: 12, bottom: 0 }}
            >
              <defs>
                {chartData.map((entry, index) => {
                  const isOthers = entry.name === t("others");
                  const color = isOthers
                    ? "hsl(var(--muted-foreground))"
                    : getStringColor(entry.name);
                  return (
                    <linearGradient
                      key={`gradient-${index}`}
                      id={`fillBar-${chartId}-${index}`}
                      x1="0"
                      y1="0"
                      x2="1"
                      y2="0"
                    >
                      <stop offset="0%" stopColor={color} stopOpacity={0.5} />
                      <stop offset="100%" stopColor={color} stopOpacity={0.9} />
                    </linearGradient>
                  );
                })}
              </defs>
              <CartesianGrid
                horizontal={false}
                vertical={true}
                strokeDasharray="3 3"
                className="stroke-border/30 dark:stroke-white/[0.06]"
              />
              <XAxis
                type="number"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                tickFormatter={(value) => formatCurrency(value, "USD", 0)}
                className="text-[10px] fill-muted-foreground"
              />
              <YAxis
                type="category"
                dataKey="name"
                tickLine={false}
                axisLine={false}
                tickMargin={8}
                width={100}
                className="text-[10px] fill-muted-foreground"
                tickFormatter={(value: string) =>
                  value.length > 14 ? `${value.slice(0, 14)}...` : value
                }
              />
              <ChartTooltip
                cursor={{ fill: "hsl(var(--muted))", opacity: 0.1 }}
                wrapperStyle={{ zIndex: 1000 }}
                content={({ active, payload }) => {
                  if (!active || !payload?.length) return <div className="hidden" />;

                  const data = payload[0].payload as AccountingBreakdownData;
                  const percentage =
                    totalValue > 0 ? ((data.value / totalValue) * 100).toFixed(1) : "0.0";

                  const isOthers = data.name === t("others");
                  const color = isOthers
                    ? "hsl(var(--muted-foreground))"
                    : getStringColor(data.name);

                  return (
                    <div className="min-w-[160px] rounded-lg border bg-background p-3 shadow-sm">
                      <div className="grid gap-2 text-sm">
                        <div className="flex items-center gap-2 truncate font-medium">
                          <div
                            className="h-2 w-2 rounded-full flex-shrink-0"
                            style={{ backgroundColor: color }}
                          />
                          {data.name}
                        </div>
                        <div className="flex items-center justify-between gap-4">
                          <span className="text-muted-foreground">{t("revenue")}:</span>
                          <span className="font-mono font-semibold">
                            {formatCurrency(data.value, "USD", 2)}
                          </span>
                        </div>
                        <div className="flex items-center justify-between gap-4">
                          <span className="text-muted-foreground">{t("percentage")}:</span>
                          <span className="font-mono">{percentage}%</span>
                        </div>
                      </div>
                    </div>
                  );
                }}
              />
              <Bar dataKey="value" radius={[0, 4, 4, 0]} maxBarSize={24}>
                {chartData.map((entry, index) => {
                  const isOthers = entry.name === t("others");
                  const color = isOthers
                    ? "hsl(var(--muted-foreground))"
                    : getStringColor(entry.name);
                  return (
                    <Cell
                      key={`cell-${index}`}
                      fill={`url(#fillBar-${chartId}-${index})`}
                      stroke={color}
                      strokeWidth={1}
                    />
                  );
                })}
              </Bar>
            </BarChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  );
}
