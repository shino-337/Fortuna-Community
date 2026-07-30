import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { PodCapabilityDetail } from '../types';
import { AlertTriangle, Shield, Clock } from 'lucide-react';

interface CapabilityStateChartProps {
  podUid: string;
  capabilityId: string;
}

export const CapabilityStateChart: React.FC<CapabilityStateChartProps> = ({ podUid, capabilityId }) => {
  const [currentCapability, setCurrentCapability] = useState<PodCapabilityDetail | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        // Get current capability details
        const capabilities = await api.getPodCapabilities(podUid);
        const cap = capabilities.find(c => c.capabilityId === capabilityId);
        if (cap) {
          setCurrentCapability(cap);
        }
      } catch {
        setCurrentCapability(null);
      } finally {
        setLoading(false);
      }
    };

    if (podUid && capabilityId) {
      fetchData();
    }
  }, [podUid, capabilityId]);

  const getStateClass = (state: string) => {
    const classes: Record<string, string> = {
      detected: 'bg-info/10 text-info border-info/40',
      confirmed: 'bg-warning/10 text-warning border-warning/40',
      exploited: 'bg-critical/10 text-critical border-critical/40',
      chained: 'bg-brand/10 text-brand border-brand/40',
    };
    return classes[state] || 'bg-muted/10 text-muted border-border';
  };

  const getStateLabel = (state: string) => {
    return state.charAt(0).toUpperCase() + state.slice(1);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="w-8 h-8 border-4 border-brand border-t-transparent rounded-full animate-spin"></div>
      </div>
    );
  }

  if (!currentCapability) {
    return (
      <div className="text-center py-8 text-muted">
        <Shield size={24} className="mx-auto mb-2 opacity-50" />
        <p className="text-body">Capability not found.</p>
      </div>
    );
  }

  const currentState = currentCapability.state || 'detected';
  const confidenceLabel =
    currentCapability.confidence != null
      ? `${Math.round(currentCapability.confidence * 100)}% confidence`
      : 'Confidence unavailable';

  return (
    <div className="space-y-4">
      {/* Current State Info */}
      <div className="bg-surface/70 border border-border rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Shield size={16} className="text-brand" />
              <span className="font-semibold text-text">{capabilityId}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className={`px-2 py-1 rounded text-caption font-medium border ${getStateClass(currentState)}`}>
              {getStateLabel(currentState)}
            </span>
            <span className="text-caption text-muted">
              {confidenceLabel}
            </span>
          </div>
        </div>
        
        <div className="grid grid-cols-2 gap-4 text-caption">
          <div>
            <span className="text-muted">First Seen:</span>
            <span className="text-text ml-2">
              {currentCapability.createdAt ? new Date(currentCapability.createdAt).toLocaleString() : 'N/A'}
            </span>
          </div>
          <div>
            <span className="text-muted">Last Updated:</span>
            <span className="text-text ml-2">
              {currentCapability.updatedAt ? new Date(currentCapability.updatedAt).toLocaleString() : 'N/A'}
            </span>
          </div>
        </div>
      </div>

      <div className="bg-amber-950/20 border border-amber-500/30 rounded-lg p-4">
        <div className="flex items-start gap-3">
          <AlertTriangle size={16} className="text-amber-300 mt-0.5 shrink-0" />
          <div>
            <h3 className="text-body font-semibold text-amber-100">State history unavailable</h3>
            <p className="text-caption text-amber-100/80 mt-1 leading-relaxed">
              Only the current capability state is available for this workload. No transition timeline is rendered because the API does not provide timestamped state-change evidence.
            </p>
            <div className="mt-3 flex flex-wrap items-center gap-3 text-caption text-muted">
              <span className="inline-flex items-center gap-1">
                <Clock size={13} />
                First seen: {currentCapability.createdAt ? new Date(currentCapability.createdAt).toLocaleString() : 'N/A'}
              </span>
              <span>
                Last updated: {currentCapability.updatedAt ? new Date(currentCapability.updatedAt).toLocaleString() : 'N/A'}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* State Legend */}
      <div className="flex items-center gap-4 text-caption">
        <span className="text-muted">State model:</span>
        {['detected', 'confirmed', 'exploited', 'chained'].map((state) => (
          <div key={state} className="flex items-center gap-1">
            <div className={`w-3 h-3 rounded-full border ${getStateClass(state)}`} />
            <span className="text-muted">{getStateLabel(state)}</span>
          </div>
        ))}
      </div>
    </div>
  );
};
