import React, { useCallback, useEffect, useState } from 'react';
import { CapabilityMetadataBrowser } from '../components/CapabilityMetadataBrowser';
import { Card } from '../design-system/components/Card';
import { RefreshCw } from 'lucide-react';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { FilterBar } from '../design-system/components/FilterBar';
import { PageEmpty, PageError, PageLoading } from '../design-system/components/PageStatus';
import { Pagination } from '../components/Pagination';
import { Button } from '../components/ui/Button';
import { api } from '../lib/api';
import { UI_FILTER_SELECT } from '../lib/formChrome';
import { CapabilityMetadata, SecurityRule } from '../types';
import { usePolling, REFRESH_INTERVALS } from '../hooks/usePolling';
import { useRefreshIntervalStore } from '../store/refreshIntervalStore';
import { useRefreshTriggerStore } from '../store/refreshTriggerStore';
import { PAGE_TITLES } from '../lib/pageTitles';

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100];

export const Capabilities: React.FC = () => {
  const [metadata, setMetadata] = useState<CapabilityMetadata[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(25);
  const [searchTerm, setSearchTerm] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [domain, setDomain] = useState<string>('all');
  const [domains, setDomains] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [rules, setRules] = useState<SecurityRule[]>([]);
  const [rulesError, setRulesError] = useState<string | null>(null);

  useEffect(() => {
    const t = window.setTimeout(() => setDebouncedSearch(searchTerm.trim()), 300);
    return () => window.clearTimeout(t);
  }, [searchTerm]);

  useEffect(() => {
    setPage(1);
  }, [debouncedSearch, domain, pageSize]);

  const loadDomainsOnce = useCallback(async () => {
    try {
      const res = await api.getCapabilityMetadata({ limit: 2000, offset: 0 });
      const d = new Set<string>();
      for (const m of res.metadata) {
        if (m.domain) d.add(m.domain);
      }
      setDomains(Array.from(d).sort());
    } catch {
      setDomains([]);
    }
  }, []);

  const loadPage = useCallback(async () => {
    setError(null);
    setLoading(true);
    try {
      const offset = (page - 1) * pageSize;
      const res = await api.getCapabilityMetadata({
        search: debouncedSearch || undefined,
        domain: domain === 'all' ? undefined : domain,
        limit: pageSize,
        offset,
      });
      setMetadata(res.metadata);
      setTotal(res.total);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load capability metadata');
      setMetadata([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [page, pageSize, debouncedSearch, domain]);

  const loadRules = useCallback(async () => {
    try {
      const r = await api.getRules();
      setRules(r);
      setRulesError(null);
    } catch {
      setRules([]);
      setRulesError('Detection rule enrichment is unavailable. Capability metadata is still shown without related rule counts.');
    }
  }, []);

  const intervalMs = useRefreshIntervalStore((s) => s.getIntervalMs(REFRESH_INTERVALS.STATS_CLUSTERS));
  const refreshTrigger = useRefreshTriggerStore((s) => s.trigger);
  usePolling(loadPage, intervalMs, { refreshTrigger });
  usePolling(loadRules, intervalMs, { refreshTrigger });

  useEffect(() => {
    loadDomainsOnce();
  }, [loadDomainsOnce]);

  if (loading && metadata.length === 0 && !error) {
    return <PageLoading message="Loading capability knowledge…" className="min-h-[40dvh]" />;
  }

  return (
    <PageLayout
      title={PAGE_TITLES.capabilities}
      description="Semantic reference for capabilities (MITRE, impact, mitigations). Use Risk Findings for incident triage."
      actions={
        <Button variant="secondary" isLoading={loading} onClick={() => { loadPage(); loadRules(); }}>
          <RefreshCw className="w-4 h-4 mr-2" /> Refresh
        </Button>
      }
      toolbar={
        <FilterBar
          embedded
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: 'Search id, name, summary, MITRE, kill chain…',
            inputClassName: 'max-w-md',
          }}
          trailing={
            <select
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              className={`${UI_FILTER_SELECT} sm:w-56 focus:ring-2 focus:ring-brand/30`}
            >
              <option value="all">All domains</option>
              {domains.map((d) => (
                <option key={d} value={d}>
                  {d}
                </option>
              ))}
            </select>
          }
        />
      }
    >
      {error && (
        <PageError
          title="Could not load capabilities"
          description={error}
          action={<Button variant="secondary" onClick={loadPage} isLoading={loading}>Retry capabilities</Button>}
          className="mb-4 rounded-lg border border-warning/30 bg-warning/10"
        />
      )}
      {rulesError && metadata.length > 0 ? (
        <div className="mb-4 rounded-lg border border-warning/30 bg-warning/10 px-4 py-3 text-body text-warning" role="status">
          {rulesError}
        </div>
      ) : null}

      <Card className="p-0 overflow-hidden">
        {metadata.length === 0 && !loading ? (
          <div className="p-8">
            <PageEmpty title="No capabilities" description="Adjust search or domain, or sync capability metadata in Core." className="py-6" />
          </div>
        ) : (
          <div className="p-0">
            <CapabilityMetadataBrowser metadata={metadata} rules={rules} loading={false} />
          </div>
        )}
      </Card>

      {searchTerm.trim() !== debouncedSearch && (
        <p className="text-caption text-muted mt-2">Updating search (300ms debounce)…</p>
      )}

      {total > 0 && (
        <Pagination
          page={page}
          pageSize={pageSize}
          total={total}
          onPageChange={setPage}
          onPageSizeChange={(size) => {
            setPageSize(size);
            setPage(1);
          }}
          pageSizeOptions={PAGE_SIZE_OPTIONS}
          itemLabel="capabilities"
        />
      )}
    </PageLayout>
  );
};
