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
  useKeys,
  useCreateKey,
  useUpdateKey,
  useDeleteKey,
  useToggleKey,
} from "@/api/hooks";
import type { ApiKey, CreateKeyRequest, UpdateKeyRequest } from "@/api/types";
import { Plus, Search, Pencil, Trash2, Copy, Check } from "lucide-react";

function maskKey(key: string): string {
  if (!key || key.length < 12) return key;
  return key.slice(0, 8) + "..." + key.slice(-4);
}

function KeysPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingKey, setEditingKey] = useState<ApiKey | null>(null);
  const [deletingKey, setDeletingKey] = useState<ApiKey | null>(null);
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<string | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useKeys({ page, pageSize, search });
  const createMutation = useCreateKey();
  const updateMutation = useUpdateKey();
  const deleteMutation = useDeleteKey();
  const toggleMutation = useToggleKey();

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

  const columns: Column<ApiKey>[] = [
    {
      key: "id",
      header: "ID",
      className: "w-20",
      render: (row) => (
        <span className="font-mono text-xs">{row.id.slice(0, 8)}</span>
      ),
    },
    {
      key: "key",
      header: t("keys.key"),
      render: (row) => (
        <span className="font-mono text-xs">{maskKey(row.key)}</span>
      ),
    },
    {
      key: "name",
      header: t("common.name"),
      render: (row) => <span className="font-medium">{row.name}</span>,
    },
    {
      key: "userName",
      header: t("keys.user"),
      render: (row) => (
        <span className="text-sm">{row.userName ?? row.userId.slice(0, 8)}</span>
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
      header: t("keys.rateLimit"),
      render: (row) => (
        <span className="text-sm">{row.rateLimit || t("users.unlimited")}</span>
      ),
    },
    {
      key: "createdAt",
      header: t("keys.createdAt"),
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
            onClick={() => setEditingKey(row)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeletingKey(row)}
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
          {t("keys.title")}
        </h1>
        <Button onClick={() => setShowCreateDialog(true)}>
          <Plus className="mr-2 h-4 w-4" />
          {t("keys.createKey")}
        </Button>
      </div>

      {/* Search bar */}
      <Card className="mb-4">
        <CardContent className="p-4">
          <div className="flex gap-2">
            <div className="flex-1">
              <Input
                placeholder={t("keys.searchPlaceholder")}
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
        <Table<ApiKey>
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
        <KeyFormDialog
          title={t("keys.createKey")}
          onClose={() => {
            setShowCreateDialog(false);
            setNewlyCreatedKey(null);
          }}
          onSubmit={(formData) => {
            createMutation.mutate(formData, {
              onSuccess: (created) => {
                setNewlyCreatedKey(created.key);
              },
            });
          }}
          isLoading={createMutation.isPending}
          newlyCreatedKey={newlyCreatedKey}
        />
      )}

      {/* Edit dialog */}
      {editingKey && (
        <KeyEditDialog
          title={t("keys.editKey")}
          initialData={editingKey}
          onClose={() => setEditingKey(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingKey.id, data: formData },
              { onSuccess: () => setEditingKey(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}

      {/* Delete confirmation */}
      {deletingKey && (
        <Dialog
          open={true}
          onClose={() => setDeletingKey(null)}
          title={t("keys.deleteKey")}
        >
          <p className="mb-4 text-[var(--color-text)]">
            {t("keys.deleteConfirm", { name: deletingKey.name })}
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              onClick={() => setDeletingKey(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="danger"
              disabled={deleteMutation.isPending}
              onClick={() => {
                deleteMutation.mutate(deletingKey.id, {
                  onSuccess: () => setDeletingKey(null),
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

// ---- Create key dialog ----

interface KeyFormDialogProps {
  title: string;
  onClose: () => void;
  onSubmit: (data: CreateKeyRequest) => void;
  isLoading: boolean;
  newlyCreatedKey: string | null;
}

function KeyFormDialog({
  title,
  onClose,
  onSubmit,
  isLoading,
  newlyCreatedKey,
}: KeyFormDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState("");
  const [userId, setUserId] = useState("");
  const [rateLimit, setRateLimit] = useState("0");
  const [dailyCostLimit, setDailyCostLimit] = useState("0");
  const [copied, setCopied] = useState(false);

  const handleCopy = async () => {
    if (!newlyCreatedKey) return;
    try {
      await navigator.clipboard.writeText(newlyCreatedKey);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Fallback: do nothing
    }
  };

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name,
      userId,
      rateLimit: Number(rateLimit) || 0,
      dailyCostLimit: Number(dailyCostLimit) || 0,
    });
  };

  if (newlyCreatedKey) {
    return (
      <Dialog open={true} onClose={onClose} title={t("keys.keyCreated")}>
        <div className="space-y-4">
          <p className="text-sm text-[var(--color-text-secondary)]">
            {t("keys.copyKeyWarning")}
          </p>
          <div className="flex items-center gap-2 rounded-md border border-[var(--color-border)] bg-[var(--color-bg-secondary)] p-3">
            <code className="flex-1 break-all font-mono text-sm text-[var(--color-text)]">
              {newlyCreatedKey}
            </code>
            <Button variant="ghost" size="sm" onClick={handleCopy}>
              {copied ? (
                <Check className="h-4 w-4 text-green-500" />
              ) : (
                <Copy className="h-4 w-4" />
              )}
            </Button>
          </div>
          <div className="flex justify-end">
            <Button onClick={onClose}>{t("common.confirm")}</Button>
          </div>
        </div>
      </Dialog>
    );
  }

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
          label={t("keys.userId")}
          value={userId}
          onChange={(e) => setUserId(e.target.value)}
          required
          placeholder={t("keys.userIdPlaceholder")}
        />
        <Input
          label={t("keys.rateLimit")}
          type="number"
          value={rateLimit}
          onChange={(e) => setRateLimit(e.target.value)}
          min="0"
        />
        <Input
          label={t("keys.dailyCostLimit")}
          type="number"
          value={dailyCostLimit}
          onChange={(e) => setDailyCostLimit(e.target.value)}
          min="0"
          step="0.01"
        />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isLoading || !name.trim() || !userId.trim()}
          >
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.create")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

// ---- Edit key dialog ----

interface KeyEditDialogProps {
  title: string;
  initialData: ApiKey;
  onClose: () => void;
  onSubmit: (data: UpdateKeyRequest) => void;
  isLoading: boolean;
}

function KeyEditDialog({
  title,
  initialData,
  onClose,
  onSubmit,
  isLoading,
}: KeyEditDialogProps) {
  const { t } = useTranslation();
  const [name, setName] = useState(initialData.name);
  const [rateLimit, setRateLimit] = useState(String(initialData.rateLimit ?? "0"));
  const [dailyCostLimit, setDailyCostLimit] = useState(
    String(initialData.dailyCostLimit ?? "0"),
  );

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      name,
      rateLimit: Number(rateLimit) || 0,
      dailyCostLimit: Number(dailyCostLimit) || 0,
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
          label={t("keys.rateLimit")}
          type="number"
          value={rateLimit}
          onChange={(e) => setRateLimit(e.target.value)}
          min="0"
        />
        <Input
          label={t("keys.dailyCostLimit")}
          type="number"
          value={dailyCostLimit}
          onChange={(e) => setDailyCostLimit(e.target.value)}
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

export { KeysPage };
