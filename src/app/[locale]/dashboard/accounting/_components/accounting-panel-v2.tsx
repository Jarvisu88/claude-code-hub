"use client";

import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
  type AccountingSummary,
  getRevenueTimeline,
  type RevenueTimelineData,
  saveProviderSellMultiplier,
} from "@/actions/accounting";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { formatCurrency } from "@/lib/utils/currency";
import { RevenueTimelineChart } from "./revenue-timeline-chart";

interface AccountingPanelV2Props {
  summary: AccountingSummary;
}

function SellMultiplierCell({
  providerId,
  value,
  t,
}: {
  providerId: number;
  value: number | null;
  t: (key: string) => string;
}) {
  const [editing, setEditing] = useState(false);
  const [inputValue, setInputValue] = useState(value !== null ? String(value) : "");

  if (!editing) {
    return (
      <span
        className="cursor-pointer rounded px-1 py-0.5 text-right font-mono hover:bg-muted"
        onClick={() => setEditing(true)}
        onKeyDown={(e) => e.key === "Enter" && setEditing(true)}
        role="button"
        tabIndex={0}
      >
        {value !== null ? value.toFixed(2) : t("sellMultiplier.auto")}
      </span>
    );
  }

  const handleSave = async () => {
    setEditing(false);
    const trimmed = inputValue.trim();
    const newValue = trimmed === "" ? null : Number(trimmed);

    if (newValue !== null && (!Number.isFinite(newValue) || newValue < 0)) {
      setInputValue(value !== null ? String(value) : "");
      return;
    }

    if (newValue === value) return;

    const result = await saveProviderSellMultiplier({
      providerId,
      sellMultiplier: newValue,
    });
    if (result.ok) {
      toast.success(t("sellMultiplier.saved"));
    } else {
      toast.error(result.error);
      setInputValue(value !== null ? String(value) : "");
    }
  };

  return (
    <Input
      type="number"
      step="0.01"
      min="0"
      value={inputValue}
      placeholder={t("sellMultiplier.auto")}
      className="h-7 w-24 text-right font-mono text-xs"
      autoFocus
      onChange={(e) => setInputValue(e.target.value)}
      onBlur={() => void handleSave()}
      onKeyDown={(e) => {
        if (e.key === "Enter") void handleSave();
        if (e.key === "Escape") {
          setEditing(false);
          setInputValue(value !== null ? String(value) : "");
        }
      }}
    />
  );
}

