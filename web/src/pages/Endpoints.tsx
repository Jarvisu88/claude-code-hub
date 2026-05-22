import { useState } from "react";
import { useTranslation } from "react-i18next";
import { Card, CardContent } from "@/components/ui/Card";
import { Button } from "@/components/ui/Button";
import { Table, type Column } from "@/components/ui/Table";
import { Badge } from "@/components/ui/Badge";
import { Dialog } from "@/components/ui/Dialog";
import { Spinner } from "@/components/ui/Spinner";
import {
  useEndpoints,
  useProbeEndpoint,
  useEndpointProbeLogs,
} from "@/api/hooks";
import type { ProviderEndpoint, ProbeLog } from "@/api/types";
import { Zap, FileText } from "lucide-react";

function probeBadgeVariant(
  status: string,
): "success" | "danger" | "secondary" {
  switch (status) {
    case "ok":
      return "success";
    case "fail":
      return "danger";
    default:
      return "secondary";
  }
}

function EndpointsPage() {
  const { t } = useTranslation();
  const [page, setPage] = useState(1);
  const [probingId, setProbingId] = useState<string | null>(null);
  const [logsEndpointId, setLogsEndpointId] = useState<string | null>(null);

  const pageSize = 20;
  const { data, isLoading, error } = useEndpoints({ page, pageSize });
  const probeMutation = useProbeEndpoint();
  const { data: probeLogs, isLoading: logsLoading } = useEndpointProbeLogs(logsEndpointId);

  const handleProbe = (id: string) => {
    setProbingId(id);
    probeMutation.mutate(id, {
      onSettled: () => setProbingId(null),
    });
  };

  const columns: Column<ProviderEndpoint>[] = [
    {
      key: "id",
      header: "ID",
      className: "w-20",
      render: (row) => (
        <span className="font-mono text-xs">{row.id.slice(0, 8)}</span>
      ),
    },
    {
      key: "vendor",
      header: t("endpoints.vendor"),
      render: (row) => <Badge variant="secondary">{row.vendor}</Badge>,
    },
    {
      key: "type",
      header: t("endpoints.type"),
      render: (row) => <span className="text-sm">{row.type}</span>,
    },
    {
      key: "url",
      header: t("endpoints.url"),
      render: (row) => (
        <span
          className="font-mono text-xs text-[var(--color-text-secondary)]"
          title={row.url}
        >
          {row.url.length > 40 ? row.url.slice(0, 40) + "..." : row.url}
        </span>
      ),
    },
    {
      key: "enabled",
      header: t("common.status"),
      render: (row) => (
        <Badge variant={row.enabled ? "success" : "danger"}>
          {row.enabled ? t("common.enabled") : t("common.disabled")}
        </Badge>
      ),
    },
    {
      key: "probeStatus",
      header: t("endpoints.probeStatus"),
      render: (row) => (
        <Badge variant={probeBadgeVariant(row.probeStatus)}>
          {t(`endpoints.probeStatusValues.${row.probeStatus}`)}
        </Badge>
      ),
    },
    {
      key: "latency",
      header: t("endpoints.latency"),
      render: (row) => (
        <span className="text-sm">
          {row.latency > 0 ? `${row.latency}ms` : "-"}
        </span>
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
            onClick={() => handleProbe(row.id)}
            disabled={probingId === row.id}
          >
            {probingId === row.id ? (
              <Spinner size="sm" />
            ) : (
              <Zap className="h-4 w-4" />
            )}
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setLogsEndpointId(row.id)}
          >
            <FileText className="h-4 w-4" />
          </Button>
        </div>
      ),
    },
  ];

  return (
    <div>
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-[var(--color-text)]">
          {t("endpoints.title")}
        </h1>
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
        <Table<ProviderEndpoint>
          columns={columns}
          data={data?.data ?? []}
          keyExtractor={(row) => row.id}
          page={page}
          pageSize={pageSize}
          total={data?.total ?? 0}
          onPageChange={setPage}
        />
      )}

      {logsEndpointId && (
        <Dialog
          open={true}
          onClose={() => setLogsEndpointId(null)}
          title={t("endpoints.probeLogs")}
          className="max-w-2xl"
        >
          {logsLoading ? (
            <div className="flex h-32 items-center justify-center">
              <Spinner size="lg" />
            </div>
          ) : (
            <div className="max-h-96 overflow-y-auto">
              {(!probeLogs || probeLogs.length === 0) ? (
                <p className="text-center text-[var(--color-text-secondary)] py-8">
                  {t("common.noData")}
                </p>
              ) : (
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-[var(--color-border)]">
                      <th className="px-3 py-2 text-left text-[var(--color-text-secondary)]">
                        {t("endpoints.probeTime")}
                      </th>
                      <th className="px-3 py-2 text-left text-[var(--color-text-secondary)]">
                        {t("common.status")}
                      </th>
                      <th className="px-3 py-2 text-left text-[var(--color-text-secondary)]">
                        {t("endpoints.latency")}
                      </th>
                      <th className="px-3 py-2 text-left text-[var(--color-text-secondary)]">
                        {t("endpoints.error")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {probeLogs.map((log) => (
                      <tr
                        key={log.id}
                        className="border-b border-[var(--color-border)] last:border-b-0"
                      >
                        <td className="px-3 py-2 text-[var(--color-text)]">
                          {new Date(log.createdAt).toLocaleString()}
                        </td>
                        <td className="px-3 py-2">
                          <Badge variant={log.status === "ok" ? "success" : "danger"}>
                            {log.status}
                          </Badge>
                        </td>
                        <td className="px-3 py-2 text-[var(--color-text)]">
                          {log.latency}ms
                        </td>
                        <td className="px-3 py-2 text-[var(--color-text-secondary)]">
                          {log.errorMessage || "-"}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          )}
        </Dialog>
      )}
    </div>
  );
}

export { EndpointsPage };
