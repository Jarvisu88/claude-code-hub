"use client";

import { useQueryClient } from "@tanstack/react-query";
import { ArrowRight, ListOrdered, Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useState, useTransition } from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  autoSortProviderPriority,
  runProviderLatencyPriorityWorkflowNow,
} from "@/lib/api-client/v1/actions/providers";

type AutoSortResult = {
  groups: Array<{
    metric: number | null;
    costMultiplier?: number;
    avgLatencyMs?: number | null;
    priority: number;
    providers: Array<{ id: number; name: string }>;
  }>;
  changes: Array<{
    providerId: number;
    name: string;
    oldPriority: number;
    newPriority: number;
    costMultiplier: number;
    avgLatencyMs: number | null;
  }>;
  summary: {
    totalProviders: number;
    changedCount: number;
    groupCount: number;
  };
  applied: boolean;
  mode: "price" | "latency";
  providerGroup: string | null;
  targetGroup: string | null;
};

export function AutoSortPriorityDialog() {
  const queryClient = useQueryClient();
  const t = useTranslations("settings.providers.autoSort");
  const tCommon = useTranslations("settings.common");
  const tErrors = useTranslations("errors");

  const [open, setOpen] = useState(false);
  const [previewData, setPreviewData] = useState<AutoSortResult | null>(null);
  const [sortMode, setSortMode] = useState<"price" | "latency">("price");
  const [providerGroup, setProviderGroup] = useState("");
  const [targetGroup, setTargetGroup] = useState("");
  const [isPending, startTransition] = useTransition();
  const [isApplying, setIsApplying] = useState(false);
  const [isRunningWorkflow, setIsRunningWorkflow] = useState(false);

  const getActionErrorMessage = (result: {
    errorCode?: string;
    errorParams?: Record<string, string | number>;
    error?: string | null;
  }): string => {
    if (result.errorCode) {
      try {
        return tErrors(result.errorCode, result.errorParams);
      } catch {
        return t("error");
      }
    }

    if (result.error) {
      try {
        return tErrors(result.error);
      } catch {
        return t("error");
      }
    }

    return t("error");
  };

  const handleOpenChange = (isOpen: boolean) => {
    setOpen(isOpen);
    if (isOpen) {
      loadPreview(sortMode, providerGroup);
    } else {
      setPreviewData(null);
    }
  };

  const loadPreview = (mode: "price" | "latency", group: string) => {
    startTransition(async () => {
      try {
        const result = await autoSortProviderPriority({
          confirm: false,
          mode,
          providerGroup: group.trim() || null,
          targetGroup: targetGroup.trim() || group.trim() || null,
        });
        if (result.ok) {
          setPreviewData(result.data);
        } else {
          toast.error(getActionErrorMessage(result));
          setOpen(false);
        }
      } catch (error) {
        console.error("autoSortProviderPriority preview failed", error);
        toast.error(t("error"));
        setOpen(false);
      }
    });
  };

  const handleApply = async () => {
    setIsApplying(true);
    try {
      const result = await autoSortProviderPriority({
        confirm: true,
        mode: sortMode,
        providerGroup: providerGroup.trim() || null,
        targetGroup: targetGroup.trim() || providerGroup.trim() || null,
      });
      if (result.ok) {
        toast.success(t("success", { count: result.data.summary.changedCount }));
        queryClient.invalidateQueries({ queryKey: ["providers"] });
        setOpen(false);
      } else {
        toast.error(getActionErrorMessage(result));
      }
    } catch (error) {
      console.error("autoSortProviderPriority apply failed", error);
      toast.error(t("error"));
    } finally {
      setIsApplying(false);
    }
  };

  const handleRunLatencyWorkflow = async () => {
    const resolvedTargetGroup = targetGroup.trim() || providerGroup.trim();
    if (!resolvedTargetGroup) {
      toast.error(t("targetGroupRequired"));
      return;
    }

    setIsRunningWorkflow(true);
    try {
      const result = await runProviderLatencyPriorityWorkflowNow({
        confirm: true,
        providerGroup: providerGroup.trim() || null,
        targetGroup: resolvedTargetGroup,
      });
      if (result.ok) {
        toast.success(t("latencyWorkflowSuccess", { count: result.data.summary.changedCount }));
        queryClient.invalidateQueries({ queryKey: ["providers"] });
        setPreviewData(result.data);
      } else {
        toast.error(getActionErrorMessage(result));
      }
    } catch (error) {
      console.error("runProviderLatencyPriorityWorkflowNow failed", error);
      toast.error(t("error"));
    } finally {
      setIsRunningWorkflow(false);
    }
  };

  const hasChanges = previewData && previewData.summary.changedCount > 0;

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogTrigger asChild>
        <Button variant="outline" size="sm">
          <ListOrdered className="h-4 w-4" />
          {t("button")}
        </Button>
      </DialogTrigger>
      <DialogContent className="max-w-2xl max-h-[var(--cch-viewport-height-80)] flex flex-col">
        <DialogHeader>
          <DialogTitle>{t("dialogTitle")}</DialogTitle>
          <DialogDescription>{t("dialogDescription")}</DialogDescription>
        </DialogHeader>

        <div className="flex-1 overflow-y-auto space-y-4 py-4">
          <div className="grid gap-3 sm:grid-cols-[180px_1fr_1fr_auto] sm:items-end">
            <div className="space-y-2">
              <Label htmlFor="provider-priority-sort-mode">{t("modeLabel")}</Label>
              <Select
                value={sortMode}
                onValueChange={(value) => setSortMode(value as "price" | "latency")}
              >
                <SelectTrigger id="provider-priority-sort-mode">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="price">{t("modePrice")}</SelectItem>
                  <SelectItem value="latency">{t("modeLatency")}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className="space-y-2">
              <Label htmlFor="provider-priority-sort-group">{t("providerGroupLabel")}</Label>
              <Input
                id="provider-priority-sort-group"
                name="provider-priority-sort-group"
                value={providerGroup}
                placeholder={t("providerGroupPlaceholder")}
                onChange={(event) => setProviderGroup(event.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="provider-priority-target-group">{t("targetGroupLabel")}</Label>
              <Input
                id="provider-priority-target-group"
                name="provider-priority-target-group"
                value={targetGroup}
                placeholder={t("targetGroupPlaceholder")}
                onChange={(event) => setTargetGroup(event.target.value)}
              />
            </div>
            <Button
              type="button"
              variant="outline"
              onClick={() => loadPreview(sortMode, providerGroup)}
              disabled={isPending}
            >
              {t("preview")}
            </Button>
          </div>

          {isPending ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : previewData ? (
            <>
              {/* Summary */}
              <div className="text-sm text-muted-foreground">
                {hasChanges
                  ? t("changeCount", { count: previewData.summary.changedCount })
                  : t("noChanges")}
              </div>

              {/* Groups Preview Table */}
              {previewData.groups.length > 0 && (
                <div className="border rounded-lg">
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-[140px]">
                          {sortMode === "latency"
                            ? t("latencyHeader")
                            : t("costMultiplierHeader")}
                        </TableHead>
                        <TableHead className="w-[100px]">{t("priorityHeader")}</TableHead>
                        <TableHead>{t("providersHeader")}</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {previewData.groups.map((group) => (
                        <TableRow key={`${group.priority}-${group.metric ?? "unknown"}`}>
                          <TableCell className="font-mono">
                            {formatMetric(group, sortMode, t)}
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline">{group.priority}</Badge>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-wrap gap-1">
                              {group.providers.map((provider) => (
                                <Badge key={provider.id} variant="secondary">
                                  {provider.name}
                                </Badge>
                              ))}
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              )}

              {/* Changes Detail */}
              {hasChanges && (
                <div className="space-y-2">
                  <h4 className="text-sm font-medium">{t("changesTitle")}</h4>
                  <div className="border rounded-lg">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>{t("providerHeader")}</TableHead>
                          <TableHead className="w-[180px] text-center">
                            {t("priorityChangeHeader")}
                          </TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {previewData.changes.map((change) => (
                          <TableRow key={change.providerId}>
                            <TableCell>
                              <span className="font-medium">{change.name}</span>
                              <span className="text-muted-foreground text-xs ml-2">
                                ({formatChangeMetric(change, sortMode, t)})
                              </span>
                            </TableCell>
                            <TableCell>
                              <div className="flex items-center justify-center gap-2">
                                <Badge variant="outline" className="font-mono">
                                  {change.oldPriority}
                                </Badge>
                                <ArrowRight className="h-4 w-4 text-muted-foreground" />
                                <Badge variant="default" className="font-mono">
                                  {change.newPriority}
                                </Badge>
                              </div>
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                </div>
              )}
            </>
          ) : null}
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => setOpen(false)} disabled={isApplying}>
            {tCommon("cancel")}
          </Button>
          <Button onClick={handleApply} disabled={isPending || isApplying || !hasChanges}>
            {isApplying && <Loader2 className="h-4 w-4 animate-spin" />}
            {t("confirm")}
          </Button>
          <Button
            onClick={handleRunLatencyWorkflow}
            disabled={sortMode !== "latency" || isPending || isApplying || isRunningWorkflow}
          >
            {isRunningWorkflow && <Loader2 className="h-4 w-4 animate-spin" />}
            {t("runLatencyWorkflow")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function formatMetric(
  group: AutoSortResult["groups"][number],
  mode: "price" | "latency",
  t: ReturnType<typeof useTranslations>
) {
  if (mode === "latency") {
    return group.avgLatencyMs == null ? t("unknownLatency") : `${Math.round(group.avgLatencyMs)}ms`;
  }
  return group.costMultiplier == null ? "-" : `${group.costMultiplier}x`;
}

function formatChangeMetric(
  change: AutoSortResult["changes"][number],
  mode: "price" | "latency",
  t: ReturnType<typeof useTranslations>
) {
  if (mode === "latency") {
    return change.avgLatencyMs == null ? t("unknownLatency") : `${Math.round(change.avgLatencyMs)}ms`;
  }
  return `${change.costMultiplier}x`;
}
