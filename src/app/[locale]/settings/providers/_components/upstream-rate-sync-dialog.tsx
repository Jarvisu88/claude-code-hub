"use client";

import { Download, ExternalLink, Loader2, RefreshCw, Settings2, Trash2 } from "lucide-react";
import { useTranslations } from "next-intl";
import { useEffect, useState, useTransition } from "react";
import { toast } from "sonner";
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
import { Switch } from "@/components/ui/switch";
import type {
  ProviderUpstreamRateSyncConfigResult,
  ProviderUpstreamRateSyncInput,
} from "@/actions/providers";
import {
  deleteProviderUpstreamRateSyncConfig,
  getProviderUpstreamRateSyncConfig,
  importProviderUpstreamRateAuthFromBrowser,
  openProviderUpstreamRateAuthBrowser,
  saveProviderUpstreamRateSyncConfig,
  syncProviderUpstreamRateNow,
} from "@/lib/api-client/v1/actions/providers";

type UpstreamRateSource = "sub2api" | "newapi";

interface UpstreamRateSyncDialogProps {
  providerId: number;
  providerUrl: string;
  providerName: string;
  maskedKey: string;
  onSynced?: () => void;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
}

interface FormState {
  source: UpstreamRateSource;
  isEnabled: boolean;
  baseUrl: string;
  accessToken: string;
  refreshToken: string;
  tokenExpiresAt: string;
  cookie: string;
  userId: string;
  syncIntervalMinutes: string;
}

const EMPTY_FORM: FormState = {
  source: "newapi",
  isEnabled: false,
  baseUrl: "",
  accessToken: "",
  refreshToken: "",
  tokenExpiresAt: "",
  cookie: "",
  userId: "",
  syncIntervalMinutes: "60",
};

