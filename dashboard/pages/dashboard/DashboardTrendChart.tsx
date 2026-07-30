import React, { useMemo, useState, useEffect } from 'react';
import { Card } from '../../design-system/components/Card';
import { Button } from '../../components/ui/Button';
import { getChartThemeColors } from '../../lib/chartTheme';

export interface TrendChartPoint {
  name: string;
  risk: number;
  pce: number;
}

export interface DashboardTrendChartProps {
  data: TrendChartPoint[];
  trendDays: number;
  onTrendDaysChange: (days: number) => void;
  loading?: boolean;
}

export const DashboardTrendChart: React.FC<DashboardTrendChartProps> = ({
  data,
  trendDays,
  onTrendDaysChange,
  loading,
}) => {
  const [colors, setColors] = useState(getChartThemeColors());
  useEffect(() => {
    setColors(getChartThemeColors());
  }, []);

  const trendHasSignal = useMemo(() => data.some((d) => d.risk > 0 || d.pce > 0), [data]);

  const trendChartYDomain = useMemo((): [number, number] => {
    if (!data.length) return [0, 1];
    const maxVal = Math.max(...data.map((d) => Math.max(d.risk, d.pce)), 1);
    return [0, maxVal];
  }, [data]);

  const trendSpikeIdx = useMemo(() => {
    if (data.length < 2) return -1;
    let bestI = -1;
    let bestD = 0;
    for (let i = 1; i < data.length; i += 1) {
      const prev = (data[i - 1].risk ?? 0) + (data[i - 1].pce ?? 0);
      const cur = (data[i].risk ?? 0) + (data[i].pce ?? 0);
      const jump = cur - prev;
      if (jump > bestD) {
        bestD = jump;
        bestI = i;
      }
    }
    return bestD > 0 ? bestI : -1;
  }, [data]);

  const trendThresholdY = useMemo(
    () => Math.max(1, Math.ceil(trendChartYDomain[1] * 0.25)),
    [trendChartYDomain],
  );

  const chartGeometry = useMemo(() => {
    const width = 640;
    const height = 250;
    const pad = { top: 20, right: 18, bottom: 36, left: 36 };
    const plotW = width - pad.left - pad.right;
    const plotH = height - pad.top - pad.bottom;
    const maxVal = Math.max(trendChartYDomain[1], 1);
    const xFor = (idx: number) => pad.left + (data.length <= 1 ? 0 : (idx / (data.length - 1)) * plotW);
    const yFor = (value: number) => pad.top + plotH - (Math.max(0, value) / maxVal) * plotH;
    const toPoints = (key: 'risk' | 'pce') => data.map((d, idx) => `${xFor(idx).toFixed(1)},${yFor(d[key]).toFixed(1)}`).join(' ');
    return {
      width,
      height,
      pad,
      plotW,
      plotH,
      riskPoints: toPoints('risk'),
      pcePoints: toPoints('pce'),
      thresholdY: yFor(trendThresholdY),
      spike: trendSpikeIdx >= 0 ? { x: xFor(trendSpikeIdx), y: yFor(data[trendSpikeIdx]?.risk ?? 0) } : null,
      maxVal,
    };
  }, [data, trendChartYDomain, trendThresholdY, trendSpikeIdx]);

  if (loading) {
    return (
      <Card variant="primary" title={`Trend: findings vs capability exposure (${trendDays}d)`} className="border border-border/60 shadow-none">
        <div className="h-[280px] animate-pulse rounded-md bg-surface-2/40" aria-busy="true" />
      </Card>
    );
  }

  if (!trendHasSignal) {
    return (
      <p className="rounded-lg border border-border/60 bg-surface/15 px-3 py-3 text-caption leading-relaxed text-muted-2">
        No non-zero trend in the last {trendDays} days. Widen the time range in the header or open{' '}
        <span className="text-text font-medium">Risk overview</span> below for current counts.
      </p>
    );
  }

  return (
    <Card variant="primary" title={`Trend: findings vs capability exposure (${trendDays}d)`} className="border border-border/60 shadow-none">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2 text-caption text-muted">
        <span className="text-meta text-muted-2 max-w-[min(100%,28rem)]">
          Findings and capability exposure in the selected scope.
        </span>
        <div className="flex items-center gap-2" role="group" aria-label="Trend range">
          <span>Range:</span>
          {[7, 30].map((d) => (
            <Button
              key={d}
              variant={trendDays === d ? 'primary' : 'ghost'}
              size="sm"
              onClick={() => onTrendDaysChange(d)}
              className="!rounded-full !px-2 !py-0.5 !text-caption"
              aria-pressed={trendDays === d}
            >
              {d === 7 ? '7d' : '30d'}
            </Button>
          ))}
        </div>
      </div>
      <div className="h-[280px] w-full min-w-0 overflow-hidden rounded-md border border-border/50 bg-surface-2/20">
        <svg
          role="img"
          aria-label={`Findings and capability exposure trend for the last ${trendDays} days`}
          viewBox={`0 0 ${chartGeometry.width} ${chartGeometry.height}`}
          preserveAspectRatio="none"
          className="h-[252px] w-full"
        >
          {[0.25, 0.5, 0.75].map((ratio) => {
            const y = chartGeometry.pad.top + chartGeometry.plotH * ratio;
            return (
              <line
                key={ratio}
                x1={chartGeometry.pad.left}
                x2={chartGeometry.pad.left + chartGeometry.plotW}
                y1={y}
                y2={y}
                stroke={colors.grid}
                strokeDasharray="3 3"
                vectorEffect="non-scaling-stroke"
              />
            );
          })}
          <line
            x1={chartGeometry.pad.left}
            x2={chartGeometry.pad.left + chartGeometry.plotW}
            y1={chartGeometry.thresholdY}
            y2={chartGeometry.thresholdY}
            stroke={colors.threshold}
            strokeDasharray="4 4"
            vectorEffect="non-scaling-stroke"
          />
          <text x={chartGeometry.pad.left + chartGeometry.plotW - 4} y={chartGeometry.thresholdY - 5} textAnchor="end" fill={colors.threshold} fontSize="10">
            threshold
          </text>
          <polyline points={chartGeometry.pcePoints} fill="none" stroke={colors.pceLine} strokeWidth="2" vectorEffect="non-scaling-stroke" />
          <polyline points={chartGeometry.riskPoints} fill="none" stroke={colors.riskLine} strokeWidth="2" vectorEffect="non-scaling-stroke" />
          {data.map((d, idx) => {
            const x = chartGeometry.pad.left + (data.length <= 1 ? 0 : (idx / (data.length - 1)) * chartGeometry.plotW);
            return (
              <text key={d.name} x={x} y={chartGeometry.height - 12} textAnchor="middle" fill={colors.axis} fontSize="9">
                {d.name.slice(5)}
              </text>
            );
          })}
          <text x={8} y={chartGeometry.pad.top + 3} fill={colors.axis} fontSize="10">
            {chartGeometry.maxVal}
          </text>
          <text x={12} y={chartGeometry.pad.top + chartGeometry.plotH} fill={colors.axis} fontSize="10">
            0
          </text>
          {chartGeometry.spike ? (
            <circle
              cx={chartGeometry.spike.x}
              cy={chartGeometry.spike.y}
              r="5"
              fill={colors.spike}
              stroke={colors.tooltipBg}
              strokeWidth="1"
              vectorEffect="non-scaling-stroke"
            />
          ) : null}
        </svg>
        <div className="flex items-center gap-4 px-2 py-1 text-micro text-muted">
          <span className="inline-flex items-center gap-1"><span className="h-0.5 w-4 rounded-full" style={{ backgroundColor: colors.riskLine }} />Findings</span>
          <span className="inline-flex items-center gap-1"><span className="h-0.5 w-4 rounded-full" style={{ backgroundColor: colors.pceLine }} />Capability exposure</span>
        </div>
      </div>
    </Card>
  );
};
