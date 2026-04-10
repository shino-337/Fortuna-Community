import React, { useState, useEffect, useCallback, useMemo, useId, useRef } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  RefreshCw,
  Search,
  Box,
  Globe,
  Layers,
  ZoomIn,
  ZoomOut,
  RotateCcw,
  Eye,
  EyeOff,
} from 'lucide-react';
import { api } from '../lib/api';
import { useClusterStore } from '../store/clusterStore';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { PageEmpty } from '../components/PageEmpty';
import { NetworkTopologyGraph } from '../components/NetworkTopologyGraph';
import type {
  NetworkActivityConnectionRow,
  NetworkActivityDestinationRow,
  NetworkActivityTalkerRow,
} from '../types';

/** Core view=edges: số nhóm (pod×đích×proto) tối đa / request — khớp NETWORK_ACTIVITY_TOPOLOGY_EDGES_MAX (mặc định 2500). */
const TOPOLOGY_EDGE_PAGE_SIZE = 2500;
/** Phân trang talkers/destinations; khớp NETWORK_ACTIVITY_MAX_PAGE_SIZE (mặc định 200, tối đa 500). */
const NETWORK_SUMMARY_PAGE_SIZE = 200;
const MAX_TALKER_FETCH_PAGES = 50;

/**
 * Vùng graph: tối thiểu theo viewport (svh) để không còn khoảng trống lớn dưới card;
 * flex-1 vẫn cho phép cao hơn khi layout cha có đủ chỗ.
 */
const GRAPH_AREA_CLASS =
  'min-h-[max(17rem,calc(100svh-13.5rem))] sm:min-h-[max(19rem,calc(100svh-12.5rem))] lg:min-h-[max(20rem,calc(100svh-11.5rem))]';

const LEGEND_SCALE_MIN = 0.65;
const LEGEND_SCALE_MAX = 1.85;
const LEGEND_SCALE_STEP = 0.1;

function clampLegendScale(v: number): number {
  return Math.min(LEGEND_SCALE_MAX, Math.max(LEGEND_SCALE_MIN, v));
}

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

function parseAppliedPort(q: string): number | null {
  const t = q.trim();
  if (!/^\d+$/.test(t)) return null;
  const n = parseInt(t, 10);
  if (n < 1 || n > 65535) return null;
  return n;
}

