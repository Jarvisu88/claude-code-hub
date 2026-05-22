import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Input } from "@/components/ui/Input";
import { Table, type Column } from "@/components/ui/Table";
import { Badge } from "@/components/ui/Badge";
import { Spinner } from "@/components/ui/Spinner";
import { useAuditLogs } from "@/api/hooks";
import type { AuditLog, AuditLogFilters } from "@/api/types";
import { Filter } from "lucide-react";

function AuditLogsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [filters, setFilters] = useState<AuditLogFilters>({
    page: 1,
    pageSize: 20,
  });
  const [actionFilter, setActionFilter] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");

  const { data, isLoading, error } = useAuditLogs({
    ...filters,
    page,
  });

  const handleApplyFilters = () => {
    setFilters({
      ...filters,
      action: actionFilter || undefined,
      startDate: startDate || undefined,
      endDate: endDate || undefined,
    });
    setPage(1);
  };

  const handleClearFilters = () => {
    setActionFilter("");
    setStartDate("");
    setEndDate("");
    setFilters({ page: 1, pageSize: 20 });
    setPage(1);
  };

  const columns: Column<AuditLog>[] = [
    {
      key: "createdAt",
      header: t("auditLogs.time"),
      render: (row) => (
        <span className="text-sm text-[var(--color-text-secondary)]">
          {new Date(row.createdAt).toLocaleString()}
        </span>
      ),
    },
    {
      key: "action",
      header: t("auditLogs.action"),
      render: (row) => (
        <Badge variant="secondary">{row.action}</Badge>
      ),
    },
    {
      key: "userName",
      header: t("auditLogs.user"),
      render: (row) => (
        <span className="font-medium">{row.userName}</span>
      ),
    },
    {
      key: "details",
      header: t("auditLogs.details"),
      render: (row) => (
        <span
          className="text-sm text-[var(--color-text-secondary)]"
          title={row.details}
        >
          {row.details.length > 60 ? row.details.slice(0, 60) + "..." : row.details}
        </span>
      ),
    },
    {
      key: "ip",
      header: t("auditLogs.ip"),
      render: (row) => (
        <span className="font-mono text-xs text-[var(--color-text-secondary)]">
          {row.ip}
        </span>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("auditLogs.title")}
        </h1>
      </div>

      {/* Filters */}
      <Card className="mb-4">
        <CardContent className="p-4">
          <div className="flex flex-wrap gap-3 items-end">
            <div className="flex-1 min-w-[150px]">
              <Input
                label={t("auditLogs.actionFilter")}
                placeholder={t("auditLogs.actionFilterPlaceholder")}
                value={actionFilter}
                onChange={(e) => setActionFilter(e.target.value)}
              />
            </div>
            <div className="min-w-[150px]">
              <Input
                label={t("auditLogs.startDate")}
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
              />
            </div>
            <div className="min-w-[150px]">
              <Input
                label={t("auditLogs.endDate")}
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
              />
            </div>
            <div className="flex gap-2">
              <Button variant="secondary" onClick={handleApplyFilters}>
                <Filter className="mr-2 h-4 w-4" />
                {t("auditLogs.apply")}
              </Button>
              <Button variant="ghost" onClick={handleClearFilters}>
                {t("auditLogs.clear")}
              </Button>
            </div>
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
        <Table<AuditLog>
          columns={columns}
          data={data?.data ?? []}
          keyExtractor={(row) => row.id}
          page={page}
          pageSize={filters.pageSize ?? 20}
          total={data?.total ?? 0}
          onPageChange={setPage}
        />
      )}
    </div>
  );
}

export { AuditLogsPage };
