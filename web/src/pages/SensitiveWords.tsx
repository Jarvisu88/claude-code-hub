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
  useSensitiveWords,
  useCreateSensitiveWord,
  useUpdateSensitiveWord,
  useDeleteSensitiveWord,
  useToggleSensitiveWord,
  useRefreshSensitiveWordCache,
} from "@/api/hooks";
import type {
  SensitiveWord,
  CreateSensitiveWordRequest,
  UpdateSensitiveWordRequest,
} from "@/api/types";
import { Plus, Pencil, Trash2, RefreshCw } from "lucide-react";

function SensitiveWordsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [showCreateDialog, setShowCreateDialog] = useState(false);
  const [editingWord, setEditingWord] = useState<SensitiveWord | null>(null);
  const [deletingWord, setDeletingWord] = useState<SensitiveWord | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useSensitiveWords({ page, pageSize });
  const createMutation = useCreateSensitiveWord();
  const updateMutation = useUpdateSensitiveWord();
  const deleteMutation = useDeleteSensitiveWord();
  const toggleMutation = useToggleSensitiveWord();
  const refreshCacheMutation = useRefreshSensitiveWordCache();

  const columns: Column<SensitiveWord>[] = [
    {
      key: "word",
      header: t("sensitiveWords.word"),
      render: (row) => (
        <span className="font-mono text-sm">{row.word}</span>
      ),
    },
    {
      key: "matchType",
      header: t("sensitiveWords.matchType"),
      render: (row) => (
        <Badge variant="secondary">
          {t(`sensitiveWords.matchTypes.${row.matchType}`)}
        </Badge>
      ),
    },
    {
      key: "description",
      header: t("sensitiveWords.description"),
      render: (row) => (
        <span className="text-sm text-[var(--color-text-secondary)]">
          {row.description || "-"}
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
      key: "actions",
      header: t("common.actions"),
      className: "w-24",
      render: (row) => (
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setEditingWord(row)}
          >
            <Pencil className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeletingWord(row)}
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
          {t("sensitiveWords.title")}
        </h1>
        <div className="flex gap-2">
          <Button
            variant="secondary"
            onClick={() => refreshCacheMutation.mutate()}
            disabled={refreshCacheMutation.isPending}
          >
            {refreshCacheMutation.isPending ? (
              <Spinner size="sm" className="mr-2" />
            ) : (
              <RefreshCw className="mr-2 h-4 w-4" />
            )}
            {t("sensitiveWords.refreshCache")}
          </Button>
          <Button onClick={() => setShowCreateDialog(true)}>
            <Plus className="mr-2 h-4 w-4" />
            {t("sensitiveWords.createWord")}
          </Button>
        </div>
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
        <Table<SensitiveWord>
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
        <WordFormDialog
          title={t("sensitiveWords.createWord")}
          onClose={() => setShowCreateDialog(false)}
          onSubmit={(formData) => {
            createMutation.mutate(formData as CreateSensitiveWordRequest, {
              onSuccess: () => setShowCreateDialog(false),
            });
          }}
          isLoading={createMutation.isPending}
        />
      )}

      {editingWord && (
        <WordFormDialog
          title={t("sensitiveWords.editWord")}
          initialData={editingWord}
          onClose={() => setEditingWord(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingWord.id, data: formData as UpdateSensitiveWordRequest },
              { onSuccess: () => setEditingWord(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}

      {deletingWord && (
        <Dialog
          open={true}
          onClose={() => setDeletingWord(null)}
          title={t("sensitiveWords.deleteWord")}
        >
          <p className="mb-4 text-[var(--color-text)]">
            {t("sensitiveWords.deleteConfirm", { word: deletingWord.word })}
          </p>
          <div className="flex justify-end gap-2">
            <Button
              variant="secondary"
              onClick={() => setDeletingWord(null)}
            >
              {t("common.cancel")}
            </Button>
            <Button
              variant="danger"
              disabled={deleteMutation.isPending}
              onClick={() => {
                deleteMutation.mutate(deletingWord.id, {
                  onSuccess: () => setDeletingWord(null),
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

interface WordFormDialogProps {
  title: string;
  initialData?: SensitiveWord;
  onClose: () => void;
  onSubmit: (data: CreateSensitiveWordRequest | UpdateSensitiveWordRequest) => void;
  isLoading: boolean;
}

function WordFormDialog({
  title,
  initialData,
  onClose,
  onSubmit,
  isLoading,
}: WordFormDialogProps) {
  const { t } = useTranslation();
  const [word, setWord] = useState(initialData?.word ?? "");
  const [matchType, setMatchType] = useState(initialData?.matchType ?? "contains");
  const [description, setDescription] = useState(initialData?.description ?? "");

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      word,
      matchType: matchType as "contains" | "regex" | "exact",
      description,
    });
  };

  const matchTypes = ["contains", "regex", "exact"];

  return (
    <Dialog open={true} onClose={onClose} title={title}>
      <form onSubmit={handleSubmit} className="space-y-4">
        <Input
          label={t("sensitiveWords.word")}
          value={word}
          onChange={(e) => setWord(e.target.value)}
          required
        />
        <div>
          <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
            {t("sensitiveWords.matchType")}
          </label>
          <select
            value={matchType}
            onChange={(e) => setMatchType(e.target.value as "contains" | "regex" | "exact")}
            className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
          >
            {matchTypes.map((mt) => (
              <option key={mt} value={mt}>
                {t(`sensitiveWords.matchTypes.${mt}`)}
              </option>
            ))}
          </select>
        </div>
        <Input
          label={t("sensitiveWords.description")}
          value={description}
          onChange={(e) => setDescription(e.target.value)}
        />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isLoading || !word.trim()}
          >
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export { SensitiveWordsPage };
