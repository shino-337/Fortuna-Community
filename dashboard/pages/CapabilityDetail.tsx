import React, { useCallback, useEffect, useState } from 'react';
import { Link, matchPath, useLocation, useNavigate, useParams } from 'react-router-dom';
import { api } from '../lib/api';
import { CapabilityMetadata, SecurityRule } from '../types';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { Card } from '../design-system/components/Card';
import { Button } from '../components/ui/Button';
import { ArrowLeft, ExternalLink, Info, AlertTriangle, CheckCircle2 } from 'lucide-react';
import { getSeverityBadgeClass } from '../lib/severity';
import { PageLoading } from '../design-system/components/PageStatus';

export const CapabilityDetail: React.FC = () => {
  const params = useParams<{ id: string }>();
  const location = useLocation();
  const id = params.id ?? matchPath({ path: '/capabilities/:id', end: true }, location.pathname)?.params.id;
  const navigate = useNavigate();
  const [meta, setMeta] = useState<CapabilityMetadata | null>(null);
  const [rules, setRules] = useState<SecurityRule[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!id) {
      setMeta(null);
      setRules([]);
      setLoadError('Capability id is missing from the route.');
      setLoading(false);
      return;
    }
    setLoading(true);
    setLoadError(null);
    try {
      const [m, r] = await Promise.all([
        api.getCapabilityMetadataById(id),
        api.getRules().catch(() => []),
      ]);
      setMeta(m);
      setRules(r);
      if (!m) {
        setLoadError('Capability metadata was not found for this id.');
      }
    } catch (err) {
      setMeta(null);
      setRules([]);
      setLoadError(err instanceof Error ? err.message : 'Capability detail could not be loaded.');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const linkedRules = (rules || []).filter((r) => (r.relatedCapabilities || []).includes(id || ''));

  if (loading) {
    return <PageLoading message="Loading capability…" className="min-h-[40dvh]" />;
  }

  if (!meta) {
    return (
      <PageLayout title="Capability not found" description={loadError ?? 'Unknown or removed capability id.'}>
        <Button variant="secondary" onClick={() => navigate('/capabilities')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back
        </Button>
      </PageLayout>
    );
  }

  const tooltipLabelClass = 'inline-flex items-center gap-1 underline decoration-dotted underline-offset-2 cursor-help';

  return (
    <PageLayout
      title={meta.name || meta.capabilityId}
      description={meta.summary || meta.description}
      actions={
        <Button variant="secondary" onClick={() => navigate('/capabilities')}>
          <ArrowLeft className="w-4 h-4 mr-2" /> Back to list
        </Button>
      }
    >
      <Card className="p-6 mb-6">
        <div className="flex flex-wrap items-center gap-3 text-body">
          <span className={`px-2 py-0.5 rounded text-caption font-medium border ${getSeverityBadgeClass(meta.severityBase?.toLowerCase())}`}>{meta.severityBase}</span>
          <span className="text-muted">Domain: {meta.domain}</span>
          <span className="text-muted">Category: {meta.category}</span>
          <span className="text-muted">Confidence: {Math.round((meta.confidenceBase ?? 0) * 100)}%</span>
          {meta.supportsRuntimePromotion && (
            <span className="flex items-center gap-1 text-green-400 text-caption">
              <CheckCircle2 size={12} /> Runtime promotion
            </span>
          )}
        </div>
        <p className="text-caption text-muted font-mono mt-3">{meta.capabilityId}</p>
        {(meta.mitreTactic || meta.mitreTechnique) && (
          <p className="text-caption text-muted mt-2">
            MITRE: {meta.mitreTactic} {meta.mitreTechnique && <span className="font-mono text-amber-400">{meta.mitreTechnique}</span>}
            {meta.mitreSubtechnique && ` ${meta.mitreSubtechnique}`}
          </p>
        )}
        {meta.killChainStage && <p className="text-caption text-indigo-300 mt-1">Kill chain: {meta.killChainStage}</p>}
      </Card>

      <div className="grid gap-6 lg:grid-cols-2">
        <Card className="p-6">
          <h3 className="text-body font-semibold text-text mb-3 flex items-center gap-2">
            <Info className="w-4 h-4 text-brand" /> Linked rules
          </h3>
          {linkedRules.length === 0 ? (
            <p className="text-body text-muted">No rules reference this capability in recent analytics.</p>
          ) : (
            <ul className="space-y-2 text-body text-text">
              {linkedRules.slice(0, 20).map((r) => (
                <li key={r.id} className="flex justify-between gap-2 border-b border-border/80 pb-2">
                  <span className="truncate">{r.name}</span>
                  <button type="button" className="text-brand text-caption shrink-0 hover:underline" onClick={() => navigate(`/rules/uid/${encodeURIComponent(r.id)}`)}>
                    Open
                  </button>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card className="p-6">
          <h3 className="text-body font-semibold text-text mb-3">Preconditions</h3>
          {meta.preconditions && meta.preconditions.length > 0 ? (
            <div className="flex flex-wrap gap-2">
              {meta.preconditions.map((pre, idx) => (
                <span key={idx} className="px-2 py-1 rounded text-caption bg-blue-500/20 text-blue-400 border border-blue-500/30">
                  {pre}
                </span>
              ))}
            </div>
          ) : (
            <p className="text-body text-muted">None listed.</p>
          )}
          {meta.producesAttackSteps && meta.producesAttackSteps.length > 0 && (
            <div className="mt-6">
              <h4 className="text-caption font-semibold text-muted mb-2 flex items-center gap-1">
                <AlertTriangle size={12} className="text-rose-400" /> Produces attack steps
              </h4>
              <div className="flex flex-wrap gap-2">
                {meta.producesAttackSteps.map((step, idx) => (
                  <span key={idx} className="px-2 py-1 rounded text-caption bg-rose-500/20 text-rose-400 border border-rose-500/30">
                    {step}
                  </span>
                ))}
              </div>
            </div>
          )}
        </Card>
      </div>

      {meta.fullDescription && (
        <Card className="p-6 mt-6">
          <h3 className="text-body font-semibold text-text mb-2">Full description</h3>
          <p className="text-body text-text whitespace-pre-wrap">{meta.fullDescription}</p>
        </Card>
      )}

      {meta.technicalIndicators && meta.technicalIndicators.length > 0 && (
        <Card className="p-6 mt-6">
          <h3 className="text-body font-semibold text-text mb-2">Technical indicators</h3>
          <ul className="list-disc list-inside text-body text-text space-y-1">
            {meta.technicalIndicators.map((t, i) => (
              <li key={i}>{t}</li>
            ))}
          </ul>
        </Card>
      )}

      {(meta.impact && meta.impact.length > 0) || (meta.recommendedMitigations && meta.recommendedMitigations.length > 0) ? (
        <div className="grid gap-6 lg:grid-cols-2 mt-6">
          {meta.impact && meta.impact.length > 0 && (
            <Card className="p-6">
              <h3 className="text-body font-semibold text-text mb-2">Impact</h3>
              <ul className="list-disc list-inside text-body text-text space-y-1">
                {meta.impact.map((x, i) => (
                  <li key={i}>{x}</li>
                ))}
              </ul>
            </Card>
          )}
          {meta.recommendedMitigations && meta.recommendedMitigations.length > 0 && (
            <Card className="p-6">
              <h3 className="text-body font-semibold text-green-400/90 mb-2">Recommended mitigations</h3>
              <ul className="list-disc list-inside text-body text-text space-y-1">
                {meta.recommendedMitigations.map((x, i) => (
                  <li key={i}>{x}</li>
                ))}
              </ul>
            </Card>
          )}
        </div>
      ) : null}

      {meta.falsePositiveConsiderations && meta.falsePositiveConsiderations.length > 0 && (
        <Card className="p-6 mt-6">
          <h3 className="text-body font-semibold text-amber-400/90 mb-2">False positive considerations</h3>
          <ul className="list-disc list-inside text-body text-text space-y-1">
            {meta.falsePositiveConsiderations.map((x, i) => (
              <li key={i}>{x}</li>
            ))}
          </ul>
        </Card>
      )}

      {meta.references && meta.references.length > 0 && (
        <Card className="p-6 mt-6">
          <h3 className="text-body font-semibold text-text mb-2">References</h3>
          <ul className="space-y-2">
            {meta.references.map((url, i) => (
              <li key={i}>
                <a href={url} target="_blank" rel="noopener noreferrer" className="text-body text-brand hover:underline inline-flex items-center gap-1">
                  <ExternalLink size={12} />
                  {url}
                </a>
              </li>
            ))}
          </ul>
        </Card>
      )}

      <p className="text-caption text-muted-2 mt-8">
        <span className={tooltipLabelClass} title="Operational context">
          Note <Info className="w-3 h-3 inline" />
        </span>
        : For incident triage use <Link to="/risks/findings" className="text-brand hover:underline">Risk Findings</Link>.
      </p>
    </PageLayout>
  );
};
