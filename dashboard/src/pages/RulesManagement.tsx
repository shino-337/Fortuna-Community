import React, { useEffect, useState } from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import api from '../lib/api';

interface Rule {
  id: string;
  name: string;
  category: string;
  severity: 'critical' | 'high' | 'medium' | 'low';
  enabled: boolean;
  description: string;
  conditions: any[];
  base_score: number;
  tags?: string[];
}

export const RulesManagement: React.FC = () => {
  const [rules, setRules] = useState<Rule[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'active' | 'disabled' | 'templates' | 'custom'>('active');
  const [filter, setFilter] = useState<'all' | 'critical' | 'high' | 'medium' | 'low'>('all');
  const [categoryFilter, setCategoryFilter] = useState<string>('all');
  const [selectedRule, setSelectedRule] = useState<Rule | null>(null);
  const [reloading, setReloading] = useState(false);
  const [testResult, setTestResult] = useState<any>(null);

  useEffect(() => {
    fetchRules();
    const interval = setInterval(fetchRules, 60000); // Refresh every minute
    return () => clearInterval(interval);
  }, []);

  const fetchRules = async () => {
    try {
      const params: any = {};
      if (filter !== 'all') {
        params.severity = filter;
      }
      if (categoryFilter !== 'all') {
        params.category = categoryFilter;
      }
      if (activeTab === 'active') {
        params.status = 'active';
      } else if (activeTab === 'disabled') {
        params.status = 'disabled';
      }

      const response = await api.get('/api/v1/rules', { params });
      const data = response.data?.rules || response.data || [];
      setRules(Array.isArray(data) ? data : []);
    } catch (err) {
      console.error('Failed to fetch rules:', err);
      setRules([]);
    } finally {
      setLoading(false);
    }
  };

  const handleReload = async () => {
    setReloading(true);
    try {
      await api.post('/api/v1/rules/reload');
      await fetchRules();
      alert('Rules reloaded successfully');
    } catch (err) {
      console.error('Failed to reload rules:', err);
      alert('Failed to reload rules');
    } finally {
      setReloading(false);
    }
  };

  const handleToggleRule = async (ruleId: string, enabled: boolean) => {
    try {
      await api.put(`/api/v1/rules/${ruleId}`, { enabled: !enabled });
      await fetchRules();
    } catch (err) {
      console.error('Failed to toggle rule:', err);
      alert('Failed to toggle rule');
    }
  };

  const handleTestRule = async (ruleId: string) => {
    try {
      // Sample test resource
      const testResource = {
        kind: 'ClusterRoleBinding',
        metadata: { name: 'test-binding' },
        roleRef: { name: 'cluster-admin', kind: 'ClusterRole' },
        subjects: [{ kind: 'ServiceAccount', name: 'test-sa' }],
      };

      const response = await api.post(`/api/v1/rules/${ruleId}/test`, {
        resource: testResource,
      });
      setTestResult(response.data);
      setSelectedRule(rules.find((r) => r.id === ruleId) || null);
    } catch (err) {
      console.error('Failed to test rule:', err);
      alert('Failed to test rule');
    }
  };

  const getSeverityBadge = (severity: string) => {
    const colors = {
      critical: 'bg-red-100 text-red-800',
      high: 'bg-orange-100 text-orange-800',
      medium: 'bg-yellow-100 text-yellow-800',
      low: 'bg-green-100 text-green-800',
    };
    return (
      <span className={`px-2 py-1 rounded-full text-xs font-medium ${colors[severity.toLowerCase() as keyof typeof colors] || colors.low}`}>
        {severity.toUpperCase()}
      </span>
    );
  };

  const getCategoryIcon = (category: string) => {
    const icons: Record<string, string> = {
      rbac: '🔐',
      'network-policy': '🌐',
      'pod-security': '🛡️',
      secrets: '🔑',
      'runtime-behavior': '⚡',
      compliance: '📋',
    };
    return icons[category] || '📄';
  };

  const activeRules = rules.filter((r) => r.enabled);
  const disabledRules = rules.filter((r) => !r.enabled);

  const displayedRules =
    activeTab === 'active'
      ? activeRules
      : activeTab === 'disabled'
      ? disabledRules
      : rules;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <div className="text-center">
          <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-600 mx-auto mb-4"></div>
          <p className="text-gray-600">Loading rules...</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Rules Management</h1>
          <p className="text-gray-600 mt-1">Configure and manage detection rules</p>
        </div>
        <div className="flex space-x-2">
          <Button variant="secondary" onClick={handleReload} disabled={reloading}>
            {reloading ? '⏳ Reloading...' : '🔄 Reload Rules'}
          </Button>
          <Button variant="secondary">📤 Upload YAML</Button>
          <Button>➕ New Rule</Button>
        </div>
      </div>

      {/* Stats Overview */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
        <Card className="p-4">
          <div className="text-sm text-gray-600">Active Rules</div>
          <div className="text-3xl font-bold text-gray-900 mt-1">{activeRules.length}</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">Disabled</div>
          <div className="text-3xl font-bold text-gray-500 mt-1">{disabledRules.length}</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">Total Rules</div>
          <div className="text-3xl font-bold text-gray-900 mt-1">{rules.length}</div>
        </Card>
        <Card className="p-4">
          <div className="text-sm text-gray-600">Last Reload</div>
          <div className="text-sm text-gray-900 mt-1">Just now</div>
        </Card>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8">
          <button
            onClick={() => setActiveTab('active')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'active'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            Active Rules ({activeRules.length})
          </button>
          <button
            onClick={() => setActiveTab('disabled')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'disabled'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            Disabled ({disabledRules.length})
          </button>
          <button
            onClick={() => setActiveTab('templates')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'templates'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            Templates
          </button>
          <button
            onClick={() => setActiveTab('custom')}
            className={`py-4 px-1 border-b-2 font-medium text-sm ${
              activeTab === 'custom'
                ? 'border-blue-500 text-blue-600'
                : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
            }`}
          >
            Custom
          </button>
        </nav>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <div className="flex gap-2">
          <label className="text-sm font-medium text-gray-700 self-center">Severity:</label>
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
          <label className="text-sm font-medium text-gray-700 self-center">Category:</label>
          <select
            value={categoryFilter}
            onChange={(e) => setCategoryFilter(e.target.value)}
            className="px-3 py-1.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
          >
            <option value="all">All Categories</option>
            <option value="rbac">RBAC</option>
            <option value="network-policy">Network Policy</option>
            <option value="pod-security">Pod Security</option>
            <option value="secrets">Secrets</option>
            <option value="runtime-behavior">Runtime Behavior</option>
            <option value="compliance">Compliance</option>
          </select>
        </div>
      </div>

      {/* Rules Table */}
      <Card>
        <div className="overflow-x-auto">
          <table className="min-w-full divide-y divide-gray-200">
            <thead className="bg-gray-50">
              <tr>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  ID
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Name
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Type
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Severity
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Status
                </th>
                <th className="px-6 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody className="bg-white divide-y divide-gray-200">
              {displayedRules.length === 0 ? (
                <tr>
                  <td colSpan={6} className="px-6 py-8 text-center text-gray-500">
                    No rules found
                  </td>
                </tr>
              ) : (
                displayedRules.map((rule) => (
                  <tr key={rule.id} className="hover:bg-gray-50">
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-mono text-gray-500">
                      {rule.id}
                    </td>
                    <td className="px-6 py-4">
                      <div className="text-sm font-medium text-gray-900">{rule.name}</div>
                      <div className="text-sm text-gray-500">{rule.description}</div>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <span className="text-sm text-gray-500">
                        {getCategoryIcon(rule.category)} {rule.category}
                      </span>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">{getSeverityBadge(rule.severity)}</td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {rule.enabled ? (
                        <span className="px-2 py-1 bg-green-100 text-green-800 rounded-full text-xs font-medium">
                          ✅ Enabled
                        </span>
                      ) : (
                        <span className="px-2 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-medium">
                          ⏸️ Disabled
                        </span>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-sm font-medium">
                      <div className="flex space-x-2">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => setSelectedRule(rule)}
                        >
                          View
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleTestRule(rule.id)}
                        >
                          Test
                        </Button>
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={() => handleToggleRule(rule.id, rule.enabled)}
                        >
                          {rule.enabled ? 'Disable' : 'Enable'}
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

      {/* Rule Detail Modal */}
      {selectedRule && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4">
          <Card className="max-w-3xl w-full max-h-[90vh] overflow-y-auto">
            <div className="flex justify-between items-start mb-4">
              <h2 className="text-2xl font-bold text-gray-900">Rule Details: {selectedRule.id}</h2>
              <button
                onClick={() => {
                  setSelectedRule(null);
                  setTestResult(null);
                }}
                className="text-gray-400 hover:text-gray-600 text-2xl"
              >
                ×
              </button>
            </div>

            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="text-sm font-medium text-gray-700">Name</label>
                  <p className="mt-1 text-gray-900">{selectedRule.name}</p>
                </div>
                <div>
                  <label className="text-sm font-medium text-gray-700">Severity</label>
                  <div className="mt-1">{getSeverityBadge(selectedRule.severity)}</div>
                </div>
                <div>
                  <label className="text-sm font-medium text-gray-700">Category</label>
                  <p className="mt-1 text-gray-900">
                    {getCategoryIcon(selectedRule.category)} {selectedRule.category}
                  </p>
                </div>
                <div>
                  <label className="text-sm font-medium text-gray-700">Status</label>
                  <p className="mt-1">
                    {selectedRule.enabled ? (
                      <span className="px-2 py-1 bg-green-100 text-green-800 rounded-full text-xs font-medium">
                        ✅ Enabled
                      </span>
                    ) : (
                      <span className="px-2 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-medium">
                        ⏸️ Disabled
                      </span>
                    )}
                  </p>
                </div>
              </div>

              <div>
                <label className="text-sm font-medium text-gray-700">Description</label>
                <p className="mt-1 text-gray-900">{selectedRule.description}</p>
              </div>

              <div>
                <label className="text-sm font-medium text-gray-700">Base Score</label>
                <p className="mt-1 text-gray-900">{selectedRule.base_score}/10</p>
              </div>

              {selectedRule.conditions && selectedRule.conditions.length > 0 && (
                <div>
                  <label className="text-sm font-medium text-gray-700">Conditions</label>
                  <pre className="mt-1 p-3 bg-gray-50 rounded text-xs overflow-auto">
                    {JSON.stringify(selectedRule.conditions, null, 2)}
                  </pre>
                </div>
              )}

              {testResult && (
                <div className="border-t pt-4">
                  <label className="text-sm font-medium text-gray-700">Test Result</label>
                  <div className={`mt-2 p-3 rounded ${testResult.match ? 'bg-green-50 border border-green-200' : 'bg-gray-50 border border-gray-200'}`}>
                    <p className={`font-semibold ${testResult.match ? 'text-green-800' : 'text-gray-800'}`}>
                      {testResult.match ? '✅ MATCH' : '❌ NO MATCH'}
                    </p>
                    {testResult.details && (
                      <pre className="mt-2 text-xs overflow-auto">
                        {JSON.stringify(testResult.details, null, 2)}
                      </pre>
                    )}
                  </div>
                </div>
              )}

              <div className="flex space-x-2 pt-4 border-t">
                <Button
                  variant="primary"
                  onClick={() => handleTestRule(selectedRule.id)}
                >
                  🧪 Test Rule
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => handleToggleRule(selectedRule.id, selectedRule.enabled)}
                >
                  {selectedRule.enabled ? '⏸️ Disable' : '▶️ Enable'}
                </Button>
                <Button variant="secondary">📝 Edit</Button>
                <Button variant="secondary">📤 Export YAML</Button>
              </div>
            </div>
          </Card>
        </div>
      )}
    </div>
  );
};

