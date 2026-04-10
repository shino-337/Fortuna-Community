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
  Share2,
  List,
  Table2,
  ExternalLink,
  ChevronLeft,
  ChevronRight,
  Copy,
  Check,
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
  NetworkActivityWorkloadRow,
} from '../types';

type NetworkMainTab = 'topology' | 'pods' | 'connections';

/** Poll topology: chỉ tải cạnh (edges/connections). Full: destinations + talkers + inventory + cạnh. */
type TopologyFetchOpts = { mode: 'quick' | 'full'; manual?: boolean };

const TABLE_PAGE_SIZES = [25, 50, 100, 200] as const;

function podTableLabel(name?: string, uid?: string): string {
  const n = (name ?? '').trim();
  if (n) return n;
  const u = (uid ?? '').trim();
  if (!u) return '—';
  return u.length <= 12 ? u : `${u.slice(0, 12)}…`;
}

function formatObservedAt(iso?: string): string {
  if (!iso) return '—';
  try {
    return new Date(iso).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'medium' });
  } catch {
    return iso;
  }
}

/** Tooltip: ghi rõ local vs ISO để không so sánh nhầm với bucket UTC. */
function observedAtTooltip(iso?: string): string {
  if (!iso) return '';
  try {
    const d = new Date(iso);
    const local = d.toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'medium' });
    return `Thời điểm quan sát — hiển thị: giờ địa phương (${local}). ISO (UTC): ${d.toISOString()}`;
  } catch {
    return iso;
  }
}

/** Hiển thị mốc đầu bucket 5m UTC (ngắn gọn + tooltip đủ ISO). */
function formatBucket5mLine(iso?: string): { short: string; title: string } {
  if (!iso) return { short: '—', title: '' };
  try {
    const d = new Date(iso);
    const full = d.toISOString();
    return {
      short: `${full.slice(0, 10)} ${full.slice(11, 16)} UTC`,
      title: `Bucket 5m bắt đầu: ${full}`,
    };
  } catch {
    return { short: iso, title: iso };
  }
}

function isListenRow(row: NetworkActivityConnectionRow): boolean {
  const st = (row.state ?? '').toUpperCase();
  return st === 'LISTEN' || st === 'LISTENING';
}

/** Socket LISTEN — hiển thị ở cột Local; Remote = — (đúng ngữ nghĩa). */
function formatListenLocal(row: NetworkActivityConnectionRow): string {
  const p = row.destPort ?? row.sourcePort;
  const dip = (row.destIp ?? '').trim();
  const bind =
    dip && dip !== '0.0.0.0' && dip !== '::' && dip !== '*' && dip !== '[::]'
      ? ` @${dip}`
      : '';
  return `LISTEN :${p ?? '?'}${bind}`;
}

function formatLocalEndpoint(row: NetworkActivityConnectionRow): string {
  if (isListenRow(row)) return formatListenLocal(row);
  return `${(row.sourceIp ?? '—').trim()}:${row.sourcePort ?? '—'}`;
}

function formatRemoteEndpoint(row: NetworkActivityConnectionRow): string {
  if (isListenRow(row)) return '—';
  const dip = (row.destIp ?? '').trim();
  const dp = row.destPort;
  if (!dip && (dp == null || Number.isNaN(Number(dp)))) return '—';
  return `${dip || '—'}:${dp ?? '—'}`;
}

/** Chuỗi copy — khớp cột Remote. */
function remoteEndpointClipboard(row: NetworkActivityConnectionRow): string {
  return formatRemoteEndpoint(row);
}

/** Chuỗi copy — khớp cột Local (gồm LISTEN). */
function localEndpointClipboard(row: NetworkActivityConnectionRow): string {
  return formatLocalEndpoint(row);
}

async function copyTextWithFallback(text: string): Promise<boolean> {
  const t = text.trim();
  if (!t || t === '—') return false;
  try {
    if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(t);
      return true;
    }
  } catch {
    /* fallback */
  }
  try {
    const ta = document.createElement('textarea');
    ta.value = t;
    ta.setAttribute('readonly', '');
    ta.style.position = 'fixed';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand('copy');
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
}

/** Core view=edges: số nhóm (pod×đích×proto) tối đa / request — khớp NETWORK_ACTIVITY_TOPOLOGY_EDGES_MAX (mặc định 2500). */
const TOPOLOGY_EDGE_PAGE_SIZE = 2500;
/** Phân trang talkers/destinations; khớp NETWORK_ACTIVITY_MAX_PAGE_SIZE (mặc định 200, tối đa 500). */
const NETWORK_SUMMARY_PAGE_SIZE = 200;
/** Trần số trang talkers / lần tải topology (tránh >80 request khi cluster lớn). */
const MAX_TALKER_FETCH_PAGES_CAP = 10;
/** Tab topology: polling chậm hơn bảng để giảm tải định kỳ (vẫn có Làm mới + refreshTrigger). */
const TOPOLOGY_POLL_INTERVAL_MS = 3 * 60 * 1000;

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

/** Tên pod từ inventory — khớp uid với topology khi API network-activity thiếu podName. */
async function fetchInventoryPodNamesByUid(clusterId: string, namespace?: string): Promise<Record<string, string>> {
  const map: Record<string, string> = {};
  const pageSize = 500;
  for (let page = 1; page <= 30; page++) {
    const { pods, total } = await api.getPods({
      cluster: clusterId,
      namespace: namespace || undefined,
      page,
      pageSize,
    });
    for (const p of pods) {
      const u = (p.uid ?? '').trim();
      const n = (p.name ?? '').trim();
      if (u && n) map[u] = n;
    }
    if (pods.length < pageSize || page * pageSize >= total) break;
  }
  return map;
}

const inventoryPodNamesCache = new Map<string, { at: number; data: Record<string, string> }>();
const INVENTORY_POD_NAMES_CACHE_TTL_MS = 30_000;
const INVENTORY_CACHE_MAX_KEYS = 24;

function clearInventoryPodNamesCache(): void {
  inventoryPodNamesCache.clear();
}

function trimInventoryPodNamesCache(): void {
  while (inventoryPodNamesCache.size > INVENTORY_CACHE_MAX_KEYS) {
    let oldestK: string | null = null;
    let oldestAt = Infinity;
    for (const [k, v] of inventoryPodNamesCache) {
      if (v.at < oldestAt) {
        oldestAt = v.at;
        oldestK = k;
      }
    }
    if (oldestK == null) break;
    inventoryPodNamesCache.delete(oldestK);
  }
}

async function fetchInventoryPodNamesByUidCached(
  clusterId: string,
  namespace?: string,
): Promise<Record<string, string>> {
  const key = `${clusterId}\x1f${namespace ?? ''}`;
  const now = Date.now();
  const hit = inventoryPodNamesCache.get(key);
  if (hit && now - hit.at < INVENTORY_POD_NAMES_CACHE_TTL_MS) return hit.data;
  const data = await fetchInventoryPodNamesByUid(clusterId, namespace);
  inventoryPodNamesCache.set(key, { at: now, data });
  trimInventoryPodNamesCache();
  return data;
}

