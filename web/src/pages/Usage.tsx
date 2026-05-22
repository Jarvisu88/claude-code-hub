import { useState, useCallback } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Table, type Column } from "@/components/ui/Table";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { useUsageLogs } from "@/api/hooks";
import type { UsageLog, UsageFilters } from "@/api/types";
import { Search, ChevronDown, ChevronUp, Filter } from "lucide-react";

function statusBadgeVariant(
  status: number,
): "success" | "warning" | "danger" | "secondary" {
  if (status >= 200 && status < 300) return "success";
  if (status >= 400 && status < 500) return "warning";
  if (status >= 500) return "danger";
  return "secondary";
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function UsagePage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [showFilters, setShowFilters] = useState(false);
  const [expandedRow, setExpandedRow] = useState<string | null>(null);

  // Filter state
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [userId, setUserId] = useState("");
  const [model, setModel] = useState("");
  const [status, setStatus] = useState("");

  const pageSize = 20;

  const filters: UsageFilters = {
    page,
    pageSize,
    startDate: startDate || undefined,
    endDate: endDate || undefined,
    userId: userId || undefined,
    model: model || undefined,
    status: status ? Number(status) : undefined,
  };

  const { data, isLoading, error, refetch } = useUsageLogs(filters);

  const handleApplyFilters = useCallback(() => {
    setPage(1);
    refetch();
  }, [refetch]);

  const handleClearFilters = useCallback(() => {
    setStartDate("");
    setEndDate("");
    setUserId("");
    setModel("");
    setStatus("");
    setPage(1);
  }, []);

  const toggleRow = useCallback(
    (id: string) => {
      setExpandedRow(expandedRow === id ? null : id);
    },
    [expandedRow],
  );

  const columns: Column<UsageLog>[] = [
    {
      key: "createdAt",
      header: t("usage.time"),
      render: (row) => (
        <span className="text-sm">
          {new Date(row.createdAt).toLocaleString()}
        </span>
      ),
    },
    {
      key: "userName",
      header: t("usage.user"),
      render: (row) => (
        <span className="text-sm">
          {row.userName ?? row.userId.slice(0, 8)}
        </span>
      ),
    },
    {
      key: "keyName",
      header: t("usage.key"),
      render: (row) => (
        <span className="font-mono text-xs">
          {row.keyName ?? row.keyId.slice(0, 8)}
        </span>
      ),
    },
    {
      key: "model",
      header: t("usage.model"),
      render: (row) => (
        <Badge variant="secondary">{row.model}</Badge>
      ),
    },
    {
      key: "providerName",
      header: t("usage.provider"),
      render: (row) => (
        <span className="text-sm">
          {row.providerName ?? row.providerId.slice(0, 8)}
        </span>
      ),
    },
    {
      key: "cost",
      header: t("usage.cost"),
      render: (row) => (
        <span className="text-sm font-medium">
          ${row.cost.toFixed(4)}
        </span>
      ),
    },
    {
      key: "totalTokens",
      header: t("usage.tokens"),
      render: (row) => (
        <span className="text-sm" title={`${row.inputTokens} + ${row.outputTokens}`}>
          {formatTokens(row.totalTokens)}
        </span>
      ),
    },
    {
      key: "status",
      header: t("common.status"),
      render: (row) => (
        <Badge variant={statusBadgeVariant(row.status)}>
          {row.status}
        </Badge>
      ),
    },
    {
      key: "duration",
      header: t("usage.duration"),
      render: (row) => (
        <span className="text-sm">{formatDuration(row.duration)}</span>
      ),
    },
    {
      key: "expand",
      header: "",
      className: "w-10",
      render: (row) => (
        <button
          type="button"
          onClick={() => toggleRow(row.id)}
          className="p-1 hover:bg-[var(--color-bg-tertiary)] rounded"
        >
          {expandedRow === row.id ? (
            <ChevronUp className="h-4 w-4 text-[var(--color-text-secondary)]" />
          ) : (
            <ChevronDown className="h-4 w-4 text-[var(--color-text-secondary)]" />
          )}
        </button>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("usage.title")}
        </h1>
        <Button
          variant="secondary"
          onClick={() => setShowFilters(!showFilters)}
        >
          <Filter className="mr-2 h-4 w-4" />
          {t("usage.filters")}
        </Button>
      </div>

      {/* Filters panel */}
      {showFilters && (
        <Card className="mb-4">
          <CardContent className="p-4">
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
              <Input
                label={t("usage.startDate")}
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
              />
              <Input
                label={t("usage.endDate")}
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
              />
              <Input
                label={t("usage.userId")}
                value={userId}
                onChange={(e) => setUserId(e.target.value)}
                placeholder={t("usage.userIdPlaceholder")}
              />
              <Input
                label={t("usage.model")}
                value={model}
                onChange={(e) => setModel(e.target.value)}
                placeholder={t("usage.modelPlaceholder")}
              />
              <div>
                <label className="mb-1.5 block text-sm font-medium text-[var(--color-text)]">
                  {t("usage.statusCode")}
                </label>
                <select
                  value={status}
                  onChange={(e) => setStatus(e.target.value)}
                  className="flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm text-[var(--color-text)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)]"
                >
                  <option value="">{t("usage.allStatuses")}</option>
                  <option value="200">200</option>
                  <option value="400">400</option>
                  <option value="401">401</option>
                  <option value="429">429</option>
                  <option value="500">500</option>
                  <option value="502">502</option>
                  <option value="503">503</option>
                </select>
              </div>
            </div>
            <div className="mt-4 flex justify-end gap-2">
              <Button variant="secondary" onClick={handleClearFilters}>
                {t("usage.clearFilters")}
              </Button>
              <Button onClick={handleApplyFilters}>
                <Search className="mr-2 h-4 w-4" />
                {t("usage.applyFilters")}
              </Button>
            </div>
          </CardContent>
        </Card>
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
        <>
          <Table<UsageLog>
            columns={columns}
            data={(data as any)?.logs ?? (data as any)?.data ?? []}
            keyExtractor={(row) => row.id}
            page={page}
            pageSize={pageSize}
            total={(data as any)?.total ?? 0}
            onPageChange={setPage}
          />

          {/* Expanded row details */}
          {expandedRow && ((data as any)?.logs ?? (data as any)?.data) && (
            <ExpandedRowDetails
              log={((data as any)?.logs ?? (data as any)?.data ?? []).find((l: UsageLog) => l.id === expandedRow) ?? null}
            />
          )}
        </>
      )}
    </div>
  );
}

// ---- Expanded row details ----

function ExpandedRowDetails({ log }: { log: UsageLog | null }) {
  const { t } = useTranslation();
  if (!log) return null;

  return (
    <Card className="mt-2">
      <CardContent className="p-4">
        <h4 className="mb-3 text-sm font-semibold text-[var(--color-text)]">
          {t("usage.details")}
        </h4>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <DetailItem label="ID" value={log.id} />
          <DetailItem label={t("usage.user")} value={log.userName ?? log.userId} />
          <DetailItem label={t("usage.key")} value={log.keyName ?? log.keyId} />
          <DetailItem label={t("usage.model")} value={log.model} />
          <DetailItem
            label={t("usage.provider")}
            value={log.providerName ?? log.providerId}
          />
          <DetailItem label={t("usage.cost")} value={`$${log.cost.toFixed(6)}`} />
          <DetailItem
            label={t("usage.inputTokens")}
            value={String(log.inputTokens)}
          />
          <DetailItem
            label={t("usage.outputTokens")}
            value={String(log.outputTokens)}
          />
          <DetailItem
            label={t("usage.totalTokens")}
            value={String(log.totalTokens)}
          />
          <DetailItem label={t("common.status")} value={String(log.status)} />
          <DetailItem
            label={t("usage.duration")}
            value={formatDuration(log.duration)}
          />
          <DetailItem
            label={t("usage.time")}
            value={new Date(log.createdAt).toLocaleString()}
          />
        </div>
        {log.errorMessage && (
          <div className="mt-3">
            <span className="text-sm font-medium text-[var(--color-danger)]">
              {t("usage.errorMessage")}:
            </span>
            <pre className="mt-1 rounded-md bg-[var(--color-bg-secondary)] p-2 text-xs text-[var(--color-text)]">
              {log.errorMessage}
            </pre>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

function DetailItem({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <span className="text-xs text-[var(--color-text-tertiary)]">{label}</span>
      <p className="mt-0.5 text-sm font-medium text-[var(--color-text)] break-all">
        {value}
      </p>
    </div>
  );
}

export { UsagePage };
