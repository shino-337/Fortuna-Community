import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import api from '../lib/api';
import { Insight } from '../types';

export const RiskCenter: React.FC = () => {
  const navigate = useNavigate();
  const [insights, setInsights] = useState<Insight[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'critical' | 'high' | 'medium' | 'low'>('all');
  const [search, setSearch] = useState('');
  const [selectedInsight, setSelectedInsight] = useState<Insight | null>(null);
  const [acknowledging, setAcknowledging] = useState<number | null>(null);
  const [resolving, setResolving] = useState<number | null>(null);

  useEffect(() => {
    const fetchInsights = async () => {
      try {
        const params: any = { pageSize: 100 };
        if (filter !== 'all') {
          params.severity = filter;
        }
        const response = await api.get('/api/v1/insights', { params });
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
    const interval = setInterval(fetchInsights, 30000); // Refresh every 30 seconds
    return () => clearInterval(interval);
  }, [filter]);

  const handleAcknowledge = async (insightId: number) => {
    setAcknowledging(insightId);
    try {
      await api.post(`/api/v1/insights/${insightId}/acknowledge`);
      // Refresh insights
      const response = await api.get('/api/v1/insights', { params: { pageSize: 100 } });
      const data = response.data?.insights || response.data || [];
      setInsights(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error('Failed to acknowledge insight:', err);
      alert('Failed to acknowledge insight');
    } finally {
      setAcknowledging(null);
    }
  };

  const handleResolve = async (insightId: number) => {
    setResolving(insightId);
    try {
      await api.post(`/api/v1/insights/${insightId}/resolve`, {
        resolution: 'Resolved via Risk Center',
      });
      // Refresh insights
      const response = await api.get('/api/v1/insights', { params: { pageSize: 100 } });
      const data = response.data?.insights || response.data || [];
      setInsights(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error('Failed to resolve insight:', err);
      alert('Failed to resolve insight');
    } finally {
      setResolving(null);
    }
  };

  const handleRunAttackPath = (insight: Insight) => {
    // Extract resource UID from affected_resources if available
    const affectedResources = insight.affected_resources || [];
    if (Array.isArray(affectedResources) && affectedResources.length > 0) {
      const firstResource = affectedResources[0];
      const resourceUid = firstResource.uid || firstResource.id || firstResource.name;
      if (resourceUid) {
        navigate(`/attack-paths?resource=${resourceUid}`);
      } else {
        navigate('/attack-paths');
      }
    } else {
      navigate('/attack-paths');
    }
  };

  const getSeverityBadge = (severity: string) => {
    const colors = {
      critical: 'bg-red-100 text-red-800 border-red-300',
      high: 'bg-orange-100 text-orange-800 border-orange-300',
      medium: 'bg-yellow-100 text-yellow-800 border-yellow-300',
      low: 'bg-green-100 text-green-800 border-green-300',
    };
    const icons = {
      critical: '🔴',
      high: '🟠',
      medium: '🟡',
      low: '🟢',
    };
    return (
      <span className={`px-3 py-1 rounded-full text-xs font-semibold border ${colors[severity.toLowerCase() as keyof typeof colors] || colors.low}`}>
        {icons[severity.toLowerCase() as keyof typeof icons] || '🟢'} {severity.toUpperCase()}
      </span>
    );
  };

  const filteredInsights = insights.filter((insight) =>
    insight.description.toLowerCase().includes(search.toLowerCase())
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading risks...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Risk Center</h1>
          <p className="text-gray-600 mt-1">Manage and investigate security risks</p>
        </div>
        <div className="flex space-x-2">
          <Button variant="secondary" onClick={() => window.location.reload()}>
            🔄 Refresh
          </Button>
          <Button variant="secondary">📤 Export</Button>
        </div>
      </div>

      {/* Stats Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card className="p-4">
          <div className="text-sm text-gray-600">Total Risks</div>
          <div className="text-3xl font-bold text-gray-900 mt-1">{insights.length}</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">🔴 Critical</div>
          <div className="text-3xl font-bold text-red-600 mt-1">
            {insights.filter((i) => i.severity === 'critical').length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">🟠 High</div>
          <div className="text-3xl font-bold text-orange-600 mt-1">
            {insights.filter((i) => i.severity === 'high').length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">🟡 Medium</div>
          <div className="text-3xl font-bold text-yellow-600 mt-1">
            {insights.filter((i) => i.severity === 'medium').length}
          </div>
        </Card>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-2">
        <button
          onClick={() => setFilter('all')}
          className={`px-4 py-2 rounded-lg font-medium transition-colors ${
            filter === 'all'
              ? 'bg-blue-600 text-white'
              : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
          }`}
        >
          All ({insights.length})
        </button>
        <button
          onClick={() => setFilter('critical')}
          className={`px-4 py-2 rounded-lg font-medium transition-colors ${
            filter === 'critical'
              ? 'bg-red-600 text-white'
              : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
          }`}
        >
          🔴 Critical ({insights.filter((i) => i.severity === 'critical').length})
        </button>
        <button
          onClick={() => setFilter('high')}
          className={`px-4 py-2 rounded-lg font-medium transition-colors ${
            filter === 'high'
              ? 'bg-orange-600 text-white'
              : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
          }`}
        >
          🟠 High ({insights.filter((i) => i.severity === 'high').length})
        </button>
        <button
          onClick={() => setFilter('medium')}
          className={`px-4 py-2 rounded-lg font-medium transition-colors ${
            filter === 'medium'
              ? 'bg-yellow-600 text-white'
              : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
          }`}
        >
          🟡 Medium ({insights.filter((i) => i.severity === 'medium').length})
        </button>
        <button
          onClick={() => setFilter('low')}
          className={`px-4 py-2 rounded-lg font-medium transition-colors ${
            filter === 'low'
              ? 'bg-green-600 text-white'
              : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
          }`}
        >
          🟢 Low ({insights.filter((i) => i.severity === 'low').length})
        </button>
      </div>

      {/* Search */}
      <div className="relative">
        <input
          type="text"
          placeholder="Search risks by description, type, or resource..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-full px-4 py-3 pl-10 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <span className="absolute left-3 top-3.5 text-gray-400">🔍</span>
      </div>

      {/* Risks List */}
      <div className="space-y-4">
        {filteredInsights.length === 0 ? (
          <Card>
            <div className="text-center py-12">
              <div className="text-6xl mb-4">🔍</div>
              <p className="text-gray-600 text-lg">No risks found</p>
              <p className="text-gray-500 text-sm mt-2">
                {search ? 'Try adjusting your search or filters' : 'All risks have been resolved'}
              </p>
            </div>
          </Card>
        ) : (
          filteredInsights.map((insight) => (
            <Card key={insight.id} className="hover:shadow-lg transition-shadow">
              <div className="flex items-start justify-between">
                <div className="flex-1">
                  <div className="flex items-center space-x-3 mb-3">
                    {getSeverityBadge(insight.severity)}
                    <span className="text-sm text-gray-500 bg-gray-100 px-2 py-1 rounded">
                      {insight.type}
                    </span>
                    <span className="text-xs text-gray-400">
                      ID: {insight.id}
                    </span>
                  </div>
                  
                  <h3 className="text-lg font-semibold text-gray-900 mb-2">
                    {insight.description}
                  </h3>

                  {((insight as any).recommendedAction || (insight as any).recommended_action) && (
                    <div className="bg-blue-50 border-l-4 border-blue-500 p-3 rounded mb-3">
                      <p className="text-sm text-blue-900">
                        <strong>💡 Recommendation:</strong>{' '}
                        {(insight as any).recommendedAction || (insight as any).recommended_action}
                      </p>
                    </div>
                  )}

                  {insight.affected_resources && Array.isArray(insight.affected_resources) && insight.affected_resources.length > 0 && (
                    <div className="mb-3">
                      <p className="text-sm text-gray-600 mb-1">
                        <strong>Affected Resources:</strong>
                      </p>
                      <div className="flex flex-wrap gap-2">
                        {insight.affected_resources.slice(0, 3).map((resource: any, idx: number) => (
                          <span
                            key={idx}
                            className="text-xs bg-gray-100 text-gray-700 px-2 py-1 rounded"
                          >
                            {resource.name || resource.type || JSON.stringify(resource)}
                          </span>
                        ))}
                        {insight.affected_resources.length > 3 && (
                          <span className="text-xs text-gray-500">
                            +{insight.affected_resources.length - 3} more
                          </span>
                        )}
                      </div>
                    </div>
                  )}

                  <p className="text-xs text-gray-500">
                    Detected: {new Date((insight as any).createdAt || (insight as any).created_at).toLocaleString()}
                  </p>
                </div>

                <div className="flex flex-col space-y-2 ml-6">
                  <Button
                    variant="primary"
                    size="sm"
                    onClick={() => handleRunAttackPath(insight)}
                    className="whitespace-nowrap"
                  >
                    🔍 Run Attack Path
                  </Button>
                  <Button
                    variant="secondary"
                    size="sm"
                    onClick={() => setSelectedInsight(insight)}
                    className="whitespace-nowrap"
                  >
                    View Details
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleAcknowledge(insight.id)}
                    disabled={acknowledging === insight.id}
                    className="whitespace-nowrap"
                  >
                    {acknowledging === insight.id ? '⏳ Acknowledging...' : '✓ Acknowledge'}
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => handleResolve(insight.id)}
                    disabled={resolving === insight.id}
                    className="whitespace-nowrap text-green-600 hover:text-green-700"
                  >
                    {resolving === insight.id ? '⏳ Resolving...' : '✓ Resolve'}
                  </Button>
                </div>
              </div>
            </Card>
          ))
        )}
      </div>

      {/* Insight Detail Modal */}
      {selectedInsight && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <Card className="max-w-2xl w-full max-h-[90vh] overflow-y-auto">
            <div className="flex justify-between items-start mb-4">
              <h2 className="text-2xl font-bold text-gray-900">Risk Details</h2>
              <button
                onClick={() => setSelectedInsight(null)}
                className="text-gray-400 hover:text-gray-600 text-2xl"
              >
                ×
              </button>
            </div>

            <div className="space-y-4">
              <div>
                <label className="text-sm font-medium text-gray-700">Severity</label>
                <div className="mt-1">{getSeverityBadge(selectedInsight.severity)}</div>
              </div>

              <div>
                <label className="text-sm font-medium text-gray-700">Type</label>
                <p className="mt-1 text-gray-900">{selectedInsight.type}</p>
              </div>

              <div>
                <label className="text-sm font-medium text-gray-700">Description</label>
                <p className="mt-1 text-gray-900">{selectedInsight.description}</p>
              </div>

              {((selectedInsight as any).recommendedAction || (selectedInsight as any).recommended_action) && (
                <div>
                  <label className="text-sm font-medium text-gray-700">Recommendation</label>
                  <p className="mt-1 text-gray-900">
                    {(selectedInsight as any).recommendedAction || (selectedInsight as any).recommended_action}
                  </p>
                </div>
              )}

              {selectedInsight.affected_resources && (
                <div>
                  <label className="text-sm font-medium text-gray-700">Affected Resources</label>
                  <pre className="mt-1 p-3 bg-gray-50 rounded text-xs overflow-auto">
                    {JSON.stringify(selectedInsight.affected_resources, null, 2)}
                  </pre>
                </div>
              )}

              <div className="flex space-x-2 pt-4 border-t">
                <Button
                  variant="primary"
                  onClick={() => {
                    handleRunAttackPath(selectedInsight);
                    setSelectedInsight(null);
                  }}
                >
                  🔍 Run Attack Path
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => {
                    handleAcknowledge(selectedInsight.id);
                    setSelectedInsight(null);
                  }}
                >
                  ✓ Acknowledge
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => {
                    handleResolve(selectedInsight.id);
                    setSelectedInsight(null);
                  }}
                >
                  ✓ Resolve
                </Button>
              </div>
            </div>
          </Card>
        </div>
      )}
    </div>
  );
};