export function NetworkActivity() {
  const navigate = useNavigate();
  const selectedClusterId = useClusterStore((s) => s.selectedClusterId);
  const [namespaceDraft, setNamespaceDraft] = useState('');
  const [searchDraft, setSearchDraft] = useState('');
  const [namespaceApplied, setNamespaceApplied] = useState('');
  const [searchApplied, setSearchApplied] = useState('');
  /** Lọc chính xác theo pod nguồn (query podUid trên Core); drill khi không có podName. */
  const [podUidApplied, setPodUidApplied] = useState('');
  const [sinceMinutes, setSinceMinutes] = useState(1440 as number | '');
  const [loading, setLoading] = useState(false);
  const [refreshSpin, setRefreshSpin] = useState(false);
  const [graphDestinations, setGraphDestinations] = useState([] as NetworkActivityDestinationRow[]);
  const [graphTalkers, setGraphTalkers] = useState([] as NetworkActivityTalkerRow[]);
  const [graphConnections, setGraphConnections] = useState([] as NetworkActivityConnectionRow[]);
  const [topologyPodNamesByUid, setTopologyPodNamesByUid] = useState<Record<string, string>>({});
  /** Thông báo khi thiếu view API / fallback — tránh nhầm với «không có dữ liệu». */
  const [topologySupportIssue, setTopologySupportIssue] = useState<string | null>(null);
  const [destTotal, setDestTotal] = useState(0);
  const [talkerTotal, setTalkerTotal] = useState(0);
  const [topologyLegendScale, setTopologyLegendScale] = useState(1);
  const [topologyLegendVisible, setTopologyLegendVisible] = useState(true);
  const [clusterNamespaces, setClusterNamespaces] = useState([] as string[]);
  const [clusterNsLoading, setClusterNsLoading] = useState(false);

  const [mainTab, setMainTab] = useState<NetworkMainTab>('topology');
  const filterDebounceMs = useMemo(
    () => (mainTab === 'topology' ? 880 : 700),
    [mainTab],
  );
  const [tablePage, setTablePage] = useState(1);
  const [tablePageSize, setTablePageSize] = useState(50);
  const [podsRows, setPodsRows] = useState([] as NetworkActivityWorkloadRow[]);
  const [podsTotal, setPodsTotal] = useState(0);
  const [connRows, setConnRows] = useState([] as NetworkActivityConnectionRow[]);
  const [connTotal, setConnTotal] = useState(0);
  const [tableLoading, setTableLoading] = useState(false);

  const namespaceDatalistId = useId();
  /** Tránh request cũ (vẫn đang pending) ghi đè state sau khi đã xóa lọc / đổi filter. */
  const fetchReqIdRef = useRef(0);
  const fetchTableReqIdRef = useRef(0);
  const [copiedTableKey, setCopiedTableKey] = useState<string | null>(null);
  const [copyErrorToast, setCopyErrorToast] = useState<string | null>(null);
  const [sinceCancelToast, setSinceCancelToast] = useState<string | null>(null);
  /** Gợi ý khi drill từ pod không có tên (tránh «q» = uid prefix không khớp backend). */
  const [connectionsScopeNote, setConnectionsScopeNote] = useState<string | null>(null);
  const copyFeedbackTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const copyErrorTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const sinceCancelTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const copyToClipboard = useCallback((key: string, text: string) => {
    void copyTextWithFallback(text).then((ok) => {
      if (ok) {
        if (copyFeedbackTimerRef.current) window.clearTimeout(copyFeedbackTimerRef.current);
        setCopiedTableKey(key);
        copyFeedbackTimerRef.current = window.setTimeout(() => setCopiedTableKey(null), 2000);
      } else {
        if (copyErrorTimerRef.current) window.clearTimeout(copyErrorTimerRef.current);
        setCopyErrorToast(
          'Không sao chép được (quyền trình duyệt, chính sách clipboard, hoặc ngữ cảnh không an toàn).',
        );
        copyErrorTimerRef.current = window.setTimeout(() => setCopyErrorToast(null), 4500);
      }
    });
  }, []);

  useEffect(
    () => () => {
      if (copyFeedbackTimerRef.current) window.clearTimeout(copyFeedbackTimerRef.current);
      if (copyErrorTimerRef.current) window.clearTimeout(copyErrorTimerRef.current);
      if (sinceCancelTimerRef.current) window.clearTimeout(sinceCancelTimerRef.current);
    },
    [],
  );

  useEffect(() => {
    clearInventoryPodNamesCache();
    setPodUidApplied('');
  }, [selectedClusterId]);

  useEffect(() => {
    if (mainTab !== 'connections') setConnectionsScopeNote(null);
  }, [mainTab]);

  useEffect(() => {
    if (connectionsScopeNote && searchApplied.trim().length > 0) setConnectionsScopeNote(null);
  }, [searchApplied, connectionsScopeNote]);

  const appliedPort = useMemo(() => parseAppliedPort(searchApplied), [searchApplied]);
  const hasTextFilters = Boolean(namespaceApplied || searchApplied || podUidApplied.trim());
  const hasDraftTextFilters = Boolean(namespaceDraft.trim() || searchDraft.trim());
  const textDraftDiffersFromApplied =
    namespaceDraft.trim() !== namespaceApplied || searchDraft.trim() !== searchApplied;
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
    setPodUidApplied('');
    setConnectionsScopeNote(null);
  }, []);

  /**
   * Đồng bộ bản lọc sau khi gõ (debounce dài hơn trên tab topology để giảm spam request).
   * Tìm kiếm chỉ auto-apply khi rỗng, ≥3 ký tự, hoặc toàn số (vd cổng 1–5 chữ số).
   */
  useEffect(() => {
    const ns = namespaceDraft.trim();
    const sq = searchDraft.trim();
    const searchMayAutoApply =
      sq.length === 0 || sq.length >= 3 || /^\d{1,5}$/.test(sq);
    const t = window.setTimeout(() => {
      setNamespaceApplied((p) => (p === ns ? p : ns));
      if (searchMayAutoApply) setSearchApplied((p) => (p === sq ? p : sq));
    }, filterDebounceMs);
    return () => window.clearTimeout(t);
  }, [namespaceDraft, searchDraft, filterDebounceMs]);

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
    async (opts: TopologyFetchOpts) => {
      const { mode, manual = false } = opts;
      const isFull = mode === 'full';

      if (!selectedClusterId) {
        fetchReqIdRef.current += 1;
        setGraphDestinations([]);
        setGraphTalkers([]);
        setGraphConnections([]);
        setTopologyPodNamesByUid({});
        setDestTotal(0);
        setTalkerTotal(0);
        setTopologySupportIssue(null);
        setLoading(false);
        setRefreshSpin(false);
        return;
      }
      const myId = ++fetchReqIdRef.current;
      if (manual) setRefreshSpin(true);
      if (isFull) setLoading(true);
      if (manual && isFull) {
        const invKey = `${selectedClusterId}\x1f${namespaceApplied ?? ''}`;
        inventoryPodNamesCache.delete(invKey);
      }
      let supportIssue: string | null = null;
      try {
        const commonList = {
          cluster: selectedClusterId,
          namespace: namespaceApplied || undefined,
          q: searchApplied || undefined,
          podUid: podUidApplied.trim() || undefined,
          sinceMinutes: sinceMinutes === '' ? undefined : sinceMinutes,
        };

        if (!isFull) {
          let edgeOrLegacy: Awaited<ReturnType<typeof api.getNetworkActivity>>;
          try {
            edgeOrLegacy = await api.getNetworkActivity({
              ...commonList,
              view: 'edges',
              page: 1,
              pageSize: TOPOLOGY_EDGE_PAGE_SIZE,
            });
          } catch {
            try {
              edgeOrLegacy = await api.getNetworkActivity({
                ...commonList,
                view: 'connections',
                page: 1,
                pageSize: NETWORK_SUMMARY_PAGE_SIZE,
              });
            } catch {
              if (myId !== fetchReqIdRef.current) return;
              return;
            }
          }
          if (myId !== fetchReqIdRef.current) return;
          setGraphConnections((edgeOrLegacy.items as NetworkActivityConnectionRow[]) ?? []);
          return;
        }

        let destData: Awaited<ReturnType<typeof api.getNetworkActivity>>;
        try {
          destData = await api.getNetworkActivity({
            ...commonList,
            view: 'destinations',
            page: 1,
            pageSize: NETWORK_SUMMARY_PAGE_SIZE,
          });
        } catch {
          supportIssue =
            'Core không trả view «destinations» (cần bản Core hỗ trợ topology) hoặc lỗi mạng.';
          destData = {
            view: 'destinations',
            clusterId: selectedClusterId,
            total: 0,
            page: 1,
            pageSize: NETWORK_SUMMARY_PAGE_SIZE,
            items: [],
          };
        }

        let edgeOrLegacy: Awaited<ReturnType<typeof api.getNetworkActivity>>;
        try {
          edgeOrLegacy = await api.getNetworkActivity({
            ...commonList,
            view: 'edges',
            page: 1,
            pageSize: TOPOLOGY_EDGE_PAGE_SIZE,
          });
        } catch {
          try {
            edgeOrLegacy = await api.getNetworkActivity({
              ...commonList,
              view: 'connections',
              page: 1,
              pageSize: NETWORK_SUMMARY_PAGE_SIZE,
            });
            if (!supportIssue) {
              supportIssue =
                'Đang dùng view «connections» thay «edges»; đồ thị topology có thể hạn chế.';
            }
          } catch {
            supportIssue =
              supportIssue ??
              'Core không hỗ trợ «edges»/«connections» cho topology hoặc lỗi tải.';
            edgeOrLegacy = {
              view: 'connections',
              clusterId: selectedClusterId,
              total: 0,
              page: 1,
              pageSize: NETWORK_SUMMARY_PAGE_SIZE,
              items: [],
            };
          }
        }

        const invNames = await fetchInventoryPodNamesByUidCached(
          selectedClusterId,
          namespaceApplied || undefined,
        ).catch(() => ({}));

        const talkerRows: NetworkActivityTalkerRow[] = [];
        let talkerTotalAcc = 0;
        let maxTalkerPages = MAX_TALKER_FETCH_PAGES_CAP;
        try {
          for (let page = 1; page <= maxTalkerPages; page += 1) {
            const talkerData = await api.getNetworkActivity({
              ...commonList,
              view: 'talkers',
              page,
              pageSize: NETWORK_SUMMARY_PAGE_SIZE,
            });
            if (page === 1) {
              talkerTotalAcc = talkerData.total ?? 0;
              const needed = Math.ceil(
                Math.max(0, talkerTotalAcc) / NETWORK_SUMMARY_PAGE_SIZE,
              );
              maxTalkerPages = Math.min(MAX_TALKER_FETCH_PAGES_CAP, Math.max(1, needed));
            }
            const batch = (talkerData.items as NetworkActivityTalkerRow[]) ?? [];
            talkerRows.push(...batch);
            if (batch.length < NETWORK_SUMMARY_PAGE_SIZE) break;
          }
        } catch {
          supportIssue =
            supportIssue ??
            'Core không trả view «talkers» (cần bản Core hỗ trợ topology) hoặc lỗi tải.';
        }

        if (myId !== fetchReqIdRef.current) return;
        setGraphDestinations((destData.items as NetworkActivityDestinationRow[]) ?? []);
        setGraphTalkers(talkerRows);
        setGraphConnections((edgeOrLegacy.items as NetworkActivityConnectionRow[]) ?? []);
        setTopologyPodNamesByUid(invNames);
        setDestTotal(destData.total ?? 0);
        setTalkerTotal(talkerTotalAcc);
        setTopologySupportIssue(supportIssue);
      } catch {
        if (myId !== fetchReqIdRef.current) return;
        setGraphDestinations([]);
        setGraphTalkers([]);
        setGraphConnections([]);
        setTopologyPodNamesByUid({});
        setDestTotal(0);
        setTalkerTotal(0);
        setTopologySupportIssue('Không tải được dữ liệu topology. Kiểm tra Core và kết nối.');
      } finally {
        if (myId === fetchReqIdRef.current) {
          if (isFull) setLoading(false);
          if (manual) setRefreshSpin(false);
        }
      }
    },
    [selectedClusterId, namespaceApplied, searchApplied, podUidApplied, sinceMinutes],
  );

  const fetchTableData = useCallback(
    async (manual = false) => {
      if (!selectedClusterId || (mainTab !== 'pods' && mainTab !== 'connections')) {
        fetchTableReqIdRef.current += 1;
        setPodsRows([]);
        setPodsTotal(0);
        setConnRows([]);
        setConnTotal(0);
        setTableLoading(false);
        if (manual) setRefreshSpin(false);
        return;
      }
      const myId = ++fetchTableReqIdRef.current;
      if (manual) setRefreshSpin(true);
      setTableLoading(true);
      try {
        const commonList = {
          cluster: selectedClusterId,
          namespace: namespaceApplied || undefined,
          q: searchApplied || undefined,
          podUid: podUidApplied.trim() || undefined,
          sinceMinutes: sinceMinutes === '' ? undefined : sinceMinutes,
          page: tablePage,
          pageSize: tablePageSize,
        };
        const view = mainTab === 'pods' ? 'pods' : 'connections';
        const data = await api.getNetworkActivity({ ...commonList, view });
        if (myId !== fetchTableReqIdRef.current) return;
        if (view === 'pods') {
          setPodsRows((data.items as NetworkActivityWorkloadRow[]) ?? []);
          setPodsTotal(data.total ?? 0);
        } else {
          setConnRows((data.items as NetworkActivityConnectionRow[]) ?? []);
          setConnTotal(data.total ?? 0);
        }
      } catch {
        if (myId !== fetchTableReqIdRef.current) return;
        if (mainTab === 'pods') {
          setPodsRows([]);
          setPodsTotal(0);
        } else {
          setConnRows([]);
          setConnTotal(0);
        }
      } finally {
        if (myId === fetchTableReqIdRef.current) {
          setTableLoading(false);
          if (manual) setRefreshSpin(false);
        }
      }
    },
    [
      selectedClusterId,
      mainTab,
      namespaceApplied,
      searchApplied,
      podUidApplied,
      sinceMinutes,
      tablePage,
      tablePageSize,
    ],
  );

  useEffect(() => {
    if (mainTab !== 'topology') return;
    void fetchTopology({ mode: 'full' });
  }, [mainTab, fetchTopology]);

  useEffect(() => {
    if (mainTab !== 'pods' && mainTab !== 'connections') return;
    void fetchTableData(false);
  }, [mainTab, fetchTableData]);

  useEffect(() => {
    setTablePage(1);
  }, [namespaceApplied, searchApplied, podUidApplied, sinceMinutes, selectedClusterId, mainTab, tablePageSize]);

  const listPollIntervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  const activePollIntervalMs =
    mainTab === 'topology' ? TOPOLOGY_POLL_INTERVAL_MS : listPollIntervalMs;
  usePolling(
    () => {
      if (mainTab === 'topology') void fetchTopology({ mode: 'quick' });
      else void fetchTableData(false);
    },
    activePollIntervalMs,
    { refreshTrigger },
  );

  /** Refresh toàn app: topology tải đầy đủ (không chỉ poll nhanh cạnh). */
  useEffect(() => {
    if (refreshTrigger <= 0) return;
    if (mainTab !== 'topology') return;
    void fetchTopology({ mode: 'full' });
  }, [refreshTrigger, mainTab, fetchTopology]);

  const goPod = (podUid: string) => {
    navigate(`/resources/pods/uid/${encodeURIComponent(podUid)}`);
  };

  const drillConnectionsForPod = useCallback((row: NetworkActivityWorkloadRow) => {
    const ns = (row.namespace ?? '').trim();
    const name = (row.podName ?? '').trim();
    const uid = (row.podUid ?? '').trim();
    setNamespaceDraft(ns);
    setNamespaceApplied(ns);
    if (name) {
      setPodUidApplied('');
      setSearchDraft(name);
      setSearchApplied(name);
      setConnectionsScopeNote(null);
    } else {
      setSearchDraft('');
      setSearchApplied('');
      setPodUidApplied(uid);
      setConnectionsScopeNote(
        uid
          ? 'Chưa có tên pod từ API; đang lọc theo namespace + podUid (API, khớp chính xác). Có thể thêm «q» để thu hẹp thêm.'
          : null,
      );
    }
    setMainTab('connections');
  }, []);

  const handleManualRefresh = useCallback(() => {
    if (mainTab === 'topology') void fetchTopology({ mode: 'full', manual: true });
    else void fetchTableData(true);
  }, [mainTab, fetchTopology, fetchTableData]);

  const topologyBusy = loading && mainTab === 'topology';
  const tableListBusy = tableLoading && (mainTab === 'pods' || mainTab === 'connections');

  /** Topology: cạnh từ view=edges (hoặc connections nếu Core cũ); pod orphan từ talkers đã phân trang đầy đủ. */
  const hasGraphData =
    graphConnections.length > 0 || (graphDestinations.length > 0 && graphTalkers.length > 0);
  /** Chỉ suy đồ thị từ cạnh — không có aggregate destinations/talkers (Core hạn chế hoặc lỗi view). */
  const topologyLimitedFromConnectionsOnly =
    graphConnections.length > 0 && graphDestinations.length === 0 && graphTalkers.length === 0;
  const emptyContextLine = `Khoảng thời gian: ${sinceHuman}.`;

  const podUidLocksNamespace = Boolean(podUidApplied.trim());
  const namespaceLockedByPodUidTitle =
    'Đang lọc theo pod UID — namespace đi kèm drill. Bấm «Đặt lại» để đổi namespace hoặc bỏ lọc pod.';

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

  const tableTotal = mainTab === 'pods' ? podsTotal : connTotal;
  const tableRowsLen = mainTab === 'pods' ? podsRows.length : connRows.length;
  const tableTotalPages = Math.max(1, Math.ceil(Math.max(0, tableTotal) / tablePageSize));

  function formatQueueBytes(n?: number): string {
    if (n == null || Number.isNaN(n)) return '—';
    return n.toLocaleString();
  }

  return (
    <PageLayout
      compact
      fillHeight
      className="!gap-2"
      title="Network activity"
      description="Topology pod→đích; bảng Pods / Connections từ runtime. Không phải NetworkPolicy. Cluster: header."
      actions={mainTab === 'topology' ? legendToggleButton : undefined}
    >
      {!selectedClusterId ? (
        <PageEmpty
          title="Chưa chọn cluster"
          description="Chọn một cluster trong menu để tải topology."
          className="py-16"
        />
      ) : (
        <div className="flex flex-col flex-1 min-h-0 gap-1.5">
          <Card className="relative overflow-hidden border-slate-800 bg-slate-950/20 flex flex-col flex-1 min-h-0">
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
                      onClick={() => void handleManualRefresh()}
                      disabled={topologyBusy || tableListBusy}
                    >
                      <RefreshCw className={`w-3.5 h-3.5 mr-1 ${refreshSpin ? 'animate-spin' : ''}`} />
                      Làm mới
                    </Button>
                    {mainTab === 'topology' && !loading && topologyLimitedFromConnectionsOnly && (
                      <span className="text-[10px] text-sky-500/90 leading-tight max-w-[14rem]">
                        {graphConnections.length} cạnh · topology rút gọn (không có top đích/nguồn)
                      </span>
                    )}
                    {mainTab === 'topology' && !loading && !topologyLimitedFromConnectionsOnly && (destTotal > 0 || talkerTotal > 0) && (
                      <span className="text-[10px] text-slate-500 leading-tight line-clamp-2 xl:line-clamp-3 max-w-[10rem] 2xl:max-w-[14rem]">
                        {destTotal} đích · {talkerTotal} nguồn · {graphConnections.length} cạnh
                      </span>
                    )}
                    {mainTab === 'pods' && !tableLoading && (
                      <span className="text-[10px] text-slate-500 tabular-nums">
                        {podsTotal} pod · trang {tablePage}/{tableTotalPages}
                      </span>
                    )}
                    {mainTab === 'connections' && !tableLoading && (
                      <span className="text-[10px] text-slate-500 tabular-nums">
                        {connTotal} dòng · trang {tablePage}/{tableTotalPages}
                      </span>
                    )}
                    {mainTab === 'topology' && (
                      <span
                        className="text-[10px] text-slate-600 leading-tight max-w-[14rem] xl:max-w-[18rem]"
                        title="Mỗi chu kỳ chỉ cập nhật cạnh (edges/connections), không tải lại destinations/talkers/inventory. Tab Pods/Connections ~30s. «Làm mới» và refresh toàn app: tải đầy đủ topology."
                      >
                        Topology: ~3 phút chỉ cập nhật cạnh; «Làm mới» = đầy đủ
                      </span>
                    )}
                  </div>
                </div>

                <div
                  className={`flex flex-col gap-1 min-w-0 ${podUidLocksNamespace ? 'opacity-80' : ''}`}
                  title={podUidLocksNamespace ? namespaceLockedByPodUidTitle : undefined}
                >
                  <label
                    className={`text-[10px] text-slate-500 leading-4 h-4 shrink-0 ${podUidLocksNamespace ? 'cursor-help' : ''}`}
                    htmlFor="na-namespace-input"
                    title={podUidLocksNamespace ? namespaceLockedByPodUidTitle : undefined}
                  >
                    Namespace
                    {podUidLocksNamespace ? (
                      <span className="text-slate-600 font-normal"> (khóa khi có podUid)</span>
                    ) : null}
                  </label>
                  <div className="flex flex-col sm:flex-row gap-2 sm:items-center min-h-8">
                    <div className="flex-1 min-w-0">
                      <input
                        id="na-namespace-input"
                        type="text"
                        list={clusterNamespaces.length > 0 && !podUidLocksNamespace ? namespaceDatalistId : undefined}
                        value={namespaceDraft}
                        onChange={(e) => setNamespaceDraft(e.target.value)}
                        onKeyDown={(e) => e.key === 'Enter' && applyFilters()}
                        placeholder="Gõ hoặc chọn…"
                        autoComplete="off"
                        disabled={podUidLocksNamespace}
                        title={podUidLocksNamespace ? namespaceLockedByPodUidTitle : undefined}
                        aria-disabled={podUidLocksNamespace}
                        className="w-full bg-slate-900 border border-slate-700 rounded-md px-2 text-sm text-slate-200 placeholder:text-slate-600 h-8 box-border disabled:opacity-55 disabled:cursor-not-allowed"
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
                      title={
                        podUidLocksNamespace
                          ? namespaceLockedByPodUidTitle
                          : 'Danh sách namespace từ inventory cluster'
                      }
                      disabled={clusterNsLoading || clusterNamespaces.length === 0 || podUidLocksNamespace}
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
                      placeholder={
                        podUidLocksNamespace
                          ? 'Tìm kiếm trong pod này (AND với podUid)…'
                          : 'Pod, uid, IP, cổng…'
                      }
                      title={
                        podUidLocksNamespace
                          ? 'Thu hẹp thêm bằng q; API: AND với podUid đã chọn.'
                          : undefined
                      }
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
                      if (v === '') {
                        if (
                          window.confirm(
                            'Tắt lọc thời gian («Mọi thời điểm») có thể làm truy vấn rất chậm và tải rất nhiều dữ liệu. Bạn có chắc?',
                          )
                        ) {
                          setSinceMinutes('');
                        } else {
                          if (sinceCancelTimerRef.current) window.clearTimeout(sinceCancelTimerRef.current);
                          setSinceCancelToast('Đã hủy — giữ nguyên cửa sổ thời gian hiện tại.');
                          sinceCancelTimerRef.current = window.setTimeout(() => setSinceCancelToast(null), 3200);
                        }
                        return;
                      }
                      setSinceMinutes(Number(v));
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
                        title="Xóa namespace / tìm kiếm / podUid và tải lại (bỏ lọc)"
                        onClick={() => clearTextFilters()}
                      >
                        Đặt lại
                      </Button>
                    )}
                  </div>
                </div>
              </div>

              <div className="mt-2 flex flex-wrap items-center gap-2 border-t border-slate-800/80 pt-2">
                <span className="text-[10px] text-slate-600 shrink-0">Đang áp dụng:</span>
                <span
                  className="inline-flex items-center rounded-full bg-slate-800/90 text-slate-300 px-2 py-0.5 text-[10px] border border-slate-600/80"
                  title="Cửa sổ thời gian API"
                >
                  Thời gian: {sinceHuman}
                </span>
                {namespaceApplied ? (
                  <span className="inline-flex items-center rounded-full bg-slate-800/90 text-slate-300 px-2 py-0.5 text-[10px] border border-slate-600/80 font-mono">
                    ns={namespaceApplied}
                  </span>
                ) : (
                  <span className="text-[10px] text-slate-600">ns: (tất cả)</span>
                )}
                {searchApplied ? (
                  <span
                    className="inline-flex items-center rounded-full bg-slate-800/90 text-slate-300 px-2 py-0.5 text-[10px] border border-slate-600/80 font-mono max-w-[14rem] truncate"
                    title={searchApplied}
                  >
                    q={searchApplied}
                  </span>
                ) : (
                  <span className="text-[10px] text-slate-600">q: (trống)</span>
                )}
                {podUidApplied.trim() ? (
                  <span
                    className="inline-flex items-center rounded-full bg-slate-800/90 text-pink-200/90 px-2 py-0.5 text-[10px] border border-pink-600/50 font-mono max-w-[14rem] truncate"
                    title={`podUid đầy đủ: ${podUidApplied.trim()}`}
                  >
                    podUid=
                    {podUidApplied.trim().length > 18
                      ? `${podUidApplied.trim().slice(0, 8)}…${podUidApplied.trim().slice(-6)}`
                      : podUidApplied.trim()}
                  </span>
                ) : null}
                {podUidApplied.trim() && searchApplied.trim() ? (
                  <span
                    className="text-[10px] text-slate-500 max-w-[min(100%,20rem)] leading-snug"
                    title="Core áp dụng podUid và q cùng lúc: kết quả là giao (AND) — trong pod đó khớp thêm tìm kiếm."
                  >
                    podUid + q: <span className="text-slate-400">AND</span>
                  </span>
                ) : null}
                {textDraftDiffersFromApplied && (
                  <span
                    className="text-[10px] text-amber-500/95"
                    title="Namespace đồng bộ sau debounce. Tìm kiếm: tự áp dụng khi rỗng, từ 3 ký tự, hoặc cổng số (1–5 chữ số); còn lại cần Áp dụng hoặc Enter. Tab Topology debounce dài hơn."
                  >
                    Chưa áp dụng hết (draft)
                  </span>
                )}
              </div>

              <div className="mt-2 flex flex-wrap gap-1.5" role="tablist" aria-label="Chế độ xem network">
                {(
                  [
                    { id: 'topology' as const, label: 'Topology', icon: Share2 },
                    { id: 'pods' as const, label: 'Pods', icon: List },
                    { id: 'connections' as const, label: 'Connections', icon: Table2 },
                  ] as const
                ).map(({ id, label, icon: Icon }) => (
                  <button
                    key={id}
                    type="button"
                    role="tab"
                    aria-selected={mainTab === id}
                    className={`inline-flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-xs font-medium border transition-colors ${
                      mainTab === id
                        ? 'bg-pink-600/20 border-pink-500/50 text-pink-100'
                        : 'bg-slate-900/60 border-slate-700 text-slate-400 hover:text-slate-200 hover:border-slate-600'
                    }`}
                    onClick={() => setMainTab(id)}
                  >
                    <Icon className="w-3.5 h-3.5 shrink-0 opacity-90" />
                    {label}
                  </button>
                ))}
              </div>

              <div
                className="mt-2 rounded-lg border border-slate-700/80 bg-slate-900/40 px-2.5 py-2 text-[11px] text-slate-400 leading-snug"
                role="note"
              >
                Dữ liệu được dedupe theo <span className="text-slate-300">bucket 5 phút (UTC)</span>. Mỗi dòng là{' '}
                <span className="text-slate-300">snapshot cuối trong bucket</span>;{' '}
                <span className="text-slate-300">queue bytes</span> (/proc/net) là snapshot, không phải tổng traffic.
              </div>

              {mainTab === 'connections' && connectionsScopeNote && (
                <div
                  className="mt-2 rounded-lg border border-amber-600/45 bg-amber-950/35 px-2.5 py-1.5 text-[11px] text-amber-100/95 leading-snug"
                  role="status"
                >
                  {connectionsScopeNote}
                </div>
              )}

              <p className="mt-2 text-[10px] text-slate-600 leading-snug hidden lg:block border-t border-slate-800/80 pt-2">
                Namespace: inventory A→Z, <span className="font-mono">*</span> tiền tố. Tìm kiếm: LIKE tên pod/ns/IP; cổng số hoặc uid. Tối đa{' '}
                {TOPOLOGY_EDGE_PAGE_SIZE} nhóm pod×đích / lần tải (topology). Talkers: tối đa {MAX_TALKER_FETCH_PAGES_CAP} trang / lần (theo{' '}
                <span className="font-mono">total</span>). Cache tên pod inventory: TTL 30s, tối đa {INVENTORY_CACHE_MAX_KEYS} mục; xóa khi đổi
                cluster.
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
              {mainTab === 'topology' && (
              <div className="relative w-full flex-1 flex flex-col min-h-0 border-t border-slate-800 bg-slate-950/50">
                {topologyBusy &&
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
                      title={
                        topologySupportIssue
                          ? 'Topology không khả dụng hoặc thiếu view API'
                          : 'Chưa đủ dữ liệu cho topology'
                      }
                      description={
                        topologySupportIssue
                          ? `${topologySupportIssue} ${emptyContextLine}`
                          : `Cần đồng thời có bản ghi “top đích” và “top nguồn” sau khi lọc. Kiểm tra agent (Pod Detail network) và khoảng thời gian. ${emptyContextLine}`
                      }
                      className="py-8"
                    />
                    {hasTextFilters && (
                      <div className="flex justify-center mt-4">
                        <Button variant="secondary" size="sm" onClick={clearTextFilters}>
                          Xóa lọc (ns / q / podUid)
                        </Button>
                      </div>
                    )}
                  </div>
                ) : (
                  <>
                    {topologySupportIssue && (
                      <div
                        className="shrink-0 mx-2 mt-2 rounded-lg border border-amber-600/50 bg-amber-950/40 px-2.5 py-1.5 text-[11px] text-amber-200/95 leading-snug z-10"
                        role="status"
                      >
                        {topologySupportIssue}
                      </div>
                    )}
                    {topologyLimitedFromConnectionsOnly && (
                      <div
                        className="shrink-0 mx-2 mt-2 rounded-lg border border-sky-600/45 bg-sky-950/30 px-2.5 py-2 text-[11px] text-sky-100/95 leading-snug z-10 pointer-events-auto flex flex-wrap items-center gap-x-3 gap-y-2 justify-between"
                        role="note"
                      >
                        <p className="min-w-0 flex-1 basis-[min(100%,28rem)] m-0">
                          <span className="font-medium text-sky-200/95">Topology rút gọn:</span> chỉ dựa trên cạnh từ
                          edges/connections — không có «top đích» / «top nguồn». Đồ thị suy từ cạnh. Nút bên phải tải
                          đầy đủ (destinations/talkers) như «Làm mới» phía trên.
                        </p>
                        <Button
                          variant="secondary"
                          size="sm"
                          type="button"
                          className="h-8 shrink-0 text-xs border-sky-600/50 bg-sky-950/50 text-sky-100 hover:bg-sky-900/60"
                          disabled={topologyBusy}
                          onClick={() => void handleManualRefresh()}
                        >
                          <RefreshCw className={`w-3.5 h-3.5 mr-1 ${refreshSpin ? 'animate-spin' : ''}`} />
                          Làm mới đầy đủ
                        </Button>
                      </div>
                    )}
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
                        key={`${selectedClusterId}|${namespaceApplied}|${searchApplied}|${podUidApplied}|${sinceMinutes}`}
                        className="flex-1 min-h-0 w-full h-full"
                        destinations={graphDestinations}
                        talkers={graphTalkers}
                        connections={graphConnections}
                        supplementTalkers={graphTalkers}
                        maxNodes={180}
                        showCompactLegend={false}
                        podNamesByUid={topologyPodNamesByUid}
                        onNodeClick={(id, kind) => {
                          if (kind === 'pod') goPod(id);
                        }}
                      />
                    </div>
                  </>
                )}
              </div>
              )}

              {(mainTab === 'pods' || mainTab === 'connections') && (
                <div
                  className={`relative w-full flex-1 flex flex-col min-h-0 border-t border-slate-800 bg-slate-950/50 overflow-hidden ${GRAPH_AREA_CLASS}`}
                >
                  {tableListBusy && (mainTab === 'pods' ? podsRows.length === 0 : connRows.length === 0) ? (
                    <div className="flex flex-col flex-1 min-h-0 justify-center items-center space-y-3 py-8">
                      <div className="w-10 h-10 border-2 border-pink-500 border-t-transparent rounded-full animate-spin" />
                      <span className="text-slate-500 text-sm">Đang tải…</span>
                    </div>
                  ) : mainTab === 'pods' && podsRows.length === 0 ? (
                    <div className="flex flex-1 min-h-0 items-center justify-center p-4">
                      <PageEmpty title="Không có pod" description={emptyContextLine} className="py-12 max-w-md" />
                    </div>
                  ) : mainTab === 'connections' && connRows.length === 0 ? (
                    <div className="flex flex-1 min-h-0 items-center justify-center p-4">
                      <PageEmpty title="Không có dòng connection" description={emptyContextLine} className="py-12 max-w-md" />
                    </div>
                  ) : (
                    <>
                      {tableListBusy && tableRowsLen > 0 && (
                        <div className="absolute top-2 right-2 z-20 flex items-center gap-2 rounded-md bg-slate-900/95 border border-slate-600 px-2 py-1 text-[10px] text-slate-300 shadow-lg">
                          <RefreshCw className="w-3 h-3 animate-spin shrink-0" />
                          Đang làm mới…
                        </div>
                      )}
                      <div className="flex-1 min-h-0 overflow-auto p-2 sm:p-3">
                        {mainTab === 'pods' ? (
                          <table className="w-full text-left text-xs text-slate-200 border-collapse min-w-[640px]">
                            <thead className="sticky top-0 z-10 bg-slate-900/98 border-b border-slate-700 shadow-sm">
                              <tr className="text-[10px] uppercase tracking-wide text-slate-500">
                                <th className="py-2 pr-3 font-medium">Namespace</th>
                                <th className="py-2 pr-3 font-medium">Pod</th>
                                <th
                                  className="py-2 pr-3 font-medium cursor-help"
                                  title="Số bản ghi snapshot theo bucket 5 phút, không phải số kết nối duy nhất."
                                >
                                  Quan sát (bucket)
                                </th>
                                <th className="py-2 pr-3 font-medium">Cập nhật</th>
                                <th className="py-2 pr-3 font-medium">Node</th>
                                <th
                                  className="py-2 pl-2 text-right font-medium"
                                  title="Sao chép UID · Connections · Pod detail"
                                >
                                  Thao tác
                                </th>
                              </tr>
                            </thead>
                            <tbody>
                              {podsRows.map((row) => (
                                <tr
                                  key={row.podUid}
                                  className="border-b border-slate-800/80 hover:bg-slate-900/50 cursor-pointer"
                                  onClick={(e) => {
                                    if ((e.target as HTMLElement).closest('button')) return;
                                    drillConnectionsForPod(row);
                                  }}
                                  title="Click dòng (ngoài nút) để mở tab Connections lọc theo pod"
                                >
                                  <td className="py-2 pr-3 font-mono text-[11px] text-slate-300">{row.namespace}</td>
                                  <td className="py-2 pr-3">{podTableLabel(row.podName, row.podUid)}</td>
                                  <td className="py-2 pr-3 tabular-nums">{row.connectionCount}</td>
                                  <td
                                    className="py-2 pr-3 text-slate-400 whitespace-nowrap"
                                    title={observedAtTooltip(row.lastObservedAt)}
                                  >
                                    {formatObservedAt(row.lastObservedAt)}
                                  </td>
                                  <td className="py-2 pr-3 text-slate-500 text-[11px]">{row.nodeName ?? '—'}</td>
                                  <td className="py-2 pl-2 text-right whitespace-nowrap">
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      type="button"
                                      className="h-7 w-7 p-0 text-slate-500 hover:text-slate-200"
                                      title="Sao chép pod UID"
                                      aria-label="Sao chép pod UID"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        copyToClipboard(`na-pod-${row.podUid}`, row.podUid);
                                      }}
                                    >
                                      {copiedTableKey === `na-pod-${row.podUid}` ? (
                                        <Check className="w-3.5 h-3.5 text-emerald-400" />
                                      ) : (
                                        <Copy className="w-3.5 h-3.5" />
                                      )}
                                    </Button>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      type="button"
                                      className="h-7 px-1.5 text-[10px]"
                                      title="Xem connections (lọc theo pod)"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        drillConnectionsForPod(row);
                                      }}
                                    >
                                      <ChevronRight className="w-3.5 h-3.5 inline mr-0.5" />
                                      Conn
                                    </Button>
                                    <Button
                                      variant="ghost"
                                      size="sm"
                                      type="button"
                                      className="h-7 px-1.5"
                                      title="Pod detail"
                                      onClick={(e) => {
                                        e.stopPropagation();
                                        goPod(row.podUid);
                                      }}
                                    >
                                      <ExternalLink className="w-3.5 h-3.5" />
                                    </Button>
                                  </td>
                                </tr>
                              ))}
                            </tbody>
                          </table>
                        ) : (
                          <table className="w-full text-left text-xs text-slate-200 border-collapse min-w-[64rem]">
                            <thead className="sticky top-0 z-10 bg-slate-900/98 border-b border-slate-700 shadow-sm">
                              <tr className="text-[10px] uppercase tracking-wide text-slate-500">
                                <th
                                  className="py-2 pr-3 font-medium min-w-[9rem] cursor-help"
                                  title="observedAt khi Core nhận (tooltip ô: giờ local + ISO UTC); dòng phụ là mốc đầu bucket 5m UTC."
                                >
                                  Thời gian
                                </th>
                                <th className="py-2 pr-3 font-medium">NS</th>
                                <th className="py-2 pr-3 font-medium" title="Nút sao chép pod UID">
                                  Pod
                                </th>
                                <th
                                  className="py-2 pr-3 font-medium"
                                  title="Nguồn: IP:cổng. Trạng thái LISTEN: hiển thị socket lắng nghe (LISTEN :port @bind); nút sao chép cùng nội dung."
                                >
                                  Local
                                </th>
                                <th
                                  className="py-2 pr-3 font-medium"
                                  title="Đích từ xa. LISTEN: không dùng (hiện —); sao chép chỉ khi có đích."
                                >
                                  Remote
                                </th>
                                <th className="py-2 pr-2 font-medium">Proto</th>
                                <th className="py-2 pr-2 font-medium">State</th>
                                <th
                                  className="py-2 pr-2 font-medium text-right cursor-help"
                                  title="tx_queue từ /proc/net (snapshot), không phải tổng traffic gửi."
                                >
                                  Tx queue
                                </th>
                                <th
                                  className="py-2 pr-2 font-medium text-right cursor-help"
                                  title="rx_queue từ /proc/net (snapshot), không phải tổng traffic nhận."
                                >
                                  Rx queue
                                </th>
                                <th className="py-2 pr-3 font-medium">Owner</th>
                                <th className="py-2 pr-2 font-medium">Node</th>
                                <th
                                  className="py-2 pl-2 text-right font-medium w-[3.25rem]"
                                  title="Mở Pod detail"
                                >
                                  Chi tiết
                                </th>
                              </tr>
                            </thead>
                            <tbody>
                              {connRows.map((row, i) => {
                                const bucket = formatBucket5mLine(row.bucket5m);
                                const k =
                                  row.id != null
                                    ? `c-${row.id}`
                                    : `c-${i}-${row.podUid}-${row.bucket5m ?? ''}-${row.destIp}-${row.destPort}-${row.protocol}`;
                                return (
                                  <tr key={k} className="border-b border-slate-800/80 hover:bg-slate-900/50">
                                    <td className="py-2 pr-3 align-top">
                                      <div
                                        className="text-slate-200 whitespace-nowrap"
                                        title={observedAtTooltip(row.observedAt)}
                                      >
                                        {formatObservedAt(row.observedAt)}
                                      </div>
                                      <div className="text-[10px] text-slate-500 mt-0.5 font-mono" title={bucket.title}>
                                        {bucket.short}
                                      </div>
                                    </td>
                                    <td className="py-2 pr-3 font-mono text-[11px] text-slate-300 align-top">{row.namespace ?? '—'}</td>
                                    <td className="py-2 pr-3 align-top">
                                      <div className="flex items-center gap-0.5 min-w-0">
                                        <span className="min-w-0">{podTableLabel(row.podName, row.podUid)}</span>
                                        {(row.podUid ?? '').trim() ? (
                                          <Button
                                            variant="ghost"
                                            size="sm"
                                            type="button"
                                            className="h-7 w-7 p-0 shrink-0 text-slate-500 hover:text-slate-200"
                                            title="Sao chép pod UID"
                                            aria-label="Sao chép pod UID"
                                            onClick={() =>
                                              copyToClipboard(`na-cu-${k}`, (row.podUid ?? '').trim())
                                            }
                                          >
                                            {copiedTableKey === `na-cu-${k}` ? (
                                              <Check className="w-3.5 h-3.5 text-emerald-400" />
                                            ) : (
                                              <Copy className="w-3.5 h-3.5" />
                                            )}
                                          </Button>
                                        ) : null}
                                      </div>
                                    </td>
                                    <td className="py-2 pr-3 align-top">
                                      <div className="flex items-start gap-0.5 min-w-0">
                                        <span className="font-mono text-[11px] text-slate-400 whitespace-pre-wrap break-all min-w-0">
                                          {formatLocalEndpoint(row)}
                                        </span>
                                        {localEndpointClipboard(row) !== '—' ? (
                                          <Button
                                            variant="ghost"
                                            size="sm"
                                            type="button"
                                            className="h-7 w-7 p-0 shrink-0 text-slate-500 hover:text-slate-200"
                                            title="Sao chép Local (như cột hiển thị)"
                                            aria-label="Sao chép Local"
                                            onClick={() =>
                                              copyToClipboard(`na-cl-${k}`, localEndpointClipboard(row))
                                            }
                                          >
                                            {copiedTableKey === `na-cl-${k}` ? (
                                              <Check className="w-3.5 h-3.5 text-emerald-400" />
                                            ) : (
                                              <Copy className="w-3.5 h-3.5" />
                                            )}
                                          </Button>
                                        ) : null}
                                      </div>
                                    </td>
                                    <td className="py-2 pr-3 align-top">
                                      <div className="flex items-start gap-0.5 min-w-0">
                                        <span className="font-mono text-[11px] text-slate-300 whitespace-pre-wrap break-all min-w-0">
                                          {formatRemoteEndpoint(row)}
                                        </span>
                                        {remoteEndpointClipboard(row) !== '—' ? (
                                          <Button
                                            variant="ghost"
                                            size="sm"
                                            type="button"
                                            className="h-7 w-7 p-0 shrink-0 text-slate-500 hover:text-slate-200"
                                            title="Sao chép remote (như cột hiển thị)"
                                            aria-label="Sao chép remote"
                                            onClick={() =>
                                              copyToClipboard(`na-cr-${k}`, remoteEndpointClipboard(row))
                                            }
                                          >
                                            {copiedTableKey === `na-cr-${k}` ? (
                                              <Check className="w-3.5 h-3.5 text-emerald-400" />
                                            ) : (
                                              <Copy className="w-3.5 h-3.5" />
                                            )}
                                          </Button>
                                        ) : null}
                                      </div>
                                    </td>
                                    <td className="py-2 pr-2 align-top">{row.protocol ?? '—'}</td>
                                    <td className="py-2 pr-2 align-top">{row.state ?? '—'}</td>
                                    <td className="py-2 pr-2 text-right tabular-nums align-top">{formatQueueBytes(row.bytesSent)}</td>
                                    <td className="py-2 pr-2 text-right tabular-nums align-top">{formatQueueBytes(row.bytesRecv)}</td>
                                    <td className="py-2 pr-3 text-[11px] text-slate-400 align-top">
                                      {row.ownerKind || row.ownerName
                                        ? `${row.ownerKind ?? ''}${row.ownerKind && row.ownerName ? '/' : ''}${row.ownerName ?? ''}`
                                        : '—'}
                                    </td>
                                    <td className="py-2 pr-2 text-[11px] text-slate-500 align-top">{row.nodeName ?? '—'}</td>
                                    <td className="py-2 pl-2 text-right align-top whitespace-nowrap">
                                      {(row.podUid ?? '').trim() ? (
                                        <Button
                                          variant="ghost"
                                          size="sm"
                                          type="button"
                                          className="h-7 w-7 p-0"
                                          title="Mở Pod detail"
                                          aria-label="Mở Pod detail"
                                          onClick={() => goPod((row.podUid ?? '').trim())}
                                        >
                                          <ExternalLink className="w-3.5 h-3.5" />
                                        </Button>
                                      ) : (
                                        <span className="text-slate-600">—</span>
                                      )}
                                    </td>
                                  </tr>
                                );
                              })}
                            </tbody>
                          </table>
                        )}
                      </div>
                      <div className="shrink-0 border-t border-slate-800 px-2 py-2 flex flex-wrap items-center justify-between gap-2 bg-slate-950/90">
                        <div className="flex items-center gap-2 text-[10px] text-slate-500">
                          <span>Số dòng/trang</span>
                          <select
                            value={tablePageSize}
                            onChange={(e) => setTablePageSize(Number(e.target.value))}
                            className="bg-slate-900 border border-slate-700 rounded px-1.5 py-1 text-slate-200"
                          >
                            {TABLE_PAGE_SIZES.map((n) => (
                              <option key={n} value={n}>
                                {n}
                              </option>
                            ))}
                          </select>
                        </div>
                        <div className="flex items-center gap-1">
                          <Button
                            variant="secondary"
                            size="sm"
                            type="button"
                            className="h-8 px-2"
                            disabled={tablePage <= 1 || tableListBusy}
                            onClick={() => setTablePage((p) => Math.max(1, p - 1))}
                          >
                            <ChevronLeft className="w-4 h-4" />
                          </Button>
                          <span className="text-[11px] text-slate-400 tabular-nums min-w-[5rem] text-center">
                            {tablePage} / {tableTotalPages}
                          </span>
                          <Button
                            variant="secondary"
                            size="sm"
                            type="button"
                            className="h-8 px-2"
                            disabled={tablePage >= tableTotalPages || tableListBusy}
                            onClick={() => setTablePage((p) => p + 1)}
                          >
                            <ChevronRight className="w-4 h-4" />
                          </Button>
                        </div>
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>
            {copyErrorToast && (
              <div
                className="pointer-events-none absolute bottom-3 left-1/2 z-[80] max-w-[min(100%,22rem)] -translate-x-1/2 rounded-lg border border-red-500/40 bg-slate-950/95 px-3 py-2 text-center text-[11px] leading-snug text-red-200 shadow-lg"
                role="alert"
              >
                {copyErrorToast}
              </div>
            )}
            {sinceCancelToast && (
              <div
                className="pointer-events-none absolute z-[79] max-w-[min(100%,22rem)] -translate-x-1/2 rounded-lg border border-slate-600/60 bg-slate-900/95 px-3 py-2 text-center text-[11px] leading-snug text-slate-300 shadow-lg left-1/2"
                style={{ bottom: copyErrorToast ? '4.25rem' : '0.75rem' }}
                role="status"
              >
                {sinceCancelToast}
              </div>
            )}
          </Card>
        </div>
      )}
    </PageLayout>
  );
}
