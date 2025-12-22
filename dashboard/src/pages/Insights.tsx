import React, { useEffect, useState } from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import api from '../lib/api';
import { Insight } from '../types';

export const Insights: React.FC = () => {
  const [insights, setInsights] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'critical' | 'high' | 'medium' | 'low'>('all');
  const [search, setSearch] = useState('');

  useEffect(() => {
    const fetchInsights = async () => {
      try {
        const params: any = {};
        if (filter !== 'all') {
          params.severity = filter;
        }
        const response = await api.get('/api/v1/insights', { 
          params: { ...params, pageSize: 100 } 
        });
        const data = response.data?.insights || response.data || [];
        setInsights(Array.isArray(data) ? data : []);
      } catch (err) {
        console.error('Failed to fetch insights:', err);
        setInsights([]);
      } finally {
        setLoading(false);
      }
    };

    fetchInsights();
  }, [filter]);

  const getSeverityBadge = (severity: string) => {
    const colors = {
      critical: 'bg-red-100 text-red-800',
      high: 'bg-orange-100 text-orange-800',
      medium: 'bg-yellow-100 text-yellow-800',
      low: 'bg-green-100 text-green-800',
    };
    const icons = {
      critical: '🔴',
      high: '🟡',
      medium: '🟡',
      low: '🟢',
    };
    return (
      <span className={`px-2 py-1 rounded-full text-xs font-medium ${colors[severity as keyof typeof colors] || colors.low}`}>
        {icons[severity as keyof typeof icons] || '🟢'} {severity.toUpperCase()}
      </span>
    );
  };

  const filteredInsights = insights.filter((insight) =>
    insight.description.toLowerCase().includes(search.toLowerCase())
  );

  if (loading) {
    return <div className="text-center py-12">Loading insights...</div>;
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h1 className="text-2xl font-bold text-gray-900">Security Insights</h1>
        <div className="flex space-x-2">
          <Button variant="secondary">Export</Button>
          <Button>Refresh</Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex space-x-2">
        <button
          onClick={() => setFilter('all')}
          className={`px-4 py-2 rounded-lg ${filter === 'all' ? 'bg-blue-600 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          All ({insights.length})
        </button>
        <button
          onClick={() => setFilter('critical')}
          className={`px-4 py-2 rounded-lg ${filter === 'critical' ? 'bg-red-600 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          🔴 Critical
        </button>
        <button
          onClick={() => setFilter('high')}
          className={`px-4 py-2 rounded-lg ${filter === 'high' ? 'bg-orange-600 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          🟡 High
        </button>
        <button
          onClick={() => setFilter('medium')}
          className={`px-4 py-2 rounded-lg ${filter === 'medium' ? 'bg-yellow-600 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          🟡 Medium
        </button>
        <button
          onClick={() => setFilter('low')}
          className={`px-4 py-2 rounded-lg ${filter === 'low' ? 'bg-green-600 text-white' : 'bg-gray-200 text-gray-700'}`}
        >
          🟢 Low
        </button>
      </div>

      {/* Search */}
      <input
        type="text"
        placeholder="Search insights..."
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
      />

      {/* Insights List */}
      <div className="space-y-4">
        {filteredInsights.length === 0 ? (
          <Card>
            <div className="text-center py-12 text-gray-500">No insights found</div>
          </Card>
        ) : (
          filteredInsights.map((insight) => (
            <Card key={insight.id}>
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center space-x-2 mb-2">
                    {getSeverityBadge(insight.severity)}
                    <span className="text-sm text-gray-500">{insight.type}</span>
                  </div>
                  <p className="text-gray-900 mb-2">{insight.description}</p>
                  {((insight as any).recommendedAction || (insight as any).recommended_action) && (
                    <p className="text-sm text-gray-600">
                      <strong>Recommendation:</strong> {(insight as any).recommendedAction || (insight as any).recommended_action}
                    </p>
                  )}
                  <p className="text-xs text-gray-500 mt-2">
                    Detected: {new Date((insight as any).createdAt || (insight as any).created_at).toLocaleString()}
                  </p>
                </div>
                <div className="flex space-x-2 ml-4">
                  <Button variant="ghost" size="sm">
                    View Details
                  </Button>
                  <Button variant="secondary" size="sm">
                    Remediate
                  </Button>
                </div>
              </div>
            </Card>
          ))
        )}
      </div>
    </div>
  );
};

