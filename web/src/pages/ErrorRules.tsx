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
  useErrorRules,
  useCreateErrorRule,
  useUpdateErrorRule,
  useDeleteErrorRule,
  useToggleErrorRule,
} from "@/api/hooks";
import type {
  ErrorRule,
  CreateErrorRuleRequest,
  UpdateErrorRuleRequest,
} from "@/api/types";
import { Plus, Pencil, Trash2 } from "lucide-react";

function ErrorRulesPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingRule, setEditingRule] = useState<ErrorRule | null>(null);
  const [deletingRule, setDeletingRule] = useState<ErrorRule | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useErrorRules({ page, pageSize });
  const createMutation = useCreateErrorRule();
  const updateMutation = useUpdateErrorRule();
  const deleteMutation = useDeleteErrorRule();
  const toggleMutation = useToggleErrorRule();

  const columns: Column<ErrorRule>[] = [
    {
      key: "pattern",
      header: t("errorRules.pattern"),
      render: (row) => (
        <span className="font-mono text-xs">{row.pattern}</span>
      ),
    },
    {
      key: "matchType",
      header: t("errorRules.matchType"),
      render: (row) => (
        <Badge variant="secondary">
          {t(`errorRules.matchTypes.${row.matchType}`)}
        </Badge>
      ),
    },
    {
      key: "category",
      header: t("errorRules.category"),
      render: (row) => <span className="text-sm">{row.category}</span>,
    },
    {
      key: "priority",
      header: t("errorRules.priority"),
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
            onClick={() => setEditingRule(row)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeletingRule(row)}
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
          {t("errorRules.title")}
        </h1>
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("errorRules.createRule")}
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
        <Table<ErrorRule>
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
        <ErrorRuleFormDialog
          title={t("errorRules.createRule")}
          onClose={() => setShowCreateDialog(false)}
          onSubmit={(formData) => {
            createMutation.mutate(formData as CreateErrorRuleRequest, {
              onSuccess: () => setShowCreateDialog(false),
            });
          }}
          isLoading={createMutation.isPending}
        />
      )}

      {editingRule && (
        <ErrorRuleFormDialog
          title={t("errorRules.editRule")}
          initialData={editingRule}
          onClose={() => setEditingRule(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingRule.id, data: formData as UpdateErrorRuleRequest },
              { onSuccess: () => setEditingRule(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}

      {deletingRule && (
        <Dialog
          open={true}
          onClose={() => setDeletingRule(null)}
          title={t("errorRules.deleteRule")}
        >
          <p className="mb-4 text-[var(--color-text)]">
            {t("errorRules.deleteConfirm")}
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              onClick={() => setDeletingRule(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="danger"
              disabled={deleteMutation.isPending}
              onClick={() => {
                deleteMutation.mutate(deletingRule.id, {
                  onSuccess: () => setDeletingRule(null),
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

interface ErrorRuleFormDialogProps {
  title: string;
  initialData?: ErrorRule;
  onClose: () => void;
  onSubmit: (data: CreateErrorRuleRequest | UpdateErrorRuleRequest) => void;
  isLoading: boolean;
}

function ErrorRuleFormDialog({
  title,
  initialData,
  onClose,
  onSubmit,
  isLoading,
}: ErrorRuleFormDialogProps) {
  const { t } = useTranslation();
  const [pattern, setPattern] = useState(initialData?.pattern ?? "");
  const [matchType, setMatchType] = useState(initialData?.matchType ?? "contains");
  const [category, setCategory] = useState(initialData?.category ?? "");
  const [priority, setPriority] = useState(String(initialData?.priority ?? "0"));
  const [description, setDescription] = useState(initialData?.description ?? "");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      pattern,
      matchType: matchType as "contains" | "regex" | "exact",
      category,
      priority: Number(priority) || 0,
      description,
    });
  };

  const matchTypes = ["contains", "regex", "exact"];

  return (
    <Dialog open={true} onClose={onClose} title={title}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <Input
          label={t("errorRules.pattern")}
          value={pattern}
          onChange={(e) => setPattern(e.target.value)}
          required
        />
        <div>
          <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
            {t("errorRules.matchType")}
          </label>
          <select
            value={matchType}
            onChange={(e) => setMatchType(e.target.value as "contains" | "regex" | "exact")}
            className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
          >
            {matchTypes.map((mt) => (
              <option key={mt} value={mt}>
                {t(`errorRules.matchTypes.${mt}`)}
              </option>
            ))}
          </select>
        </div>
        <Input
          label={t("errorRules.category")}
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          required
        />
        <Input
          label={t("errorRules.priority")}
          type="number"
          value={priority}
          onChange={(e) => setPriority(e.target.value)}
          min="0"
        />
        <Input
          label={t("errorRules.description")}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isLoading || !pattern.trim() || !category.trim()}
          >
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export { ErrorRulesPage };
