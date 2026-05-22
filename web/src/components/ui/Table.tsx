import type { ReactNode } from "react";
import { useTranslation } from "react-i18next";
import { cn } from "@/lib/utils";

interface Column<T> {
  key: string;
  header: string;
  render?: (row: T) => ReactNode;
  sortable?: boolean;
  className?: string;
}

interface TableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyExtractor: (row: T) => string;
  onSort?: (key: string, direction: "asc" | "desc") => void;
  sortKey?: string;
  sortDirection?: "asc" | "desc";
  page?: number;
  pageSize?: number;
  total?: number;
  onPageChange?: (page: number) => void;
  className?: string;
}

function Table<T>({
  columns,
  data,
  keyExtractor,
  onSort,
  sortKey,
  sortDirection,
  page,
  pageSize,
  total,
  onPageChange,
  className,
}: TableProps<T>) {
  const { t } = useTranslation();

  const handleSort = (key: string) => {
    if (!onSort) return;
    const newDirection =
      sortKey === key && sortDirection === "asc" ? "desc" : "asc";
    onSort(key, newDirection);
  };

  const totalPages =
    total !== undefined && pageSize ? Math.ceil(total / pageSize) : 0;

  return (
    <div className={cn("w-full", className)}>
      <div className="overflow-x-auto rounded-lg border border-[var(--color-border)]">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[var(--color-border)] bg-[var(--color-bg-secondary)]">
              {columns.map((col) => (
                <th
                  key={col.key}
                  className={cn(
                    "px-4 py-3 text-left font-medium text-[var(--color-text-secondary)]",
                    col.sortable && "cursor-pointer select-none hover:text-[var(--color-text)]",
                    col.className,
                  )}
                  onClick={col.sortable ? () => handleSort(col.key) : undefined}
                >
                  <span className="inline-flex items-center gap-1">
                    {col.header}
                    {col.sortable && sortKey === col.key && (
                      <span className="text-xs">
                        {sortDirection === "asc" ? " ^" : " v"}
                      </span>
                    )}
                  </span>
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(!Array.isArray(data) || data.length === 0) ? (
              <tr>
                <td
                  colSpan={columns.length}
                  className="px-4 py-8 text-center text-[var(--color-text-tertiary)]"
                >
                  {t("common.noData")}
                </td>
              </tr>
            ) : (
              (Array.isArray(data) ? data : []).map((row) => (
                <tr
                  key={keyExtractor(row)}
                  className="border-b border-[var(--color-border)] last:border-b-0 hover:bg-[var(--color-bg-secondary)] transition-colors"
                >
                  {columns.map((col) => (
                    <td
                      key={col.key}
                      className={cn(
                        "px-4 py-3 text-[var(--color-text)]",
                        col.className,
                      )}
                    >
                      {col.render
                        ? col.render(row)
                        : String((row as Record<string, unknown>)[col.key] ?? "")}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {total !== undefined && pageSize && onPageChange && page !== undefined && (
        <div className="mt-4 flex items-center justify-between text-sm text-[var(--color-text-secondary)]">
          <span>
            {t("common.total")}: {total}
          </span>
          <div className="flex items-center gap-2">
            <button
              disabled={page <= 1}
              onClick={() => onPageChange(page - 1)}
              className="rounded px-3 py-1 hover:bg-[var(--color-bg-tertiary)] disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {t("common.previous")}
            </button>
            <span>
              {page} / {totalPages}
            </span>
            <button
              disabled={page >= totalPages}
              onClick={() => onPageChange(page + 1)}
              className="rounded px-3 py-1 hover:bg-[var(--color-bg-tertiary)] disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {t("common.next")}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

export { Table, type Column, type TableProps };
