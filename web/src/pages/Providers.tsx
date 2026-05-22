import { useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Table, type Column } from "@/components/ui/Table";
import { Badge } from "@/components/ui/Badge";
import { Dialog } from "@/components/ui/Dialog";
import { Spinner } from "@/components/ui/Spinner";
import {
  useProviders,
  useCreateProvider,
  useUpdateProvider,
  useDeleteProvider,
  useToggleProvider,
  useTestProvider,
} from "@/api/hooks";
import type {
  Provider,
  CreateProviderRequest,
  UpdateProviderRequest,
} from "@/api/types";
import {
  Plus,
  Search,
  Pencil,
  Trash2,
  Zap,
} from "lucide-react";

function healthBadgeVariant(
  status: string,
): "success" | "warning" | "danger" | "secondary" {
  switch (status) {
    case "healthy":
      return "success";
    case "degraded":
      return "warning";
    case "down":
      return "danger";
    default:
      return "secondary";
  }
}

function truncateUrl(url: string, maxLen = 30): string {
  if (url.length <= maxLen) return url;
  return url.slice(0, maxLen) + "...";
}

function ProvidersPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingProvider, setEditingProvider] = useState<Provider | null>(null);
  const [deletingProvider, setDeletingProvider] = useState<Provider | null>(
    null,
  );
  const [testingId, setTestingId] = useState<string | null>(null);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    message: string;
    latency: number;
  } | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useProviders({ page, pageSize, search });
  const createMutation = useCreateProvider();
  const updateMutation = useUpdateProvider();
  const deleteMutation = useDeleteProvider();
  const toggleMutation = useToggleProvider();
  const testMutation = useTestProvider();

  const handleSearch = useCallback(() => {
    setSearch(searchInput);
    setPage(1);
  }, [searchInput]);

  const handleSearchKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") handleSearch();
    },
    [handleSearch],
  );

  const handleTestConnection = (id: string) => {
    setTestingId(id);
    setTestResult(null);
    testMutation.mutate(id, {
      onSuccess: (result) => {
        setTestResult(result);
      },
      onError: () => {
        setTestResult({
          success: false,
          message: t("providers.testFailed"),
          latency: 0,
        });
      },
      onSettled: () => {
        setTestingId(null);
      },
    });
  };

  const columns: Column<Provider>[] = [
    {
      key: "id",
      header: "ID",
      className: "w-20",
      render: (row) => (
        <span className="font-mono text-xs">{row.id.slice(0, 8)}</span>
      ),
    },
    {
      key: "name",
      header: t("common.name"),
      render: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      key: "type",
      header: t("providers.type"),
      render: (row) => (
        <Badge variant="secondary">{row.type}</Badge>
      ),
    },
    {
      key: "endpoint",
      header: t("providers.endpoint"),
      render: (row) => (
        <span
          className="font-mono text-xs text-[var(--color-text-secondary)]"
          title={row.endpoint}
        >
          {truncateUrl(row.endpoint)}
        </span>
      ),
    },
    {
      key: "enabled",
      header: t("common.status"),
      render: (row) => (
        <button
          type="button"
          onClick={() =>
            toggleMutation.mutate({ id: row.id, enabled: !row.enabled })
          }
          className="cursor-pointer"
          disabled={toggleMutation.isPending}
        >
          <Badge variant={row.enabled ? "success" : "danger"}>
            {row.enabled ? t("common.enabled") : t("common.disabled")}
          </Badge>
        </button>
      ),
    },
    {
      key: "weight",
      header: t("providers.weight"),
      render: (row) => <span className="text-sm">{row.weight}</span>,
    },
    {
      key: "priority",
      header: t("providers.priority"),
      render: (row) => <span className="text-sm">{row.priority}</span>,
    },
    {
      key: "healthStatus",
      header: t("providers.health"),
      render: (row) => (
        <Badge variant={healthBadgeVariant(row.healthStatus)}>
          {t(`providers.healthStatus.${row.healthStatus}`)}
        </Badge>
      ),
    },
    {
      key: "actions",
      header: t("common.actions"),
      className: "w-36",
      render: (row) => (
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleTestConnection(row.id)}
            disabled={testingId === row.id}
          >
            {testingId === row.id ? (
              <Spinner size="sm" />
            ) : (
              <Zap className="h-4 w-4" />
            )}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setEditingProvider(row)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeletingProvider(row)}
          >
            <Trash2 className="h-4 w-4 text-[var(--color-danger)]" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("providers.title")}
        </h1>
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("providers.createProvider")}
        </Button>
      </div>

      {/* Search bar */}
      <Card className="mb-4">
        <CardContent className="p-4">
          <div className="flex gap-2">
            <div className="flex-1">
              <Input
                placeholder={t("providers.searchPlaceholder")}
                value={searchInput}
                onChange={(e) => setSearchInput(e.target.value)}
                onKeyDown={handleSearchKeyDown}
              />
            </div>
            <Button variant="secondary" onClick={handleSearch}>
              <Search className="mr-2 h-4 w-4" />
              {t("common.search")}
            </Button>
          </div>
        </CardContent>
      </Card>

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
            {testResult.success && (
              <span className="text-sm">
                {t("providers.latency")}: {testResult.latency}ms
              </span>
            )}
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setTestResult(null)}
            >
              x
            </Button>
          </div>
        </div>
      )}

      {/* Table */}
      {error ? (
        <Card>
          <CardContent>
            <div className="flex h-64 items-center justify-center text-[var(--color-danger)]">
              {t("common.loadError")}
            </div>
          </CardContent>
        </Card>
      ) : isLoading ? (
        <div className="flex h-64 items-center justify-center">
          <Spinner size="lg" />
        </div>
      ) : (
        <Table<Provider>
          columns={columns}
          data={data?.data ?? []}
          keyExtractor={(row) => row.id}
          page={page}
          pageSize={pageSize}
          total={data?.total ?? 0}
          onPageChange={setPage}
        />
      )}

      {/* Create dialog */}
      {showCreateDialog && (
        <ProviderFormDialog
          title={t("providers.createProvider")}
          onClose={() => setShowCreateDialog(false)}
          onSubmit={(formData) => {
            createMutation.mutate(formData as CreateProviderRequest, {
              onSuccess: () => setShowCreateDialog(false),
            });
          }}
          isLoading={createMutation.isPending}
        />
      )}

      {/* Edit dialog */}
      {editingProvider && (
        <ProviderFormDialog
          title={t("providers.editProvider")}
          initialData={editingProvider}
          onClose={() => setEditingProvider(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingProvider.id, data: formData as UpdateProviderRequest },
              { onSuccess: () => setEditingProvider(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}

      {/* Delete confirmation */}
      {deletingProvider && (
        <Dialog
          open={true}
          onClose={() => setDeletingProvider(null)}
          title={t("providers.deleteProvider")}
        >
          <p className="mb-4 text-[var(--color-text)]">
            {t("providers.deleteConfirm", { name: deletingProvider.name })}
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              onClick={() => setDeletingProvider(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="danger"
              disabled={deleteMutation.isPending}
              onClick={() => {
                deleteMutation.mutate(deletingProvider.id, {
                  onSuccess: () => setDeletingProvider(null),
                });
              }}
            >
              {deleteMutation.isPending ? (
                <Spinner size="sm" className="mr-2" />
              ) : null}
              {t("common.delete")}
            </Button>
          </div>
        </Dialog>
      )}
    </div>
  );
}

// ---- Provider form dialog ----

interface ProviderFormDialogProps {
  title: string;
  initialData?: Provider;
  onClose: () => void;
  onSubmit: (data: CreateProviderRequest | UpdateProviderRequest) => void;
  isLoading: boolean;
}

function ProviderFormDialog({
  title,
  initialData,
  onClose,
  onSubmit,
  isLoading,
}: ProviderFormDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialData?.name ?? "");
  const [type, setType] = useState(initialData?.type ?? "openai");
  const [endpoint, setEndpoint] = useState(initialData?.endpoint ?? "");
  const [apiKey, setApiKey] = useState(initialData?.apiKey ?? "");
  const [weight, setWeight] = useState(String(initialData?.weight ?? "1"));
  const [priority, setPriority] = useState(
    String(initialData?.priority ?? "0"),
  );
  const [models, setModels] = useState(
    initialData?.models?.join(", ") ?? "",
  );
  const [maxRetries, setMaxRetries] = useState(
    String(initialData?.maxRetries ?? "3"),
  );
  const [timeout, setTimeout] = useState(
    String(initialData?.timeout ?? "30000"),
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const modelList = models
      .split(",")
      .map((m) => m.trim())
      .filter(Boolean);

    onSubmit({
      name,
      type,
      endpoint,
      apiKey,
      weight: Number(weight) || 1,
      priority: Number(priority) || 0,
      models: modelList,
      maxRetries: Number(maxRetries) || 3,
      timeout: Number(timeout) || 30000,
    });
  };

  const providerTypes = [
    "openai",
    "anthropic",
    "azure",
    "google",
    "cohere",
    "custom",
  ];

  return (
    <Dialog open={true} onClose={onClose} title={title} className="max-w-2xl">
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label={t("common.name")}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
          />
          <div>
            <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
              {t("providers.type")}
            </label>
            <select
              value={type}
              onChange={(e) => setType(e.target.value)}
              className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
            >
              {providerTypes.map((pt) => (
                <option key={pt} value={pt}>
                  {pt}
                </option>
              ))}
            </select>
          </div>
        </div>
        <Input
          label={t("providers.endpoint")}
          value={endpoint}
          onChange={(e) => setEndpoint(e.target.value)}
          required
          placeholder="https://api.openai.com/v1"
        />
        <Input
          label={t("providers.apiKey")}
          value={apiKey}
          onChange={(e) => setApiKey(e.target.value)}
          type="password"
          required={!initialData}
          placeholder={initialData ? t("providers.apiKeyUnchanged") : ""}
        />
        <Input
          label={t("providers.models")}
          value={models}
          onChange={(e) => setModels(e.target.value)}
          placeholder={t("providers.modelsPlaceholder")}
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label={t("providers.weight")}
            type="number"
            value={weight}
            onChange={(e) => setWeight(e.target.value)}
            min="0"
          />
          <Input
            label={t("providers.priority")}
            type="number"
            value={priority}
            onChange={(e) => setPriority(e.target.value)}
            min="0"
          />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            label={t("providers.maxRetries")}
            type="number"
            value={maxRetries}
            onChange={(e) => setMaxRetries(e.target.value)}
            min="0"
          />
          <Input
            label={t("providers.timeout")}
            type="number"
            value={timeout}
            onChange={(e) => setTimeout(e.target.value)}
            min="1000"
            step="1000"
          />
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isLoading || !name.trim() || !endpoint.trim()}
          >
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export { ProvidersPage };
