import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { PodCapabilityDetail } from '../types';
import { Card } from './ui/Card';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { TrendingUp, AlertTriangle, Shield, Clock } from 'lucide-react';

interface CapabilityStateChartProps {
  podUid: string;
  capabilityId: string;
}

interface StateHistoryPoint {
  timestamp: string;
  state: 'detected' | 'confirmed' | 'exploited' | 'chained';
  confidence: number;
}

export const CapabilityStateChart: React.FC<CapabilityStateChartProps> = ({ podUid, capabilityId }) => {
  const [history, setHistory] = useState<StateHistoryPoint[]>([]);
  const [currentCapability, setCurrentCapability] = useState<PodCapabilityDetail | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      try {
        setLoading(true);
        // Get current capability details
        const capabilities = await api.getPodCapabilities(podUid);
        const cap = capabilities.find(c => c.capabilityId === capabilityId);
        if (cap) {
          setCurrentCapability(cap);
          
          // For now, create a simple history from current state
          // TODO: Implement API endpoint to get state history
          const historyData: StateHistoryPoint[] = [];
          if (cap.createdAt) {
            historyData.push({
              timestamp: cap.createdAt,
              state: 'detected',
              confidence: 0.5,
            });
          }
          if (cap.updatedAt && cap.updatedAt !== cap.createdAt) {
            historyData.push({
              timestamp: cap.updatedAt,
              state: cap.state || 'detected',
              confidence: cap.confidence || 0.5,
            });
          }
          setHistory(historyData);
        }
      } catch (err) {
        console.error('Failed to fetch capability state history:', err);
      } finally {
        setLoading(false);
      }
    };

    if (podUid && capabilityId) {
      fetchData();
    }
  }, [podUid, capabilityId]);

  const getStateColor = (state: string) => {
    const colors: Record<string, string> = {
      'detected': '#3b82f6',   // blue
      'confirmed': '#f59e0b',   // amber
      'exploited': '#ef4444',  // red
      'chained': '#8b5cf6',    // purple
    };
    return colors[state] || '#64748b';
  };

  const getStateLabel = (state: string) => {
    return state.charAt(0).toUpperCase() + state.slice(1);
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-8">
        <div className="w-8 h-8 border-4 border-pink-500 border-t-transparent rounded-full animate-spin"></div>
      </div>
    );
  }

  if (!currentCapability) {
    return (
      <div className="text-center py-8 text-slate-500">
        <Shield size={24} className="mx-auto mb-2 opacity-50" />
        <p className="text-sm">Capability not found.</p>
      </div>
    );
  }

  // Prepare chart data
  const chartData = history.length > 0 ? history.map((point, idx) => ({
    time: new Date(point.timestamp).toLocaleDateString(),
    state: point.state,
    confidence: Math.round(point.confidence * 100),
    index: idx,
  })) : [
    {
      time: currentCapability.createdAt ? new Date(currentCapability.createdAt).toLocaleDateString() : 'Now',
      state: currentCapability.state || 'detected',
      confidence: Math.round((currentCapability.confidence || 0.5) * 100),
      index: 0,
    }
  ];

  return (
    <div className="space-y-4">
      {/* Current State Info */}
      <div className="bg-slate-900/70 border border-slate-800 rounded-lg p-4">
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2">
            <Shield size={16} className="text-pink-400" />
            <span className="font-semibold text-white">{capabilityId}</span>
          </div>
          <div className="flex items-center gap-2">
            <span className={`px-2 py-1 rounded text-xs font-medium border`}
              style={{
                backgroundColor: `${getStateColor(currentCapability.state || 'detected')}20`,
                color: getStateColor(currentCapability.state || 'detected'),
                borderColor: `${getStateColor(currentCapability.state || 'detected')}50`,
              }}
            >
              {getStateLabel(currentCapability.state || 'detected')}
            </span>
            <span className="text-xs text-slate-400">
              {Math.round((currentCapability.confidence || 0.5) * 100)}% confidence
            </span>
          </div>
        </div>
        
        <div className="grid grid-cols-2 gap-4 text-xs">
          <div>
            <span className="text-slate-500">First Seen:</span>
            <span className="text-slate-300 ml-2">
              {currentCapability.createdAt ? new Date(currentCapability.createdAt).toLocaleString() : 'N/A'}
            </span>
          </div>
          <div>
            <span className="text-slate-500">Last Updated:</span>
            <span className="text-slate-300 ml-2">
              {currentCapability.updatedAt ? new Date(currentCapability.updatedAt).toLocaleString() : 'N/A'}
            </span>
          </div>
        </div>
      </div>

      {/* State Timeline Chart */}
      {chartData.length > 0 && (
        <div className="bg-slate-900/70 border border-slate-800 rounded-lg p-4">
          <div className="flex items-center justify-between mb-4">
            <h3 className="text-sm font-semibold text-slate-300 flex items-center gap-2">
              <TrendingUp size={14} className="text-pink-400" />
              State Transition Timeline
            </h3>
          </div>
          <ResponsiveContainer width="100%" height={200}>
            <LineChart data={chartData} margin={{ top: 5, right: 10, left: 0, bottom: 5 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" />
              <XAxis 
                dataKey="time" 
                stroke="#64748b" 
                fontSize={11}
                tick={{ fill: '#94a3b8' }}
              />
              <YAxis 
                stroke="#64748b" 
                fontSize={11}
                tick={{ fill: '#94a3b8' }}
                domain={[0, 100]}
                label={{ value: 'Confidence %', angle: -90, position: 'insideLeft', style: { fill: '#94a3b8', fontSize: 11 } }}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#0f172a',
                  borderRadius: '8px',
                  border: '1px solid #1e293b',
                  padding: '8px',
                }}
                itemStyle={{ color: '#cbd5e1', fontSize: '12px' }}
                formatter={(value: any, name: string) => {
                  if (name === 'confidence') return [`${value}%`, 'Confidence'];
                  return [value, name];
                }}
              />
              <Line
                type="monotone"
                dataKey="confidence"
                stroke={getStateColor(currentCapability.state || 'detected')}
                strokeWidth={2}
                dot={{ fill: getStateColor(currentCapability.state || 'detected'), r: 4 }}
                activeDot={{ r: 6 }}
              />
            </LineChart>
          </ResponsiveContainer>
        </div>
      )}

      {/* State Legend */}
      <div className="flex items-center gap-4 text-xs">
        <span className="text-slate-500">States:</span>
        {['detected', 'confirmed', 'exploited', 'chained'].map((state) => (
          <div key={state} className="flex items-center gap-1">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: getStateColor(state) }}
            />
            <span className="text-slate-400">{getStateLabel(state)}</span>
          </div>
        ))}
      </div>
    </div>
  );
};
