import React from 'react';
import { Info, Tag, Boxes } from 'lucide-react';

export type SbomMetaBadgesProps = {
  sbomSource?: string;
  confidence?: string;
  goVersion?: string;
  className?: string;
  /** Smaller text for list rows */
  compact?: boolean;
};

/**
 * SBOM provenance: source (parsers | distroless-heuristic | label-metadata), confidence, Go toolchain.
 */
export function SbomMetaBadges({ sbomSource, confidence, goVersion, className = '', compact }: SbomMetaBadgesProps) {
  const src = (sbomSource || '').trim();
  const conf = (confidence || '').trim().toLowerCase();
  const go = (goVersion || '').trim();
  const sz = compact ? 'text-[10px] px-1.5 py-0.5' : 'text-xs px-2 py-0.5';
  const iconSz = compact ? 'w-3 h-3' : 'w-3.5 h-3.5';

  if (!src && !conf && !go) return null;

  return (
    <div className={`flex flex-wrap items-center gap-2 ${className}`}>
      {src === 'distroless-heuristic' && (
        <span
          className={`inline-flex items-center gap-1 rounded font-medium bg-amber-500/20 text-amber-400 border border-amber-500/40 ${sz}`}
          title="SBOM inferred from image ref/labels (no package DB). CVE match may use NVD fallback."
        >
          <Info className={iconSz} />
          Distroless (heuristic)
        </span>
      )}
      {src === 'label-metadata' && (
        <span
          className={`inline-flex items-center gap-1 rounded font-medium bg-sky-500/15 text-sky-300 border border-sky-500/35 ${sz}`}
          title="SBOM derived from image labels"
        >
          <Tag className={iconSz} />
          Label metadata
        </span>
      )}
      {src === 'parsers' && (
        <span
          className={`inline-flex items-center gap-1 rounded font-medium bg-emerald-500/15 text-emerald-300 border border-emerald-500/35 ${sz}`}
          title="Packages from filesystem / lockfiles"
        >
          <Boxes className={iconSz} />
          Package parsers
        </span>
      )}
      {src && src !== 'distroless-heuristic' && src !== 'label-metadata' && src !== 'parsers' && (
        <span
          className={`rounded font-mono bg-slate-800 text-slate-300 border border-slate-600 ${sz}`}
          title="SBOM source"
        >
          {src}
        </span>
      )}
      {conf && (
        <span
          className={`rounded font-medium border ${sz} ${
            conf === 'high'
              ? 'bg-emerald-500/15 text-emerald-300 border-emerald-500/35'
              : conf === 'medium'
                ? 'bg-amber-500/15 text-amber-300 border-amber-500/35'
                : 'bg-slate-600/30 text-slate-400 border-slate-500/40'
          }`}
          title="SBOM extraction confidence"
        >
          {compact ? conf : `Confidence: ${conf}`}
        </span>
      )}
      {go && (
        <span className={`font-mono text-slate-400 ${compact ? 'text-[10px]' : 'text-xs'}`} title="Go toolchain (stdlib/CVE context)">
          Go: {go}
        </span>
      )}
    </div>
  );
}
