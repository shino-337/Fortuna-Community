/** Shared visual mapping for runtime signal types (Insights drawer, Risk detail, signals table). */
export type RuntimeSignalVisual = {
  signalClass: string;
  severity: string;
  severityClass: string;
};

export function runtimeSignalVisual(signalType: string): RuntimeSignalVisual {
  const t = (signalType || '').trim().toUpperCase();
  const byType: Record<string, RuntimeSignalVisual> = {
    NETWORK_QUEUE_ANOMALY: {
      signalClass: 'bg-cyan-500/20 text-cyan-300 border-cyan-500/40',
      severity: 'MEDIUM',
      severityClass: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
    },
    SUSPICIOUS_EXEC_FROM_SNAPSHOT: {
      signalClass: 'bg-orange-500/20 text-orange-300 border-orange-500/40',
      severity: 'HIGH',
      severityClass: 'bg-red-500/20 text-red-300 border-red-500/40',
    },
    PROC_ROOT_PIVOT: {
      signalClass: 'bg-red-500/20 text-red-300 border-red-500/50',
      severity: 'CRITICAL',
      severityClass: 'bg-red-600/30 text-red-200 border-red-500/50',
    },
    FS_ESCAPE_ATTEMPT: {
      signalClass: 'bg-amber-500/20 text-amber-200 border-amber-500/40',
      severity: 'HIGH',
      severityClass: 'bg-orange-500/20 text-orange-300 border-orange-500/40',
    },
    NAMESPACE_ESCAPE: {
      signalClass: 'bg-purple-500/20 text-purple-200 border-purple-500/40',
      severity: 'HIGH',
      severityClass: 'bg-orange-500/20 text-orange-300 border-orange-500/40',
    },
    CAPABILITY_MISUSE: {
      signalClass: 'bg-rose-500/20 text-rose-200 border-rose-500/40',
      severity: 'MEDIUM',
      severityClass: 'bg-amber-500/20 text-amber-300 border-amber-500/40',
    },
  };
  return (
    byType[t] ?? {
      signalClass: 'bg-muted/20 text-text border-border/40',
      severity: 'INFO',
      severityClass: 'bg-muted/20 text-text border-border/40',
    }
  );
}
