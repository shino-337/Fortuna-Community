import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { PodAttackStep } from '../types';
import { Card } from './ui/Card';
import { AlertTriangle, Shield, TrendingUp, Clock } from 'lucide-react';

interface AttackStepsTableProps {
  podUid: string;
}

export const AttackStepsTable: React.FC<AttackStepsTableProps> = ({ podUid }) => {
  const [steps, setSteps] = useState<PodAttackStep[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!podUid) {
      setLoading(false);
      return;
    }

    api.getPodAttackSteps(podUid).then((data) => {
      setSteps(data);
      setLoading(false);
    }).catch(() => {
      setLoading(false);
    });
  }, [podUid]);

  const getCategoryColor = (category: string) => {
    const colors: Record<string, string> = {
      'FILESYSTEM': 'bg-red-500/20 text-red-400 border-red-500/30',
      'KERNEL': 'bg-orange-500/20 text-orange-400 border-orange-500/30',
      'PROCESS': 'bg-yellow-500/20 text-yellow-400 border-yellow-500/30',
      'IPC': 'bg-amber-500/20 text-amber-400 border-amber-500/30',
      'CREDENTIALS': 'bg-purple-500/20 text-purple-400 border-purple-500/30',
      'PERSISTENCE': 'bg-pink-500/20 text-pink-400 border-pink-500/30',
      'ESCAPE': 'bg-rose-500/20 text-rose-400 border-rose-500/30',
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

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="w-8 h-8 border-4 border-pink-500 border-t-transparent rounded-full animate-spin"></div>
      </div>
    );
  }

  if (steps.length === 0) {
    return (
      <div className="text-center py-8 text-slate-500">
        <Shield size={24} className="mx-auto mb-2 opacity-50" />
        <p className="text-sm">No attack steps detected for this pod.</p>
        <p className="text-xs mt-1 opacity-75">Attack steps are generated when capabilities reach "exploited" state.</p>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {steps.map((step, idx) => (
        <div
          key={`${step.stepId}-${idx}`}
          className="bg-slate-900/70 border border-slate-800 rounded-lg p-4 hover:border-slate-700 transition-colors"
        >
          <div className="flex items-start justify-between mb-2">
            <div className="flex items-center gap-2">
              <AlertTriangle size={16} className="text-rose-400" />
              <span className="font-semibold text-white text-sm">{step.stepId}</span>
              <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${getCategoryColor(step.category)}`}>
                {step.category}
              </span>
            </div>
            <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${getConfidenceBadge(step.confidence)}`}>
              {Math.round(step.confidence * 100)}%
            </span>
          </div>
          
          {step.description && (
            <p className="text-sm text-slate-400 mb-2">{step.description}</p>
          )}

          <div className="flex items-center gap-4 text-xs text-slate-500 mt-3">
            {step.createdAt && (
              <div className="flex items-center gap-1">
                <Clock size={12} />
                <span>Detected: {new Date(step.createdAt).toLocaleString()}</span>
              </div>
            )}
            {step.evidence && typeof step.evidence === 'object' && 'capability_id' in step.evidence && (
              <div className="flex items-center gap-1">
                <TrendingUp size={12} />
                <span>From: {String(step.evidence.capability_id)}</span>
              </div>
            )}
          </div>
        </div>
      ))}
    </div>
  );
};
