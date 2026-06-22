"use client";

import { Activity, Loader2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useMemo, useState, useTransition } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip";
import {
  editProvider,
  runProviderLatencyProbe,
} from "@/lib/api-client/v1/actions/providers";
import type { ProviderDisplay } from "@/types/provider";

interface MiniProbeDialogProps {
  provider: ProviderDisplay;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  onUpdated?: () => void | Promise<void>;
}

type MiniProbeForm = {
  enabled: boolean;
  model: string;
  intervalMs: string;
  timeStart: string;
  timeEnd: string;
};

export function MiniProbeDialog({
  provider,
  open,
  onOpenChange,
  onUpdated,
}: MiniProbeDialogProps) {
  const t = useTranslations("settings.providers.list.miniProbe");
  const tList = useTranslations("settings.providers.list");
  const [internalOpen, setInternalOpen] = useState(false);
  const [form, setForm] = useState<MiniProbeForm>(() => createForm(provider));
  const [savePending, startSaveTransition] = useTransition();
  const [runPending, startRunTransition] = useTransition();
  const isOpen = open ?? internalOpen;
  const setOpen = onOpenChange ?? setInternalOpen;

  const lastResult = useMemo(() => formatLastResult(provider, t), [provider, t]);

  useEffect(() => {
    if (isOpen) {
      setForm(createForm(provider));
    }
  }, [isOpen, provider]);

  const handleOpenChange = (nextOpen: boolean) => {
    if (nextOpen) setForm(createForm(provider));
    setOpen(nextOpen);
  };

  const handleSave = () => {
    startSaveTransition(async () => {
      try {
        const payload = toProviderPatch(form);
        const result = await editProvider(provider.id, payload);
        if (!result.ok) {
          toast.error(t("saveFailed"), { description: result.error || tList("unknownError") });
          return;
        }
        toast.success(t("saveSuccess"));
        await onUpdated?.();
      } catch (error) {
        console.error("Failed to save Mini probe settings:", error);
        toast.error(t("saveFailed"), { description: tList("unknownError") });
      }
    });
  };

  const handleRun = () => {
    startRunTransition(async () => {
      try {
        const result = await runProviderLatencyProbe(provider.id);
        if (!result.ok) {
          toast.error(t("runFailed"), { description: result.error || tList("unknownError") });
          return;
        }
        const description =
          result.data.avgLatencyMs === null
            ? t("failedStatus")
            : t("avgLatency", { value: Math.round(result.data.avgLatencyMs) });
        toast.success(t("runSuccess"), { description });
        await onUpdated?.();
      } catch (error) {
        console.error("Failed to run Mini probe:", error);
        toast.error(t("runFailed"), { description: tList("unknownError") });
      }
    });
  };

  return (
    <Dialog open={isOpen} onOpenChange={handleOpenChange}>
      {open === undefined && (
        <Tooltip delayDuration={200}>
          <TooltipTrigger asChild>
            <DialogTrigger asChild>
              <Button
                type="button"
                size="icon"
                variant="ghost"
                aria-label={tList("actionMiniProbe")}
                onClick={(event) => event.stopPropagation()}
              >
                <Activity className="h-4 w-4" />
              </Button>
            </DialogTrigger>
          </TooltipTrigger>
          <TooltipContent side="top" className="text-xs">
            {tList("actionMiniProbe")}
          </TooltipContent>
        </Tooltip>
      )}
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>

        <div className="space-y-5">
          <div className="flex items-start justify-between gap-4 rounded-md border p-3">
            <div className="space-y-1">
              <Label htmlFor={`latency-probe-enabled-${provider.id}`}>{t("enabled")}</Label>
              <p className="text-xs text-muted-foreground">{t("enabledDescription")}</p>
            </div>
            <Switch
              id={`latency-probe-enabled-${provider.id}`}
              checked={form.enabled}
              onCheckedChange={(enabled) => setForm((prev) => ({ ...prev, enabled }))}
            />
          </div>

          <div className="grid gap-4 sm:grid-cols-2">
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor={`latency-probe-model-${provider.id}`}>{t("model")}</Label>
              <Input
                id={`latency-probe-model-${provider.id}`}
                value={form.model}
                placeholder={t("modelPlaceholder")}
                onChange={(event) =>
                  setForm((prev) => ({ ...prev, model: event.target.value }))
                }
              />
            </div>
            <div className="space-y-2 sm:col-span-2">
              <Label htmlFor={`latency-probe-interval-${provider.id}`}>{t("intervalMs")}</Label>
              <Input
                id={`latency-probe-interval-${provider.id}`}
                type="number"
                min={10000}
                step={1000}
                value={form.intervalMs}
                placeholder={t("intervalPlaceholder")}
                onChange={(event) =>
                  setForm((prev) => ({ ...prev, intervalMs: event.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor={`latency-probe-start-${provider.id}`}>{t("timeStart")}</Label>
              <Input
                id={`latency-probe-start-${provider.id}`}
                type="time"
                value={form.timeStart}
                onChange={(event) =>
                  setForm((prev) => ({ ...prev, timeStart: event.target.value }))
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor={`latency-probe-end-${provider.id}`}>{t("timeEnd")}</Label>
              <Input
                id={`latency-probe-end-${provider.id}`}
                type="time"
                value={form.timeEnd}
                onChange={(event) =>
                  setForm((prev) => ({ ...prev, timeEnd: event.target.value }))
                }
              />
            </div>
          </div>

          <div className="rounded-md bg-muted/40 p-3 text-sm">
            <div className="text-xs font-medium uppercase text-muted-foreground">
              {t("lastResult")}
            </div>
            <div className="mt-1 font-medium">{lastResult}</div>
          </div>

          <div className="flex flex-wrap justify-between gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => setForm((prev) => ({ ...prev, timeStart: "", timeEnd: "" }))}
            >
              {t("clearWindow")}
            </Button>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="outline"
                onClick={handleRun}
                disabled={runPending || savePending}
              >
                {runPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {runPending ? t("running") : t("runNow")}
              </Button>
              <Button type="button" onClick={handleSave} disabled={savePending || runPending}>
                {savePending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                {savePending ? t("saving") : t("save")}
              </Button>
            </div>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

function createForm(provider: ProviderDisplay): MiniProbeForm {
  return {
    enabled: provider.latencyProbeEnabled ?? false,
    model: provider.latencyProbeModel ?? "",
    intervalMs: provider.latencyProbeIntervalMs?.toString() ?? "",
    timeStart: provider.latencyProbeTimeStart ?? "",
    timeEnd: provider.latencyProbeTimeEnd ?? "",
  };
}

function toProviderPatch(form: MiniProbeForm) {
  const trimmedModel = form.model.trim();
  const intervalMs = Number(form.intervalMs);
  const hasWindow = form.timeStart.trim() && form.timeEnd.trim();
  return {
    latency_probe_enabled: form.enabled,
    latency_probe_model: trimmedModel || null,
    latency_probe_interval_ms:
      Number.isFinite(intervalMs) && intervalMs > 0 ? Math.round(intervalMs) : null,
    latency_probe_time_start: hasWindow ? form.timeStart : null,
    latency_probe_time_end: hasWindow ? form.timeEnd : null,
  };
}

function formatLastResult(
  provider: ProviderDisplay,
  t: ReturnType<typeof useTranslations>
): string {
  if (!provider.latencyProbeLastRunAt) return t("neverRun");
  if (provider.latencyProbeLastStatus === "failed" || provider.latencyProbeLastAvgMs === null) {
    return t("failedStatus");
  }
  return t("avgLatency", { value: Math.round(provider.latencyProbeLastAvgMs) });
}
