import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { CapabilityMetadata, SecurityRule } from '../types';
import { Search, Shield, Info, AlertTriangle, CheckCircle2, ChevronDown, ChevronRight, ExternalLink } from 'lucide-react';

export const CapabilityMetadataBrowser: React.FC = () => {
  const [metadata, setMetadata] = useState<CapabilityMetadata[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedDomain, setSelectedDomain] = useState<string>('all');
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [rules, setRules] = useState<SecurityRule[]>([]);

  useEffect(() => {
    api.getCapabilityMetadata().then((data) => {
      setMetadata(data);
      setLoading(false);
    }).catch(() => {
      setLoading(false);
    });
  }, []);

  useEffect(() => {
    api.getRules().then(setRules).catch(() => setRules([]));
  }, []);

  const domains = Array.from(new Set(metadata.map(m => m.domain)));

  const searchLower = searchTerm.toLowerCase();
  const filteredMetadata = metadata.filter(m => {
    const matchesSearch = !searchTerm ||
      m.capabilityId.toLowerCase().includes(searchLower) ||
      m.description?.toLowerCase().includes(searchLower) ||
      m.category?.toLowerCase().includes(searchLower) ||
      (m.name && m.name.toLowerCase().includes(searchLower)) ||
      (m.summary && m.summary.toLowerCase().includes(searchLower)) ||
      (m.mitreTechnique && m.mitreTechnique.toLowerCase().includes(searchLower)) ||
      (m.killChainStage && m.killChainStage.toLowerCase().includes(searchLower));
    const matchesDomain = selectedDomain === 'all' || m.domain === selectedDomain;
    return matchesSearch && matchesDomain;
  });

  const getSeverityColor = (severity: string) => {
    const colors: Record<string, string> = {
      'CRITICAL': 'bg-red-500/20 text-red-400 border-red-500/50',
      'HIGH': 'bg-orange-500/20 text-orange-400 border-orange-500/50',
      'MEDIUM': 'bg-yellow-500/20 text-yellow-400 border-yellow-500/50',
      'LOW': 'bg-blue-500/20 text-blue-400 border-blue-500/50',
    };
    return colors[severity] || 'bg-slate-500/20 text-slate-400 border-slate-500/50';
  };

  const toggleExpanded = (id: string) => {
    setExpandedId(prev => prev === id ? null : id);
  };

  const capabilityRuleStats = (capabilityId: string): { count: number; topRules: SecurityRule[] } => {
    const matches = rules.filter((r) => (r.relatedCapabilities || []).includes(capabilityId));
    const sorted = [...matches].sort((a, b) => (b.impactedFindings7d || 0) - (a.impactedFindings7d || 0));
    return { count: matches.length, topRules: sorted.slice(0, 3) };
  };
  const tooltipLabelClass = 'inline-flex items-center gap-1 underline decoration-dotted underline-offset-2 cursor-help';

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <div className="w-10 h-10 border-4 border-pink-500 border-t-transparent rounded-full animate-spin"></div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <div className="flex-1 min-w-[200px] relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-slate-400" size={16} />
          <input
            type="text"
            placeholder="Search by ID, name, summary, MITRE, kill chain..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-pink-500/50"
          />
        </div>
        <select
          value={selectedDomain}
          onChange={(e) => setSelectedDomain(e.target.value)}
          className="px-4 py-2 bg-slate-900/50 border border-slate-800 rounded-lg text-white focus:outline-none focus:border-pink-500/50"
        >
          <option value="all">All Domains</option>
          {domains.map(domain => (
            <option key={domain} value={domain}>{domain}</option>
          ))}
        </select>
      </div>

      {/* Metadata List */}
      <div className="grid grid-cols-1 gap-4">
        {filteredMetadata.map((meta) => {
          const hasExtended = meta.summary || meta.fullDescription || (meta.mitreTechnique || meta.mitreTactic) ||
            meta.killChainStage || (meta.technicalIndicators && meta.technicalIndicators.length > 0) ||
            (meta.impact && meta.impact.length > 0) || (meta.recommendedMitigations && meta.recommendedMitigations.length > 0) ||
            (meta.falsePositiveConsiderations && meta.falsePositiveConsiderations.length > 0) ||
            (meta.references && meta.references.length > 0);
          const isExpanded = expandedId === meta.capabilityId;
          const triggerStats = capabilityRuleStats(meta.capabilityId);

          return (
            <div
              key={meta.capabilityId}
              className="bg-slate-900/70 border border-slate-800 rounded-lg overflow-hidden hover:border-slate-700 transition-colors"
            >
              {/* Header - always visible */}
              <div className="p-4">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex-1 min-w-0">
                    <div className="flex flex-wrap items-center gap-2 mb-2">
                      <button
                        type="button"
                        onClick={() => hasExtended && toggleExpanded(meta.capabilityId)}
                        className="flex items-center gap-1 text-left"
                      >
                        {hasExtended ? (
                          isExpanded ? <ChevronDown size={16} className="text-slate-400 shrink-0" /> : <ChevronRight size={16} className="text-slate-400 shrink-0" />
                        ) : (
                          <span className="w-4 shrink-0" />
                        )}
                        <Shield size={16} className="text-pink-400 shrink-0" />
                        <span className="font-semibold text-white">
                          {meta.name || meta.capabilityId}
                        </span>
                        {meta.name && (
                          <span className="text-slate-500 font-mono text-xs truncate">({meta.capabilityId})</span>
                        )}
                      </button>
                      <span className={`px-2 py-0.5 rounded text-[10px] font-medium border shrink-0 ${getSeverityColor(meta.severityBase)}`}>
                        {meta.severityBase}
                      </span>
                      <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-800 text-slate-400 border border-slate-700 shrink-0">
                        {meta.domain}
                      </span>
                      {meta.killChainStage && (
                        <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-indigo-500/20 text-indigo-300 border border-indigo-500/30 shrink-0">
                          {meta.killChainStage}
                        </span>
                      )}
                    </div>
                    <p className="text-sm text-slate-300 mb-2 pl-6">
                      {meta.summary || meta.description}
                    </p>
                    {(meta.mitreTactic || meta.mitreTechnique) && (
                      <div className="flex flex-wrap items-center gap-2 pl-6 text-xs text-slate-400">
                        {meta.mitreTactic && <span>MITRE: {meta.mitreTactic}</span>}
                        {meta.mitreTechnique && <span className="font-mono text-amber-400">{meta.mitreTechnique}</span>}
                        {meta.mitreSubtechnique && <span>{meta.mitreSubtechnique}</span>}
                      </div>
                    )}
                    <div className="flex flex-wrap items-center gap-4 pl-6 mt-2 text-xs text-slate-500">
                      <span>Category: {meta.category}</span>
                      <span>Confidence Base: {Math.round((meta.confidenceBase ?? 0) * 100)}%</span>
                      <span title="Number of rules whose matched findings frequently carry this capability in evidence">
                        <span className={tooltipLabelClass}>Triggered by rules <Info size={12} /></span>: {triggerStats.count}
                      </span>
                      {meta.supportsRuntimePromotion && (
                        <span className="flex items-center gap-1 text-green-400">
                          <CheckCircle2 size={12} />
                          Runtime Promotion
                        </span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Preconditions */}
                {meta.preconditions && meta.preconditions.length > 0 && (
                  <div className="mt-3 pt-3 border-t border-slate-800">
                    <div className="text-xs font-semibold text-slate-400 mb-2">Preconditions:</div>
                    <div className="flex flex-wrap gap-2">
                      {meta.preconditions.map((pre, idx) => (
                        <span key={idx} className="px-2 py-1 rounded text-[10px] bg-blue-500/20 text-blue-400 border border-blue-500/30">
                          {pre}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {/* Attack Steps */}
                {meta.producesAttackSteps && meta.producesAttackSteps.length > 0 && (
                  <div className="mt-3 pt-3 border-t border-slate-800">
                    <div className="text-xs font-semibold text-slate-400 mb-2 flex items-center gap-1">
                      <AlertTriangle size={12} className="text-rose-400" />
                      Produces Attack Steps:
                    </div>
                    <div className="flex flex-wrap gap-2">
                      {meta.producesAttackSteps.map((step, idx) => (
                        <span key={idx} className="px-2 py-1 rounded text-[10px] bg-rose-500/20 text-rose-400 border border-rose-500/30">
                          {step}
                        </span>
                      ))}
                    </div>
                  </div>
                )}

                {/* Expanded: full description + technical indicators, impact, mitigations, false positives, references */}
                {hasExtended && isExpanded && (
                  <div className="mt-4 pt-4 border-t border-slate-700 space-y-4">
                    <div>
                      <div className="text-xs font-semibold text-slate-400 mb-1">
                        <span className={tooltipLabelClass} title="Top 3 rules most associated with this capability over recent 7-day impacted findings">
                          Triggered by Rules <Info size={12} />
                        </span>
                      </div>
                      {triggerStats.topRules.length === 0 ? (
                        <p className="text-sm text-slate-500">No linked rules in recent rule analytics.</p>
                      ) : (
                        <ul className="space-y-1 text-sm text-slate-300">
                          {triggerStats.topRules.map((rule) => (
                            <li key={rule.id} className="flex items-center justify-between gap-2">
                              <span className="truncate">
                                {rule.name} <span className="text-slate-500 text-xs">({rule.id})</span>
                              </span>
                              <span className="text-xs text-slate-500 whitespace-nowrap">{rule.impactedFindings7d ?? 0} / 7d</span>
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                    {meta.fullDescription && (
                      <div>
                        <div className="text-xs font-semibold text-slate-400 mb-1">Full Description</div>
                        <p className="text-sm text-slate-300">{meta.fullDescription}</p>
                      </div>
                    )}
                    {meta.technicalIndicators && meta.technicalIndicators.length > 0 && (
                      <div>
                        <div className="text-xs font-semibold text-slate-400 mb-1">Technical Indicators</div>
                        <ul className="list-disc list-inside text-sm text-slate-300 space-y-1">
                          {meta.technicalIndicators.map((t, i) => (
                            <li key={i}>{t}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {meta.impact && meta.impact.length > 0 && (
                      <div>
                        <div className="text-xs font-semibold text-slate-400 mb-1">Impact</div>
                        <ul className="list-disc list-inside text-sm text-slate-300 space-y-1">
                          {meta.impact.map((i, idx) => (
                            <li key={idx}>{i}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {meta.recommendedMitigations && meta.recommendedMitigations.length > 0 && (
                      <div>
                        <div className="text-xs font-semibold text-green-400/80 mb-1">Recommended Mitigations</div>
                        <ul className="list-disc list-inside text-sm text-slate-300 space-y-1">
                          {meta.recommendedMitigations.map((m, i) => (
                            <li key={i}>{m}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {meta.falsePositiveConsiderations && meta.falsePositiveConsiderations.length > 0 && (
                      <div>
                        <div className="text-xs font-semibold text-amber-400/80 mb-1">False Positive Considerations</div>
                        <ul className="list-disc list-inside text-sm text-slate-300 space-y-1">
                          {meta.falsePositiveConsiderations.map((f, i) => (
                            <li key={i}>{f}</li>
                          ))}
                        </ul>
                      </div>
                    )}
                    {meta.references && meta.references.length > 0 && (
                      <div>
                        <div className="text-xs font-semibold text-slate-400 mb-1">References</div>
                        <ul className="space-y-1">
                          {meta.references.map((url, i) => (
                            <li key={i}>
                              <a
                                href={url}
                                target="_blank"
                                rel="noopener noreferrer"
                                className="text-sm text-pink-400 hover:underline flex items-center gap-1"
                              >
                                <ExternalLink size={12} />
                                {url}
                              </a>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )}
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {filteredMetadata.length === 0 && (
        <div className="text-center py-12 text-slate-500">
          <Info size={24} className="mx-auto mb-2 opacity-50" />
          <p>No capabilities found matching your search.</p>
        </div>
      )}
    </div>
  );
};
