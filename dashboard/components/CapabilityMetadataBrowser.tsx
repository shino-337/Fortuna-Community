import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { CapabilityMetadata } from '../types';
import { Card } from './ui/Card';
import { Search, Shield, Info, AlertTriangle, CheckCircle2 } from 'lucide-react';

export const CapabilityMetadataBrowser: React.FC = () => {
  const [metadata, setMetadata] = useState<CapabilityMetadata[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedDomain, setSelectedDomain] = useState<string>('all');

  useEffect(() => {
    api.getCapabilityMetadata().then((data) => {
      setMetadata(data);
      setLoading(false);
    }).catch(() => {
      setLoading(false);
    });
  }, []);

  const domains = Array.from(new Set(metadata.map(m => m.domain)));

  const filteredMetadata = metadata.filter(m => {
    const matchesSearch = m.capabilityId.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         m.description.toLowerCase().includes(searchTerm.toLowerCase()) ||
                         m.category.toLowerCase().includes(searchTerm.toLowerCase());
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
      <div className="flex gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-slate-400" size={16} />
          <input
            type="text"
            placeholder="Search capabilities..."
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
        {filteredMetadata.map((meta) => (
          <div
            key={meta.capabilityId}
            className="bg-slate-900/70 border border-slate-800 rounded-lg p-4 hover:border-slate-700 transition-colors"
          >
            <div className="flex items-start justify-between mb-3">
              <div className="flex-1">
                <div className="flex items-center gap-2 mb-2">
                  <Shield size={16} className="text-pink-400" />
                  <span className="font-semibold text-white">{meta.capabilityId}</span>
                  <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${getSeverityColor(meta.severityBase)}`}>
                    {meta.severityBase}
                  </span>
                  <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-800 text-slate-400 border border-slate-700">
                    {meta.domain}
                  </span>
                </div>
                <p className="text-sm text-slate-300 mb-2">{meta.description}</p>
                <div className="flex items-center gap-4 text-xs text-slate-500">
                  <span>Category: {meta.category}</span>
                  <span>Confidence Base: {Math.round(meta.confidenceBase * 100)}%</span>
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
          </div>
        ))}
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
