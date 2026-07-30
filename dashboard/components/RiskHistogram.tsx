import React from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ReferenceLine,
  Legend,
  Cell,
} from 'recharts';
import type { RiskHistogramResponse } from '../types';
import { getChartThemeColors } from '../lib/chartTheme';

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

/** Histogram of risk scores, stacked by unified risk level with interactive bin selection. */
export const RiskHistogram: React.FC<RiskHistogramProps> = ({
  data,
  loading,
  compact,
  onBinClick,
  selectedBin,
}) => {
  const height = compact ? 220 : 320;
  const bins = data?.bins ?? [];
  const chartRef = React.useRef<HTMLDivElement | null>(null);
  const [chartWidth, setChartWidth] = React.useState(0);
  const colors = getChartThemeColors();

  React.useEffect(() => {
    const el = chartRef.current;
    if (!el || typeof ResizeObserver === 'undefined') return undefined;
    const update = () => {
      setChartWidth(Math.max(0, Math.floor(el.getBoundingClientRect().width)));
    };
    update();
    const observer = new ResizeObserver(update);
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  if (loading) {
    return (
      <div
        className="bg-surface-2/30 rounded-lg border border-border/50 flex items-center justify-center text-muted"
        style={{ height }}
      >
        <span className="text-body">Loading histogram…</span>
      </div>
    );
  }

  if (!data) {
    return (
      <div
        className="bg-surface-2/30 rounded-lg border border-border/50 flex items-center justify-center text-muted"
        style={{ height }}
      >
        <span className="text-body">No score data. Check scope and time window; risk_scores are computed for findings when enabled.</span>
      </div>
    );
  }
  if (bins.length === 0) {
    return (
      <div
        className="bg-surface-2/30 rounded-lg border border-border/50 flex items-center justify-center text-muted"
        style={{ height }}
      >
        <span className="text-body">No bins returned. Ensure risk_scores are calculated for insights.</span>
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
      <div className="bg-surface border border-border rounded-lg shadow-xl p-3 text-left min-w-[180px]">
        <div className="text-text font-medium">Score {row.bin}–{row.bin + 10}</div>
        <div className="text-muted text-body mt-1">
          {total} findings ({pct}%)
        </div>
        <div className="flex flex-wrap gap-x-3 gap-y-0.5 mt-2 text-caption">
          {critical_count > 0 && <span style={{ color: colors.critical }}>Critical: {critical_count}</span>}
          {high_count > 0 && <span style={{ color: colors.high }}>High: {high_count}</span>}
          {medium_count > 0 && <span style={{ color: colors.medium }}>Medium: {medium_count}</span>}
          {low_count > 0 && <span style={{ color: colors.low }}>Low: {low_count}</span>}
        </div>
      </div>
    );
  };

  return (
    <div className={compact ? 'w-full max-w-md' : 'w-full'}>
      <div
        ref={chartRef}
        className="bg-surface-2/30 rounded-lg border border-border/50"
        style={{ height }}
      >
        {chartWidth > 0 ? (
          <BarChart
            data={bins}
            width={chartWidth}
            height={height}
            margin={{ top: 12, right: 12, left: 0, bottom: 8 }}
            onClick={(state) => {
              const st = state as { activePayload?: Array<{ payload?: RiskHistogramResponse['bins'][0] }> };
              const payload = st?.activePayload?.[0]?.payload;
              if (payload != null && onBinClick) {
                onBinClick(payload.bin);
              }
            }}
          >
            <XAxis
              dataKey="bin"
              tickFormatter={(v) => BIN_LABEL(v)}
              axisLine={false}
              tickLine={false}
              tick={{ fill: colors.axis, fontSize: compact ? 10 : 11 }}
              interval={0}
            />
            <YAxis
              axisLine={false}
              tickLine={false}
              tick={{ fill: colors.axis, fontSize: 11 }}
              width={28}
              allowDecimals={false}
            />
            <Tooltip content={<CustomTooltip />} cursor={{ fill: colors.grid }} />
            {/* Stacked bars by unified risk level */}
            <Bar dataKey="critical_count" stackId="a" fill={colors.critical} name="Critical" radius={[0, 0, 0, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={colors.critical}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? colors.tooltipBg : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            <Bar dataKey="high_count" stackId="a" fill={colors.high} name="High" radius={[0, 0, 0, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={colors.high}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? colors.tooltipBg : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            <Bar dataKey="medium_count" stackId="a" fill={colors.medium} name="Medium" radius={[0, 0, 0, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={colors.medium}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? colors.tooltipBg : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            <Bar dataKey="low_count" stackId="a" fill={colors.low} name="Low" radius={[0, 2, 2, 0]}>
              {bins.map((entry, i) => (
                <Cell
                  key={i}
                  fill={colors.low}
                  opacity={selectedBin != null && entry.bin === selectedBin ? 1 : 0.9}
                  stroke={selectedBin != null && entry.bin === selectedBin ? colors.tooltipBg : 'none'}
                  strokeWidth={2}
                />
              ))}
            </Bar>
            {/* Priority thresholds (vertical) */}
            <ReferenceLine x={90} stroke={colors.critical} strokeDasharray="3 3" strokeOpacity={0.7} />
            <ReferenceLine x={70} stroke={colors.high} strokeDasharray="3 3" strokeOpacity={0.7} />
            {/* Average score (vertical line at bin containing average; Y-axis is count so we use x) */}
            {data.averageScore > 0 && data.averageScore < 100 && (
              <ReferenceLine
                x={Math.min(90, Math.floor(data.averageScore / 10) * 10)}
                stroke={colors.threshold}
                strokeDasharray="5 5"
                strokeOpacity={0.8}
                label={{ value: `Avg ${data.averageScore.toFixed(0)}`, position: 'top', fill: colors.axis, fontSize: 10 }}
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
        ) : null}
      </div>
      <div className="flex items-center justify-between mt-1 px-1 text-caption text-muted">
        <span>Total: {data.totalFindings} findings</span>
        {data.p0Count > 0 && <span>Critical band (≥70): {data.p0Count}</span>}
        {data.averageScore > 0 && data.averageScore < 100 && (
          <span>Avg score: {typeof data.averageScore === 'number' ? data.averageScore.toFixed(1) : data.averageScore}</span>
        )}
      </div>
      {data.totalFindings === 0 && (
        <p className="mt-1 px-1 text-caption text-muted">No findings with risk scores in this scope. Scores are computed when risk scoring is enabled.</p>
      )}
    </div>
  );
};
