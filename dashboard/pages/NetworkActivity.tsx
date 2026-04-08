import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { RefreshCw, Search, ExternalLink } from 'lucide-react';
import { api } from '../lib/api';
import { useClusterStore } from '../store/clusterStore';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { Pagination } from '../components/Pagination';
import { PageEmpty } from '../components/PageEmpty';
import { formatDateTime } from '../lib/display';
import type { NetworkActivityWorkloadRow, NetworkActivityConnectionRow } from '../types';

const PAGE_SIZES = [20, 50, 100, 200];
const SINCE_OPTIONS: { label: string; value: number | '' }[] = [
  { label: 'Mọi thời điểm', value: '' },
  { label: '15 phút', value: 15 },
  { label: '1 giờ', value: 60 },
  { label: '6 giờ', value: 360 },
  { label: '24 giờ', value: 1440 },
];

export const NetworkActivity: React.FC = () => {
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const [view, setView] = useState<'pods' | 'connections'>('pods');
  const [namespaceDraft, setNamespaceDraft] = useState('');
  const [searchDraft, setSearchDraft] = useState('');
  const [namespaceApplied, setNamespaceApplied] = useState('');
  const [searchApplied, setSearchApplied] = useState('');
  const [sinceMinutes, setSinceMinutes] = useState<number | ''>('');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(0);
  const [workloads, setWorkloads] = useState<NetworkActivityWorkloadRow[]>([]);
  const [connections, setConnections] = useState<NetworkActivityConnectionRow[]>([]);

  const applyFilters = useCallback(() => {
    setNamespaceApplied(namespaceDraft.trim());
    setSearchApplied(searchDraft.trim());
    setPage(1);
  }, [namespaceDraft, searchDraft]);

  const fetchData = useCallback(async () => {
    if (!selectedClusterId) {
      setWorkloads([]);
      setConnections([]);
      setTotal(0);
      return;
    }
    setLoading(true);
    try {
      const data = await api.getNetworkActivity({
        cluster: selectedClusterId,
        view,
        namespace: namespaceApplied || undefined,
        q: searchApplied || undefined,
        sinceMinutes: sinceMinutes === '' ? undefined : sinceMinutes,
        page,
        pageSize,
      });
      setTotal(data.total);
      if (data.view === 'pods') {
        setWorkloads((data.items as NetworkActivityWorkloadRow[]) ?? []);
        setConnections([]);
      } else {
        setConnections((data.items as NetworkActivityConnectionRow[]) ?? []);
        setWorkloads([]);
      }
    } catch {
      setTotal(0);
      setWorkloads([]);
      setConnections([]);
    } finally {
      setLoading(false);
    }
  }, [selectedClusterId, view, namespaceApplied, searchApplied, sinceMinutes, page, pageSize]);

  useEffect(() => {
    setPage(1);
  }, [view, selectedClusterId, sinceMinutes]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(fetchData, intervalMs, { refreshTrigger });

  const goPod = (podUid: string) => {
    navigate(`/resources/pods/uid/${encodeURIComponent(podUid)}`);
  };

  return (
    <PageLayout
      title="Network activity"
      description="Tổng hợp kết nối mạng từ agent (Pod Detail). Chọn cluster ở thanh trên cùng."
    >
      {!selectedClusterId ? (
        <PageEmpty
          title="Chưa chọn cluster"
          description="Chọn một cluster trong menu để xem network activity trên toàn workload."
          className="py-16"
        />
      ) : (
        <Card className="overflow-hidden">
          <div className="p-4 border-b border-border flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => setView('pods')}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                  view === 'pods' ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
                }`}
              >
                Theo workload (Pod)
              </button>
              <button
                type="button"
                onClick={() => setView('connections')}
                className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                  view === 'connections' ? 'bg-pink-600 text-white' : 'bg-slate-800 text-slate-300 hover:bg-slate-700'
                }`}
              >
                Tất cả kết nối
              </button>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button variant="secondary" size="sm" onClick={() => fetchData()} disabled={loading}>
                <RefreshCw className={`w-4 h-4 mr-1.5 ${loading ? 'animate-spin' : ''}`} />
                Làm mới
              </Button>
            </div>
          </div>

          <div className="p-4 border-b border-border flex flex-col sm:flex-row flex-wrap gap-3">
            <div className="flex-1 min-w-[140px]">
              <label className="block text-xs text-slate-500 mb-1">Namespace</label>
              <input
                type="text"
                value={namespaceDraft}
                onChange={(e) => setNamespaceDraft(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && applyFilters()}
                placeholder="Lọc namespace"
                className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200 placeholder:text-slate-600"
              />
            </div>
            <div className="flex-1 min-w-[180px]">
              <label className="block text-xs text-slate-500 mb-1">Tìm kiếm</label>
              <div className="relative">
                <Search className="absolute left-2.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-500" />
                <input
                  type="text"
                  value={searchDraft}
                  onChange={(e) => setSearchDraft(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && applyFilters()}
                  placeholder={view === 'pods' ? 'Tên pod, namespace…' : 'Pod, IP đích, cổng…'}
                  className="w-full bg-slate-900 border border-slate-700 rounded-lg pl-9 pr-3 py-2 text-sm text-slate-200 placeholder:text-slate-600"
                />
              </div>
            </div>
            <div className="w-full sm:w-44">
              <label className="block text-xs text-slate-500 mb-1">Thời gian quan sát</label>
              <select
                value={sinceMinutes === '' ? '' : String(sinceMinutes)}
                onChange={(e) => {
                  const v = e.target.value;
                  setSinceMinutes(v === '' ? '' : Number(v));
                  setPage(1);
                }}
                className="w-full bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-sm text-slate-200"
              >
                {SINCE_OPTIONS.map((o) => (
                  <option key={o.label} value={o.value === '' ? '' : String(o.value)}>
                    {o.label}
                  </option>
                ))}
              </select>
            </div>
            <div className="flex items-end">
              <Button size="sm" onClick={() => applyFilters()}>
                Áp dụng
              </Button>
            </div>
          </div>

          <div className="overflow-x-auto">
            {loading && total === 0 && workloads.length === 0 && connections.length === 0 ? (
              <p className="p-8 text-slate-500 text-sm text-center">Đang tải…</p>
            ) : view === 'pods' ? (
              workloads.length === 0 ? (
                <PageEmpty
                  title="Chưa có dữ liệu network"
                  description="Agent cần thu thập Pod Detail (host /proc hoặc exec ss). Kiểm tra agent và tab Network trên từng Pod."
                  className="py-12"
                />
              ) : (
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-slate-500">
                      <th className="px-4 py-3">Pod</th>
                      <th className="px-4 py-3">Namespace</th>
                      <th className="px-4 py-3">Owner</th>
                      <th className="px-4 py-3">Node</th>
                      <th className="px-4 py-3 text-right">Kết nối</th>
                      <th className="px-4 py-3">Cập nhật cuối</th>
                      <th className="px-4 py-3 text-right">Chi tiết</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {workloads.map((w) => (
                      <tr key={`${w.podUid}-${w.namespace}`} className="hover:bg-muted/30">
                        <td className="px-4 py-3 font-medium text-white">
                          {w.podName || <span className="text-slate-500 font-mono text-xs">{w.podUid.slice(0, 12)}…</span>}
                        </td>
                        <td className="px-4 py-3 text-slate-300">{w.namespace}</td>
                        <td className="px-4 py-3 text-slate-400 text-xs">
                          {w.ownerKind && w.ownerName ? `${w.ownerKind}/${w.ownerName}` : '—'}
                        </td>
                        <td className="px-4 py-3 text-slate-400 text-xs">{w.nodeName || '—'}</td>
                        <td className="px-4 py-3 text-right tabular-nums text-slate-200">{w.connectionCount}</td>
                        <td className="px-4 py-3 text-slate-500 text-xs whitespace-nowrap">
                          {w.lastObservedAt ? formatDateTime(w.lastObservedAt) : '—'}
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            type="button"
                            onClick={() => goPod(w.podUid)}
                            className="inline-flex items-center gap-1 text-pink-400 hover:text-pink-300 text-xs font-medium"
                          >
                            Pod <ExternalLink className="w-3 h-3" />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )
            ) : connections.length === 0 ? (
              <PageEmpty
                title="Chưa có kết nối"
                description="Không có bản ghi pod_network_connections trong khoảng lọc. Thu thập từ agent như trên Pod Detail."
                className="py-12"
              />
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-slate-500">
                    <th className="px-4 py-3">Pod</th>
                    <th className="px-4 py-3">NS</th>
                    <th className="px-4 py-3">Remote</th>
                    <th className="px-4 py-3">Proto</th>
                    <th className="px-4 py-3">State</th>
                    <th className="px-4 py-3 text-right">Tx/Rx Q</th>
                    <th className="px-4 py-3">Nguồn</th>
                    <th className="px-4 py-3">Time</th>
                    <th className="px-4 py-3 text-right">Pod</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {connections.map((c, i) => (
                    <tr key={c.id ?? i} className="hover:bg-muted/30">
                      <td className="px-4 py-3 text-slate-200 max-w-[10rem] truncate" title={c.podName}>
                        {c.podName || '—'}
                      </td>
                      <td className="px-4 py-3 text-slate-400 text-xs">{c.namespace}</td>
                      <td className="px-4 py-3 font-mono text-xs text-slate-300 whitespace-nowrap">
                        {c.destIp ?? '—'}:{c.destPort ?? 0}
                      </td>
                      <td className="px-4 py-3 text-xs">{c.protocol ?? '—'}</td>
                      <td className="px-4 py-3 text-xs text-slate-500 max-w-[6rem] truncate">{c.state ?? '—'}</td>
                      <td className="px-4 py-3 text-right font-mono text-xs text-slate-500">
                        {(c.bytesSent ?? 0)}/{(c.bytesRecv ?? 0)}
                      </td>
                      <td className="px-4 py-3 text-xs text-slate-500">{c.runtimeSource ?? '—'}</td>
                      <td className="px-4 py-3 text-xs text-slate-500 whitespace-nowrap">
                        {c.observedAt ? formatDateTime(c.observedAt) : '—'}
                      </td>
                      <td className="px-4 py-3 text-right">
                        {c.podUid ? (
                          <button
                            type="button"
                            onClick={() => goPod(c.podUid!)}
                            className="text-pink-400 hover:text-pink-300"
                            aria-label="Open pod"
                          >
                            <ExternalLink className="w-4 h-4 inline" />
                          </button>
                        ) : (
                          '—'
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          {total > 0 && (
            <Pagination
              page={page}
              pageSize={pageSize}
              total={total}
              onPageChange={setPage}
              onPageSizeChange={(n) => {
                setPageSize(n);
                setPage(1);
              }}
              pageSizeOptions={PAGE_SIZES}
              itemLabel={view === 'pods' ? 'workloads' : 'connections'}
            />
          )}
        </Card>
      )}
    </PageLayout>
  );
};
