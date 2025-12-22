import React, { useState } from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';

type SettingsTab = 'general' | 'users' | 'permissions' | 'integrations' | 'notifications' | 'api-keys' | 'audit-logs' | 'backup';

export const Settings: React.FC = () => {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');
  const [refreshInterval, setRefreshInterval] = useState('10');
  const [timeRange, setTimeRange] = useState('1h');
  const [theme, setTheme] = useState('light');
  const [scanFrequency, setScanFrequency] = useState('10');
  const [deepScan, setDeepScan] = useState(true);
  const [autoRemediation, setAutoRemediation] = useState(false);
  const [emailNotifications, setEmailNotifications] = useState(true);
  const [slackNotifications, setSlackNotifications] = useState(true);
  const [minSeverity, setMinSeverity] = useState('high');

  const tabs: Array<{ id: SettingsTab; name: string; icon: string }> = [
    { id: 'general', name: 'General', icon: '⚙️' },
    { id: 'users', name: 'Users', icon: '👥' },
    { id: 'permissions', name: 'Permissions', icon: '🔐' },
    { id: 'integrations', name: 'Integrations', icon: '🔗' },
    { id: 'notifications', icon: '📧', name: 'Notifications' },
    { id: 'api-keys', name: 'API Keys', icon: '🔑' },
    { id: 'audit-logs', name: 'Audit Logs', icon: '📋' },
    { id: 'backup', name: 'Backup & Restore', icon: '💾' },
  ];

  const handleSaveSettings = async () => {
    try {
      // In a real implementation, this would save to backend
      alert('Settings saved successfully');
    } catch (err) {
      console.error('Failed to save settings:', err);
      alert('Failed to save settings');
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-3xl font-bold text-gray-900">Settings</h1>
          <p className="text-gray-600 mt-1">System configuration and administration</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8 overflow-x-auto">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`py-4 px-1 border-b-2 font-medium text-sm whitespace-nowrap ${
                activeTab === tab.id
                  ? 'border-blue-500 text-blue-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              <span className="mr-2">{tab.icon}</span>
              {tab.name}
            </button>
          ))}
        </nav>
      </div>

      {/* Tab Content */}
      {activeTab === 'general' && (
        <div className="space-y-6">
          <Card title="General Settings">
            <div className="space-y-6">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Appearance</label>
                <div className="space-y-2">
                  <div className="flex items-center space-x-4">
                    <label className="flex items-center">
                      <input
                        type="radio"
                        name="theme"
                        value="light"
                        checked={theme === 'light'}
                        onChange={(e) => setTheme(e.target.value)}
                        className="mr-2"
                      />
                      Light
                    </label>
                    <label className="flex items-center">
                      <input
                        type="radio"
                        name="theme"
                        value="dark"
                        checked={theme === 'dark'}
                        onChange={(e) => setTheme(e.target.value)}
                        className="mr-2"
                      />
                      Dark
                    </label>
                    <label className="flex items-center">
                      <input
                        type="radio"
                        name="theme"
                        value="auto"
                        checked={theme === 'auto'}
                        onChange={(e) => setTheme(e.target.value)}
                        className="mr-2"
                      />
                      Auto (follow system)
                    </label>
                  </div>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Dashboard</label>
                <div className="space-y-4">
                  <div>
                    <label className="block text-xs text-gray-600 mb-1">Refresh Interval</label>
                    <select
                      value={refreshInterval}
                      onChange={(e) => setRefreshInterval(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="5">5 seconds</option>
                      <option value="10">10 seconds</option>
                      <option value="30">30 seconds</option>
                      <option value="60">1 minute</option>
                      <option value="300">5 minutes</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs text-gray-600 mb-1">Default Time Range</label>
                    <select
                      value={timeRange}
                      onChange={(e) => setTimeRange(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="1h">1 hour</option>
                      <option value="6h">6 hours</option>
                      <option value="24h">24 hours</option>
                      <option value="7d">7 days</option>
                    </select>
                  </div>
                </div>
              </div>

              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">Scan Settings</label>
                <div className="space-y-4">
                  <div>
                    <label className="block text-xs text-gray-600 mb-1">Scan Frequency</label>
                    <select
                      value={scanFrequency}
                      onChange={(e) => setScanFrequency(e.target.value)}
                      className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                    >
                      <option value="5">Every 5 minutes</option>
                      <option value="10">Every 10 minutes</option>
                      <option value="30">Every 30 minutes</option>
                      <option value="60">Every 1 hour</option>
                    </select>
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <label className="text-sm text-gray-700">Deep Scan</label>
                      <p className="text-xs text-gray-500">Runs daily at 02:00 UTC</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={deepScan}
                      onChange={(e) => setDeepScan(e.target.checked)}
                      className="w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
                    />
                  </div>
                  <div className="flex items-center justify-between">
                    <div>
                      <label className="text-sm text-gray-700">Auto-remediation</label>
                      <p className="text-xs text-gray-500">Requires manual approval if disabled</p>
                    </div>
                    <input
                      type="checkbox"
                      checked={autoRemediation}
                      onChange={(e) => setAutoRemediation(e.target.checked)}
                      className="w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
                    />
                  </div>
                </div>
              </div>

              <Button onClick={handleSaveSettings}>Save Settings</Button>
            </div>
          </Card>
        </div>
      )}

      {activeTab === 'users' && (
        <Card title="User Management">
          <div className="space-y-4">
            <div className="flex justify-between items-center">
              <input
                type="text"
                placeholder="Search users..."
                className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <Button>➕ Invite User</Button>
            </div>
            <div className="text-center py-12 text-gray-500">
              User management interface coming soon...
            </div>
          </div>
        </Card>
      )}

      {activeTab === 'permissions' && (
        <Card title="Role Permissions">
          <div className="space-y-6">
            <div>
              <h3 className="text-lg font-semibold text-gray-900 mb-4">Admin</h3>
              <div className="space-y-2 text-sm">
                <div className="flex items-center text-green-600">✅ View all resources</div>
                <div className="flex items-center text-green-600">✅ Manage clusters</div>
                <div className="flex items-center text-green-600">✅ Manage rules</div>
                <div className="flex items-center text-green-600">✅ Manage users</div>
                <div className="flex items-center text-green-600">✅ Run attack paths</div>
                <div className="flex items-center text-green-600">✅ Acknowledge/resolve risks</div>
                <div className="flex items-center text-green-600">✅ Export reports</div>
                <div className="flex items-center text-green-600">✅ Rotate certificates</div>
                <div className="flex items-center text-green-600">✅ Access audit logs</div>
              </div>
            </div>
            <div>
              <h3 className="text-lg font-semibold text-gray-900 mb-4">User</h3>
              <div className="space-y-2 text-sm">
                <div className="flex items-center text-green-600">✅ View resources</div>
                <div className="flex items-center text-green-600">✅ View risks</div>
                <div className="flex items-center text-green-600">✅ Run attack paths</div>
                <div className="flex items-center text-green-600">✅ Acknowledge risks</div>
                <div className="flex items-center text-red-600">❌ Manage clusters</div>
                <div className="flex items-center text-red-600">❌ Manage rules</div>
                <div className="flex items-center text-red-600">❌ Manage users</div>
              </div>
            </div>
            <div>
              <h3 className="text-lg font-semibold text-gray-900 mb-4">Viewer</h3>
              <div className="space-y-2 text-sm">
                <div className="flex items-center text-green-600">✅ View resources (read-only)</div>
                <div className="flex items-center text-green-600">✅ View risks (read-only)</div>
                <div className="flex items-center text-green-600">✅ View attack paths</div>
                <div className="flex items-center text-red-600">❌ All write operations</div>
              </div>
            </div>
          </div>
        </Card>
      )}

      {activeTab === 'integrations' && (
        <div className="space-y-4">
          <Card title="Slack Integration">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">Status</span>
                <span className="px-2 py-1 bg-green-100 text-green-800 rounded-full text-xs font-medium">
                  ✅ Connected
                </span>
              </div>
              <div className="text-sm">
                <div className="text-gray-600">Workspace: ACME Corp</div>
                <div className="text-gray-600">Channel: #security-alerts</div>
                <div className="text-gray-500 text-xs mt-1">Last notification: 5 min ago</div>
              </div>
              <div className="flex space-x-2">
                <Button variant="secondary">Configure</Button>
                <Button variant="secondary">Test</Button>
                <Button variant="danger">Disconnect</Button>
              </div>
            </div>
          </Card>

          <Card title="Jira Integration">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">Status</span>
                <span className="px-2 py-1 bg-gray-100 text-gray-800 rounded-full text-xs font-medium">
                  ❌ Not configured
                </span>
              </div>
              <Button>Connect to Jira</Button>
            </div>
          </Card>

          <Card title="PagerDuty Integration">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">Status</span>
                <span className="px-2 py-1 bg-green-100 text-green-800 rounded-full text-xs font-medium">
                  ✅ Connected
                </span>
              </div>
              <div className="text-sm">
                <div className="text-gray-600">Service: KSAM Alerts</div>
                <div className="text-gray-500 text-xs mt-1">Last alert: 2 hours ago</div>
              </div>
              <div className="flex space-x-2">
                <Button variant="secondary">Configure</Button>
                <Button variant="secondary">Test</Button>
                <Button variant="danger">Disconnect</Button>
              </div>
            </div>
          </Card>

          <Card title="Email (SMTP)">
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-600">Status</span>
                <span className="px-2 py-1 bg-green-100 text-green-800 rounded-full text-xs font-medium">
                  ✅ Configured
                </span>
              </div>
              <div className="text-sm">
                <div className="text-gray-600">Server: smtp.company.com:587</div>
                <div className="text-gray-600">From: ksam-alerts@company.com</div>
              </div>
              <div className="flex space-x-2">
                <Button variant="secondary">Configure</Button>
                <Button variant="secondary">Test</Button>
              </div>
            </div>
          </Card>
        </div>
      )}

      {activeTab === 'notifications' && (
        <Card title="Notification Settings">
          <div className="space-y-4">
            <div className="flex items-center justify-between">
              <div>
                <label className="text-sm font-medium text-gray-700">Email Notifications</label>
                <p className="text-xs text-gray-500">Receive email alerts for security risks</p>
              </div>
              <input
                type="checkbox"
                checked={emailNotifications}
                onChange={(e) => setEmailNotifications(e.target.checked)}
                className="w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
              />
            </div>
            <div className="flex items-center justify-between">
              <div>
                <label className="text-sm font-medium text-gray-700">Slack Notifications</label>
                <p className="text-xs text-gray-500">Send alerts to Slack channel</p>
              </div>
              <input
                type="checkbox"
                checked={slackNotifications}
                onChange={(e) => setSlackNotifications(e.target.checked)}
                className="w-4 h-4 text-blue-600 rounded focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-2">Minimum Severity</label>
              <select
                value={minSeverity}
                onChange={(e) => setMinSeverity(e.target.value)}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              >
                <option value="critical">Critical</option>
                <option value="high">High</option>
                <option value="medium">Medium</option>
                <option value="low">Low</option>
              </select>
            </div>
            <Button onClick={handleSaveSettings}>Save Settings</Button>
          </div>
        </Card>
      )}

      {activeTab === 'api-keys' && (
        <Card title="API Keys">
          <div className="space-y-4">
            <Button>➕ Generate New API Key</Button>
            <div className="text-center py-12 text-gray-500">
              API key management coming soon...
            </div>
          </div>
        </Card>
      )}

      {activeTab === 'audit-logs' && (
        <Card title="Audit Logs">
          <div className="space-y-4">
            <div className="flex space-x-2">
              <input
                type="text"
                placeholder="Search..."
                className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
              <select className="px-3 py-2 border border-gray-300 rounded-lg text-sm">
                <option>All Users</option>
              </select>
              <select className="px-3 py-2 border border-gray-300 rounded-lg text-sm">
                <option>All Actions</option>
              </select>
              <select className="px-3 py-2 border border-gray-300 rounded-lg text-sm">
                <option>Last 7 days</option>
              </select>
              <Button variant="secondary">Export Logs</Button>
            </div>
            <div className="text-center py-12 text-gray-500">
              Audit log viewer coming soon...
            </div>
          </div>
        </Card>
      )}

      {activeTab === 'backup' && (
        <Card title="Backup & Restore">
          <div className="space-y-4">
            <div>
              <h3 className="text-sm font-medium text-gray-700 mb-2">Configuration Backup</h3>
              <p className="text-sm text-gray-600 mb-4">
                Export all system configuration, rules, and settings
              </p>
              <Button variant="secondary">📥 Download Backup</Button>
            </div>
            <div className="border-t pt-4">
              <h3 className="text-sm font-medium text-gray-700 mb-2">Restore Configuration</h3>
              <p className="text-sm text-gray-600 mb-4">
                Import configuration from a backup file
              </p>
              <div className="flex space-x-2">
                <input
                  type="file"
                  accept=".json,.yaml"
                  className="flex-1 px-3 py-2 border border-gray-300 rounded-lg text-sm"
                />
                <Button variant="secondary">📤 Upload & Restore</Button>
              </div>
            </div>
          </div>
        </Card>
      )}
    </div>
  );
};
