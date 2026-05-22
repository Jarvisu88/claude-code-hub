import { useTranslation } from "react-i18next";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { useMyUsage } from "@/api/hooks";

function quotaVariant(used: number, limit: number): "success" | "warning" | "danger" {
  if (limit <= 0) return "success";
  const ratio = used / limit;
  if (ratio >= 0.9) return "danger";
  if (ratio >= 0.7) return "warning";
  return "success";
}

function MyUsagePage() {
  const { t } = useTranslation();
  const { data, isLoading, error } = useMyUsage();

  if (isLoading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Spinner size="lg" />
      </div>
    );
  }

  if (error) {
    return (
      <Card>
        <CardContent>
          <div className="flex h-64 items-center justify-center text-[var(--color-danger)]">
            {t("common.loadError")}
          </div>
        </CardContent>
      </Card>
    );
  }

  const quotas = data?.quotas;
  const recentUsage = data?.recentUsage ?? [];
  const availableModels = data?.availableModels ?? [];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("myUsage.title")}
        </h1>
      </div>

      {/* Quota Overview */}
      {quotas && (
        <div className="mb-6 grid gap-4 sm:grid-cols-3">
          <Card>
            <CardContent className="p-4">
              <h3 className="text-sm font-medium text-[var(--color-text-secondary)]">
                {t("myUsage.dailyQuota")}
              </h3>
              <div className="mt-2 flex items-center justify-between">
                <span className="text-2xl font-bold text-[var(--color-text)]">
                  ${quotas.daily.used.toFixed(4)}
                </span>
                <Badge variant={quotaVariant(quotas.daily.used, quotas.daily.limit)}>
                  {quotas.daily.limit > 0
                    ? `/ $${quotas.daily.limit.toFixed(2)}`
                    : t("myUsage.unlimited")}
                </Badge>
              </div>
              {quotas.daily.limit > 0 && (
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--color-bg-tertiary)]">
                  <div
                    className="h-full rounded-full bg-[var(--color-primary)] transition-all"
                    style={{
                      width: `${Math.min(100, (quotas.daily.used / quotas.daily.limit) * 100)}%`,
                    }}
                  />
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-4">
              <h3 className="text-sm font-medium text-[var(--color-text-secondary)]">
                {t("myUsage.monthlyQuota")}
              </h3>
              <div className="mt-2 flex items-center justify-between">
                <span className="text-2xl font-bold text-[var(--color-text)]">
                  ${quotas.monthly.used.toFixed(4)}
                </span>
                <Badge variant={quotaVariant(quotas.monthly.used, quotas.monthly.limit)}>
                  {quotas.monthly.limit > 0
                    ? `/ $${quotas.monthly.limit.toFixed(2)}`
                    : t("myUsage.unlimited")}
                </Badge>
              </div>
              {quotas.monthly.limit > 0 && (
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--color-bg-tertiary)]">
                  <div
                    className="h-full rounded-full bg-[var(--color-primary)] transition-all"
                    style={{
                      width: `${Math.min(100, (quotas.monthly.used / quotas.monthly.limit) * 100)}%`,
                    }}
                  />
                </div>
              )}
            </CardContent>
          </Card>

          <Card>
            <CardContent className="p-4">
              <h3 className="text-sm font-medium text-[var(--color-text-secondary)]">
                {t("myUsage.rateLimit")}
              </h3>
              <div className="mt-2 flex items-center justify-between">
                <span className="text-2xl font-bold text-[var(--color-text)]">
                  {quotas.rateLimit.used}
                </span>
                <Badge variant={quotaVariant(quotas.rateLimit.used, quotas.rateLimit.limit)}>
                  {quotas.rateLimit.limit > 0
                    ? `/ ${quotas.rateLimit.limit} ${t("myUsage.perMinute")}`
                    : t("myUsage.unlimited")}
                </Badge>
              </div>
              {quotas.rateLimit.limit > 0 && (
                <div className="mt-2 h-2 overflow-hidden rounded-full bg-[var(--color-bg-tertiary)]">
                  <div
                    className="h-full rounded-full bg-[var(--color-primary)] transition-all"
                    style={{
                      width: `${Math.min(100, (quotas.rateLimit.used / quotas.rateLimit.limit) * 100)}%`,
                    }}
                  />
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Recent Usage */}
        <Card>
          <CardHeader>
            <CardTitle>{t("myUsage.recentUsage")}</CardTitle>
          </CardHeader>
          <CardContent>
            {recentUsage.length === 0 ? (
              <p className="text-center text-[var(--color-text-secondary)] py-4">
                {t("common.noData")}
              </p>
            ) : (
              <div className="space-y-3">
                {recentUsage.map((item) => (
                  <div
                    key={item.id}
                    className="flex items-center justify-between rounded-md border border-[var(--color-border)] p-3"
                  >
                    <div>
                      <span className="font-mono text-sm font-medium text-[var(--color-text)]">
                        {item.model}
                      </span>
                      <div className="mt-1 text-xs text-[var(--color-text-secondary)]">
                        {item.inputTokens} / {item.outputTokens} {t("myUsage.tokens")}
                      </div>
                    </div>
                    <div className="text-right">
                      <span className="text-sm font-medium text-[var(--color-text)]">
                        ${item.cost.toFixed(6)}
                      </span>
                      <div className="mt-1 text-xs text-[var(--color-text-secondary)]">
                        {new Date(item.createdAt).toLocaleString()}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Available Models */}
        <Card>
          <CardHeader>
            <CardTitle>{t("myUsage.availableModels")}</CardTitle>
          </CardHeader>
          <CardContent>
            {availableModels.length === 0 ? (
              <p className="text-center text-[var(--color-text-secondary)] py-4">
                {t("common.noData")}
              </p>
            ) : (
              <div className="flex flex-wrap gap-2">
                {availableModels.map((model) => (
                  <Badge key={model} variant="secondary">
                    {model}
                  </Badge>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  );
}

export { MyUsagePage };
