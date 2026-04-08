import React, { useState, useEffect, useCallback, useMemo } from 'react';
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
import { formatDateTime, formatBucketClock } from '../lib/display';
import type { NetworkActivityWorkloadRow, NetworkActivityConnectionRow } from '../types';

const PAGE_SIZES = [20, 50, 100, 200];

/** Thứ tự: mặc định 24h trước; “mọi thời điểm” cuối cùng (debug). */
const SINCE_OPTIONS: { label: string; value: number | '' }[] = [
  { label: '15 phút', value: 15 },
  { label: '1 giờ', value: 60 },
  { label: '6 giờ', value: 360 },
  { label: '24 giờ', value: 1440 },
  { label: 'Mọi thời điểm (debug — có thể chậm)', value: '' },
];

function sinceRangeHuman(sinceMinutes: number | ''): string {
  if (sinceMinutes === '') return 'Mọi thời điểm';
  switch (sinceMinutes) {
    case 15:
      return '15 phút';
    case 60:
      return '1 giờ';
    case 360:
      return '6 giờ';
    case 1440:
      return '24 giờ';
    default:
      return `${sinceMinutes} phút`;
  }
}

/** q đã apply chỉ là số cổng 1–65535 (khớp tối ưu backend). */
function parseAppliedPort(q: string): number | null {
  const t = q.trim();
  if (!/^\d+$/.test(t)) return null;
  const n = parseInt(t, 10);
  if (n < 1 || n > 65535) return null;
  return n;
}

function formatRemoteCell(c: NetworkActivityConnectionRow): string {
  const st = (c.state || '').toUpperCase();
  const port = c.destPort ?? 0;
  const ip = (c.destIp || '').trim();
  if (st === 'LISTEN' || st.includes('LISTEN')) {
    return `LISTEN :${port}`;
  }
  return `${ip || '—'}:${port}`;
}

function podDisplayName(podName?: string, podUid?: string) {
  const name = podName?.trim();
  if (name) return name;
  if (podUid) {
    return <span className="text-slate-500 font-mono text-xs">{podUid.slice(0, 12)}…</span>;
  }
  return '—';
}

const OBS_BUCKET_HEADER_TITLE =
  'Số bản ghi trong khoảng lọc (mỗi hàng = một chữ ký kết nối trong một bucket 5 phút UTC, snapshot cuối trong bucket). Không phải số kết nối duy nhất.';

const TX_RX_HEADER_TITLE =
  'Queue bytes đọc từ /proc/net tại thời điểm quan sát (snapshot), không phải tổng lưu lượng tích lũy.';

const BUCKET_COL_TITLE = 'Mốc bucket 5 phút (UTC). Các hàng cùng bucket có thể trùng “Time” gần nhau — đây là bình thường.';

