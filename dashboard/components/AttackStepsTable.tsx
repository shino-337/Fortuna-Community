import React, { useEffect, useState } from 'react';
import { AlertTriangle, Clock, Shield, TrendingUp } from 'lucide-react';
import { Card } from '../design-system/components/Card';
import { api } from '../lib/api';
import { PodAttackStep } from '../types';

interface AttackStepsTableProps {
  podUid: string;
}

const getCategoryColor = (category: string) => {
  const colors: Record<string, string> = {
    FILESYSTEM: 'bg-error-background text-error-foreground border-error-border',
    KERNEL: 'bg-warning-background text-warning-foreground border-warning-border',
    PROCESS: 'bg-warning-muted text-warning-foreground border-warning-border',
    IPC: 'bg-warning-background text-warning-foreground border-warning-border',
    CREDENTIALS: 'bg-brand/15 text-brand border-brand/35',
    PERSISTENCE: 'bg-brand/15 text-brand border-brand/35',
    ESCAPE: 'bg-error-background text-error-foreground border-error-border',
    RBAC: 'bg-info-background text-info-foreground border-info-border',
    NETWORK: 'bg-info-background text-info-foreground border-info-border',
    CONTROL_PLANE: 'bg-info-muted text-info-foreground border-info-border',
  };
  return colors[category] || 'bg-surface-2/50 text-muted border-border/70';
};

const getConfidenceBadge = (confidence: number) => {
  if (confidence >= 0.8) return 'bg-error-background text-error-foreground border-error-border';
  if (confidence >= 0.6) return 'bg-warning-background text-warning-foreground border-warning-border';
  return 'bg-info-background text-info-foreground border-info-border';
};

export const AttackStepsTable: React.FC<AttackStepsTableProps> = ({ podUid }) => {
  const [steps, setSteps] = useState<PodAttackStep[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    if (!podUid) {
      setSteps([]);
      setLoading(false);
      return;
    }

    let cancelled = false;
    setLoading(true);
    api
      .getPodAttackSteps(podUid)
      .then((data) => {
        if (!cancelled) setSteps(data);
      })
      .catch(() => {
        if (!cancelled) setSteps([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [podUid]);

  if (loading) {
    return (
      <Card variant="panel" contentClassName="p-4">
        <div className="space-y-3" role="status" aria-label="Loading attack steps">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-16 animate-pulse rounded-lg border border-border/70 bg-surface/45 motion-reduce:animate-none" />
          ))}
        </div>
      </Card>
    );
  }

  if (steps.length === 0) {
    return (
      <Card variant="panel" contentClassName="p-6">
        <div className="text-center text-muted">
          <Shield size={24} className="mx-auto mb-2 opacity-70" />
          <p className="text-body text-text">No attack steps detected for this pod.</p>
          <p className="mt-1 text-caption text-muted">Attack steps appear when capabilities reach exploited state.</p>
        </div>
      </Card>
    );
  }

  return (
    <div className="space-y-3">
      {steps.map((step, idx) => (
        <Card key={`${step.stepId}-${idx}`} variant="panel" contentClassName="p-4">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <AlertTriangle size={16} className="shrink-0 text-error" />
              <span className="text-body font-semibold text-text">{step.stepId}</span>
              <span className={`rounded border px-2 py-0.5 text-caption font-medium ${getCategoryColor(step.category)}`}>
                {step.category}
              </span>
            </div>
            <span className={`shrink-0 rounded border px-2 py-0.5 text-caption font-medium ${getConfidenceBadge(step.confidence)}`}>
              {Math.round(step.confidence * 100)}%
            </span>
          </div>

          {step.description ? <p className="mt-2 text-body text-muted">{step.description}</p> : null}

          <div className="mt-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-caption text-muted">
            {step.createdAt ? (
              <span className="inline-flex items-center gap-1">
                <Clock size={12} />
                Detected: {new Date(step.createdAt).toLocaleString()}
              </span>
            ) : null}
            {step.evidence && typeof step.evidence === 'object' && 'capability_id' in step.evidence ? (
              <span className="inline-flex items-center gap-1">
                <TrendingUp size={12} />
                From: {String(step.evidence.capability_id)}
              </span>
            ) : null}
          </div>
        </Card>
      ))}
    </div>
  );
};