export function AccountingPanelV2({ summary }: AccountingPanelV2Props) {
  const t = useTranslations("settings.accounting");
  const [timeRange, setTimeRange] = useState<1 | 7 | 30>(1);
  const [timelineData, setTimelineData] = useState<RevenueTimelineData | null>(null);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [isLoadingTimeline, setIsLoadingTimeline] = useState(false);

  const localTotals = useMemo(() => {
    return summary.providers.reduce(
      (acc, provider) => {
        acc.supplierCostUsd += provider.supplierCostUsd;
        acc.estimatedRevenueUsd += provider.estimatedRevenueUsd;
        acc.estimatedProfitUsd += provider.estimatedProfitUsd;
        return acc;
      },
      { supplierCostUsd: 0, estimatedRevenueUsd: 0, estimatedProfitUsd: 0 }
    );
  }, [summary.providers]);

  const profitMargin =
    localTotals.estimatedRevenueUsd > 0
      ? (localTotals.estimatedProfitUsd / localTotals.estimatedRevenueUsd) * 100
      : 0;

  const providerRows = useMemo(() => {
    const usageMap = new Map(summary.providers.map((p) => [p.providerId, p]));
    return summary.allProviders.map((p) => {
      const usage = usageMap.get(p.providerId);
      return {
        providerId: p.providerId,
        providerName: p.providerName,
        sellMultiplier: p.sellMultiplier,
        requestCount: usage?.requestCount ?? 0,
        supplierCostUsd: usage?.supplierCostUsd ?? 0,
        estimatedRevenueUsd: usage?.estimatedRevenueUsd ?? 0,
        estimatedProfitUsd: usage?.estimatedProfitUsd ?? 0,
      };
    });
  }, [summary.providers, summary.allProviders]);

  const loadTimelineData = useCallback(async () => {
    setIsLoadingTimeline(true);
    try {
      const result = await getRevenueTimeline({ days: timeRange });
      if (result.ok) {
        setTimelineData(result.data);
      } else {
        toast.error(result.error);
      }
    } finally {
      setIsLoadingTimeline(false);
    }
  }, [timeRange]);

  useEffect(() => {
    void loadTimelineData();
  }, [loadTimelineData]);

  useEffect(() => {
    if (!autoRefresh) return;
    const timer = setInterval(() => void loadTimelineData(), 30000);
    return () => clearInterval(timer);
  }, [autoRefresh, loadTimelineData]);

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-1 rounded-md border border-border bg-background p-1">
          {([1, 7, 30] as const).map((days) => (
            <Button
              key={days}
              type="button"
              variant={timeRange === days ? "default" : "ghost"}
              size="sm"
              className="h-7 px-3 text-xs"
              onClick={() => setTimeRange(days)}
            >
              {t(`timeRange.${days === 1 ? "today" : days === 7 ? "week" : "month"}`)}
            </Button>
          ))}
        </div>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Switch
            id="accounting-auto-refresh"
            checked={autoRefresh}
            onCheckedChange={setAutoRefresh}
          />
          <Label htmlFor="accounting-auto-refresh" className="cursor-pointer">
            {t("autoRefresh")}
          </Label>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-4">
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("cards.cost")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(localTotals.supplierCostUsd, "USD", 2)}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("cards.revenue")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div className="text-2xl font-bold">
              {formatCurrency(localTotals.estimatedRevenueUsd, "USD", 2)}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("cards.profit")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div
              className={
                localTotals.estimatedProfitUsd >= 0
                  ? "text-2xl font-bold text-emerald-600"
                  : "text-2xl font-bold text-destructive"
              }
            >
              {formatCurrency(localTotals.estimatedProfitUsd, "USD", 2)}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-medium text-muted-foreground">
              {t("cards.margin")}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <div
              className={
                profitMargin >= 0
                  ? "text-2xl font-bold text-emerald-600"
                  : "text-2xl font-bold text-destructive"
              }
            >
              {profitMargin.toFixed(1)}%
            </div>
          </CardContent>
        </Card>
      </div>

      {timelineData && (
        <RevenueTimelineChart
          title={t("charts.revenueTimeline")}
          data={timelineData}
          isLoading={isLoadingTimeline}
        />
      )}

      <Card>
        <CardHeader>
          <CardTitle className="text-base">{t("providerCosts")}</CardTitle>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t("table.provider")}</TableHead>
                <TableHead className="text-right">{t("table.sellMultiplier")}</TableHead>
                <TableHead className="text-right">{t("table.requests")}</TableHead>
                <TableHead className="text-right">{t("table.cost")}</TableHead>
                <TableHead className="text-right">{t("table.revenue")}</TableHead>
                <TableHead className="text-right">{t("table.profit")}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {providerRows.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={6} className="text-center text-muted-foreground">
                    {t("empty")}
                  </TableCell>
                </TableRow>
              ) : (
                providerRows.map((provider) => (
                  <TableRow key={provider.providerId}>
                    <TableCell className="font-medium">{provider.providerName}</TableCell>
                    <TableCell className="text-right">
                      <SellMultiplierCell
                        providerId={provider.providerId}
                        value={provider.sellMultiplier}
                        t={t}
                      />
                    </TableCell>
                    <TableCell className="text-right font-mono">{provider.requestCount}</TableCell>
                    <TableCell className="text-right font-mono">
                      {formatCurrency(provider.supplierCostUsd, "USD", 2)}
                    </TableCell>
                    <TableCell className="text-right font-mono">
                      {formatCurrency(provider.estimatedRevenueUsd, "USD", 2)}
                    </TableCell>
                    <TableCell
                      className={
                        provider.estimatedProfitUsd >= 0
                          ? "text-right font-mono text-emerald-600"
                          : "text-right font-mono text-destructive"
                      }
                    >
                      {formatCurrency(provider.estimatedProfitUsd, "USD", 2)}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>
    </div>
  );
}
