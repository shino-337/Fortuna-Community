import React, { useEffect, useState } from 'react';
import { AlertTriangle, Clock, Shield, TrendingUp } from 'lucide-react';
import { api, isApiError } from '../lib/api';
import { PodAttackStep } from '../types';
import { useCan } from '../hooks/usePermUser';
import { P } from '../lib/permissions';
import { PageError } from '../design-system/components/PageStatus';
import { SemanticEmptyState } from '../design-system/components/SemanticEmptyState';
import type { SemanticVisibilityState } from '../lib/visibilityEngine';

interface AttackStepsTimelineProps {
  podUid: string;
}

type TimelineIssue = {
  tone: 'warning' | 'error';
  state?: SemanticVisibilityState;
  title: string;
  description: string;
  detail?: string;
};

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

export const AttackStepsTimeline: React.FC<AttackStepsTimelineProps> = ({ podUid }) => {
  const [steps, setSteps] = useState<PodAttackStep[]>([]);
  const [loading, setLoading] = useState(true);
  const [issue, setIssue] = useState<TimelineIssue | null>(null);
  const canReadAttackSteps = useCan(P.findingsRead);

  const classifyIssue = (error: unknown): TimelineIssue => {
    if (isApiError(error)) {
      if (error.status === 401) {
        return {
          tone: 'warning',
          state: 'no_permission',
          title: 'Session required',
          description: 'Sign in again to load attack-step evidence.',
          detail: error.body?.code ? `Core returned ${error.body.code}.` : undefined,
        };
      }
      if (error.status === 403 && error.body?.reason === 'cluster_scope') {
        return {
          tone: 'error',
          state: 'no_scope',
          title: 'Pod outside scope',
          description: 'Your account is not scoped for this pod.',
          detail: 'Required scope: pod cluster access.',
        };
      }
      if (error.status === 403) {
        const required = error.body?.required_permission ?? error.body?.required_permissions?.join(', ');
        return {
          tone: 'error',
          state: 'no_permission',
          title: 'Permission required',
          description: required ? `Requires ${required}.` : 'Requires permission to read risk findings.',
          detail: 'Attack steps are served by the risk attack-steps API.',
        };
      }
      return {
        tone: 'warning',
        title: 'Attack steps unavailable',
        description: error.status >= 500 ? 'Core could not build attack-step evidence.' : error.message,
        detail: `HTTP ${error.status}`,
      };
    }
    return {
      tone: 'warning',
      title: 'Attack steps unavailable',
      description: error instanceof Error ? error.message : 'The request failed before evidence was returned.',
    };
  };

  useEffect(() => {
    if (!podUid || !canReadAttackSteps) {
      setSteps([]);
      setIssue(null);
      setLoading(false);
      return;
    }

    let cancelled = false;
    const fetchSteps = async () => {
      try {
        setLoading(true);
        setIssue(null);
        const data = await api.getPodAttackStepsStrict(podUid);
        const sorted = [...data].sort((a, b) => {
          const timeA = a.createdAt ? new Date(a.createdAt).getTime() : 0;
          const timeB = b.createdAt ? new Date(b.createdAt).getTime() : 0;
          return timeB - timeA;
        });
        if (!cancelled) setSteps(sorted);
      } catch (err) {
        if (!cancelled) {
          setSteps([]);
          setIssue(classifyIssue(err));
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void fetchSteps();
    return () => {
      cancelled = true;
    };
  }, [podUid, canReadAttackSteps]);

  if (!canReadAttackSteps) {
    return (
      <SemanticEmptyState
        state="no_permission"
        title="Attack-step evidence hidden"
        reason="Requires findings.read."
        compact
      />
    );
  }

  if (loading) {
    return (
      <div className="space-y-4" role="status" aria-label="Loading attack timeline">
        {[0, 1, 2].map((i) => (
          <div key={i} className="ml-8 h-20 animate-pulse rounded-lg border border-border/70 bg-surface/45 motion-reduce:animate-none" />
        ))}
      </div>
    );
  }

  if (issue) {
    if (issue.state) {
      return (
        <SemanticEmptyState
          state={issue.state}
          title={issue.title}
          reason={`${issue.description}${issue.detail ? ` ${issue.detail}` : ''}`}
          compact
        />
      );
    }
    return (
      <PageError
        title={issue.title}
        description={`${issue.description}${issue.detail ? ` ${issue.detail}` : ''}`}
        className="py-4"
      />
    );
  }

  if (steps.length === 0) {
    return (
      <SemanticEmptyState
        state="no_data"
        title="No attack steps detected"
        reason="The API returned no exploited-state evidence for this pod."
        compact
      />
    );
  }

  return (
    <div className="relative">
      <div className="absolute bottom-0 left-8 top-0 w-px bg-border" aria-hidden />
      <div className="space-y-5">
        {steps.map((step, idx) => {
          const stepDate = step.createdAt ? new Date(step.createdAt) : new Date();
          const isRecent = Date.now() - stepDate.getTime() < 24 * 60 * 60 * 1000;

          return (
            <div key={`${step.stepId}-${idx}`} className="relative flex items-start gap-4">
              <div
                className={`relative z-10 flex h-16 w-16 shrink-0 items-center justify-center rounded-full border-2 ${
                  isRecent ? 'border-error-border bg-error-background' : 'border-border bg-surface-2'
                }`}
              >
                <AlertTriangle size={20} className={isRecent ? 'text-error' : 'text-muted'} />
              </div>

              <div className="min-w-0 flex-1 rounded-lg border border-border bg-surface/70 p-4 transition-colors hover:border-border/80">
                <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
                  <div className="min-w-0 flex-1">
                    <div className="mb-2 flex flex-wrap items-center gap-2">
                      <span className="text-body font-semibold text-text">{step.stepId}</span>
                      <span className={`rounded border px-2 py-0.5 text-caption font-medium ${getCategoryColor(step.category)}`}>
                        {step.category}
                      </span>
                      {isRecent ? (
                        <span className="rounded border border-error-border bg-error-background px-2 py-0.5 text-caption font-medium text-error-foreground">
                          New
                        </span>
                      ) : null}
                    </div>

                    {step.description ? <p className="mb-3 text-body text-muted">{step.description}</p> : null}

                    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-caption text-muted">
                      <span className="inline-flex items-center gap-1">
                        <Clock size={12} />
                        {stepDate.toLocaleString()}
                      </span>
                      <span className="inline-flex items-center gap-1">
                        <TrendingUp size={12} />
                        Confidence: {Math.round(step.confidence * 100)}%
                      </span>
                      {step.evidence && typeof step.evidence === 'object' && 'capability_id' in step.evidence ? (
                        <span className="inline-flex items-center gap-1">
                          <Shield size={12} />
                          From: {String(step.evidence.capability_id)}
                        </span>
                      ) : null}
                    </div>
                  </div>

                  <span className={`shrink-0 rounded border px-2 py-1 text-caption font-medium ${getConfidenceBadge(step.confidence)}`}>
                    {Math.round(step.confidence * 100)}%
                  </span>
                </div>

                {step.evidence && typeof step.evidence === 'object' && Object.keys(step.evidence).length > 0 ? (
                  <div className="mt-3 border-t border-border pt-3">
                    <div className="mb-2 text-caption font-semibold text-muted">Evidence</div>
                    <pre className="max-h-48 overflow-auto rounded border border-border bg-base/50 p-2 text-caption text-muted">
                      {JSON.stringify(step.evidence, null, 2)}
                    </pre>
                  </div>
                ) : null}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
