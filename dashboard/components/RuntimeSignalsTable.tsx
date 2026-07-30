import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { RuntimeSignal } from '../types';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { Search, AlertTriangle, Clock, TrendingUp, Shield, ChevronDown, ChevronRight, Copy, Check } from 'lucide-react';
import { Pagination } from './Pagination';
import { runtimeSignalVisual } from '../lib/runtimeSignalVisual';
import { formatMinutesHuman } from '../lib/formatDuration';
import { UI_TABLE, UI_THEAD_STICKY, UI_TH_COMPACT, UI_TR, UI_TD_COMPACT_TIGHT } from '../lib/tableChrome';

interface RuntimeSignalsTableProps {
  podUid?: string;
  initialFilters?: {
    signalType?: string;
    category?: string;
    startDate?: string;
    endDate?: string;
  };
  /**
   * When set (including `null`), API time window follows Risk Operations scope instead of the global header picker.
   * `null` = all time (no sinceMinutes). `number` &gt; 0 = that many minutes. Omit prop = legacy: use header store only.
   */
  riskAlignedSinceMinutes?: number | null;
}

/** Table of runtime evidence signals — filterable by type/category/time and paginated. */
export const RuntimeSignalsTable: React.FC<RuntimeSignalsTableProps> = ({
  podUid,
  initialFilters,
  riskAlignedSinceMinutes,
}) => {
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
  const sinceMinutesForQuery =
    riskAlignedSinceMinutes !== undefined
      ? riskAlignedSinceMinutes != null && riskAlignedSinceMinutes > 0
        ? riskAlignedSinceMinutes
        : undefined
      : timeWindowMinutes > 0
        ? timeWindowMinutes
        : undefined;
  const [signals, setSignals] = useState<RuntimeSignal[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [filters, setFilters] = useState({
    signalType: initialFilters?.signalType || '',
    category: initialFilters?.category || '',
    startDate: initialFilters?.startDate || '',
    endDate: initialFilters?.endDate || '',
    search: '',
  });
  const [currentPage, setCurrentPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);
  const [expandedEvidence, setExpandedEvidence] = useState<Record<number, boolean>>({});
  const [copiedId, setCopiedId] = useState<number | null>(null);
  const [sortBy, setSortBy] = useState<'newest' | 'oldest' | 'confidence_desc' | 'confidence_asc' | 'signal_asc'>('newest');

  useEffect(() => {
    const fetchSignals = async () => {
      try {
        setLoading(true);
        const params: Record<string, string | number> = {
          limit: pageSize,
          offset: (currentPage - 1) * pageSize,
        };
        if (sinceMinutesForQuery != null && sinceMinutesForQuery > 0) {
          params.sinceMinutes = sinceMinutesForQuery;
        }
        if (podUid) {
          params.podUid = podUid;
        }
        if (filters.signalType) {
          params.signalType = filters.signalType;
        }
        if (filters.category) {
          params.category = filters.category;
        }
        if (filters.startDate) {
          params.startDate = filters.startDate;
        }
        if (filters.endDate) {
          params.endDate = filters.endDate;
        }

        const data = await api.getRuntimeSignals(params);
        setSignals(data.signals);
        setTotal(data.total);
      } catch {
        setSignals([]);
        setTotal(0);
      } finally {
        setLoading(false);
      }
    };

    fetchSignals();
  }, [podUid, filters, currentPage, pageSize, sinceMinutesForQuery]);

  const getCategoryColor = (category: string | undefined) => {
    const colors: Record<string, string> = {
      'ESCAPE': 'bg-red-500/20 text-red-400 border-red-500/30',
      'FILESYSTEM': 'bg-orange-500/20 text-orange-400 border-orange-500/30',
      'KERNEL': 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
      'PROCESS': 'bg-amber-500/20 text-amber-400 border-amber-500/30',
      'IPC': 'bg-purple-500/20 text-purple-400 border-purple-500/30',
      'CREDENTIALS': 'bg-brand/20 text-brand border-brand/30',
      'PERSISTENCE': 'bg-rose-500/20 text-rose-400 border-rose-500/30',
      'RBAC': 'bg-blue-500/20 text-blue-400 border-blue-500/30',
      'NETWORK': 'bg-cyan-500/20 text-cyan-400 border-cyan-500/30',
      'CONTROL_PLANE': 'bg-indigo-500/20 text-indigo-400 border-indigo-500/30',
    };
    const key = (category ?? '').trim().toUpperCase();
    return colors[key] || 'bg-muted/20 text-muted border-border/40';
  };

  const getConfidenceBadge = (confidence: number) => {
    const c = Number.isFinite(confidence) ? confidence : 0;
    if (c >= 0.8) {
      return 'bg-red-500/20 text-red-400 border-red-500/50';
    } else if (c >= 0.6) {
      return 'bg-orange-500/20 text-orange-400 border-orange-500/50';
    } else {
      return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50';
    }
  };

  const getSignalBadge = (signalType: string) => {
    const v = runtimeSignalVisual(signalType);
    return {
      className: v.signalClass,
      severity: v.severity,
      severityClass: v.severityClass,
    };
  };

  const filteredSignals = signals.filter((signal) => {
    if (!filters.search) return true;
    const searchLower = filters.search.toLowerCase();
    return (
      (signal.signalType || '').toLowerCase().includes(searchLower) ||
      (signal.category || '').toLowerCase().includes(searchLower) ||
      (signal.podUid || '').toLowerCase().includes(searchLower)
    );
  });

  const sortedSignals = [...filteredSignals].sort((a, b) => {
    switch (sortBy) {
      case 'oldest':
        return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
      case 'confidence_desc':
        return (b.confidence ?? 0) - (a.confidence ?? 0);
      case 'confidence_asc':
        return (a.confidence ?? 0) - (b.confidence ?? 0);
      case 'signal_asc':
        return (a.signalType || '').localeCompare(b.signalType || '');
      case 'newest':
      default:
        return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
    }
  });

  const toggleEvidence = (id: number) => setExpandedEvidence((prev) => ({ ...prev, [id]: !prev[id] }));
  const copyEvidence = (id: number, text: string) => {
    navigator.clipboard.writeText(text).then(() => {
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    });
  };

  if (loading) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-12">
        <div className="w-10 h-10 border-4 border-brand border-t-transparent rounded-full animate-spin" />
        <p className="text-caption text-muted">Loading runtime evidence...</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Filters */}
      <div className="flex flex-wrap gap-2">
        <div className="flex-1 min-w-[200px] relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-muted" size={16} />
          <input
            type="text"
            placeholder="Search signal, category, or pod UID"
            value={filters.search}
            onChange={(e) => setFilters({ ...filters, search: e.target.value })}
            className="w-full rounded-lg border border-border bg-surface/50 py-2 pl-9 pr-3 text-caption text-text placeholder:text-muted-2 focus:outline-none focus:border-brand/50"
          />
        </div>
        <select
          value={filters.signalType}
          onChange={(e) => setFilters({ ...filters, signalType: e.target.value })}
          className="rounded-lg border border-border bg-surface/50 px-3 py-2 text-caption text-text focus:outline-none focus:border-brand/50"
        >
          <option value="">All event types</option>
          <option value="PROC_ROOT_PIVOT">PROC_ROOT_PIVOT</option>
          <option value="FS_ESCAPE_ATTEMPT">FS_ESCAPE_ATTEMPT</option>
          <option value="NAMESPACE_ESCAPE">NAMESPACE_ESCAPE</option>
          <option value="CAPABILITY_MISUSE">CAPABILITY_MISUSE</option>
        </select>
        <select
          value={filters.category}
          onChange={(e) => setFilters({ ...filters, category: e.target.value })}
          className="rounded-lg border border-border bg-surface/50 px-3 py-2 text-caption text-text focus:outline-none focus:border-brand/50"
        >
          <option value="">All categories</option>
          <option value="ESCAPE">ESCAPE</option>
          <option value="FILESYSTEM">FILESYSTEM</option>
          <option value="KERNEL">KERNEL</option>
          <option value="PROCESS">PROCESS</option>
          <option value="IPC">IPC</option>
          <option value="CREDENTIALS">CREDENTIALS</option>
          <option value="RBAC">RBAC</option>
        </select>
        <input
          type="date"
          value={filters.startDate}
          onChange={(e) => setFilters({ ...filters, startDate: e.target.value })}
          className="rounded-lg border border-border bg-surface/50 px-3 py-2 text-caption text-text focus:outline-none focus:border-brand/50"
          placeholder="Start Date"
        />
        <input
          type="date"
          value={filters.endDate}
          onChange={(e) => setFilters({ ...filters, endDate: e.target.value })}
          className="rounded-lg border border-border bg-surface/50 px-3 py-2 text-caption text-text focus:outline-none focus:border-brand/50"
          placeholder="End Date"
        />
        <select
          value={sortBy}
          onChange={(e) => setSortBy(e.target.value as typeof sortBy)}
          className="rounded-lg border border-border bg-surface/50 px-3 py-2 text-caption text-text focus:outline-none focus:border-brand/50"
        >
          <option value="newest">Sort: Newest first</option>
          <option value="oldest">Sort: Oldest first</option>
          <option value="confidence_desc">Sort: Confidence high to low</option>
          <option value="confidence_asc">Sort: Confidence low to high</option>
          <option value="signal_asc">Sort: Event type A-Z</option>
        </select>
      </div>

      {/* Summary: total evidence and selected time window */}
      <div className="flex items-center text-caption text-muted">
        <span>
          {total} runtime event{total !== 1 ? 's' : ''}
          <span className="ml-2 text-amber-500/80">({formatMinutesHuman(sinceMinutesForQuery)})</span>
        </span>
      </div>

      {/* Runtime evidence list */}
      {filteredSignals.length === 0 ? (
        <div className="rounded-lg border border-border bg-base/25 py-10 text-center text-muted">
          <AlertTriangle size={24} className="mx-auto mb-2 opacity-50" />
          <p className="text-body text-text">No runtime evidence found</p>
          <p className="text-caption mt-1 opacity-75">Runtime events appear here when agents detect suspicious behavior.</p>
        </div>
      ) : (
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className={UI_TABLE}>
            <thead className={UI_THEAD_STICKY}>
              <tr>
                <th className={UI_TH_COMPACT}>Event</th>
                <th className={`${UI_TH_COMPACT} w-32`}>Category</th>
                <th className={UI_TH_COMPACT}>Pod</th>
                <th className={`${UI_TH_COMPACT} w-24`}>Confidence</th>
                <th className={`${UI_TH_COMPACT} w-44`}>Detected</th>
                <th className={`${UI_TH_COMPACT} text-right w-28`}>Evidence</th>
              </tr>
            </thead>
            <tbody>
          {sortedSignals.map((signal) => {
            const conf = Number(signal.confidence);
            const confNorm = Number.isFinite(conf) ? Math.min(1, Math.max(0, conf)) : 0;
            const podUid = signal.podUid?.trim() || '';
            const podLabel =
              podUid.length === 0 ? '—' : podUid.length <= 12 ? podUid : `${podUid.slice(0, 8)}…`;
            let createdLabel = '—';
            try {
              const t = signal.createdAt ? new Date(signal.createdAt).getTime() : NaN;
              createdLabel = Number.isFinite(t) ? new Date(signal.createdAt).toLocaleString() : String(signal.createdAt ?? '—');
            } catch {
              createdLabel = String(signal.createdAt ?? '—');
            }
            const evidenceRaw = signal.evidence;
            const hasObjectEvidence =
              evidenceRaw != null &&
              typeof evidenceRaw === 'object' &&
              !Array.isArray(evidenceRaw) &&
              Object.keys(evidenceRaw as object).length > 0;
            const hasStringEvidence =
              typeof evidenceRaw === 'string' && evidenceRaw.trim().length > 0;
            const evidenceStr = hasObjectEvidence
              ? JSON.stringify(evidenceRaw, null, 2)
              : hasStringEvidence
                ? evidenceRaw.trim()
                : '';
            const visual = getSignalBadge(signal.signalType || '');
            const isExpanded = expandedEvidence[signal.id];

            return (
              <React.Fragment key={signal.id}>
                <tr className={UI_TR}>
                  <td className={UI_TD_COMPACT_TIGHT}>
                    <div className="min-w-[14rem]">
                      <div className="flex items-center gap-2">
                        <AlertTriangle size={14} className="text-brand shrink-0" />
                        <span className={`max-w-[22rem] truncate rounded border px-2 py-0.5 text-caption font-semibold ${visual.className}`} title={signal.signalType || undefined}>
                          {signal.signalType || '—'}
                        </span>
                      </div>
                      <div className="mt-1">
                        <span className={`rounded border px-1.5 py-0.5 text-micro font-semibold uppercase ${visual.severityClass}`}>
                          {visual.severity}
                        </span>
                      </div>
                    </div>
                  </td>
                  <td className={UI_TD_COMPACT_TIGHT}>
                    <span className={`rounded border px-2 py-0.5 text-caption font-medium ${getCategoryColor(signal.category)}`}>
                      {signal.category || '—'}
                    </span>
                  </td>
                  <td className={`${UI_TD_COMPACT_TIGHT} text-caption text-muted`}>
                    <div className="flex min-w-[10rem] items-center gap-1">
                      <Shield size={12} className="shrink-0" />
                      <span className="truncate font-mono" title={podUid || undefined}>{podLabel}</span>
                    </div>
                  </td>
                  <td className={UI_TD_COMPACT_TIGHT}>
                    <span className={`rounded border px-2 py-0.5 text-caption font-medium ${getConfidenceBadge(confNorm)}`}>
                      {Math.round(confNorm * 100)}%
                    </span>
                  </td>
                  <td className={`${UI_TD_COMPACT_TIGHT} text-caption text-muted whitespace-nowrap`}>
                    <span className="inline-flex items-center gap-1">
                      <Clock size={12} />
                      {createdLabel}
                    </span>
                  </td>
                  <td className={`${UI_TD_COMPACT_TIGHT} text-right`}>
                    {evidenceStr ? (
                      <button
                        type="button"
                        onClick={() => toggleEvidence(signal.id)}
                        className="inline-flex items-center gap-1 rounded border border-border bg-surface/50 px-2 py-1 text-caption text-muted hover:text-text"
                      >
                        {isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                        Payload
                      </button>
                    ) : (
                      <span className="text-caption text-muted">—</span>
                    )}
                  </td>
                </tr>
                {isExpanded && evidenceStr ? (
                  <tr className="border-b border-border/60 bg-base/30">
                    <td colSpan={6} className="px-3 py-3 sm:px-4">
                      <div className="mb-2 flex items-center justify-between gap-2">
                        <span className="inline-flex items-center gap-1 text-caption font-semibold text-muted">
                          <TrendingUp size={12} />
                          Technical evidence payload
                        </span>
                      <button
                        type="button"
                        onClick={() => copyEvidence(signal.id, evidenceStr)}
                        className="inline-flex items-center gap-1 rounded border border-border bg-surface/50 px-2 py-1 text-caption text-muted hover:text-text"
                        title="Copy payload"
                      >
                        {copiedId === signal.id ? <Check size={12} /> : <Copy size={12} />}
                        {copiedId === signal.id ? 'Copied' : 'Copy'}
                      </button>
                      </div>
                      <pre className="max-h-48 overflow-y-auto overscroll-contain rounded border border-border bg-base/50 p-3 font-mono text-caption text-muted whitespace-pre-wrap break-words">
                        {evidenceStr}
                      </pre>
                    </td>
                  </tr>
                ) : null}
              </React.Fragment>
            );
          })}
            </tbody>
          </table>
        </div>
      )}

      {/* Pagination: use shared component for consistency */}
      {total > 0 && (
        <Pagination
          page={currentPage}
          pageSize={pageSize}
          total={total}
          onPageChange={setCurrentPage}
          onPageSizeChange={(size) => { setPageSize(size); setCurrentPage(1); }}
          pageSizeOptions={[10, 20, 50]}
          itemLabel="events"
        />
      )}
    </div>
  );
};
