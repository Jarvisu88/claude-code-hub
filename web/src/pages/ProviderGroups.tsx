import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Table, type Column } from "@/components/ui/Table";
import { Dialog } from "@/components/ui/Dialog";
import { Spinner } from "@/components/ui/Spinner";
import {
  useProviderGroups,
  useCreateProviderGroup,
  useUpdateProviderGroup,
  useDeleteProviderGroup,
} from "@/api/hooks";
import type {
  ProviderGroup,
  CreateProviderGroupRequest,
  UpdateProviderGroupRequest,
} from "@/api/types";
import { Plus, Pencil, Trash2 } from "lucide-react";

function ProviderGroupsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingGroup, setEditingGroup] = useState<ProviderGroup | null>(null);
  const [deletingGroup, setDeletingGroup] = useState<ProviderGroup | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useProviderGroups({ page, pageSize });
  const createMutation = useCreateProviderGroup();
  const updateMutation = useUpdateProviderGroup();
  const deleteMutation = useDeleteProviderGroup();

  const columns: Column<ProviderGroup>[] = [
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
      key: "costMultiplier",
      header: t("providerGroups.costMultiplier"),
      render: (row) => <span className="text-sm">{row.costMultiplier}x</span>,
    },
    {
      key: "description",
      header: t("providerGroups.description"),
      render: (row) => (
        <span className="text-sm text-[var(--color-text-secondary)]">
          {row.description || "-"}
        </span>
      ),
    },
    {
      key: "providerCount",
      header: t("providerGroups.providerCount"),
      render: (row) => <span className="text-sm">{row.providerCount}</span>,
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
            onClick={() => setEditingGroup(row)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeletingGroup(row)}
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
          {t("providerGroups.title")}
        </h1>
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("providerGroups.createGroup")}
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
        <Table<ProviderGroup>
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
        <GroupFormDialog
          title={t("providerGroups.createGroup")}
          onClose={() => setShowCreateDialog(false)}
          onSubmit={(formData) => {
            createMutation.mutate(formData, {
              onSuccess: () => setShowCreateDialog(false),
            });
          }}
          isLoading={createMutation.isPending}
        />
      )}

      {editingGroup && (
        <GroupFormDialog
          title={t("providerGroups.editGroup")}
          initialData={editingGroup}
          onClose={() => setEditingGroup(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingGroup.id, data: formData },
              { onSuccess: () => setEditingGroup(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}

      {deletingGroup && (
        <Dialog
          open={true}
          onClose={() => setDeletingGroup(null)}
          title={t("providerGroups.deleteGroup")}
        >
          <p className="mb-4 text-[var(--color-text)]">
            {t("providerGroups.deleteConfirm", { name: deletingGroup.name })}
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              onClick={() => setDeletingGroup(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="danger"
              disabled={deleteMutation.isPending}
              onClick={() => {
                deleteMutation.mutate(deletingGroup.id, {
                  onSuccess: () => setDeletingGroup(null),
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

interface GroupFormDialogProps {
  title: string;
  initialData?: ProviderGroup;
  onClose: () => void;
  onSubmit: (data: CreateProviderGroupRequest | UpdateProviderGroupRequest) => void;
  isLoading: boolean;
}

function GroupFormDialog({
  title,
  initialData,
  onClose,
  onSubmit,
  isLoading,
}: GroupFormDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialData?.name ?? "");
  const [description, setDescription] = useState(initialData?.description ?? "");
  const [costMultiplier, setCostMultiplier] = useState(
    String(initialData?.costMultiplier ?? "1"),
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name,
      description,
      costMultiplier: Number(costMultiplier) || 1,
    });
  };

  return (
    <Dialog open={true} onClose={onClose} title={title}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <Input
          label={t("common.name")}
          value={name}
          onChange={(e) => setName(e.target.value)}
          required
        />
        <Input
          label={t("providerGroups.description")}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <Input
          label={t("providerGroups.costMultiplier")}
          type="number"
          value={costMultiplier}
          onChange={(e) => setCostMultiplier(e.target.value)}
          min="0"
          step="0.01"
        />
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

export { ProviderGroupsPage };
