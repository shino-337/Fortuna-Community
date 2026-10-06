import React from 'react';
import clsx from 'clsx';
import { PageEmpty } from './PageStatus';
import { UI_TABLE, UI_TD, UI_TH, UI_TR, UI_THEAD_STICKY } from '../../lib/tableChrome';

export type TableColumn<T> = {
  key: string;
  header: React.ReactNode;
  className?: string;
  headerClassName?: string;
  cell: (row: T) => React.ReactNode;
};

export interface TableProps<T> {
  columns: TableColumn<T>[];
  data: T[];
  loading?: boolean;
  emptyTitle?: string;
  emptyDescription?: string;
  rowKey: (row: T) => string;
  className?: string;
  scrollClassName?: string;
  /** Makes each row a button (click, Enter or Space). Links and buttons inside a cell keep their own action. */
  onRowClick?: (row: T) => void;
  isRowSelected?: (row: T) => boolean;
}

export function Table<T>({
  columns,
  data,
  loading = false,
  emptyTitle = 'No data',
  emptyDescription,
  rowKey,
  className = '',
  scrollClassName = 'ui-table-scroll',
  onRowClick,
  isRowSelected,
}: TableProps<T>): React.ReactElement {
  return (
    <div className={clsx('relative min-w-0', className)}>
      {loading ? (
        <div className="absolute inset-0 z-overlay flex items-center justify-center rounded-lg bg-base/40 backdrop-blur-[1px]">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-brand border-t-transparent motion-reduce:animate-none" />
        </div>
      ) : null}
      <div className={scrollClassName}>
        <table className={UI_TABLE}>
          <thead className={UI_THEAD_STICKY}>
            <tr>
              {columns.map((col) => (
                <th key={col.key} className={clsx(UI_TH, col.headerClassName)}>
                  {col.header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {!loading && data.length === 0 ? (
              <tr>
                <td colSpan={columns.length} className="p-0">
                  <PageEmpty title={emptyTitle} description={emptyDescription} variant="compact" className="py-10" />
                </td>
              </tr>
            ) : null}
            {!loading &&
              data.map((row) => (
                <tr
                  key={rowKey(row)}
                  className={clsx(
                    UI_TR,
                    onRowClick && 'cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand/70',
                    isRowSelected?.(row) && 'bg-brand/10',
                  )}
                  {...(onRowClick
                    ? {
                        tabIndex: 0,
                        'aria-current': isRowSelected?.(row) ? ('true' as const) : undefined,
                        onClick: (e: React.MouseEvent<HTMLTableRowElement>) => {
                          if ((e.target as HTMLElement).closest('a, button, input, select, label')) return;
                          onRowClick(row);
                        },
                        onKeyDown: (e: React.KeyboardEvent<HTMLTableRowElement>) => {
                          if (e.target !== e.currentTarget || (e.key !== 'Enter' && e.key !== ' ')) return;
                          e.preventDefault();
                          onRowClick(row);
                        },
                      }
                    : {})}
                >
                  {columns.map((col) => (
                    <td key={col.key} className={clsx(UI_TD, col.className)}>
                      {col.cell(row)}
                    </td>
                  ))}
                </tr>
              ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