export function NetworkActivity() {
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const [view, setView] = useState('pods' as 'pods' | 'connections');
  const [namespaceDraft, setNamespaceDraft] = useState('');
  const [searchDraft, setSearchDraft] = useState('');
  const [namespaceApplied, setNamespaceApplied] = useState('');
  const [searchApplied, setSearchApplied] = useState('');
  const [sinceMinutes, setSinceMinutes] = useState(1440 as number | '');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(50);
  const [loading, setLoading] = useState(false);
  const [refreshSpin, setRefreshSpin] = useState(false);
  const [total, setTotal] = useState(0);
  const [workloads, setWorkloads] = useState([] as NetworkActivityWorkloadRow[]);
  const [connections, setConnections] = useState([] as NetworkActivityConnectionRow[]);

  const appliedPort = useMemo(() => parseAppliedPort(searchApplied), [searchApplied]);
  const hasTextFilters = Boolean(namespaceApplied || searchApplied);
  const sinceHuman = useMemo(() => sinceRangeHuman(sinceMinutes), [sinceMinutes]);

  const applyFilters = useCallback(() => {
    setNamespaceApplied(namespaceDraft.trim());
    setSearchApplied(searchDraft.trim());
    setPage(1);
  }, [namespaceDraft, searchDraft]);

  const clearTextFilters = useCallback(() => {
    setNamespaceDraft('');
    setSearchDraft('');
    setNamespaceApplied('');
    setSearchApplied('');
    setPage(1);
  }, []);

  const fetchData = useCallback(
    async (manual = false) => {
      if (!selectedClusterId) {
        setWorkloads([]);
        setConnections([]);
        setTotal(0);
        return;
      }
      if (manual) setRefreshSpin(true);
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
        if (manual) setRefreshSpin(false);
      }
    },
    [selectedClusterId, view, namespaceApplied, searchApplied, sinceMinutes, page, pageSize]
  );

  useEffect(() => {
    setPage(1);
  }, [view, selectedClusterId, sinceMinutes]);

  useEffect(() => {
    void fetchData(false);
  }, [fetchData]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(() => void fetchData(false), intervalMs, { refreshTrigger });

  const goPod = (podUid: string) => {
    navigate(`/resources/pods/uid/${encodeURIComponent(podUid)}`);
  };

  const emptyContextLine = `Khoảng thời gian: ${sinceHuman}.`;

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
              <Button
                variant="secondary"
                size="sm"
                onClick={() => void fetchData(true)}
                disabled={loading}
              >
                <RefreshCw className={`w-4 h-4 mr-1.5 ${refreshSpin ? 'animate-spin' : ''}`} />
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
              {view === 'connections' && (
                <p className="mt-1 text-[11px] text-slate-600">
                  Gợi ý: chỉ nhập số (ví dụ <span className="font-mono text-slate-500">443</span>) rồi Áp dụng để
                  lọc nhanh theo cổng đích hoặc nguồn.
                </p>
              )}
            </div>
            <div className="w-full sm:w-52">
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
              {sinceMinutes === '' && (
                <p className="mt-1 text-[11px] text-amber-600/90">
                  Không lọc theo thời gian — truy vấn có thể nặng hơn; dùng khi debug.
                </p>
              )}
            </div>
            <div className="flex flex-col justify-end gap-2">
              <Button size="sm" onClick={() => applyFilters()}>
                Áp dụng
              </Button>
            </div>
          </div>

          {appliedPort != null && view === 'connections' && (
            <div className="px-4 pb-2 flex flex-wrap gap-2 items-center text-xs">
              <span className="inline-flex items-center rounded-full bg-slate-800 text-slate-300 px-2.5 py-0.5 border border-slate-600">
                Port = {appliedPort}
              </span>
              <span className="text-slate-600">Lọc cổng (index-friendly)</span>
            </div>
          )}

          <div className="overflow-x-auto">
            {loading && total === 0 && workloads.length === 0 && connections.length === 0 ? (
              <p className="p-8 text-slate-500 text-sm text-center">Đang tải…</p>
            ) : view === 'pods' ? (
              workloads.length === 0 ? (
                <div className="py-8">
                  <PageEmpty
                    title="Chưa có dữ liệu network"
                    description={`Agent cần thu thập Pod Detail (host /proc hoặc exec ss). Kiểm tra agent và tab Network trên từng Pod. ${emptyContextLine}`}
                    className="py-8"
                  />
                  {hasTextFilters && (
                    <div className="flex justify-center">
                      <Button variant="secondary" size="sm" onClick={clearTextFilters}>
                        Xóa lọc namespace / tìm kiếm
                      </Button>
                    </div>
                  )}
                </div>
              ) : (
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-slate-500">
                      <th className="px-4 py-3">Pod</th>
                      <th className="px-4 py-3">Namespace</th>
                      <th className="px-4 py-3">Owner</th>
                      <th className="px-4 py-3">Node</th>
                      <th className="px-4 py-3 text-right">
                        <span title={OBS_BUCKET_HEADER_TITLE} className="cursor-help border-b border-dotted border-slate-500">
                          Quan sát (bucket)
                        </span>
                      </th>
                      <th className="px-4 py-3">Cập nhật cuối</th>
                      <th className="px-4 py-3 text-right">Chi tiết</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {workloads.map((w) => (
                      <tr key={`${w.podUid}-${w.namespace}`} className="hover:bg-muted/30">
                        <td className="px-4 py-3 font-medium text-white">
                          {w.podName || (
                            <span className="text-slate-500 font-mono text-xs">{w.podUid.slice(0, 12)}…</span>
                          )}
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
              <div className="py-8">
                <PageEmpty
                  title="Chưa có kết nối"
                  description={`Không có bản ghi pod_network_connections trong khoảng lọc. Thu thập từ agent như trên Pod Detail. ${emptyContextLine}`}
                  className="py-8"
                />
                {hasTextFilters && (
                  <div className="flex justify-center">
                    <Button variant="secondary" size="sm" onClick={clearTextFilters}>
                      Xóa lọc namespace / tìm kiếm
                    </Button>
                  </div>
                )}
              </div>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border text-left text-xs uppercase tracking-wide text-slate-500">
                    <th className="px-4 py-3">Pod</th>
                    <th className="px-4 py-3">NS</th>
                    <th className="px-4 py-3">Remote</th>
                    <th className="px-4 py-3">Proto</th>
                    <th className="px-4 py-3">State</th>
                    <th className="px-4 py-3">
                      <span title={BUCKET_COL_TITLE} className="cursor-help border-b border-dotted border-slate-500">
                        Bucket
                      </span>
                    </th>
                    <th className="px-4 py-3 text-right">
                      <span title={TX_RX_HEADER_TITLE} className="cursor-help border-b border-dotted border-slate-500">
                        Tx/Rx queue
                      </span>
                    </th>
                    <th className="px-4 py-3">Nguồn</th>
                    <th className="px-4 py-3">Time</th>
                    <th className="px-4 py-3 text-right">Pod</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {connections.map((c, i) => (
                    <tr key={c.id ?? i} className="hover:bg-muted/30">
                      <td
                        className="px-4 py-3 text-slate-200 max-w-[10rem] truncate"
                        title={c.podName || c.podUid || ''}
                      >
                        {podDisplayName(c.podName, c.podUid)}
                      </td>
                      <td className="px-4 py-3 text-slate-400 text-xs">{c.namespace}</td>
                      <td
                        className="px-4 py-3 font-mono text-xs text-slate-300 whitespace-nowrap"
                        title={`${c.destIp ?? ''}:${c.destPort ?? 0}`}
                      >
                        {formatRemoteCell(c)}
                      </td>
                      <td className="px-4 py-3 text-xs">{c.protocol ?? '—'}</td>
                      <td className="px-4 py-3 text-xs text-slate-500 max-w-[6rem] truncate">{c.state ?? '—'}</td>
                      <td
                        className="px-4 py-3 text-xs text-slate-400 font-mono whitespace-nowrap"
                        title={c.bucket5m ? formatDateTime(c.bucket5m) : undefined}
                      >
                        {c.bucket5m ? formatBucketClock(c.bucket5m) : '—'}
                      </td>
                      <td className="px-4 py-3 text-right font-mono text-xs text-slate-500" title={TX_RX_HEADER_TITLE}>
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
}
