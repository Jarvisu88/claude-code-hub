"use client";

import { RefreshCw, Save, Settings } from "lucide-react";
import { useTranslations } from "next-intl";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import {
  type AccountingSummary,
  fetchNewApiRevenue,
  type NewApiRevenueRow,
  saveGlobalSellMultiplier,
  saveNewApiConfig,
} from "@/actions/accounting";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
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
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { formatCurrency } from "@/lib/utils/currency";

interface AccountingPanelProps {
  summary: AccountingSummary;
}

interface NewApiConfigForm {
  baseUrl: string;
  accessToken: string;
  userId: string;
  pageSize: string;
}

type AccountingPlatform = "new-api";

function formatUsd(value: number): string {
  return formatCurrency(value, "USD", 2);
}

function formatMultiplier(value: number): string {
  return Number.isFinite(value) ? value.toFixed(2) : "0.00";
}

function formatUsage(value: number): string {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 6,
  }).format(value);
}

function parsePositiveInt(value: string): number | null {
  const parsed = Number.parseInt(value, 10);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : null;
}

function calculateDisplayedRevenue(row: NewApiRevenueRow, configuredMultiplier: string): number {
  const manualMultiplier = Number(configuredMultiplier);
  if (!Number.isFinite(manualMultiplier) || manualMultiplier <= 0) {
    return row.revenueUsd;
  }

  const sourceMultiplier =
    Number.isFinite(row.multiplier) && row.multiplier > 0 ? row.multiplier : 1;
  return (row.revenueUsd / sourceMultiplier) * manualMultiplier;
}

