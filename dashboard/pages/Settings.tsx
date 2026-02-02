
import React, { useState, useEffect } from 'react';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { User, Notification, AuditLog } from '../types';
import { api } from '../lib/api';
import { 
    UserPlus, Shield, MoreVertical, 
    Bell, Check, Key, Lock, CheckCircle, XCircle, Slack, Mail, Server, FileText, Download, RotateCcw
} from 'lucide-react';

export const Settings: React.FC = () => {
  const [activeTab, setActiveTab] = useState('General');
  const [users, setUsers] = useState<User[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);

  useEffect(() => {
    if (activeTab === 'Users') api.getUsers().then(setUsers);
    if (activeTab === 'Audit Logs') api.getAuditLogs().then((data) => setAuditLogs(data.logs));
  }, [activeTab]);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Settings & Administration</h1>
      
      <div className="border-b border-slate-800">
          <nav className="flex space-x-6 overflow-x-auto">
              {['General', 'Users', 'Integrations', 'Audit Logs', 'API Keys'].map(tab => (
                  <button
                      key={tab}
                      onClick={() => setActiveTab(tab)}
                      className={`pb-4 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${
                          activeTab === tab 
                          ? 'border-pink-500 text-pink-500' 
                          : 'border-transparent text-slate-400 hover:text-slate-200 hover:border-slate-700'
                      }`}
                  >
                      {tab}
                  </button>
              ))}
          </nav>
      </div>
      
      <div className="grid gap-6">
        {/* === GENERAL TAB === */}
        {activeTab === 'General' && (
            <Card title="System Preferences" description="Global configuration for the dashboard.">
                <div className="space-y-6 max-w-lg">
                    <div className="space-y-4">
                        <h4 className="text-sm font-semibold text-white uppercase tracking-wider border-b border-slate-800 pb-2">Appearance</h4>
                        <div className="flex items-center justify-between">
                            <div>
                                <div className="font-medium text-white">Theme</div>
                                <div className="text-sm text-slate-400">Dark Mode (Default)</div>
                            </div>
                            <div className="flex bg-slate-900 rounded-lg p-1 border border-slate-800">
                                <button className="px-3 py-1 text-xs font-medium bg-slate-800 text-white rounded">Dark</button>
                                <button className="px-3 py-1 text-xs font-medium text-slate-500">Light</button>
                            </div>
                        </div>
                    </div>

                    <div className="space-y-4">
                         <h4 className="text-sm font-semibold text-white uppercase tracking-wider border-b border-slate-800 pb-2">Scanning</h4>
                        <div className="space-y-2">
                            <label className="text-sm font-medium text-slate-300">Scan Frequency</label>
                            <select className="w-full bg-slate-950 border border-slate-700 rounded-lg px-4 py-2 text-white outline-none focus:border-pink-500">
                                <option>Every 10 minutes</option>
                                <option>Every 30 minutes</option>
                                <option>Every hour</option>
                            </select>
                        </div>
                        <div className="flex items-center space-x-3">
                             <input type="checkbox" className="rounded bg-slate-900 border-slate-700 text-pink-600 focus:ring-pink-500" defaultChecked />
                             <label className="text-sm text-slate-300">Enable Deep Scan (Daily at 02:00 UTC)</label>
                        </div>
                    </div>

                    <div className="pt-4">
                        <Button>Save Settings</Button>
                    </div>
                </div>
            </Card>
        )}

        {/* === USERS TAB === */}
        {activeTab === 'Users' && (
            <div className="space-y-6">
                <div className="flex justify-between items-center">
                     <h3 className="text-lg font-medium text-white">User Access Control</h3>
                     <Button size="sm"><UserPlus className="w-4 h-4 mr-2"/> Invite User</Button>
                </div>
                
                <Card className="overflow-hidden p-0">
                    <table className="w-full text-sm text-left">
                        <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
                            <tr>
                                <th className="px-6 py-4">User</th>
                                <th className="px-6 py-4">Role</th>
                                <th className="px-6 py-4">Status</th>
                                <th className="px-6 py-4 text-right">Actions</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800">
                            {users.map(user => (
                                <tr key={user.id} className="hover:bg-slate-800/50">
                                    <td className="px-6 py-4">
                                        <div className="flex items-center">
                                            <div className="w-8 h-8 rounded-full bg-slate-700 flex items-center justify-center text-xs font-bold mr-3">{user.name.charAt(0)}</div>
                                            <div>
                                                <div className="text-white font-medium">{user.name}</div>
                                                <div className="text-slate-500 text-xs">{user.email}</div>
                                            </div>
                                        </div>
                                    </td>
                                    <td className="px-6 py-4 capitalize text-slate-300">
                                        <div className="flex items-center">
                                            <Shield className={`w-3 h-3 mr-2 ${user.role === 'admin' ? 'text-pink-500' : 'text-slate-500'}`} />
                                            {user.role}
                                        </div>
                                    </td>
                                    <td className="px-6 py-4">
                                        <span className="px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-500 text-xs border border-emerald-500/20 capitalize">{user.status}</span>
                                    </td>
                                    <td className="px-6 py-4 text-right">
                                        <button className="text-slate-400 hover:text-white"><MoreVertical className="w-4 h-4"/></button>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </Card>

                <Card title="Role Permissions Matrix">
                    <div className="overflow-x-auto">
                        <table className="w-full text-sm text-left">
                            <thead className="text-xs text-slate-500 uppercase border-b border-slate-800">
                                <tr>
                                    <th className="py-3 font-medium">Permission</th>
                                    <th className="py-3 text-center">Admin</th>
                                    <th className="py-3 text-center">User</th>
                                    <th className="py-3 text-center">Viewer</th>
                                </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-800 text-slate-300">
                                <tr>
                                    <td className="py-3">View Resources</td>
                                    <td className="text-center"><CheckCircle className="w-4 h-4 text-emerald-500 mx-auto"/></td>
                                    <td className="text-center"><CheckCircle className="w-4 h-4 text-emerald-500 mx-auto"/></td>
                                    <td className="text-center"><CheckCircle className="w-4 h-4 text-emerald-500 mx-auto"/></td>
                                </tr>
                                <tr>
                                    <td className="py-3">Manage Clusters</td>
                                    <td className="text-center"><CheckCircle className="w-4 h-4 text-emerald-500 mx-auto"/></td>
                                    <td className="text-center"><XCircle className="w-4 h-4 text-slate-600 mx-auto"/></td>
                                    <td className="text-center"><XCircle className="w-4 h-4 text-slate-600 mx-auto"/></td>
                                </tr>
                                <tr>
                                    <td className="py-3">Acknowledge Risks</td>
                                    <td className="text-center"><CheckCircle className="w-4 h-4 text-emerald-500 mx-auto"/></td>
                                    <td className="text-center"><CheckCircle className="w-4 h-4 text-emerald-500 mx-auto"/></td>
                                    <td className="text-center"><XCircle className="w-4 h-4 text-slate-600 mx-auto"/></td>
                                </tr>
                            </tbody>
                        </table>
                    </div>
                </Card>
            </div>
        )}

        {/* === INTEGRATIONS TAB === */}
        {activeTab === 'Integrations' && (
            <div className="grid md:grid-cols-2 gap-6">
                 <Card title="Slack Integration" actions={<Button variant="secondary" size="sm">Configure</Button>}>
                     <div className="flex items-center mb-4">
                         <div className="p-2 bg-slate-800 rounded mr-3">
                             <Slack className="w-6 h-6 text-white" />
                         </div>
                         <div>
                             <div className="text-white font-medium">Slack</div>
                             <div className="text-slate-400 text-xs">Connected to #ksam-alerts</div>
                         </div>
                         <div className="ml-auto">
                              <span className="text-xs font-medium text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">Active</span>
                         </div>
                     </div>
                     <p className="text-xs text-slate-500">Sends critical risk alerts and daily summaries to your workspace.</p>
                 </Card>

                 <Card title="Email Notifications" actions={<Button variant="secondary" size="sm">Configure</Button>}>
                     <div className="flex items-center mb-4">
                         <div className="p-2 bg-slate-800 rounded mr-3">
                             <Mail className="w-6 h-6 text-white" />
                         </div>
                         <div>
                             <div className="text-white font-medium">SMTP Server</div>
                             <div className="text-slate-400 text-xs">smtp.company.com</div>
                         </div>
                          <div className="ml-auto">
                              <span className="text-xs font-medium text-emerald-400 bg-emerald-500/10 px-2 py-0.5 rounded border border-emerald-500/20">Active</span>
                         </div>
                     </div>
                     <p className="text-xs text-slate-500">Sends email reports to admin@fortuna.io.</p>
                 </Card>

                 <Card title="PagerDuty" actions={<Button variant="secondary" size="sm">Connect</Button>}>
                     <div className="flex items-center mb-4">
                         <div className="p-2 bg-slate-800 rounded mr-3">
                             <Server className="w-6 h-6 text-white" />
                         </div>
                         <div>
                             <div className="text-white font-medium">PagerDuty</div>
                             <div className="text-slate-400 text-xs">Not connected</div>
                         </div>
                     </div>
                     <p className="text-xs text-slate-500">Trigger incidents for critical security events.</p>
                 </Card>
            </div>
        )}

        {/* === AUDIT LOGS TAB === */}
        {activeTab === 'Audit Logs' && (
             <div className="space-y-4">
                <div className="flex justify-between items-center bg-slate-900 p-4 rounded-lg border border-slate-800">
                    <div className="text-sm text-slate-300">
                        Filtering by: <span className="text-white font-medium">Last 7 Days</span>
                    </div>
                    <Button variant="secondary" size="sm">
                        <Download className="w-4 h-4 mr-2" /> Export CSV
                    </Button>
                </div>

                <Card className="overflow-hidden p-0">
                    <table className="w-full text-sm text-left">
                        <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
                            <tr>
                                <th className="px-6 py-4">Time</th>
                                <th className="px-6 py-4">Actor</th>
                                <th className="px-6 py-4">Action</th>
                                <th className="px-6 py-4">Resource</th>
                                <th className="px-6 py-4">Status</th>
                            </tr>
                        </thead>
                        <tbody className="divide-y divide-slate-800">
                            {auditLogs.map(log => (
                                <tr key={log.id} className="hover:bg-slate-800/50">
                                    <td className="px-6 py-4 font-mono text-xs text-slate-500">{log.timestamp}</td>
                                    <td className="px-6 py-4 text-white font-medium">{log.actor}</td>
                                    <td className="px-6 py-4 text-slate-300">{log.action}</td>
                                    <td className="px-6 py-4 text-slate-400 font-mono text-xs">{log.resource}</td>
                                    <td className="px-6 py-4">
                                        <span className={`text-xs font-medium px-2 py-0.5 rounded capitalize ${
                                            log.status === 'success' ? 'text-emerald-400 bg-emerald-500/10' : 
                                            log.status === 'denied' ? 'text-orange-400 bg-orange-500/10' : 'text-slate-400'
                                        }`}>{log.status}</span>
                                    </td>
                                </tr>
                            ))}
                        </tbody>
                    </table>
                </Card>
             </div>
        )}

        {/* === API KEYS TAB === */}
        {activeTab === 'API Keys' && (
             <Card>
                 <div className="text-center py-12 text-slate-500">
                      <Key className="w-12 h-12 mx-auto mb-4 opacity-20" />
                      <p>No active API keys found.</p>
                      <Button variant="secondary" className="mt-4">Generate New Key</Button>
                 </div>
             </Card>
        )}
      </div>
    </div>
  );
};
