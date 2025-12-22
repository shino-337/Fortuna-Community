import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import api from '../lib/api';

interface RiskyResource {
  id: string;
  name: string;
  namespace: string;
  type: 'Pod' | 'ServiceAccount' | 'Role' | 'ClusterRole' | 'RoleBinding' | 'ClusterRoleBinding';
  riskScore: number;
  riskLevel: 'critical' | 'high' | 'medium' | 'low';
  insights: number;
  clusterId: string;
}

export const RiskDrivenInventory: React.FC = () => {
  const navigate = useNavigate();
  const [resources, setResources] = useState<RiskyResource[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState<'all' | 'critical' | 'high' | 'medium' | 'low'>('all');
  const [typeFilter, setTypeFilter] = useState<string>('all');
  const [sortBy, setSortBy] = useState<'riskScore' | 'name' | 'insights'>('riskScore');

  useEffect(() => {
    const fetchRiskyResources = async () => {
      try {
        // Fetch all resources and calculate risk scores
        const [podsRes, sasRes, insightsRes] = await Promise.all([
          api.get('/api/v1/pods', { params: { pageSize: 1000 } }),
          api.get('/api/v1/serviceaccounts', { params: { pageSize: 1000 } }),
          api.get('/api/v1/insights', { params: { pageSize: 1000 } }),
        ]);

        const pods = podsRes.data?.pods || podsRes.data || [];
        const serviceAccounts = sasRes.data?.serviceAccounts || sasRes.data || [];
        const insights = insightsRes.data?.insights || insightsRes.data || [];

        // Calculate risk scores for each resource
        const riskyResources: RiskyResource[] = [];

        // Process Pods
        pods.forEach((pod: any) => {
          const podInsights = insights.filter((insight: any) => {
            const affected = insight.affected_resources || [];
            return Array.isArray(affected) && affected.some((r: any) => 
              r.name === pod.name && r.namespace === pod.namespace
            );
          });

          const riskScore = calculateRiskScore(podInsights);
          riskyResources.push({
            id: pod.uid || pod.id,
            name: pod.name,
            namespace: pod.namespace || '',
            type: 'Pod',
            riskScore,
            riskLevel: getRiskLevel(riskScore),
            insights: podInsights.length,
            clusterId: pod.clusterId || 'default',
          });
        });

        // Process ServiceAccounts
        serviceAccounts.forEach((sa: any) => {
          const saInsights = insights.filter((insight: any) => {
            const affected = insight.affected_resources || [];
            return Array.isArray(affected) && affected.some((r: any) => 
              r.name === sa.name && r.namespace === sa.namespace
            );
          });

          const riskScore = calculateRiskScore(saInsights);
          riskyResources.push({
            id: sa.uid || sa.id,
            name: sa.name,
            namespace: sa.namespace || '',
            type: 'ServiceAccount',
            riskScore,
            riskLevel: getRiskLevel(riskScore),
            insights: saInsights.length,
            clusterId: sa.clusterId || 'default',
          });
        });

        // Sort by risk score
        riskyResources.sort((a, b) => b.riskScore - a.riskScore);
        setResources(riskyResources);
      } catch (err) {
        console.error('Failed to fetch risky resources:', err);
        setResources([]);
      } finally {
        setLoading(false);
      }
    };

    fetchRiskyResources();
    const interval = setInterval(fetchRiskyResources, 60000); // Refresh every minute
    return () => clearInterval(interval);
  }, []);

  const calculateRiskScore = (insights: any[]): number => {
    let score = 0;
    insights.forEach((insight: any) => {
      const severity = insight.severity?.toLowerCase() || 'low';
      switch (severity) {
        case 'critical':
          score += 10;
          break;
        case 'high':
          score += 7;
          break;
        case 'medium':
          score += 4;
          break;
        case 'low':
          score += 1;
          break;
      }
    });
    return Math.min(100, score); // Cap at 100
  };

  const getRiskLevel = (score: number): 'critical' | 'high' | 'medium' | 'low' => {
    if (score >= 70) return 'critical';
    if (score >= 40) return 'high';
    if (score >= 15) return 'medium';
    return 'low';
  };

  const getRiskBadge = (level: string) => {
    const colors = {
      critical: 'bg-red-100 text-red-800 border-red-300',
      high: 'bg-orange-100 text-orange-800 border-orange-300',
      medium: 'bg-yellow-100 text-yellow-800 border-yellow-300',
      low: 'bg-green-100 text-green-800 border-green-300',
    };
    return (
      <span className={`px-2 py-1 rounded-full text-xs font-semibold border ${colors[level as keyof typeof colors] || colors.low}`}>
        {level.toUpperCase()}
      </span>
    );
  };

  const getTypeIcon = (type: string) => {
    const icons: Record<string, string> = {
      Pod: '📦',
      ServiceAccount: '👤',
      Role: '🔐',
      ClusterRole: '🔐',
      RoleBinding: '🔗',
      ClusterRoleBinding: '🔗',
    };
    return icons[type] || '📄';
  };

  const filteredAndSorted = resources
    .filter((r) => filter === 'all' || r.riskLevel === filter)
    .filter((r) => typeFilter === 'all' || r.type === typeFilter)
    .sort((a, b) => {
      if (sortBy === 'riskScore') return b.riskScore - a.riskScore;
      if (sortBy === 'name') return a.name.localeCompare(b.name);
      return b.insights - a.insights;
    });

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading risky resources...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Risk-Driven Resource Inventory</h1>
          <p className="text-gray-600 mt-1">Resources sorted by security risk score</p>
        </div>
        <div className="flex space-x-2">
          <Button variant="secondary" onClick={() => window.location.reload()}>
            🔄 Refresh
          </Button>
          <Button variant="secondary">📤 Export</Button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card className="p-4">
          <div className="text-sm text-gray-600">Total Resources</div>
          <div className="text-3xl font-bold text-gray-900 mt-1">{resources.length}</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">🔴 Critical Risk</div>
          <div className="text-3xl font-bold text-red-600 mt-1">
            {resources.filter((r) => r.riskLevel === 'critical').length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">🟠 High Risk</div>
          <div className="text-3xl font-bold text-orange-600 mt-1">
            {resources.filter((r) => r.riskLevel === 'high').length}
          </div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">Avg Risk Score</div>
          <div className="text-3xl font-bold text-gray-900 mt-1">
            {resources.length > 0
              ? Math.round(resources.reduce((sum, r) => sum + r.riskScore, 0) / resources.length)
              : 0}
          </div>
        </Card>
      </div>

      {/* Filters and Sort */}
      <div className="flex flex-wrap gap-4 items-center">
        <div className="flex gap-2">
          <label className="text-sm font-medium text-gray-700 self-center">Risk Level:</label>
          {['all', 'critical', 'high', 'medium', 'low'].map((level) => (
            <button
              key={level}
              onClick={() => setFilter(level as any)}
              className={`px-3 py-1.5 rounded-lg text-sm font-medium transition-colors ${
                filter === level
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-200 text-gray-700 hover:bg-gray-300'
              }`}
            >
              {level === 'all' ? 'All' : level.charAt(0).toUpperCase() + level.slice(1)}
            </button>
          ))}
        </div>

        <div className="flex gap-2">
          <label className="text-sm font-medium text-gray-700 self-center">Type:</label>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="all">All Types</option>
            <option value="Pod">Pods</option>
            <option value="ServiceAccount">ServiceAccounts</option>
            <option value="Role">Roles</option>
            <option value="ClusterRole">ClusterRoles</option>
          </select>
        </div>

        <div className="flex gap-2">
          <label className="text-sm font-medium text-gray-700 self-center">Sort By:</label>
          <select
            value={sortBy}
            onChange={(e) => setSortBy(e.target.value as any)}
            className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="riskScore">Risk Score (High → Low)</option>
            <option value="name">Name (A → Z)</option>
            <option value="insights">Insights Count</option>
          </select>
        </div>
      </div>

      {/* Resources Table */}
      <Card>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Resource
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Namespace
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Risk Score
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Risk Level
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Insights
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {filteredAndSorted.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-6 py-8 text-center text-gray-500">
                    No risky resources found
                  </td>
                </tr>
              ) : (
                filteredAndSorted.map((resource) => (
                  <tr key={resource.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <span className="text-lg mr-2">{getTypeIcon(resource.type)}</span>
                        <span className="text-sm font-medium text-gray-900">{resource.name}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {resource.type}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {resource.namespace || '<cluster>'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <div className="flex items-center">
                        <div className="w-24 bg-gray-200 rounded-full h-2 mr-2">
                          <div
                            className={`h-2 rounded-full ${
                              resource.riskScore >= 70
                                ? 'bg-red-600'
                                : resource.riskScore >= 40
                                ? 'bg-orange-600'
                                : resource.riskScore >= 15
                                ? 'bg-yellow-600'
                                : 'bg-green-600'
                            }`}
                            style={{ width: `${resource.riskScore}%` }}
                          ></div>
                        </div>
                        <span className="text-sm font-medium text-gray-900">{resource.riskScore}</span>
                      </div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">{getRiskBadge(resource.riskLevel)}</td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm text-gray-500">
                      {resource.insights} {resource.insights === 1 ? 'insight' : 'insights'}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                      <div className="flex space-x-2">
                        <Button
                          variant="primary"
                          size="sm"
                          onClick={() => navigate(`/attack-paths?resource=${resource.id}`)}
                        >
                          🔍 Attack Path
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => navigate(`/insights?resource=${resource.id}`)}
                        >
                          View Risks
                        </Button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </Card>
    </div>
  );
};

