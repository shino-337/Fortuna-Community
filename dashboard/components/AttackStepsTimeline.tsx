import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { PodAttackStep } from '../types';
import { Card } from './ui/Card';
import { AlertTriangle, Clock, TrendingUp, Shield } from 'lucide-react';

interface AttackStepsTimelineProps {
  podUid: string;
}

export const AttackStepsTimeline: React.FC<AttackStepsTimelineProps> = ({ podUid }) => {
  const [steps, setSteps] = useState<PodAttackStep[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!podUid) {
      setLoading(false);
      return;
    }

    const fetchSteps = async () => {
      try {
        setLoading(true);
        const data = await api.getPodAttackSteps(podUid);
        // Sort by creation time (newest first)
        const sorted = data.sort((a, b) => {
          const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
          const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
          return timeB - timeA;
        });
        setSteps(sorted);
      } catch (err) {
        console.error('Failed to fetch attack steps:', err);
        setSteps([]);
      } finally {
        setLoading(false);
      }
    };

    fetchSteps();
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
    <div className="relative">
      {/* Timeline Line */}
      <div className="absolute left-8 top-0 bottom-0 w-0.5 bg-slate-800"></div>

      {/* Timeline Items */}
      <div className="space-y-6">
        {steps.map((step, idx) => {
          const stepDate = step.createdAt ? new Date(step.createdAt) : new Date();
          const isRecent = (Date.now() - stepDate.getTime()) < 24 * 60 * 60 * 1000; // Last 24 hours

          return (
            <div key={`${step.stepId}-${idx}`} className="relative flex items-start gap-4">
              {/* Timeline Dot */}
              <div className={`relative z-10 flex items-center justify-center w-16 h-16 rounded-full border-2 ${
                isRecent ? 'bg-red-500/20 border-red-500/50' : 'bg-slate-800 border-slate-700'
              }`}>
                <AlertTriangle 
                  size={20} 
                  className={isRecent ? 'text-red-400' : 'text-slate-500'} 
                />
              </div>

              {/* Content Card */}
              <div className="flex-1 bg-slate-900/70 border border-slate-800 rounded-lg p-4 hover:border-slate-700 transition-colors">
                <div className="flex items-start justify-between mb-2">
                  <div className="flex-1">
                    <div className="flex items-center gap-2 mb-2">
                      <span className="font-semibold text-white text-sm">{step.stepId}</span>
                      <span className={`px-2 py-0.5 rounded text-[10px] font-medium border ${getCategoryColor(step.category)}`}>
                        {step.category}
                      </span>
                      {isRecent && (
                        <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-red-500/20 text-red-400 border border-red-500/30">
                          NEW
                        </span>
                      )}
                    </div>
                    
                    {step.description && (
                      <p className="text-sm text-slate-400 mb-3">{step.description}</p>
                    )}

                    <div className="flex items-center gap-4 text-xs text-slate-500">
                      <div className="flex items-center gap-1">
                        <Clock size={12} />
                        <span>{stepDate.toLocaleString()}</span>
                      </div>
                      <div className="flex items-center gap-1">
                        <TrendingUp size={12} />
                        <span>Confidence: {Math.round(step.confidence * 100)}%</span>
                      </div>
                      {step.evidence && typeof step.evidence === 'object' && 'capability_id' in step.evidence && (
                        <div className="flex items-center gap-1">
                          <Shield size={12} />
                          <span>From: {String(step.evidence.capability_id)}</span>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Confidence Badge */}
                  <span className={`px-2 py-1 rounded text-xs font-medium border ${getConfidenceBadge(step.confidence)}`}>
                    {Math.round(step.confidence * 100)}%
                  </span>
                </div>

                {/* Evidence Details */}
                {step.evidence && typeof step.evidence === 'object' && Object.keys(step.evidence).length > 0 && (
                  <div className="mt-3 pt-3 border-t border-slate-800">
                    <div className="text-xs font-semibold text-slate-400 mb-2">Evidence:</div>
                    <div className="text-xs text-slate-500 font-mono bg-slate-950/50 p-2 rounded border border-slate-800">
                      {JSON.stringify(step.evidence, null, 2)}
                    </div>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
