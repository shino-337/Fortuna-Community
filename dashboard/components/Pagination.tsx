import React from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';

export interface PaginationProps {
  /** Current 1-based page */
  page: number;
  /** Items per page */
  pageSize: number;
  /** Total number of items */
  total: number;
  /** Called when page changes (1-based) */
  onPageChange: (page: number) => void;
  /** Optional: change page size (if not provided, size selector is hidden) */
  onPageSizeChange?: (size: number) => void;
  /** Page size options (e.g. [10, 20, 50, 100]) */
  pageSizeOptions?: number[];
  /** Optional label for "items" (e.g. "clusters", "pods") */
  itemLabel?: string;
  className?: string;
}

/** Pagination control — page navigation and items per page selector. */
export const Pagination: React.FC<PaginationProps> = ({
  page,
  pageSize,
  total,
  onPageChange,
  onPageSizeChange,
  pageSizeOptions = [10, 20, 50, 100],
  itemLabel = 'items',
  className = '',
}) => {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(page * pageSize, total);

  return (
    <div
      className={`flex flex-col sm:flex-row items-center justify-between gap-3 px-4 py-3 border-t border-border bg-base/30 rounded-b-lg ${className}`}
      role="navigation"
      aria-label="Pagination"
    >
      <div className="flex items-center gap-4 text-body text-muted order-2 sm:order-1">
        <span>
          Showing <span className="font-medium text-text">{start}</span>
          {' – '}
          <span className="font-medium text-text">{end}</span>
          {' of '}
          <span className="font-medium text-text">{total}</span>
          {' '}{itemLabel}
        </span>
        {onPageSizeChange && (
          <div className="flex items-center gap-2">
            <label htmlFor="pagination-size" className="text-muted-2 text-caption">
              Per page
            </label>
            <select
              id="pagination-size"
              value={pageSize}
              onChange={(e) => onPageSizeChange(Number(e.target.value))}
              className="bg-surface border border-border rounded-md px-2 py-1 text-body text-text focus:ring-1 focus:ring-brand focus:border-brand"
            >
              {pageSizeOptions.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      <div className="flex items-center gap-1 order-1 sm:order-2">
        <button
          type="button"
          onClick={() => onPageChange(page - 1)}
          disabled={page <= 1}
          className="min-h-10 min-w-10 rounded-lg border border-border p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-muted sm:min-h-9 sm:min-w-9"
          aria-label="Previous page"
        >
          <ChevronLeft className="w-5 h-5" />
        </button>
        <span className="px-3 py-1 text-body text-text min-w-[6rem] text-center">
          Page <span className="font-medium">{page}</span> of <span className="font-medium">{totalPages}</span>
        </span>
        <button
          type="button"
          onClick={() => onPageChange(page + 1)}
          disabled={page >= totalPages}
          className="min-h-10 min-w-10 rounded-lg border border-border p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:bg-transparent disabled:hover:text-muted sm:min-h-9 sm:min-w-9"
          aria-label="Next page"
        >
          <ChevronRight className="w-5 h-5" />
        </button>
      </div>
    </div>
  );
};