export function AccountingPanel({ summary }: AccountingPanelProps) {
  const t = useTranslations("settings.accounting");
  const [globalMultiplier, setGlobalMultiplier] = useState(
    formatMultiplier(summary.globalSellMultiplier)
  );
  const [configOpen, setConfigOpen] = useState(false);
  const [activePlatform, setActivePlatform] = useState<AccountingPlatform>("new-api");
  const [configPreview, setConfigPreview] = useState(summary.newApiConfig);
  const [configForm, setConfigForm] = useState<NewApiConfigForm>({
    baseUrl: summary.newApiConfig.baseUrl,
    accessToken: "",
    userId: summary.newApiConfig.adminUserId ? String(summary.newApiConfig.adminUserId) : "",
    pageSize: String(summary.newApiConfig.pageSize),
  });
  const [newApiRevenueRows, setNewApiRevenueRows] = useState<NewApiRevenueRow[]>([]);
  const [totalLogs, setTotalLogs] = useState(0);
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [isSavingMultiplier, setIsSavingMultiplier] = useState(false);
  const [isSavingConfig, setIsSavingConfig] = useState(false);
  const [isFetchingRevenue, setIsFetchingRevenue] = useState(false);

  const localTotals = useMemo(() => {
    return summary.providers.reduce(
      (acc, provider) => {
        acc.supplierCostUsd += provider.supplierCostUsd;
        acc.estimatedRevenueUsd += provider.estimatedRevenueUsd;
        return acc;
      },
      { supplierCostUsd: 0, estimatedRevenueUsd: 0 }
    );
  }, [summary.providers]);

  const newApiTotals = useMemo(() => {
    return newApiRevenueRows.reduce(
      (acc, row) => {
        acc.quota += row.quota;
        acc.inputTokens += row.inputTokens;
        acc.outputTokens += row.outputTokens;
        acc.revenueUsd += calculateDisplayedRevenue(row, globalMultiplier);
        acc.requestCount += row.requestCount;
        return acc;
      },
      { quota: 0, inputTokens: 0, outputTokens: 0, revenueUsd: 0, requestCount: 0 }
    );
  }, [globalMultiplier, newApiRevenueRows]);

  const hasNewApiRevenue = newApiRevenueRows.length > 0;
  const useNewApiOnly = Number(globalMultiplier) === 0;
  const revenueUsd = hasNewApiRevenue
    ? newApiTotals.revenueUsd
    : useNewApiOnly
      ? 0
      : localTotals.estimatedRevenueUsd;
  const profitUsd = revenueUsd - localTotals.supplierCostUsd;

  const handleSaveGlobalMultiplier = async () => {
    setIsSavingMultiplier(true);
    try {
      const result = await saveGlobalSellMultiplier({ multiplier: Number(globalMultiplier) });
      if (result.ok) {
        toast.success(t("saved"));
      } else {
        toast.error(result.error);
      }
    } finally {
      setIsSavingMultiplier(false);
    }
  };

  const handleSaveConfig = async () => {
    const userId = parsePositiveInt(configForm.userId);
    const pageSize = parsePositiveInt(configForm.pageSize);

    if (!configForm.baseUrl.trim() || !userId || !pageSize) {
      toast.error(t("newApi.invalidConfig"));
      return;
    }

    setIsSavingConfig(true);
    try {
      const result = await saveNewApiConfig({
        baseUrl: configForm.baseUrl,
        accessToken: configForm.accessToken,
        userId,
        pageSize,
      });

      if (result.ok) {
        setConfigPreview(result.data);
        setConfigForm((current) => ({
          ...current,
          accessToken: "",
          baseUrl: result.data.baseUrl,
          userId: result.data.adminUserId ? String(result.data.adminUserId) : "",
          pageSize: String(result.data.pageSize),
        }));
        setConfigOpen(false);
        toast.success(t("saved"));
      } else {
        toast.error(result.error);
      }
    } finally {
      setIsSavingConfig(false);
    }
  };

  const handleFetchNewApiRevenue = useCallback(
    async (silent = false) => {
      if (!configPreview.configured) {
        toast.error(t("newApi.notConfigured"));
        setConfigOpen(true);
        return;
      }

      setIsFetchingRevenue(true);
      try {
        const result = await fetchNewApiRevenue({ poll: silent });
        if (result.ok) {
          setNewApiRevenueRows(result.data.rows);
          setTotalLogs(result.data.totalLogs);
          if (!silent) {
            toast.success(t("newApi.loaded"));
          }
        } else {
          if (!silent) {
            toast.error(result.error);
          }
          setAutoRefresh(false);
        }
      } finally {
        setIsFetchingRevenue(false);
      }
    },
    [configPreview.configured, t]
  );

  useEffect(() => {
    if (!autoRefresh || !configPreview.configured) return;

    const timer = setInterval(() => {
      void handleFetchNewApiRevenue(true);
    }, 30000);

    return () => clearInterval(timer);
  }, [autoRefresh, configPreview.configured, handleFetchNewApiRevenue]);

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
          <Tabs
            value={activePlatform}
            onValueChange={(value) => setActivePlatform(value as AccountingPlatform)}
          >
            <TabsList className="w-full sm:w-auto">
              <TabsTrigger value="new-api">{t("platforms.newApi")}</TabsTrigger>
            </TabsList>
          </Tabs>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => setConfigOpen(true)}
              disabled={isSavingConfig}
            >
              <Settings className="mr-2 size-4" />
              {t("platformActions.configure")}
            </Button>
            <Button
              type="button"
              onClick={() => handleFetchNewApiRevenue(false)}
              disabled={
                isFetchingRevenue || !configPreview.configured || activePlatform !== "new-api"
              }
            >
              <RefreshCw
                className={isFetchingRevenue ? "mr-2 size-4 animate-spin" : "mr-2 size-4"}
              />
              {t("platformActions.loadLogs")}
            </Button>
            <Badge variant={configPreview.configured ? "default" : "secondary"}>
              {configPreview.configured ? t("newApi.configured") : t("newApi.notConfiguredBadge")}
            </Badge>
          </div>
        </div>
        <div className="flex items-center gap-2 text-sm text-muted-foreground">
          <Switch
            id="accounting-auto-refresh"
            checked={autoRefresh}
            disabled={!configPreview.configured}
            onCheckedChange={setAutoRefresh}
          />
          <Label htmlFor="accounting-auto-refresh">{t("newApi.autoRefresh")}</Label>
        </div>
      </div>

      <div className="grid gap-3 md:grid-cols-4">
        <Card className="rounded-lg py-4">
          <CardHeader className="px-4">
            <CardTitle className="text-sm">{t("totals.cost")}</CardTitle>
          </CardHeader>
          <CardContent className="px-4 text-2xl font-semibold">
            {formatUsd(localTotals.supplierCostUsd)}
          </CardContent>
        </Card>
        <Card className="rounded-lg py-4">
          <CardHeader className="px-4">
            <CardTitle className="text-sm">{t("totals.revenue")}</CardTitle>
          </CardHeader>
          <CardContent className="px-4 text-2xl font-semibold">{formatUsd(revenueUsd)}</CardContent>
        </Card>
        <Card className="rounded-lg py-4">
          <CardHeader className="px-4">
            <CardTitle className="text-sm">{t("totals.profit")}</CardTitle>
          </CardHeader>
          <CardContent
            className={
              profitUsd >= 0
                ? "px-4 text-2xl font-semibold text-emerald-600"
                : "px-4 text-2xl font-semibold text-destructive"
            }
          >
            {formatUsd(profitUsd)}
          </CardContent>
        </Card>
        <Card className="rounded-lg py-4">
          <CardHeader className="px-4">
            <CardTitle className="text-sm">{t("totals.logs")}</CardTitle>
          </CardHeader>
          <CardContent className="px-4 text-2xl font-semibold">{totalLogs}</CardContent>
        </Card>
      </div>

      <div className="grid gap-4">
        <div className="space-y-4">
          <Card className="rounded-lg">
            <CardHeader>
              <CardTitle className="text-base">{t("usageLog.title")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("newApi.table.username")}</TableHead>
                    <TableHead>{t("newApi.table.group")}</TableHead>
                    <TableHead>{t("newApi.table.model")}</TableHead>
                    <TableHead>{t("newApi.table.requests")}</TableHead>
                    <TableHead>{t("newApi.table.tokens")}</TableHead>
                    <TableHead>{t("newApi.table.quota")}</TableHead>
                    <TableHead>{t("newApi.table.modelRatio")}</TableHead>
                    <TableHead>{t("newApi.table.groupRatio")}</TableHead>
                    <TableHead>{t("newApi.table.revenue")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {newApiRevenueRows.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={9} className="text-muted-foreground">
                        {t("newApi.empty")}
                      </TableCell>
                    </TableRow>
                  ) : (
                    newApiRevenueRows.map((row) => (
                      <TableRow
                        key={`${row.username}-${row.group}-${row.modelName}-${row.multiplier}`}
                      >
                        <TableCell className="font-medium">{row.username}</TableCell>
                        <TableCell>{row.group}</TableCell>
                        <TableCell>{row.modelName}</TableCell>
                        <TableCell>{row.requestCount}</TableCell>
                        <TableCell>
                          {formatUsage(row.inputTokens)} / {formatUsage(row.outputTokens)}
                        </TableCell>
                        <TableCell>{formatUsage(row.quota)}</TableCell>
                        <TableCell>{formatMultiplier(row.modelRatio)}</TableCell>
                        <TableCell>{formatMultiplier(row.groupRatio)}</TableCell>
                        <TableCell>
                          {formatUsd(calculateDisplayedRevenue(row, globalMultiplier))}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>

          <Card className="rounded-lg">
            <CardHeader>
              <CardTitle className="text-base">{t("provider.title")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("table.provider")}</TableHead>
                    <TableHead>{t("table.requests")}</TableHead>
                    <TableHead>{t("table.cost")}</TableHead>
                    <TableHead>{t("table.estimatedRevenue")}</TableHead>
                    <TableHead>{t("table.profit")}</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {summary.providers.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={5} className="text-muted-foreground">
                        {t("empty")}
                      </TableCell>
                    </TableRow>
                  ) : (
                    summary.providers.map((provider) => (
                      <TableRow key={provider.providerId}>
                        <TableCell className="font-medium">{provider.providerName}</TableCell>
                        <TableCell>{provider.requestCount}</TableCell>
                        <TableCell>{formatUsd(provider.supplierCostUsd)}</TableCell>
                        <TableCell>{formatUsd(provider.estimatedRevenueUsd)}</TableCell>
                        <TableCell
                          className={
                            provider.estimatedProfitUsd >= 0
                              ? "text-emerald-600"
                              : "text-destructive"
                          }
                        >
                          {formatUsd(provider.estimatedProfitUsd)}
                        </TableCell>
                      </TableRow>
                    ))
                  )}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        </div>
      </div>

      <Dialog open={configOpen} onOpenChange={setConfigOpen}>
        <DialogContent className="sm:max-w-[520px]">
          <DialogHeader>
            <DialogTitle>{t("newApi.dialogTitle")}</DialogTitle>
            <DialogDescription>{t("newApi.dialogDescription")}</DialogDescription>
          </DialogHeader>
          <div className="space-y-4">
            <div className="space-y-2">
              <Label htmlFor="default-sale-multiplier">{t("global.title")}</Label>
              <div className="flex gap-2">
                <Input
                  id="default-sale-multiplier"
                  value={globalMultiplier}
                  inputMode="decimal"
                  onChange={(event) => setGlobalMultiplier(event.target.value)}
                />
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleSaveGlobalMultiplier}
                  disabled={isSavingMultiplier}
                >
                  <Save className="mr-2 size-4" />
                  {t("global.save")}
                </Button>
              </div>
              <p className="text-xs text-muted-foreground">{t("global.zeroRule")}</p>
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-api-base-url">{t("newApi.baseUrl")}</Label>
              <Input
                id="new-api-base-url"
                value={configForm.baseUrl}
                placeholder={t("newApi.baseUrlPlaceholder")}
                onChange={(event) =>
                  setConfigForm((current) => ({ ...current, baseUrl: event.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="new-api-access-token">{t("newApi.accessToken")}</Label>
              <Input
                id="new-api-access-token"
                value={configForm.accessToken}
                type="password"
                placeholder={
                  configPreview.configured ? t("newApi.keepExistingToken") : t("newApi.accessToken")
                }
                onChange={(event) =>
                  setConfigForm((current) => ({ ...current, accessToken: event.target.value }))
                }
              />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor="new-api-user-id">{t("newApi.userId")}</Label>
                <Input
                  id="new-api-user-id"
                  value={configForm.userId}
                  inputMode="numeric"
                  onChange={(event) =>
                    setConfigForm((current) => ({ ...current, userId: event.target.value }))
                  }
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor="new-api-page-size">{t("newApi.pageSize")}</Label>
                <Input
                  id="new-api-page-size"
                  value={configForm.pageSize}
                  inputMode="numeric"
                  onChange={(event) =>
                    setConfigForm((current) => ({ ...current, pageSize: event.target.value }))
                  }
                />
              </div>
            </div>
          </div>
          <DialogFooter>
            <Button type="button" onClick={handleSaveConfig} disabled={isSavingConfig}>
              <Save className="mr-2 size-4" />
              {t("newApi.saveConfig")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
