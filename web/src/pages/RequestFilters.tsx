import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Table, type Column } from "@/components/ui/Table";
import { Badge } from "@/components/ui/Badge";
import { Dialog } from "@/components/ui/Dialog";
import { Spinner } from "@/components/ui/Spinner";
import {
  useRequestFilters,
  useCreateRequestFilter,
  useUpdateRequestFilter,
  useDeleteRequestFilter,
  useToggleRequestFilter,
} from "@/api/hooks";
import type {
  RequestFilter,
  CreateRequestFilterRequest,
  UpdateRequestFilterRequest,
} from "@/api/types";
import { Plus, Pencil, Trash2 } from "lucide-react";

function actionBadgeVariant(
  action: string,
): "danger" | "success" | "warning" | "secondary" {
  switch (action) {
    case "block":
      return "danger";
    case "allow":
      return "success";
    case "rewrite":
      return "warning";
    default:
      return "secondary";
  }
}

function RequestFiltersPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingFilter, setEditingFilter] = useState<RequestFilter | null>(null);
  const [deletingFilter, setDeletingFilter] = useState<RequestFilter | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useRequestFilters({ page, pageSize });
  const createMutation = useCreateRequestFilter();
  const updateMutation = useUpdateRequestFilter();
  const deleteMutation = useDeleteRequestFilter();
  const toggleMutation = useToggleRequestFilter();

  const columns: Column<RequestFilter>[] = [
    {
      key: "name",
      header: t("common.name"),
      render: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      key: "scope",
      header: t("requestFilters.scope"),
      render: (row) => (
        <Badge variant="secondary">{row.scope}</Badge>
      ),
    },
    {
      key: "action",
      header: t("requestFilters.action"),
      render: (row) => (
        <Badge variant={actionBadgeVariant(row.action)}>
          {t(`requestFilters.actions.${row.action}`)}
        </Badge>
      ),
    },
    {
      key: "priority",
      header: t("requestFilters.priority"),
      render: (row) => <span className="text-sm">{row.priority}</span>,
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
      key: "actions",
      header: t("common.actions"),
      className: "w-24",
      render: (row) => (
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setEditingFilter(row)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeletingFilter(row)}
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
          {t("requestFilters.title")}
        </h1>
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("requestFilters.createFilter")}
        </Button>
      </div>

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
        <Table<RequestFilter>
          columns={columns}
          data={data?.data ?? []}
          keyExtractor={(row) => row.id}
          page={page}
          pageSize={pageSize}
          total={data?.total ?? 0}
          onPageChange={setPage}
        />
      )}

      {showCreateDialog && (
        <FilterFormDialog
          title={t("requestFilters.createFilter")}
          onClose={() => setShowCreateDialog(false)}
          onSubmit={(formData) => {
            createMutation.mutate(formData, {
              onSuccess: () => setShowCreateDialog(false),
            });
          }}
          isLoading={createMutation.isPending}
        />
      )}

      {editingFilter && (
        <FilterFormDialog
          title={t("requestFilters.editFilter")}
          initialData={editingFilter}
          onClose={() => setEditingFilter(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingFilter.id, data: formData },
              { onSuccess: () => setEditingFilter(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}

      {deletingFilter && (
        <Dialog
          open={true}
          onClose={() => setDeletingFilter(null)}
          title={t("requestFilters.deleteFilter")}
        >
          <p className="mb-4 text-[var(--color-text)]">
            {t("requestFilters.deleteConfirm", { name: deletingFilter.name })}
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              onClick={() => setDeletingFilter(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="danger"
              disabled={deleteMutation.isPending}
              onClick={() => {
                deleteMutation.mutate(deletingFilter.id, {
                  onSuccess: () => setDeletingFilter(null),
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

interface FilterFormDialogProps {
  title: string;
  initialData?: RequestFilter;
  onClose: () => void;
  onSubmit: (data: CreateRequestFilterRequest | UpdateRequestFilterRequest) => void;
  isLoading: boolean;
}

function FilterFormDialog({
  title,
  initialData,
  onClose,
  onSubmit,
  isLoading,
}: FilterFormDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialData?.name ?? "");
  const [scope, setScope] = useState(initialData?.scope ?? "global");
  const [action, setAction] = useState(initialData?.action ?? "block");
  const [priority, setPriority] = useState(String(initialData?.priority ?? "0"));
  const [config, setConfig] = useState(initialData?.config ?? "");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name,
      scope,
      action: action as "block" | "allow" | "rewrite",
      priority: Number(priority) || 0,
      config,
    });
  };

  const scopes = ["global", "user", "key", "model"];
  const actionValues = ["block", "allow", "rewrite"];

  return (
    <Dialog open={true} onClose={onClose} title={title} className="max-w-2xl">
      <form onSubmit={handleSubmit} className="space-y-4">
        <Input
          label={t("common.name")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <div className="grid gap-4 sm:grid-cols-2">
          <div>
            <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
              {t("requestFilters.scope")}
            </label>
            <select
              value={scope}
              onChange={(e) => setScope(e.target.value)}
              className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
            >
              {scopes.map((s) => (
                <option key={s} value={s}>
                  {t(`requestFilters.scopes.${s}`)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
              {t("requestFilters.action")}
            </label>
            <select
              value={action}
              onChange={(e) => setAction(e.target.value)}
              className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
            >
              {actionValues.map((a) => (
                <option key={a} value={a}>
                  {t(`requestFilters.actions.${a}`)}
                </option>
              ))}
            </select>
          </div>
        </div>
        <Input
          label={t("requestFilters.priority")}
          type="number"
          value={priority}
          onChange={(e) => setPriority(e.target.value)}
          min="0"
        />
        <div>
          <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
            {t("requestFilters.config")}
          </label>
          <textarea
            value={config}
            onChange={(e) => setConfig(e.target.value)}
            rows={4}
            className="flex w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] placeholder:text-[var(--color-text-tertiary)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] transition-colors"
            placeholder={t("requestFilters.configPlaceholder")}
          />
        </div>
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isLoading || !name.trim()}
          >
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export { RequestFiltersPage };
