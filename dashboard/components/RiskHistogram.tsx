import React from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  ReferenceLine,
  Legend,
  Cell,
} from 'recharts';
import type { RiskHistogramResponse } from '../types';

const SEVERITY_COLORS = {
  critical: '#ef4444',
  high: '#f59e0b',
  medium: '#eab308',
  low: '#84cc16',
};

const BIN_LABEL = (bin: number) => `${bin}-${bin + 10}`;

export interface RiskHistogramProps {
  data: RiskHistogramResponse | null;
  loading?: boolean;
  /** Compact layout (e.g. 1/3 width on Overview) */
  compact?: boolean;
  /** When user clicks a bar, filter findings to this score bin */
  onBinClick?: (bin: number) => void;
  /** Currently selected bin (highlight) */
  selectedBin?: number | null;
}

export const RiskHistogram: React.FC<RiskHistogramProps> = ({
  data,
  loading,
  compact,
  onBinClick,
  selectedBin,
}) => {
  const height = compact ? 220 : 320;
  const bins = data?.bins ?? [];

  if (loading) {
    return (
      <div
        className="bg-slate-800/30 rounded-lg border border-slate-700/50 flex items-center justify-center text-slate-500"
        style={{ height }}
      >
        <span className="text-sm">Loading histogram…</span>
      </div>
    );
  }

  if (!data) {
    return (
      <div
        className="bg-slate-800/30 rounded-lg border border-slate-700/50 flex items-center justify-center text-slate-500"
        style={{ height }}
      >
        <span className="text-sm">No score data. Check scope and time window; risk_scores are computed for findings when enabled.</span>
      </div>
    );
  }
  if (bins.length === 0) {
    return (
      <div
        className="bg-slate-800/30 rounded-lg border border-slate-700/50 flex items-center justify-center text-slate-500"
        style={{ height }}
      >
        <span className="text-sm">No bins returned. Ensure risk_scores are calculated for insights.</span>
      </div>
    );
  }

  const CustomTooltip = ({ active, payload, label }: { active?: boolean; payload?: Array<{ payload: RiskHistogramResponse['bins'][0] }>; label?: string }) => {
    if (!active || !payload?.length) return null;
    const row = payload[0]?.payload;
    if (!row) return null;
    const total = row.count;
    const pct = data.totalFindings ? ((total / data.totalFindings) * 100).toFixed(1) : '0';
    const { critical_count, high_count, medium_count, low_count } = row;
    return (
      <div className="bg-slate-900 border border-slate-700 rounded-lg shadow-xl p-3 text-left min-w-[180px]">
        <div className="text-slate-300 font-medium">Score {row.bin}–{row.bin + 10}</div>
        <div className="text-slate-400 text-sm mt-1">
          {total} findings ({pct}%)
        </div>
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-2 text-xs">
          {critical_count > 0 && <span style={{ color: SEVERITY_COLORS.critical }}>Critical: {critical_count}</span>}
          {high_count > 0 && <span style={{ color: SEVERITY_COLORS.high }}>High: {high_count}</span>}
          {medium_count > 0 && <span style={{ color: SEVERITY_COLORS.medium }}>Medium: {medium_count}</span>}
          {low_count > 0 && <span style={{ color: SEVERITY_COLORS.low }}>Low: {low_count}</span>}
        </div>
      </div>
    );
  };

  return (
    <div className={compact ? 'w-full max-w-md' : 'w-full'}>
      <div
        className="bg-slate-800/30 rounded-lg border border-slate-700/50"
        style={{ height }}
      >
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={bins}
            margin={{ top: 12, right: 12, left: 0, bottom: 8 }}
            onClick={(state) => {
              if (state?.activePayload?.[0]?.payload != null && onBinClick) {
                const bin = (state.activePayload[0].payload as RiskHistogramResponse['bins'][0]).bin;
                onBinClick(bin);
              }
            }}
          >
            <XAxis
              dataKey="bin"
              tickFormatter={(v) => BIN_LABEL(v)}
              axisLine={false}
              tickLine={false}
              tick={{ fill: '#94a3b8', fontSize: compact ? 10 : 11 }}
              interval={0}
            />
            <YAxis
              axisLine={false}
              tickLine={false}
              tick={{ fill: '#94a3b8', fontSize: 11 }}
              width={28}
              allowDecimals={false}
            />
            <Tooltip content={<CustomTooltip />} cursor={{ fill: 'rgba(148,163,184,0.1)' }} />
            {/* Stacked bars by severity */}
            <Bar dataKey="critical_count" stackId="a" fill={SEVERITY_COLORS.critical} name="Critical" radius={[0, 0, 0, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={SEVERITY_COLORS.critical}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? '#fca5a5' : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            <Bar dataKey="high_count" stackId="a" fill={SEVERITY_COLORS.high} name="High" radius={[0, 0, 0, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={SEVERITY_COLORS.high}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? '#fcd34d' : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            <Bar dataKey="medium_count" stackId="a" fill={SEVERITY_COLORS.medium} name="Medium" radius={[0, 0, 0, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={SEVERITY_COLORS.medium}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? '#fde047' : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            <Bar dataKey="low_count" stackId="a" fill={SEVERITY_COLORS.low} name="Low" radius={[0, 2, 2, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={SEVERITY_COLORS.low}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? '#bef264' : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            {/* Priority thresholds (vertical) */}
            <ReferenceLine x={90} stroke="#ef4444" strokeDasharray="3 3" strokeOpacity={0.7} />
            <ReferenceLine x={70} stroke="#f59e0b" strokeDasharray="3 3" strokeOpacity={0.7} />
            {/* Average score (vertical line at bin containing average; Y-axis is count so we use x) */}
            {data.averageScore > 0 && data.averageScore < 100 && (
              <ReferenceLine
                x={Math.min(90, Math.floor(data.averageScore / 10) * 10)}
                stroke="#64748b"
                strokeDasharray="5 5"
                strokeOpacity={0.8}
                label={{ value: `Avg ${data.averageScore.toFixed(0)}`, position: 'top', fill: '#94a3b8', fontSize: 10 }}
              />
            )}
            <Legend
              wrapperStyle={{ paddingTop: 4 }}
              formatter={(value) => value}
              iconType="square"
              iconSize={10}
              style={{ fontSize: 11 }}
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
      <div className="flex items-center justify-between mt-1 px-1 text-xs text-slate-500">
        <span>Total: {data.totalFindings} findings</span>
        {data.p0Count > 0 && <span>P0: {data.p0Count}</span>}
        {data.averageScore > 0 && data.averageScore < 100 && (
          <span>Avg score: {typeof data.averageScore === 'number' ? data.averageScore.toFixed(1) : data.averageScore}</span>
        )}
      </div>
      {data.totalFindings === 0 && (
        <p className="mt-1 px-1 text-xs text-slate-500">No findings with risk scores in this scope. Scores are computed when risk scoring is enabled.</p>
      )}
    </div>
  );
};
