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
  useUsers,
  useCreateUser,
  useUpdateUser,
  useDeleteUser,
  useToggleUser,
} from "@/api/hooks";
import type { User, CreateUserRequest, UpdateUserRequest } from "@/api/types";
import { Plus, Search, Pencil, Trash2 } from "lucide-react";

function UsersPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingUser, setEditingUser] = useState<User | null>(null);
  const [deletingUser, setDeletingUser] = useState<User | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useUsers({ page, pageSize, search });
  const createMutation = useCreateUser();
  const updateMutation = useUpdateUser();
  const deleteMutation = useDeleteUser();
  const toggleMutation = useToggleUser();

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

  const columns: Column<User>[] = [
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
      key: "role",
      header: t("users.role"),
      render: (row) => (
        <Badge variant={row.role === "admin" ? "default" : "secondary"}>
          {row.role}
        </Badge>
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
      key: "rateLimit",
      header: t("users.rateLimit"),
      render: (row) => (
        <span className="text-sm">{row.rateLimit || t("users.unlimited")}</span>
      ),
    },
    {
      key: "createdAt",
      header: t("users.createdAt"),
      render: (row) => (
        <span className="text-sm text-[var(--color-text-secondary)]">
          {new Date(row.createdAt).toLocaleDateString()}
        </span>
      ),
    },
    {
      key: "actions",
      header: t("common.actions"),
      className: "w-28",
      render: (row) => (
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setEditingUser(row)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeletingUser(row)}
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
          {t("users.title")}
        </h1>
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("users.createUser")}
        </Button>
      </div>

      {/* Search bar */}
      <Card className="mb-4">
        <CardContent className="p-4">
          <div className="flex gap-2">
            <div className="flex-1">
              <Input
                placeholder={t("users.searchPlaceholder")}
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
        <Table<User>
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
        <UserFormDialog
          title={t("users.createUser")}
          onClose={() => setShowCreateDialog(false)}
          onSubmit={(formData) => {
            createMutation.mutate(formData as CreateUserRequest, {
              onSuccess: () => setShowCreateDialog(false),
            });
          }}
          isLoading={createMutation.isPending}
        />
      )}

      {/* Edit dialog */}
      {editingUser && (
        <UserFormDialog
          title={t("users.editUser")}
          initialData={editingUser}
          onClose={() => setEditingUser(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingUser.id, data: formData as UpdateUserRequest },
              { onSuccess: () => setEditingUser(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}

      {/* Delete confirmation dialog */}
      {deletingUser && (
        <Dialog
          open={true}
          onClose={() => setDeletingUser(null)}
          title={t("users.deleteUser")}
        >
          <p className="mb-4 text-[var(--color-text)]">
            {t("users.deleteConfirm", { name: deletingUser.name })}
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              onClick={() => setDeletingUser(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="danger"
              disabled={deleteMutation.isPending}
              onClick={() => {
                deleteMutation.mutate(deletingUser.id, {
                  onSuccess: () => setDeletingUser(null),
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

// ---- User form dialog ----

interface UserFormDialogProps {
  title: string;
  initialData?: User;
  onClose: () => void;
  onSubmit: (data: CreateUserRequest | UpdateUserRequest) => void;
  isLoading: boolean;
}

function UserFormDialog({
  title,
  initialData,
  onClose,
  onSubmit,
  isLoading,
}: UserFormDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialData?.name ?? "");
  const [role, setRole] = useState<"admin" | "user">(
    initialData?.role ?? "user",
  );
  const [description, setDescription] = useState(
    initialData?.description ?? "",
  );
  const [rateLimit, setRateLimit] = useState(
    String(initialData?.rateLimit ?? "0"),
  );
  const [dailyCostLimit, setDailyCostLimit] = useState(
    String(initialData?.dailyCostLimit ?? "0"),
  );
  const [monthlyCostLimit, setMonthlyCostLimit] = useState(
    String(initialData?.monthlyCostLimit ?? "0"),
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name,
      role,
      description,
      rateLimit: Number(rateLimit) || 0,
      dailyCostLimit: Number(dailyCostLimit) || 0,
      monthlyCostLimit: Number(monthlyCostLimit) || 0,
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
        <div>
          <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
            {t("users.role")}
          </label>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value as "admin" | "user")}
            className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
          >
            <option value="user">{t("users.roleUser")}</option>
            <option value="admin">{t("users.roleAdmin")}</option>
          </select>
        </div>
        <Input
          label={t("users.description")}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <Input
          label={t("users.rateLimit")}
          type="number"
          value={rateLimit}
          onChange={(e) => setRateLimit(e.target.value)}
          min="0"
        />
        <Input
          label={t("users.dailyCostLimit")}
          type="number"
          value={dailyCostLimit}
          onChange={(e) => setDailyCostLimit(e.target.value)}
          min="0"
          step="0.01"
        />
        <Input
          label={t("users.monthlyCostLimit")}
          type="number"
          value={monthlyCostLimit}
          onChange={(e) => setMonthlyCostLimit(e.target.value)}
          min="0"
          step="0.01"
        />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" disabled={isLoading || !name.trim()}>
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export { UsersPage };
