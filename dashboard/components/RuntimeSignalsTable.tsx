import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { RuntimeSignal } from '../types';
import { useTimeWindowStore } from '../store/timeWindowStore';
import { Search, AlertTriangle, Clock, TrendingUp, Shield, ChevronDown, ChevronRight, Copy, Check } from 'lucide-react';
import { Pagination } from './Pagination';

interface RuntimeSignalsTableProps {
  podUid?: string;
  initialFilters?: {
    signalType?: string;
    category?: string;
    startDate?: string;
    endDate?: string;
  };
}

export const RuntimeSignalsTable: React.FC<RuntimeSignalsTableProps> = ({ 
  podUid, 
  initialFilters 
}) => {
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
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
        if (timeWindowMinutes > 0) {
          params.sinceMinutes = timeWindowMinutes;
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
      } catch (err) {
        console.error('Failed to fetch runtime signals:', err);
        setSignals([]);
        setTotal(0);
      } finally {
        setLoading(false);
      }
    };

    fetchSignals();
  }, [podUid, filters, currentPage, pageSize, timeWindowMinutes]);

  const getCategoryColor = (category: string) => {
    const colors: Record<string, string> = {
      'ESCAPE': 'bg-red-500/20 text-red-400 border-red-500/30',
      'FILESYSTEM': 'bg-orange-500/20 text-orange-400 border-orange-500/30',
      'KERNEL': 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
      'PROCESS': 'bg-amber-500/20 text-amber-400 border-amber-500/30',
      'IPC': 'bg-purple-500/20 text-purple-400 border-purple-500/30',
      'CREDENTIALS': 'bg-pink-500/20 text-pink-400 border-pink-500/30',
      'PERSISTENCE': 'bg-rose-500/20 text-rose-400 border-rose-500/30',
      'RBAC': 'bg-blue-500/20 text-blue-400 border-blue-500/30',
      'NETWORK': 'bg-cyan-500/20 text-cyan-400 border-cyan-500/30',
      'CONTROL_PLANE': 'bg-indigo-500/20 text-indigo-400 border-indigo-500/30',
    };
    return colors[category] || 'bg-slate-500/20 text-slate-400 border-slate-500/30';
  };

  const getConfidenceBadge = (confidence: number) => {
    if (confidence >= 0.8) {
      return 'bg-red-500/20 text-red-400 border-red-500/50';
    } else if (confidence >= 0.6) {
      return 'bg-orange-500/20 text-orange-400 border-orange-500/50';
    } else {
      return 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50';
    }
  };

  const filteredSignals = signals.filter(signal => {
    if (!filters.search) return true;
    const searchLower = filters.search.toLowerCase();
    return (
      signal.signalType.toLowerCase().includes(searchLower) ||
      signal.category.toLowerCase().includes(searchLower) ||
      signal.podUid.toLowerCase().includes(searchLower)
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
        <p className="text-sm text-muted">Loading runtime evidence...</p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <div className="flex-1 min-w-[200px] relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-slate-400" size={16} />
          <input
            type="text"
            placeholder="Search by signal, category, or pod UID..."
            value={filters.search}
            onChange={(e) => setFilters({ ...filters, search: e.target.value })}
            className="w-full pl-10 pr-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-pink-500/50"
          />
        </div>
        <select
          value={filters.signalType}
          onChange={(e) => setFilters({ ...filters, signalType: e.target.value })}
          className="px-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white focus:outline-none focus:border-pink-500/50"
        >
          <option value="">All Evidence Types</option>
          <option value="PROC_ROOT_PIVOT">PROC_ROOT_PIVOT</option>
          <option value="FS_ESCAPE_ATTEMPT">FS_ESCAPE_ATTEMPT</option>
          <option value="NAMESPACE_ESCAPE">NAMESPACE_ESCAPE</option>
          <option value="CAPABILITY_MISUSE">CAPABILITY_MISUSE</option>
        </select>
        <select
          value={filters.category}
          onChange={(e) => setFilters({ ...filters, category: e.target.value })}
          className="px-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white focus:outline-none focus:border-pink-500/50"
        >
          <option value="">All Evidence Categories</option>
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
          className="px-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white focus:outline-none focus:border-pink-500/50"
          placeholder="Start Date"
        />
        <input
          type="date"
          value={filters.endDate}
          onChange={(e) => setFilters({ ...filters, endDate: e.target.value })}
          className="px-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white focus:outline-none focus:border-pink-500/50"
          placeholder="End Date"
        />
        <select
          value={sortBy}
          onChange={(e) => setSortBy(e.target.value as typeof sortBy)}
          className="px-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white focus:outline-none focus:border-pink-500/50"
        >
          <option value="newest">Sort: Newest first</option>
          <option value="oldest">Sort: Oldest first</option>
          <option value="confidence_desc">Sort: Confidence high to low</option>
          <option value="confidence_asc">Sort: Confidence low to high</option>
          <option value="signal_asc">Sort: Event type A-Z</option>
        </select>
      </div>

      {/* Summary: total evidence and selected time window */}
      <div className="flex items-center text-sm text-slate-400">
        <span>
          {total} runtime event{total !== 1 ? 's' : ''}
          {timeWindowMinutes > 0 && <span className="ml-2 text-amber-500/80">(last {timeWindowMinutes}m)</span>}
        </span>
      </div>

      {/* Runtime evidence list */}
      {filteredSignals.length === 0 ? (
        <div className="text-center py-12 text-slate-500">
          <AlertTriangle size={24} className="mx-auto mb-2 opacity-50" />
          <p className="text-sm">No runtime evidence found.</p>
          <p className="text-xs mt-1 opacity-75">Runtime events appear here when agents detect suspicious behavior.</p>
        </div>
      ) : (
        <div className="space-y-3">
          {sortedSignals.map((signal) => (
            <div
              key={signal.id}
              className="bg-slate-900/70 border border-slate-800 rounded-lg p-4 hover:border-slate-700 transition-colors"
            >
              <div className="flex items-start justify-between mb-2">
                <div className="flex-1">
                  <div className="flex items-center gap-2 mb-2">
                    <AlertTriangle size={16} className="text-pink-400" />
                    <span className="font-semibold text-white text-sm">{signal.signalType}</span>
                    <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${getCategoryColor(signal.category)}`}>
                      {signal.category}
                    </span>
                    <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${getConfidenceBadge(signal.confidence)}`}>
                      {Math.round(signal.confidence * 100)}%
                    </span>
                  </div>
                  
                  <div className="flex items-center gap-4 text-xs text-slate-500">
                    <div className="flex items-center gap-1">
                      <Shield size={12} />
                      <span>Pod: {signal.podUid.substring(0, 8)}...</span>
                    </div>
                    <div className="flex items-center gap-1">
                      <Clock size={12} />
                      <span>{new Date(signal.createdAt).toLocaleString()}</span>
                    </div>
                  </div>
                </div>
              </div>

              {/* Evidence: expand/collapse + copy */}
              {signal.evidence && typeof signal.evidence === 'object' && Object.keys(signal.evidence).length > 0 && (() => {
                const evidenceStr = JSON.stringify(signal.evidence, null, 2);
                const isExpanded = expandedEvidence[signal.id];
                return (
                  <div className="mt-3 pt-3 border-t border-slate-800">
                    <div className="flex items-center justify-between gap-2 mb-2">
                      <button
                        type="button"
                        onClick={() => toggleEvidence(signal.id)}
                        className="flex items-center gap-1 text-xs font-semibold text-slate-400 hover:text-slate-300"
                      >
                        {isExpanded ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                        <TrendingUp size={12} />
                        Technical evidence
                      </button>
                      <button
                        type="button"
                        onClick={() => copyEvidence(signal.id, evidenceStr)}
                        className="flex items-center gap-1 px-2 py-1 text-xs text-muted hover:text-text bg-surface/50 border border-border rounded"
                        title="Copy JSON"
                      >
                        {copiedId === signal.id ? <Check size={12} /> : <Copy size={12} />}
                        {copiedId === signal.id ? 'Copied' : 'Copy'}
                      </button>
                    </div>
                    {isExpanded && (
                      <pre className="text-xs text-slate-500 font-mono bg-slate-950/50 p-3 rounded border border-slate-800 overflow-x-auto max-h-48 overflow-y-auto whitespace-pre-wrap break-words">
                        {evidenceStr}
                      </pre>
                    )}
                  </div>
                );
              })()}
            </div>
          ))}
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
