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

  const updateField = useCallback(
    <K extends keyof SystemSettings>(key: K, value: SystemSettings[K]) => {
      setForm((prev) => (prev ? { ...prev, [key]: value } : prev));
    },
    [],
  );

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
                label={t("settings.siteTitle")}
                value={form.siteTitle}
                onChange={(e) => updateField("siteTitle", e.target.value)}
              />
              <div>
                <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
                  {t("settings.currencyDisplay")}
                </label>
                <select
                  value={form.currencyDisplay}
                  onChange={(e) => updateField("currencyDisplay", e.target.value)}
                  className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
                >
                  <option value="USD">USD</option>
                  <option value="EUR">EUR</option>
                  <option value="GBP">GBP</option>
                  <option value="CNY">CNY</option>
                  <option value="JPY">JPY</option>
                </select>
              </div>
              <div>
                <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
                  {t("settings.billingModelSource")}
                </label>
                <select
                  value={form.billingModelSource}
                  onChange={(e) => updateField("billingModelSource", e.target.value)}
                  className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
                >
                  <option value="original">original</option>
                  <option value="custom">custom</option>
                </select>
              </div>
              <div className="flex items-end">
                <ToggleSwitch
                  label={t("settings.allowGlobalUsageView")}
                  checked={form.allowGlobalUsageView}
                  onChange={(checked) => updateField("allowGlobalUsageView", checked)}
                />
              </div>
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
                label={t("settings.enableHttp2")}
                checked={form.enableHttp2}
                onChange={(checked) => updateField("enableHttp2", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableHighConcurrencyMode")}
                checked={form.enableHighConcurrencyMode}
                onChange={(checked) => updateField("enableHighConcurrencyMode", checked)}
              />
              <ToggleSwitch
                label={t("settings.verboseProviderError")}
                checked={form.verboseProviderError}
                onChange={(checked) => updateField("verboseProviderError", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableClientVersionCheck")}
                checked={form.enableClientVersionCheck}
                onChange={(checked) => updateField("enableClientVersionCheck", checked)}
              />
              <ToggleSwitch
                label={t("settings.interceptAnthropicWarmupRequests")}
                checked={form.interceptAnthropicWarmupRequests}
                onChange={(checked) => updateField("interceptAnthropicWarmupRequests", checked)}
              />
              <ToggleSwitch
                label={t("settings.ipGeoLookupEnabled")}
                checked={form.ipGeoLookupEnabled}
                onChange={(checked) => updateField("ipGeoLookupEnabled", checked)}
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
                checked={form.enableAutoCleanup}
                onChange={(checked) => updateField("enableAutoCleanup", checked)}
              />
            </div>
            <div className="grid gap-4 sm:grid-cols-2">
              <Input
                label={t("settings.cleanupRetentionDays")}
                type="number"
                value={String(form.cleanupRetentionDays)}
                onChange={(e) =>
                  updateField("cleanupRetentionDays", Number(e.target.value) || 0)
                }
                min="1"
                disabled={!form.enableAutoCleanup}
              />
              <Input
                label={t("settings.cleanupBatchSize")}
                type="number"
                value={String(form.cleanupBatchSize)}
                onChange={(e) =>
                  updateField("cleanupBatchSize", Number(e.target.value) || 0)
                }
                min="1"
                disabled={!form.enableAutoCleanup}
              />
              <Input
                label={t("settings.cleanupSchedule")}
                value={form.cleanupSchedule}
                onChange={(e) => updateField("cleanupSchedule", e.target.value)}
                disabled={!form.enableAutoCleanup}
                placeholder="0 2 * * *"
              />
            </div>
          </CardContent>
        </Card>

        {/* Rectifiers section */}
        <Card>
          <CardHeader>
            <CardTitle>{t("settings.rectifiers")}</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-4">
              <ToggleSwitch
                label={t("settings.enableThinkingSignatureRectifier")}
                checked={form.enableThinkingSignatureRectifier}
                onChange={(checked) => updateField("enableThinkingSignatureRectifier", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableThinkingBudgetRectifier")}
                checked={form.enableThinkingBudgetRectifier}
                onChange={(checked) => updateField("enableThinkingBudgetRectifier", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableBillingHeaderRectifier")}
                checked={form.enableBillingHeaderRectifier}
                onChange={(checked) => updateField("enableBillingHeaderRectifier", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableResponseInputRectifier")}
                checked={form.enableResponseInputRectifier}
                onChange={(checked) => updateField("enableResponseInputRectifier", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableCodexSessionIdCompletion")}
                checked={form.enableCodexSessionIdCompletion}
                onChange={(checked) => updateField("enableCodexSessionIdCompletion", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableClaudeMetadataUserIdInjection")}
                checked={form.enableClaudeMetadataUserIdInjection}
                onChange={(checked) => updateField("enableClaudeMetadataUserIdInjection", checked)}
              />
              <ToggleSwitch
                label={t("settings.enableResponseFixer")}
                checked={form.enableResponseFixer}
                onChange={(checked) => updateField("enableResponseFixer", checked)}
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
