import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Badge } from "@/components/ui/Badge";
import { Dialog } from "@/components/ui/Dialog";
import { Spinner } from "@/components/ui/Spinner";
import {
  useNotificationSettings,
  useUpdateNotificationSettings,
  useWebhookTargets,
  useCreateWebhookTarget,
  useDeleteWebhookTarget,
  useTestWebhook,
  useNotificationBindings,
  useCreateNotificationBinding,
  useDeleteNotificationBinding,
} from "@/api/hooks";
import type {
  NotificationSettings,
  WebhookTarget,
  CreateWebhookTargetRequest,
} from "@/api/types";
import { Plus, Trash2, Zap, Link } from "lucide-react";

function NotificationsPage() {
  const { t } = useTranslation();
  const [showWebhookDialog, setShowWebhookDialog] = useState(false);
  const [showBindingDialog, setShowBindingDialog] = useState(false);
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    message: string;
  } | null>(null);

  const { data: settings, isLoading: settingsLoading } = useNotificationSettings();
  const updateSettingsMutation = useUpdateNotificationSettings();
  const { data: webhookTargets, isLoading: webhooksLoading } = useWebhookTargets();
  const createWebhookMutation = useCreateWebhookTarget();
  const deleteWebhookMutation = useDeleteWebhookTarget();
  const testWebhookMutation = useTestWebhook();
  const { data: bindings, isLoading: bindingsLoading } = useNotificationBindings();
  const createBindingMutation = useCreateNotificationBinding();
  const deleteBindingMutation = useDeleteNotificationBinding();

  const [localSettings, setLocalSettings] = useState<NotificationSettings | null>(null);
  const currentSettings = localSettings ?? settings ?? null;

  const handleSettingsChange = (
    field: string,
    value: boolean | number | string,
  ) => {
    if (!currentSettings) return;
    const updated = {
      ...currentSettings,
      [field]: value,
    };
    setLocalSettings(updated);
  };

  const handleSaveSettings = () => {
    if (!currentSettings) return;
    updateSettingsMutation.mutate(currentSettings, {
      onSuccess: () => setLocalSettings(null),
    });
  };

  const handleTestWebhook = (id: string) => {
    setTestingId(id);
    setTestResult(null);
    testWebhookMutation.mutate(id, {
      onSuccess: (result) => setTestResult(result),
      onError: () =>
        setTestResult({ success: false, message: t("notifications.testFailed") }),
      onSettled: () => setTestingId(null),
    });
  };

  if (settingsLoading || webhooksLoading || bindingsLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("notifications.title")}
        </h1>
      </div>

      {/* Test result banner */}
      {testResult && (
        <div
          className={`mb-4 rounded-md border p-3 ${
            testResult.success
              ? "border-green-300 bg-green-50 text-green-800 dark:border-green-700 dark:bg-green-900/20 dark:text-green-400"
              : "border-red-300 bg-red-50 text-red-800 dark:border-red-700 dark:bg-red-900/20 dark:text-red-400"
          }`}
        >
          <div className="flex items-center justify-between">
            <span>{testResult.message}</span>
            <Button variant="ghost" size="sm" onClick={() => setTestResult(null)}>
              x
            </Button>
          </div>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Notification Settings */}
        <Card>
          <CardHeader>
            <CardTitle>{t("notifications.settingsSection")}</CardTitle>
          </CardHeader>
          <CardContent>
            {currentSettings && (
              <div className="space-y-6">
                {/* Circuit Breaker */}
                <div className="space-y-2">
                  <h4 className="text-sm font-medium text-[var(--color-text)]">
                    {t("notifications.circuitBreaker")}
                  </h4>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={!!currentSettings.circuitBreakerEnabled}
                      onChange={(e) =>
                        handleSettingsChange("circuitBreakerEnabled", e.target.checked)
                      }
                      className="h-4 w-4"
                    />
                    <span className="text-sm text-[var(--color-text)]">
                      {t("common.enabled")}
                    </span>
                  </label>
                  <Input
                    label={t("notifications.threshold")}
                    type="text"
                    value={String(currentSettings.costAlertThreshold ?? "")}
                    onChange={(e) =>
                      handleSettingsChange("costAlertThreshold", e.target.value)
                    }
                  />
                </div>

                {/* Leaderboard */}
                <div className="space-y-2">
                  <h4 className="text-sm font-medium text-[var(--color-text)]">
                    {t("notifications.leaderboard")}
                  </h4>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={!!currentSettings.dailyLeaderboardEnabled}
                      onChange={(e) =>
                        handleSettingsChange("dailyLeaderboardEnabled", e.target.checked)
                      }
                      className="h-4 w-4"
                    />
                    <span className="text-sm text-[var(--color-text)]">
                      {t("common.enabled")}
                    </span>
                  </label>
                  <Input
                    label={t("notifications.cron")}
                    value={String(currentSettings.dailyLeaderboardTime ?? "09:00")}
                    onChange={(e) =>
                      handleSettingsChange("dailyLeaderboardTime", e.target.value)
                    }
                  />
                </div>

                {/* Cost Alert */}
                <div className="space-y-2">
                  <h4 className="text-sm font-medium text-[var(--color-text)]">
                    {t("notifications.costAlert")}
                  </h4>
                  <label className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={!!currentSettings.costAlertEnabled}
                      onChange={(e) =>
                        handleSettingsChange("costAlertEnabled", e.target.checked)
                      }
                      className="h-4 w-4"
                    />
                    <span className="text-sm text-[var(--color-text)]">
                      {t("common.enabled")}
                    </span>
                  </label>
                  <Input
                    label={t("notifications.dailyThreshold")}
                    type="text"
                    value={String(currentSettings.costAlertThreshold ?? "0.8")}
                    onChange={(e) =>
                      handleSettingsChange("costAlertThreshold", e.target.value)
                    }
                  />
                </div>

                {/* Check Interval */}
                <div className="space-y-2">
                  <h4 className="text-sm font-medium text-[var(--color-text)]">
                    {t("notifications.checkInterval")}
                  </h4>
                  <Input
                    label={t("notifications.intervalSeconds")}
                    type="number"
                    value={String(currentSettings.costAlertCheckInterval ?? 60)}
                    onChange={(e) =>
                      handleSettingsChange("costAlertCheckInterval", Number(e.target.value) || 60)
                    }
                    min="10"
                  />
                </div>

                <Button
                  onClick={handleSaveSettings}
                  disabled={updateSettingsMutation.isPending || !localSettings}
                >
                  {updateSettingsMutation.isPending ? (
                    <Spinner size="sm" className="mr-2" />
                  ) : null}
                  {t("common.save")}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>

        {/* Webhook Targets */}
        <div className="space-y-6">
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>{t("notifications.webhookTargets")}</CardTitle>
                <Button size="sm" onClick={() => setShowWebhookDialog(true)}>
                  <Plus className="mr-1 h-3 w-3" />
                  {t("notifications.addWebhook")}
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {(!webhookTargets || webhookTargets.length === 0) ? (
                <p className="text-center text-[var(--color-text-secondary)] py-4">
                  {t("common.noData")}
                </p>
              ) : (
                <div className="space-y-3">
                  {webhookTargets.map((wh) => (
                    <div
                      key={wh.id}
                      className="flex items-center justify-between rounded-md border border-[var(--color-border)] p-3"
                    >
                      <div>
                        <span className="font-medium text-[var(--color-text)]">
                          {wh.name}
                        </span>
                        <div className="flex items-center gap-2 mt-1">
                          <Badge variant="secondary">{wh.type}</Badge>
                          <span className="text-xs text-[var(--color-text-secondary)] font-mono">
                            {wh.url.length > 40 ? wh.url.slice(0, 40) + "..." : wh.url}
                          </span>
                        </div>
                      </div>
                      <div className="flex items-center gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleTestWebhook(wh.id)}
                          disabled={testingId === wh.id}
                        >
                          {testingId === wh.id ? (
                            <Spinner size="sm" />
                          ) : (
                            <Zap className="h-4 w-4" />
                          )}
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => deleteWebhookMutation.mutate(wh.id)}
                        >
                          <Trash2 className="h-4 w-4 text-[var(--color-danger)]" />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>

          {/* Notification Bindings */}
          <Card>
            <CardHeader>
              <div className="flex items-center justify-between">
                <CardTitle>{t("notifications.bindings")}</CardTitle>
                <Button size="sm" onClick={() => setShowBindingDialog(true)}>
                  <Link className="mr-1 h-3 w-3" />
                  {t("notifications.addBinding")}
                </Button>
              </div>
            </CardHeader>
            <CardContent>
              {(!bindings || bindings.length === 0) ? (
                <p className="text-center text-[var(--color-text-secondary)] py-4">
                  {t("common.noData")}
                </p>
              ) : (
                <div className="space-y-2">
                  {bindings.map((b) => (
                    <div
                      key={b.id}
                      className="flex items-center justify-between rounded-md border border-[var(--color-border)] p-3"
                    >
                      <div className="flex items-center gap-2">
                        <Badge variant="default">{b.notificationType}</Badge>
                        <span className="text-sm text-[var(--color-text-secondary)]">
                          {b.webhookTargetName || b.webhookTargetId.slice(0, 8)}
                        </span>
                      </div>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => deleteBindingMutation.mutate(b.id)}
                      >
                        <Trash2 className="h-4 w-4 text-[var(--color-danger)]" />
                      </Button>
                    </div>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      </div>

      {/* Create Webhook Dialog */}
      {showWebhookDialog && (
        <WebhookFormDialog
          onClose={() => setShowWebhookDialog(false)}
          onSubmit={(formData) => {
            createWebhookMutation.mutate(formData, {
              onSuccess: () => setShowWebhookDialog(false),
            });
          }}
          isLoading={createWebhookMutation.isPending}
        />
      )}

      {/* Create Binding Dialog */}
      {showBindingDialog && (
        <BindingFormDialog
          webhookTargets={webhookTargets ?? []}
          onClose={() => setShowBindingDialog(false)}
          onSubmit={(data) => {
            createBindingMutation.mutate(data, {
              onSuccess: () => setShowBindingDialog(false),
            });
          }}
          isLoading={createBindingMutation.isPending}
        />
      )}
    </div>
  );
}

interface WebhookFormDialogProps {
  onClose: () => void;
  onSubmit: (data: CreateWebhookTargetRequest) => void;
  isLoading: boolean;
}

function WebhookFormDialog({ onClose, onSubmit, isLoading }: WebhookFormDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [type, setType] = useState("generic");

  const webhookTypes = ["generic", "slack", "discord", "feishu", "dingtalk"];

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name,
      url,
      type: type as CreateWebhookTargetRequest["type"],
    });
  };

  return (
    <Dialog
      open={true}
      onClose={onClose}
      title={t("notifications.addWebhook")}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <Input
          label={t("common.name")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <Input
          label={t("notifications.webhookUrl")}
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          required
          placeholder="https://hooks.example.com/webhook"
        />
        <div>
          <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
            {t("notifications.webhookType")}
          </label>
          <select
            value={type}
            onChange={(e) => setType(e.target.value)}
            className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
          >
            {webhookTypes.map((wt) => (
              <option key={wt} value={wt}>
                {wt}
              </option>
            ))}
          </select>
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isLoading || !name.trim() || !url.trim()}
          >
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

interface BindingFormDialogProps {
  webhookTargets: WebhookTarget[];
  onClose: () => void;
  onSubmit: (data: { notificationType: string; webhookTargetId: string }) => void;
  isLoading: boolean;
}

function BindingFormDialog({
  webhookTargets,
  onClose,
  onSubmit,
  isLoading,
}: BindingFormDialogProps) {
  const { t } = useTranslation();
  const notificationTypes = ["circuit_breaker", "leaderboard", "cost_alert", "cache_hit_rate"];
  const [notificationType, setNotificationType] = useState(notificationTypes[0]);
  const [webhookTargetId, setWebhookTargetId] = useState(
    webhookTargets[0]?.id ?? "",
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({ notificationType, webhookTargetId });
  };

  return (
    <Dialog
      open={true}
      onClose={onClose}
      title={t("notifications.addBinding")}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div>
          <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
            {t("notifications.notificationType")}
          </label>
          <select
            value={notificationType}
            onChange={(e) => setNotificationType(e.target.value)}
            className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
          >
            {notificationTypes.map((nt) => (
              <option key={nt} value={nt}>
                {t(`notifications.types.${nt}`)}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
            {t("notifications.webhookTarget")}
          </label>
          <select
            value={webhookTargetId}
            onChange={(e) => setWebhookTargetId(e.target.value)}
            className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
          >
            {webhookTargets.map((wt) => (
              <option key={wt.id} value={wt.id}>
                {wt.name} ({wt.type})
              </option>
            ))}
          </select>
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isLoading || !webhookTargetId}
          >
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export { NotificationsPage };
