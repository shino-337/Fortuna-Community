import React from 'react';
import { ArrowDown, ArrowUp, Minus } from 'lucide-react';

interface StatCardProps {
  title: string;
  value: string | number;
  icon: React.ReactNode;
  /** Optional subtitle below value (e.g. "4 critical") – shown when no trend */
  subtitle?: string;
  trend?: number; // percentage
  trendLabel?: string;
  trendDirection?: 'up' | 'down' | 'neutral';
  color?: string; // Tailwind color class for icon bg
}

export const StatCard: React.FC<StatCardProps> = ({
  title,
  value,
  icon,
  subtitle,
  trend,
  trendLabel = 'vs last month',
  trendDirection = 'neutral',
  color = 'bg-slate-800 text-slate-400',
}) => {
  return (
    <div className="bg-surface rounded-lg border border-border p-4 shadow-sm hover:shadow-md transition-shadow">
      <div className="flex items-start justify-between">
        <div>
          <p className="text-xs md:text-sm font-medium text-muted-2">{title}</p>
          <h4 className="text-xl md:text-2xl font-bold text-text mt-1.5">{value}</h4>
        </div>
        <div className={`p-2.5 rounded-lg ${color}`}>
          {icon}
        </div>
      </div>

      {trend !== undefined && (
        <div className="mt-3 flex items-center text-xs md:text-sm">
          <span
            className={`flex items-center font-medium ${
              trendDirection === 'up'
                ? 'text-emerald-400'
                : trendDirection === 'down'
                ? 'text-red-400'
                : 'text-slate-400'
            }`}
          >
            {trendDirection === 'up' && <ArrowUp className="w-3 h-3 mr-1" />}
            {trendDirection === 'down' && <ArrowDown className="w-3 h-3 mr-1" />}
            {trendDirection === 'neutral' && <Minus className="w-3 h-3 mr-1" />}
            {Math.abs(trend)}%
          </span>
          <span className="text-slate-500 ml-2">{trendLabel}</span>
        </div>
      )}
      {trend === undefined && subtitle && (
        <p className="mt-1.5 text-xs md:text-sm text-slate-500">{subtitle}</p>
      )}
    </div>
  );
};