export function NetworkActivity() {
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const [namespaceDraft, setNamespaceDraft] = useState('');
  const [searchDraft, setSearchDraft] = useState('');
  const [namespaceApplied, setNamespaceApplied] = useState('');
  const [searchApplied, setSearchApplied] = useState('');
  const [sinceMinutes, setSinceMinutes] = useState(1440 as number | '');
  const [loading, setLoading] = useState(false);
  const [refreshSpin, setRefreshSpin] = useState(false);
  const [graphDestinations, setGraphDestinations] = useState([] as NetworkActivityDestinationRow[]);
  const [graphTalkers, setGraphTalkers] = useState([] as NetworkActivityTalkerRow[]);
  const [graphConnections, setGraphConnections] = useState([] as NetworkActivityConnectionRow[]);
  const [destTotal, setDestTotal] = useState(0);
  const [talkerTotal, setTalkerTotal] = useState(0);
  const [topologyLegendScale, setTopologyLegendScale] = useState(1);
  const [topologyLegendVisible, setTopologyLegendVisible] = useState(true);
  const [clusterNamespaces, setClusterNamespaces] = useState([] as string[]);
  const [clusterNsLoading, setClusterNsLoading] = useState(false);

  const namespaceDatalistId = useId();
  /** Tránh request cũ (vẫn đang pending) ghi đè state sau khi đã xóa lọc / đổi filter. */
  const fetchReqIdRef = useRef(0);

  const appliedPort = useMemo(() => parseAppliedPort(searchApplied), [searchApplied]);
  const hasTextFilters = Boolean(namespaceApplied || searchApplied);
  const hasDraftTextFilters = Boolean(namespaceDraft.trim() || searchDraft.trim());
  const sinceHuman = useMemo(() => sinceRangeHuman(sinceMinutes), [sinceMinutes]);

  const applyFilters = useCallback(() => {
    setNamespaceApplied(namespaceDraft.trim());
    setSearchApplied(searchDraft.trim());
  }, [namespaceDraft, searchDraft]);

  const clearTextFilters = useCallback(() => {
    setNamespaceDraft('');
    setSearchDraft('');
    setNamespaceApplied('');
    setSearchApplied('');
  }, []);

  /** Đồng bộ bản lọc đã áp dụng sau khi gõ (tránh quên bấm Áp dụng). Enter / nút vẫn áp dụng ngay. */
  const FILTER_DEBOUNCE_MS = 450;
  useEffect(() => {
    const t = window.setTimeout(() => {
      const ns = namespaceDraft.trim();
      const sq = searchDraft.trim();
      setNamespaceApplied((p) => (p === ns ? p : ns));
      setSearchApplied((p) => (p === sq ? p : sq));
    }, FILTER_DEBOUNCE_MS);
    return () => window.clearTimeout(t);
  }, [namespaceDraft, searchDraft]);

  useEffect(() => {
    if (!selectedClusterId) {
      setClusterNamespaces([]);
      return;
    }
    let cancelled = false;
    setClusterNsLoading(true);
    void api
      .getClusterInventory(selectedClusterId)
      .then((inv) => {
        if (cancelled) return;
        const raw = inv?.namespaces ?? [];
        setClusterNamespaces([...raw].sort((a, b) => a.localeCompare(b, undefined, { sensitivity: 'base' })));
      })
      .catch(() => {
        if (!cancelled) setClusterNamespaces([]);
      })
      .finally(() => {
        if (!cancelled) setClusterNsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedClusterId]);

  const namespaceSelectValue = useMemo(() => {
    const d = namespaceDraft.trim();
    if (!d) return '';
    return clusterNamespaces.includes(d) ? d : '';
  }, [namespaceDraft, clusterNamespaces]);

  const fetchTopology = useCallback(
    async (manual = false) => {
      if (!selectedClusterId) {
        fetchReqIdRef.current += 1;
        setGraphDestinations([]);
        setGraphTalkers([]);
        setGraphConnections([]);
        setDestTotal(0);
        setTalkerTotal(0);
        setLoading(false);
        setRefreshSpin(false);
        return;
      }
      const myId = ++fetchReqIdRef.current;
      if (manual) setRefreshSpin(true);
      setLoading(true);
      try {
        const commonList = {
          cluster: selectedClusterId,
          namespace: namespaceApplied || undefined,
          q: searchApplied || undefined,
          sinceMinutes: sinceMinutes === '' ? undefined : sinceMinutes,
        };

        const [destData, edgeOrLegacy] = await Promise.all([
          api.getNetworkActivity({
            ...commonList,
            view: 'destinations',
            page: 1,
            pageSize: NETWORK_SUMMARY_PAGE_SIZE,
          }),
          (async () => {
            try {
              return await api.getNetworkActivity({
                ...commonList,
                view: 'edges',
                page: 1,
                pageSize: TOPOLOGY_EDGE_PAGE_SIZE,
              });
            } catch {
              return await api.getNetworkActivity({
                ...commonList,
                view: 'connections',
                page: 1,
                pageSize: NETWORK_SUMMARY_PAGE_SIZE,
              });
            }
          })(),
        ]);

        const talkerRows: NetworkActivityTalkerRow[] = [];
        let talkerTotalAcc = 0;
        for (let page = 1; page <= MAX_TALKER_FETCH_PAGES; page += 1) {
          const talkerData = await api.getNetworkActivity({
            ...commonList,
            view: 'talkers',
            page,
            pageSize: NETWORK_SUMMARY_PAGE_SIZE,
          });
          if (page === 1) talkerTotalAcc = talkerData.total ?? 0;
          const batch = (talkerData.items as NetworkActivityTalkerRow[]) ?? [];
          talkerRows.push(...batch);
          if (batch.length < NETWORK_SUMMARY_PAGE_SIZE) break;
        }

        if (myId !== fetchReqIdRef.current) return;
        setGraphDestinations((destData.items as NetworkActivityDestinationRow[]) ?? []);
        setGraphTalkers(talkerRows);
        setGraphConnections((edgeOrLegacy.items as NetworkActivityConnectionRow[]) ?? []);
        setDestTotal(destData.total ?? 0);
        setTalkerTotal(talkerTotalAcc);
      } catch {
        if (myId !== fetchReqIdRef.current) return;
        setGraphDestinations([]);
        setGraphTalkers([]);
        setGraphConnections([]);
        setDestTotal(0);
        setTalkerTotal(0);
      } finally {
        if (myId === fetchReqIdRef.current) {
          setLoading(false);
          if (manual) setRefreshSpin(false);
        }
      }
    },
    [selectedClusterId, namespaceApplied, searchApplied, sinceMinutes],
  );

  useEffect(() => {
    void fetchTopology(false);
  }, [fetchTopology]);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(() => void fetchTopology(false), intervalMs, { refreshTrigger });

  const goPod = (podUid: string) => {
    navigate(`/resources/pods/uid/${encodeURIComponent(podUid)}`);
  };

  /** Topology: cạnh từ view=edges (hoặc connections nếu Core cũ); pod orphan từ talkers đã phân trang đầy đủ. */
  const hasGraphData =
    graphConnections.length > 0 || (graphDestinations.length > 0 && graphTalkers.length > 0);
  const emptyContextLine = `Khoảng thời gian: ${sinceHuman}.`;

  const legendToggleButton = (
    <Button
      variant="secondary"
      size="sm"
      type="button"
      className="gap-1.5"
      title={topologyLegendVisible ? 'Ẩn chú thích trên đồ thị' : 'Hiện chú thích trên đồ thị'}
      aria-expanded={topologyLegendVisible}
      aria-controls="na-topology-legend"
      disabled={!selectedClusterId}
      onClick={() => setTopologyLegendVisible((v) => !v)}
    >
      {topologyLegendVisible ? <EyeOff className="w-4 h-4 shrink-0" /> : <Eye className="w-4 h-4 shrink-0" />}
      <span className="hidden sm:inline">{topologyLegendVisible ? 'Ẩn chú thích' : 'Hiện chú thích'}</span>
    </Button>
  );

  return (
    <PageLayout
      compact
      fillHeight
      className="!gap-2"
      title="Network topology"
      description="Pod → đích từ runtime (edges + talkers). Không phải NetworkPolicy. Cluster: header."
      actions={legendToggleButton}
    >
      {!selectedClusterId ? (
        <PageEmpty
          title="Chưa chọn cluster"
          description="Chọn một cluster trong menu để tải topology."
          className="py-16"
        />
      ) : (
        <div className="flex flex-col flex-1 min-h-0 gap-1.5">
          <Card className="overflow-hidden border-slate-800 bg-slate-950/20 flex flex-col flex-1 min-h-0">
            {/* Hàng lọc: grid xl — cùng hàng nhãn + controls (h-8), căn đều; md/sm xếp cột */}
            <div className="px-2 py-2 sm:px-3 border-b border-border shrink-0">
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-[minmax(0,auto)_minmax(12rem,1.1fr)_minmax(12rem,1.1fr)_minmax(9.5rem,11rem)_minmax(0,auto)] xl:gap-x-3 xl:gap-y-0 xl:items-end">
                <div className="flex flex-col gap-1 min-w-0 md:col-span-2 xl:col-span-1 xl:max-w-[11rem]">
                  <span className="text-[10px] text-slate-500 leading-4 h-4 shrink-0">Tải dữ liệu</span>
                  <div className="flex flex-wrap items-center gap-2 min-h-8">
                    <Button
                      variant="secondary"
                      size="sm"
                      className="h-8 shrink-0"
                      onClick={() => void fetchTopology(true)}
                      disabled={loading}
                    >
                      <RefreshCw className={`w-3.5 h-3.5 mr-1 ${refreshSpin ? 'animate-spin' : ''}`} />
                      Làm mới
                    </Button>
                    {!loading && (destTotal > 0 || talkerTotal > 0) && (
                      <span className="text-[10px] text-slate-500 leading-tight line-clamp-2 xl:line-clamp-3 max-w-[10rem] 2xl:max-w-[14rem]">
                        {destTotal} đích · {talkerTotal} nguồn · {graphConnections.length} cạnh
                      </span>
                    )}
                  </div>
                </div>

                <div className="flex flex-col gap-1 min-w-0">
                  <label className="text-[10px] text-slate-500 leading-4 h-4 shrink-0" htmlFor="na-namespace-input">
                    Namespace
                  </label>
                  <div className="flex flex-col sm:flex-row gap-2 sm:items-center min-h-8">
                    <div className="flex-1 min-w-0">
                      <input
                        id="na-namespace-input"
                        type="text"
                        list={clusterNamespaces.length > 0 ? namespaceDatalistId : undefined}
                        value={namespaceDraft}
                        onChange={(e) => setNamespaceDraft(e.target.value)}
                        onKeyDown={(e) => e.key === 'Enter' && applyFilters()}
                        placeholder="Gõ hoặc chọn…"
                        autoComplete="off"
                        className="w-full bg-slate-900 border border-slate-700 rounded-md px-2 text-sm text-slate-200 placeholder:text-slate-600 h-8 box-border"
                      />
                      {clusterNamespaces.length > 0 && (
                        <datalist id={namespaceDatalistId}>
                          {clusterNamespaces.map((ns) => (
                            <option key={ns} value={ns} />
                          ))}
                        </datalist>
                      )}
                    </div>
                    <select
                      aria-label="Chọn namespace từ danh sách cluster"
                      title="Danh sách namespace từ inventory cluster"
                      disabled={clusterNsLoading || clusterNamespaces.length === 0}
                      value={namespaceSelectValue}
                      onChange={(e) => setNamespaceDraft(e.target.value)}
                      className="w-full sm:w-[10.5rem] shrink-0 bg-slate-900 border border-slate-700 rounded-md px-2 text-sm text-slate-200 h-8 box-border disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      <option value="">
                        {clusterNsLoading
                          ? 'Đang tải…'
                          : clusterNamespaces.length === 0
                            ? 'Không có NS'
                            : '— Chọn NS —'}
                      </option>
                      {clusterNamespaces.map((ns) => (
                        <option key={ns} value={ns}>
                          {ns}
                        </option>
                      ))}
                    </select>
                  </div>
                </div>

                <div className="flex flex-col gap-1 min-w-0">
                  <label className="text-[10px] text-slate-500 leading-4 h-4 shrink-0" htmlFor="na-search-input">
                    Tìm kiếm
                  </label>
                  <div className="relative min-h-8 flex items-center">
                    <Search className="absolute left-2 top-1/2 -translate-y-1/2 w-3.5 h-3.5 text-slate-500 pointer-events-none" />
                    <input
                      id="na-search-input"
                      type="text"
                      value={searchDraft}
                      onChange={(e) => setSearchDraft(e.target.value)}
                      onKeyDown={(e) => e.key === 'Enter' && applyFilters()}
                      placeholder="Pod, uid, IP, cổng…"
                      className="w-full bg-slate-900 border border-slate-700 rounded-md pl-8 pr-2 text-sm text-slate-200 placeholder:text-slate-600 h-8 box-border"
                    />
                  </div>
                </div>

                <div className="flex flex-col gap-1 min-w-0">
                  <label className="text-[10px] text-slate-500 leading-4 h-4 shrink-0" htmlFor="na-since-select">
                    Thời gian
                  </label>
                  <select
                    id="na-since-select"
                    value={sinceMinutes === '' ? '' : String(sinceMinutes)}
                    onChange={(e) => {
                      const v = e.target.value;
                      setSinceMinutes(v === '' ? '' : Number(v));
                    }}
                    className="w-full bg-slate-900 border border-slate-700 rounded-md px-2 text-sm text-slate-200 h-8 box-border"
                  >
                    {SINCE_OPTIONS.map((o) => (
                      <option key={o.label} value={o.value === '' ? '' : String(o.value)}>
                        {o.label}
                      </option>
                    ))}
                  </select>
                  {sinceMinutes === '' && (
                    <p className="text-[10px] text-amber-600/90 leading-snug mt-0.5">
                      Không lọc thời gian — có thể chậm.
                    </p>
                  )}
                </div>

                <div className="flex flex-col gap-1 min-w-0 md:col-span-2 xl:col-span-1 xl:justify-end">
                  <span className="hidden xl:block text-[10px] text-transparent leading-4 h-4 shrink-0 select-none" aria-hidden>
                    ·
                  </span>
                  <div className="flex flex-wrap items-center gap-2 min-h-8 xl:justify-end">
                    <Button size="sm" className="h-8 text-xs px-4 shrink-0" onClick={() => applyFilters()} title="Áp dụng ngay (không đợi debounce)">
                      Áp dụng
                    </Button>
                    {(hasTextFilters || hasDraftTextFilters) && (
                      <Button
                        variant="secondary"
                        size="sm"
                        type="button"
                        className="h-8 text-xs px-3 shrink-0"
                        title="Xóa namespace / tìm kiếm và tải lại topology (bỏ lọc)"
                        onClick={() => clearTextFilters()}
                      >
                        Đặt lại
                      </Button>
                    )}
                  </div>
                </div>
              </div>
              <p className="mt-2 text-[10px] text-slate-600 leading-snug hidden lg:block border-t border-slate-800/80 pt-2">
                Namespace: inventory A→Z, <span className="font-mono">*</span> tiền tố. Tìm kiếm: LIKE tên pod/ns/IP; cổng số hoặc uid. Tối đa{' '}
                {TOPOLOGY_EDGE_PAGE_SIZE} nhóm pod×đích / lần tải.
              </p>
            </div>

            {appliedPort != null && (
              <div className="px-3 py-1 flex flex-wrap gap-2 items-center text-xs shrink-0 border-b border-border/60">
                <span className="inline-flex items-center rounded-full bg-slate-800 text-slate-300 px-2.5 py-0.5 border border-slate-600">
                  Port = {appliedPort}
                </span>
                <span className="text-slate-600">Lọc cổng (index-friendly)</span>
              </div>
            )}

            <div className={`relative flex-1 flex flex-col min-h-0 ${GRAPH_AREA_CLASS}`}>
              <div className="relative w-full flex-1 flex flex-col min-h-0 border-t border-slate-800 bg-slate-950/50">
                {loading &&
                graphDestinations.length === 0 &&
                graphTalkers.length === 0 &&
                graphConnections.length === 0 ? (
                  <div
                    className={`flex flex-col flex-1 min-h-0 justify-center items-center space-y-3 py-8 ${GRAPH_AREA_CLASS}`}
                  >
                    <div className="w-10 h-10 border-2 border-pink-500 border-t-transparent rounded-full animate-spin" />
                    <span className="text-slate-500 text-sm">Đang tải topology…</span>
                  </div>
                ) : !hasGraphData ? (
                  <div className={`flex flex-col flex-1 min-h-0 py-8 px-4 overflow-y-auto ${GRAPH_AREA_CLASS}`}>
                    <PageEmpty
                      title="Chưa đủ dữ liệu cho topology"
                      description={`Cần đồng thời có bản ghi “top đích” và “top nguồn” sau khi lọc. Kiểm tra agent (Pod Detail network) và khoảng thời gian. ${emptyContextLine}`}
                      className="py-8"
                    />
                    {hasTextFilters && (
                      <div className="flex justify-center mt-4">
                        <Button variant="secondary" size="sm" onClick={clearTextFilters}>
                          Xóa lọc namespace / tìm kiếm
                        </Button>
                      </div>
                    )}
                  </div>
                ) : (
                  <>
                    {/* Chú thích trên đồ thị: đồng bộ với nút header «Ẩn/Hiện chú thích» */}
                    <div className="absolute bottom-3 left-3 z-20 flex flex-col-reverse items-start gap-1.5 pointer-events-none">
                      {topologyLegendVisible ? (
                        <>
                          <div
                            id="na-topology-legend"
                            className="select-none max-w-[min(100%,17rem)] sm:max-w-[19rem] rounded-xl border border-slate-700 bg-slate-900/95 backdrop-blur-md p-2.5 sm:p-3 shadow-2xl space-y-2 pointer-events-auto overflow-visible"
                            style={{
                              transform: `scale(${topologyLegendScale})`,
                              transformOrigin: 'bottom left',
                            }}
                          >
                            <div className="text-[10px] font-bold text-slate-500 uppercase tracking-widest border-b border-slate-800 pb-1.5">
                              Chú thích
                            </div>
                            <div className="space-y-1.5">
                              <div className="flex items-start gap-2">
                                <div className="p-1 rounded mt-0.5 shrink-0" style={{ background: 'rgba(219,39,119,0.12)' }}>
                                  <Box size={12} className="text-pink-600" style={{ color: '#db2777' }} />
                                </div>
                                <div>
                                  <div className="text-xs font-semibold text-white">Pod (nguồn)</div>
                                  <p className="text-[10px] text-slate-500 leading-snug">
                                    Hồng: workload; dòng 2: namespace + uid (8 ký tự). Tooltip đầy đủ khi hover.
                                  </p>
                                </div>
                              </div>
                              <div className="flex items-start gap-2">
                                <div className="p-1 bg-slate-600/20 rounded mt-0.5 shrink-0">
                                  <Globe size={12} style={{ color: '#64748b' }} />
                                </div>
                                <div>
                                  <div className="text-xs font-semibold text-white">Đích</div>
                                  <p className="text-[10px] text-slate-500 leading-snug">
                                    Xám: IP:port hoặc pod đích; &quot;external&quot; khi không khớp workload trong cluster.
                                  </p>
                                </div>
                              </div>
                              <div className="flex items-start gap-2">
                                <div className="p-1 bg-emerald-500/10 rounded mt-0.5 shrink-0">
                                  <Layers size={12} className="text-emerald-400" />
                                </div>
                                <div>
                                  <div className="text-xs font-semibold text-white">Cạnh + hạt</div>
                                  <p className="text-[10px] text-slate-500 leading-snug">
                                    Minh họa luồng quan sát (không phải policy/băng thông). ≤55 cạnh có hạt.
                                  </p>
                                </div>
                              </div>
                              <p className="text-[10px] text-slate-600 pt-1 border-t border-slate-800 leading-snug">
                                Zoom · kéo canvas/node · click pod → chi tiết · click nền bỏ chọn.
                              </p>
                            </div>
                          </div>
                          <div className="pointer-events-auto flex items-center gap-0.5 rounded-lg border border-slate-600/90 bg-slate-900/95 backdrop-blur-sm px-1 py-0.5 shadow-lg">
                            <button
                              type="button"
                              title="Thu nhỏ chú thích"
                              aria-label="Thu nhỏ chú thích topology"
                              disabled={topologyLegendScale <= LEGEND_SCALE_MIN + 0.01}
                              className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-700 disabled:opacity-30 disabled:pointer-events-none"
                              onClick={() => setTopologyLegendScale((s) => clampLegendScale(s - LEGEND_SCALE_STEP))}
                            >
                              <ZoomOut className="w-3.5 h-3.5" />
                            </button>
                            <span className="text-[10px] text-slate-500 tabular-nums min-w-[2.75rem] text-center">
                              {Math.round(topologyLegendScale * 100)}%
                            </span>
                            <button
                              type="button"
                              title="Phóng to chú thích"
                              aria-label="Phóng to chú thích topology"
                              disabled={topologyLegendScale >= LEGEND_SCALE_MAX - 0.01}
                              className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-700 disabled:opacity-30 disabled:pointer-events-none"
                              onClick={() => setTopologyLegendScale((s) => clampLegendScale(s + LEGEND_SCALE_STEP))}
                            >
                              <ZoomIn className="w-3.5 h-3.5" />
                            </button>
                            <button
                              type="button"
                              title="Đặt lại 100%"
                              aria-label="Đặt lại tỷ lệ chú thích"
                              className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-700 border-l border-slate-600 ml-0.5 pl-1.5"
                              onClick={() => setTopologyLegendScale(1)}
                            >
                              <RotateCcw className="w-3.5 h-3.5" />
                            </button>
                            <button
                              type="button"
                              title="Ẩn chú thích"
                              aria-label="Ẩn chú thích topology"
                              className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-700 border-l border-slate-600 ml-0.5 pl-1.5"
                              onClick={() => setTopologyLegendVisible(false)}
                            >
                              <EyeOff className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </>
                      ) : (
                        <button
                          type="button"
                          title="Hiện chú thích topology"
                          aria-label="Hiện chú thích topology"
                          className="pointer-events-auto flex items-center gap-1.5 rounded-lg border border-slate-600/90 bg-slate-900/95 backdrop-blur-sm px-2 py-1.5 shadow-lg text-[11px] text-slate-400 hover:text-white hover:bg-slate-800"
                          onClick={() => setTopologyLegendVisible(true)}
                        >
                          <Eye className="w-3.5 h-3.5 shrink-0" />
                          <span>Chú thích</span>
                        </button>
                      )}
                    </div>

                    <div className="absolute inset-0 p-1.5 sm:p-2 md:p-3 flex flex-col min-h-0">
                      <NetworkTopologyGraph
                        key={`${selectedClusterId}|${namespaceApplied}|${searchApplied}|${sinceMinutes}`}
                        className="flex-1 min-h-0 w-full h-full"
                        destinations={graphDestinations}
                        talkers={graphTalkers}
                        connections={graphConnections}
                        supplementTalkers={graphTalkers}
                        maxNodes={180}
                        showCompactLegend={false}
                        onNodeClick={(id, kind) => {
                          if (kind === 'pod') goPod(id);
                        }}
                      />
                    </div>
                  </>
                )}
              </div>
            </div>
          </Card>
        </div>
      )}
    </PageLayout>
  );
}
