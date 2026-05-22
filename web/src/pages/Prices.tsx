import { useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Table, type Column } from "@/components/ui/Table";
import { Dialog } from "@/components/ui/Dialog";
import { Spinner } from "@/components/ui/Spinner";
import { usePrices, useUpdatePrice, useSyncPrices } from "@/api/hooks";
import type { ModelPrice, UpdateModelPriceRequest } from "@/api/types";
import { Search, Pencil, RefreshCw } from "lucide-react";

function PricesPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState("");
  const [searchInput, setSearchInput] = useState("");
  const [editingPrice, setEditingPrice] = useState<ModelPrice | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = usePrices({ page, pageSize, search });
  const updateMutation = useUpdatePrice();
  const syncMutation = useSyncPrices();

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

  const columns: Column<ModelPrice>[] = [
    {
      key: "model",
      header: t("prices.model"),
      render: (row) => (
        <span className="font-mono text-sm font-medium">{row.model}</span>
      ),
    },
    {
      key: "inputPrice",
      header: t("prices.inputPrice"),
      render: (row) => (
        <span className="text-sm">${row.inputPrice.toFixed(6)}</span>
      ),
    },
    {
      key: "outputPrice",
      header: t("prices.outputPrice"),
      render: (row) => (
        <span className="text-sm">${row.outputPrice.toFixed(6)}</span>
      ),
    },
    {
      key: "source",
      header: t("prices.source"),
      render: (row) => (
        <span className="text-sm text-[var(--color-text-secondary)]">
          {row.source}
        </span>
      ),
    },
    {
      key: "updatedAt",
      header: t("prices.updated"),
      render: (row) => (
        <span className="text-sm text-[var(--color-text-secondary)]">
          {new Date(row.updatedAt).toLocaleString()}
        </span>
      ),
    },
    {
      key: "actions",
      header: t("common.actions"),
      className: "w-16",
      render: (row) => (
        <Button
          variant="ghost"
          size="sm"
          onClick={() => setEditingPrice(row)}
        >
          <Pencil className="h-4 w-4" />
        </Button>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("prices.title")}
        </h1>
        <Button
          onClick={() => syncMutation.mutate()}
          disabled={syncMutation.isPending}
        >
          {syncMutation.isPending ? (
            <Spinner size="sm" className="mr-2" />
          ) : (
            <RefreshCw className="mr-2 h-4 w-4" />
          )}
          {t("prices.syncFromLiteLLM")}
        </Button>
      </div>

      {/* Sync result */}
      {syncMutation.isSuccess && (
        <div className="mb-4 rounded-md border border-green-300 bg-green-50 p-3 text-green-800 dark:border-green-700 dark:bg-green-900/20 dark:text-green-400">
          {t("prices.syncSuccess", { count: syncMutation.data?.synced ?? 0 })}
        </div>
      )}

      {/* Search */}
      <Card className="mb-4">
        <CardContent className="p-4">
          <div className="flex gap-2">
            <div className="flex-1">
              <Input
                placeholder={t("prices.searchPlaceholder")}
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
        <Table<ModelPrice>
          columns={columns}
          data={data?.data ?? []}
          keyExtractor={(row) => row.id}
          page={page}
          pageSize={pageSize}
          total={data?.total ?? 0}
          onPageChange={setPage}
        />
      )}

      {editingPrice && (
        <PriceEditDialog
          price={editingPrice}
          onClose={() => setEditingPrice(null)}
          onSubmit={(formData) => {
            updateMutation.mutate(
              { id: editingPrice.id, data: formData },
              { onSuccess: () => setEditingPrice(null) },
            );
          }}
          isLoading={updateMutation.isPending}
        />
      )}
    </div>
  );
}

interface PriceEditDialogProps {
  price: ModelPrice;
  onClose: () => void;
  onSubmit: (data: UpdateModelPriceRequest) => void;
  isLoading: boolean;
}

function PriceEditDialog({
  price,
  onClose,
  onSubmit,
  isLoading,
}: PriceEditDialogProps) {
  const { t } = useTranslation();
  const [inputPrice, setInputPrice] = useState(String(price.inputPrice));
  const [outputPrice, setOutputPrice] = useState(String(price.outputPrice));

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmit({
      inputPrice: Number(inputPrice) || 0,
      outputPrice: Number(outputPrice) || 0,
    });
  };

  return (
    <Dialog
      open={true}
      onClose={onClose}
      title={t("prices.editPrice")}
    >
      <form onSubmit={handleSubmit} className="space-y-4">
        <div className="rounded-md bg-[var(--color-bg-secondary)] p-3">
          <span className="font-mono text-sm font-medium text-[var(--color-text)]">
            {price.model}
          </span>
        </div>
        <Input
          label={t("prices.inputPrice")}
          type="number"
          value={inputPrice}
          onChange={(e) => setInputPrice(e.target.value)}
          min="0"
          step="0.000001"
        />
        <Input
          label={t("prices.outputPrice")}
          type="number"
          value={outputPrice}
          onChange={(e) => setOutputPrice(e.target.value)}
          min="0"
          step="0.000001"
        />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="secondary" onClick={onClose}>
            {t("common.cancel")}
          </Button>
          <Button type="submit" disabled={isLoading}>
            {isLoading ? <Spinner size="sm" className="mr-2" /> : null}
            {t("common.save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}

export { PricesPage };
