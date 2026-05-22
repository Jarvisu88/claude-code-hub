import { useState, useEffect, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Spinner } from "@/components/ui/Spinner";
import { useSettings, useUpdateSettings } from "@/api/hooks";
import type { SystemSettings } from "@/api/types";
import { Save, RefreshCw } from "lucide-react";

function SettingsPage() {
  const { t } = useTranslation();
  const { data, isLoading, error, refetch } = useSettings();
  const updateMutation = useUpdateSettings();

  const [form, setForm] = useState<SystemSettings | null>(null);
  const [saveSuccess, setSaveSuccess] = useState(false);

  useEffect(() => {
    if (data) {
      setForm(structuredClone(data));
    }
  }, [data]);

  const handleSave = useCallback(() => {
    if (!form) return;
    setSaveSuccess(false);
    updateMutation.mutate(form, {
      onSuccess: () => {
        setSaveSuccess(true);
        setTimeout(() => setSaveSuccess(false), 3000);
      },
    });
  }, [form, updateMutation]);

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error || !form) {
    return (
      <div>
        <h1 className="mb-6 text-2xl font-bold text-[var(--color-text)]">
          {t("settings.title")}
        </h1>
        <Card>
          <CardContent>
            <div className="flex h-64 items-center justify-center text-[var(--color-danger)]">
              {t("common.loadError")}
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("settings.title")}
        </h1>
        <div className="flex gap-2">
          <Button variant="secondary" onClick={() => refetch()}>
            <RefreshCw className="mr-2 h-4 w-4" />
            {t("settings.reload")}
          </Button>
          <Button
            onClick={handleSave}
            disabled={updateMutation.isPending}
          >
            {updateMutation.isPending ? (
              <Spinner size="sm" className="mr-2" />
            ) : (
              <Save className="mr-2 h-4 w-4" />
            )}
            {t("common.save")}
          </Button>
        </div>
      </div>

      {/* Save success banner */}
      {saveSuccess && (
        <div className="mb-4 rounded-md border border-green-300 bg-green-50 p-3 text-green-800 dark:border-green-700 dark:bg-green-900/20 dark:text-green-400">
          {t("settings.saveSuccess")}
        </div>
      )}

      {/* Save error banner */}
      {updateMutation.isError && (
        <div className="mb-4 rounded-md border border-red-300 bg-red-50 p-3 text-red-800 dark:border-red-700 dark:bg-red-900/20 dark:text-red-400">
          {t("settings.saveError")}
        </div>
      )}

      <div className="space-y-6">
        {/* General section */}
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.general")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label={t("settings.siteName")}
                value={form.general.siteName}
                onChange={(e) =>
                  setForm({
                    ...form,
                    general: { ...form.general, siteName: e.target.value },
                  })
                }
              />
              <Input
                label={t("settings.adminEmail")}
                type="email"
                value={form.general.adminEmail}
                onChange={(e) =>
                  setForm({
                    ...form,
                    general: { ...form.general, adminEmail: e.target.value },
                  })
                }
              />
              <div>
                <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
                  {t("settings.logLevel")}
                </label>
                <select
                  value={form.general.logLevel}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      general: { ...form.general, logLevel: e.target.value },
                    })
                  }
                  className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
                >
                  <option value="debug">debug</option>
                  <option value="info">info</option>
                  <option value="warn">warn</option>
                  <option value="error">error</option>
                </select>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Proxy section */}
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.proxy")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label={t("settings.proxyTimeout")}
                type="number"
                value={String(form.proxy.timeout)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    proxy: {
                      ...form.proxy,
                      timeout: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1000"
                step="1000"
              />
              <Input
                label={t("settings.proxyMaxRetries")}
                type="number"
                value={String(form.proxy.maxRetries)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    proxy: {
                      ...form.proxy,
                      maxRetries: Number(e.target.value) || 0,
                    },
                  })
                }
                min="0"
              />
              <Input
                label={t("settings.proxyRetryDelay")}
                type="number"
                value={String(form.proxy.retryDelay)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    proxy: {
                      ...form.proxy,
                      retryDelay: Number(e.target.value) || 0,
                    },
                  })
                }
                min="0"
              />
              <Input
                label={t("settings.proxyStreamBufferSize")}
                type="number"
                value={String(form.proxy.streamBufferSize)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    proxy: {
                      ...form.proxy,
                      streamBufferSize: Number(e.target.value) || 0,
                    },
                  })
                }
                min="0"
              />
            </div>
          </CardContent>
        </Card>

        {/* Rate Limit section */}
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.rateLimit")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <ToggleSwitch
                label={t("settings.rateLimitEnabled")}
                checked={form.rateLimit.enabled}
                onChange={(checked) =>
                  setForm({
                    ...form,
                    rateLimit: { ...form.rateLimit, enabled: checked },
                  })
                }
              />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label={t("settings.rateLimitWindow")}
                type="number"
                value={String(form.rateLimit.windowSeconds)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    rateLimit: {
                      ...form.rateLimit,
                      windowSeconds: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1"
                disabled={!form.rateLimit.enabled}
              />
              <Input
                label={t("settings.rateLimitMaxRequests")}
                type="number"
                value={String(form.rateLimit.maxRequests)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    rateLimit: {
                      ...form.rateLimit,
                      maxRequests: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1"
                disabled={!form.rateLimit.enabled}
              />
              <Input
                label={t("settings.rateLimitMaxTokens")}
                type="number"
                value={String(form.rateLimit.maxTokensPerMinute)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    rateLimit: {
                      ...form.rateLimit,
                      maxTokensPerMinute: Number(e.target.value) || 0,
                    },
                  })
                }
                min="0"
                disabled={!form.rateLimit.enabled}
              />
            </div>
          </CardContent>
        </Card>

        {/* Circuit Breaker section */}
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.circuitBreaker")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <ToggleSwitch
                label={t("settings.circuitBreakerEnabled")}
                checked={form.circuitBreaker.enabled}
                onChange={(checked) =>
                  setForm({
                    ...form,
                    circuitBreaker: {
                      ...form.circuitBreaker,
                      enabled: checked,
                    },
                  })
                }
              />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label={t("settings.cbFailureThreshold")}
                type="number"
                value={String(form.circuitBreaker.failureThreshold)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    circuitBreaker: {
                      ...form.circuitBreaker,
                      failureThreshold: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1"
                disabled={!form.circuitBreaker.enabled}
              />
              <Input
                label={t("settings.cbRecoveryTimeout")}
                type="number"
                value={String(form.circuitBreaker.recoveryTimeout)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    circuitBreaker: {
                      ...form.circuitBreaker,
                      recoveryTimeout: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1000"
                step="1000"
                disabled={!form.circuitBreaker.enabled}
              />
              <Input
                label={t("settings.cbHalfOpenMax")}
                type="number"
                value={String(form.circuitBreaker.halfOpenMaxRequests)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    circuitBreaker: {
                      ...form.circuitBreaker,
                      halfOpenMaxRequests: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1"
                disabled={!form.circuitBreaker.enabled}
              />
            </div>
          </CardContent>
        </Card>

        {/* Cleanup section */}
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.cleanup")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="mb-4">
              <ToggleSwitch
                label={t("settings.cleanupEnabled")}
                checked={form.cleanup.enabled}
                onChange={(checked) =>
                  setForm({
                    ...form,
                    cleanup: { ...form.cleanup, enabled: checked },
                  })
                }
              />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label={t("settings.cleanupRetentionDays")}
                type="number"
                value={String(form.cleanup.retentionDays)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    cleanup: {
                      ...form.cleanup,
                      retentionDays: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1"
                disabled={!form.cleanup.enabled}
              />
              <Input
                label={t("settings.cleanupBatchSize")}
                type="number"
                value={String(form.cleanup.batchSize)}
                onChange={(e) =>
                  setForm({
                    ...form,
                    cleanup: {
                      ...form.cleanup,
                      batchSize: Number(e.target.value) || 0,
                    },
                  })
                }
                min="1"
                disabled={!form.cleanup.enabled}
              />
              <Input
                label={t("settings.cleanupCron")}
                value={form.cleanup.cronExpression}
                onChange={(e) =>
                  setForm({
                    ...form,
                    cleanup: {
                      ...form.cleanup,
                      cronExpression: e.target.value,
                    },
                  })
                }
                disabled={!form.cleanup.enabled}
                placeholder="0 3 * * *"
              />
            </div>
          </CardContent>
        </Card>

        {/* Features section */}
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.features")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <ToggleSwitch
                label={t("settings.featureRegistration")}
                checked={form.features.enableRegistration}
                onChange={(checked) =>
                  setForm({
                    ...form,
                    features: {
                      ...form.features,
                      enableRegistration: checked,
                    },
                  })
                }
              />
              <ToggleSwitch
                label={t("settings.featureUsageTracking")}
                checked={form.features.enableUsageTracking}
                onChange={(checked) =>
                  setForm({
                    ...form,
                    features: {
                      ...form.features,
                      enableUsageTracking: checked,
                    },
                  })
                }
              />
              <ToggleSwitch
                label={t("settings.featureCostTracking")}
                checked={form.features.enableCostTracking}
                onChange={(checked) =>
                  setForm({
                    ...form,
                    features: {
                      ...form.features,
                      enableCostTracking: checked,
                    },
                  })
                }
              />
              <ToggleSwitch
                label={t("settings.featureModelMapping")}
                checked={form.features.enableModelMapping}
                onChange={(checked) =>
                  setForm({
                    ...form,
                    features: {
                      ...form.features,
                      enableModelMapping: checked,
                    },
                  })
                }
              />
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

// ---- Toggle switch component ----

interface ToggleSwitchProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}

function ToggleSwitch({ label, checked, onChange, disabled }: ToggleSwitchProps) {
  return (
    <label className="flex cursor-pointer items-center gap-3">
      <div className="relative">
        <input
          type="checkbox"
          checked={checked}
          onChange={(e) => onChange(e.target.checked)}
          disabled={disabled}
          className="peer sr-only"
        />
        <div className="h-6 w-11 rounded-full bg-[var(--color-bg-tertiary)] peer-checked:bg-[var(--color-primary)] peer-disabled:opacity-50 transition-colors" />
        <div className="absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white transition-transform peer-checked:translate-x-5" />
      </div>
      <span className="text-sm font-medium text-[var(--color-text)]">
        {label}
      </span>
    </label>
  );
}

export { SettingsPage };