export function UpstreamRateSyncDialog({
  providerId,
  providerUrl,
  providerName,
  maskedKey,
  onSynced,
  open: controlledOpen,
  onOpenChange,
}: UpstreamRateSyncDialogProps) {
  const t = useTranslations("settings.providers.upstreamRateSync");
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const open = controlledOpen ?? uncontrolledOpen;
  const setOpen = onOpenChange ?? setUncontrolledOpen;
  const [form, setForm] = useState<FormState>(EMPTY_FORM);
  const [config, setConfig] = useState<ProviderUpstreamRateSyncConfigResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [pending, startTransition] = useTransition();

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoading(true);
    getProviderUpstreamRateSyncConfig(providerId)
      .then((result) => {
        if (cancelled) return;
        if (result.ok) {
          setConfig(result.data);
          setForm(result.data ? formFromConfig(result.data) : EMPTY_FORM);
        } else {
          toast.error(t("loadFailed"), { description: result.error });
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, providerId, t]);

  const updateField = <K extends keyof FormState>(key: K, value: FormState[K]) => {
    setForm((current) => ({ ...current, [key]: value }));
  };

  const ensureConfigSaved = async (): Promise<ProviderUpstreamRateSyncConfigResult | null> => {
    const payload = buildPayload(form);
    if (!payload) {
      toast.error(t("invalidForm"));
      return null;
    }

    const result = await saveProviderUpstreamRateSyncConfig(providerId, payload);
    if (!result.ok) {
      toast.error(t("saveFailed"), { description: result.error });
      return null;
    }

    setConfig(result.data);
    setForm(formFromConfig(result.data));
    return result.data;
  };

  const handleSave = () => {
    startTransition(async () => {
      const saved = await ensureConfigSaved();
      if (saved) {
        toast.success(t("saveSuccess"));
      }
    });
  };

  const handleDelete = () => {
    startTransition(async () => {
      const result = await deleteProviderUpstreamRateSyncConfig(providerId);
      if (result.ok) {
        setConfig(null);
        setForm(EMPTY_FORM);
        toast.success(t("deleteSuccess"));
      } else {
        toast.error(t("deleteFailed"), { description: result.error });
      }
    });
  };

  const handleRun = () => {
    startTransition(async () => {
      const saved = await ensureConfigSaved();
      if (!saved) return;

      const result = await syncProviderUpstreamRateNow(providerId);
      if (result.ok) {
        toast.success(t("syncSuccess"));
        onSynced?.();
        setOpen(false);
      } else {
        toast.error(t("syncFailed"), { description: result.error });
      }
    });
  };

  const handleOpenBrowser = () => {
    startTransition(async () => {
      const saved = await ensureConfigSaved();
      if (!saved) return;

      const result = await openProviderUpstreamRateAuthBrowser(providerId);
      if (result.ok) {
        toast.success(t("browserOpened"));
      } else {
        toast.error(t("browserOpenFailed"), { description: result.error });
      }
    });
  };

  const handleImportAuth = () => {
    startTransition(async () => {
      const saved = await ensureConfigSaved();
      if (!saved) return;

      const result = await importProviderUpstreamRateAuthFromBrowser(providerId);
      if (result.ok) {
        setConfig(result.data);
        setForm(formFromConfig(result.data));
        toast.success(t("authImportSuccess"));
      } else {
        toast.error(t("authImportFailed"), { description: result.error });
      }
    });
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      {controlledOpen === undefined && (
        <DialogTrigger asChild>
          <Button size="icon" variant="ghost" title={t("title")}>
            <Settings2 className="h-4 w-4" />
          </Button>
        </DialogTrigger>
      )}
      <DialogContent className="max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t("title")}</DialogTitle>
          <DialogDescription>{t("description")}</DialogDescription>
        </DialogHeader>

        {loading ? (
          <div className="flex items-center justify-center py-8 text-muted-foreground">
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            {t("loading")}
          </div>
        ) : (
          <div className="grid gap-4 py-2">
            <div className="flex items-center justify-between rounded-md border p-3">
              <div>
                <Label>{t("enabled")}</Label>
                {config?.lastSyncedAt && (
                  <p className="mt-1 text-xs text-muted-foreground">
                    {t("lastSynced", {
                      time: new Date(config.lastSyncedAt).toLocaleString(),
                      rate: config.lastSyncRate ?? "-",
                    })}
                  </p>
                )}
                {config?.lastSyncError && (
                  <p className="mt-1 text-xs text-destructive">{config.lastSyncError}</p>
                )}
              </div>
              <Switch
                checked={form.isEnabled}
                onCheckedChange={(checked) => updateField("isEnabled", checked)}
              />
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="grid gap-2">
                <Label htmlFor={`rate-source-${providerId}`}>{t("source")}</Label>
                <Select
                  value={form.source}
                  onValueChange={(value: UpstreamRateSource) => updateField("source", value)}
                >
                  <SelectTrigger id={`rate-source-${providerId}`}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="newapi">New API</SelectItem>
                    <SelectItem value="sub2api">Sub2API</SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <TextField
                id={`rate-interval-${providerId}`}
                label={t("interval")}
                value={form.syncIntervalMinutes}
                onChange={(value) => updateField("syncIntervalMinutes", value)}
                type="number"
              />
            </div>

            <TextField
              id={`rate-base-url-${providerId}`}
              label={t("baseUrl")}
              value={form.baseUrl}
              onChange={(value) => updateField("baseUrl", value)}
              placeholder={providerUrl}
            />

            <div className="grid gap-3 rounded-md border bg-muted/30 p-3 text-sm">
              <div className="grid gap-1">
                <span className="text-xs text-muted-foreground">{t("providerBaseUrl")}</span>
                <span className="break-all font-mono">{providerUrl}</span>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                <div className="grid gap-1">
                  <span className="text-xs text-muted-foreground">{t("providerKey")}</span>
                  <span className="font-mono">{maskedKey}</span>
                </div>
                <div className="grid gap-1">
                  <span className="text-xs text-muted-foreground">{t("providerKeyName")}</span>
                  <span className="truncate">{providerName}</span>
                </div>
              </div>
              {config?.lastUpstreamGroupName && (
                <div className="grid gap-1">
                  <span className="text-xs text-muted-foreground">{t("upstreamGroup")}</span>
                  <span>{config.lastUpstreamGroupName}</span>
                </div>
              )}
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <TextField
                id={`rate-access-token-${providerId}`}
                label={t("accessToken")}
                value={form.accessToken}
                onChange={(value) => updateField("accessToken", value)}
              />
              <TextField
                id={`rate-refresh-token-${providerId}`}
                label={t("refreshToken")}
                value={form.refreshToken}
                onChange={(value) => updateField("refreshToken", value)}
              />
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <TextField
                id={`rate-token-expires-${providerId}`}
                label={t("tokenExpiresAt")}
                value={form.tokenExpiresAt}
                onChange={(value) => updateField("tokenExpiresAt", value)}
                type="number"
              />
              <TextField
                id={`rate-user-id-${providerId}`}
                label={t("userId")}
                value={form.userId}
                onChange={(value) => updateField("userId", value)}
              />
            </div>

            <TextField
              id={`rate-cookie-${providerId}`}
              label={t("cookie")}
              value={form.cookie}
              onChange={(value) => updateField("cookie", value)}
            />
          </div>
        )}

        <DialogFooter className="gap-2 sm:justify-between">
          <div className="flex gap-2">
            {config && (
              <Button variant="outline" onClick={handleDelete} disabled={pending}>
                <Trash2 className="mr-2 h-4 w-4" />
                {t("delete")}
              </Button>
            )}
            <Button variant="outline" onClick={handleRun} disabled={pending || loading}>
              <RefreshCw className="mr-2 h-4 w-4" />
              {t("syncNow")}
            </Button>
            <Button variant="outline" onClick={handleOpenBrowser} disabled={pending || loading}>
              <ExternalLink className="mr-2 h-4 w-4" />
              {t("openBrowser")}
            </Button>
            <Button variant="outline" onClick={handleImportAuth} disabled={pending || loading}>
              <Download className="mr-2 h-4 w-4" />
              {t("importAuth")}
            </Button>
          </div>
          <Button onClick={handleSave} disabled={pending || loading}>
            {pending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {t("save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function TextField(props: {
  id: string;
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  type?: string;
}) {
  return (
    <div className="grid gap-2">
      <Label htmlFor={props.id}>{props.label}</Label>
      <Input
        id={props.id}
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
        placeholder={props.placeholder}
        type={props.type ?? "text"}
      />
    </div>
  );
}

function formFromConfig(config: ProviderUpstreamRateSyncConfigResult): FormState {
  return {
    source: config.source,
    isEnabled: config.isEnabled,
    baseUrl: config.baseUrl,
    accessToken: config.accessToken ?? "",
    refreshToken: config.refreshToken ?? "",
    tokenExpiresAt: config.tokenExpiresAt?.toString() ?? "",
    cookie: config.cookie ?? "",
    userId: config.userId ?? "",
    syncIntervalMinutes: config.syncIntervalMinutes.toString(),
  };
}

function buildPayload(form: FormState): ProviderUpstreamRateSyncInput | null {
  const interval = Number(form.syncIntervalMinutes);
  const tokenExpiresAt = form.tokenExpiresAt ? Number(form.tokenExpiresAt) : null;
  if (!Number.isInteger(interval) || interval < 1) return null;
  if (tokenExpiresAt != null && (!Number.isInteger(tokenExpiresAt) || tokenExpiresAt < 0)) {
    return null;
  }

  return {
    source: form.source,
    is_enabled: form.isEnabled,
    base_url: optionalString(form.baseUrl),
    access_token: optionalString(form.accessToken),
    refresh_token: optionalString(form.refreshToken),
    token_expires_at: tokenExpiresAt,
    cookie: optionalString(form.cookie),
    user_id: optionalString(form.userId),
    sync_interval_minutes: interval,
  };
}

function optionalString(value: string): string | null {
  const trimmed = value.trim();
  return trimmed ? trimmed : null;
}
