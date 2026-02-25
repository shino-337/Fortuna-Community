
import React, { useState, useEffect } from 'react';
import { Card } from '../components/ui/Card';
import { User, AuditLog } from '../types';
import { api } from '../lib/api';
import { Shield } from 'lucide-react';
import { PageLayout } from '../components/PageLayout';
import { PageEmpty } from '../components/PageEmpty';
import { formatDateTime } from '../lib/display';

export const Settings: React.FC = () => {
  const [activeTab, setActiveTab] = useState('Users');
  const [users, setUsers] = useState<User[]>([]);
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [loadingUsers, setLoadingUsers] = useState(false);
  const [loadingAudit, setLoadingAudit] = useState(false);

  useEffect(() => {
    if (activeTab === 'Users') {
      setLoadingUsers(true);
      api.getUsers().then((data) => {
        setUsers(data);
        setLoadingUsers(false);
      });
    }
    if (activeTab === 'Audit Logs') {
      setLoadingAudit(true);
      api.getAuditLogs({ page: 1, pageSize: 50 }).then((data) => {
        setAuditLogs(data.logs);
        setLoadingAudit(false);
      });
    }
  }, [activeTab]);

  return (
    <PageLayout title="Settings" description="Administration data from real APIs (users and audit logs).">
      <div className="border-b border-slate-800">
        <nav className="flex space-x-6 overflow-x-auto">
          {['Users', 'Audit Logs'].map((tab) => (
            <button
              key={tab}
              onClick={() => setActiveTab(tab)}
              className={`pb-4 text-sm font-medium border-b-2 transition-colors whitespace-nowrap ${activeTab === tab ? 'border-pink-500 text-pink-500' : 'border-transparent text-slate-400 hover:text-slate-200 hover:border-slate-700'}`}
            >
              {tab}
            </button>
          ))}
        </nav>
      </div>
      <div className="grid gap-6">
        {activeTab === 'Users' && (
          <Card className="overflow-hidden p-0">
            {loadingUsers ? (
              <div className="p-8 text-slate-500">Loading users...</div>
            ) : users.length === 0 ? (
              <PageEmpty title="No users" description="No user records returned by /api/v1/users." className="py-8" />
            ) : (
              <table className="w-full text-sm text-left">
                <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
                  <tr>
                    <th className="px-6 py-4">User</th>
                    <th className="px-6 py-4">Role</th>
                    <th className="px-6 py-4">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {users.map((user) => (
                    <tr key={user.id} className="hover:bg-slate-800/50">
                      <td className="px-6 py-4">
                        <div className="text-white font-medium">{user.username || user.name || 'unknown'}</div>
                        <div className="text-slate-500 text-xs">{user.email || 'N/A'}</div>
                      </td>
                      <td className="px-6 py-4 capitalize text-slate-300">
                        <div className="flex items-center">
                          <Shield className={`w-3 h-3 mr-2 ${user.role === 'admin' ? 'text-pink-500' : 'text-slate-500'}`} />
                          {user.role || 'viewer'}
                        </div>
                      </td>
                      <td className="px-6 py-4">
                        <span className={`px-2 py-0.5 rounded-full text-xs border capitalize ${user.active === false ? 'bg-slate-500/10 text-slate-400 border-slate-500/20' : 'bg-emerald-500/10 text-emerald-500 border-emerald-500/20'}`}>
                          {user.status || (user.active === false ? 'disabled' : 'active')}
                        </span>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Card>
        )}

        {activeTab === 'Audit Logs' && (
          <Card className="overflow-hidden p-0">
            {loadingAudit ? (
              <div className="p-8 text-slate-500">Loading audit logs...</div>
            ) : auditLogs.length === 0 ? (
              <PageEmpty title="No audit logs" description="No records returned by /api/v1/audit." className="py-8" />
            ) : (
              <table className="w-full text-sm text-left">
                <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
                  <tr>
                    <th className="px-6 py-4">Time</th>
                    <th className="px-6 py-4">Actor</th>
                    <th className="px-6 py-4">Action</th>
                    <th className="px-6 py-4">Resource</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {auditLogs.map((log) => (
                    <tr key={log.id} className="hover:bg-slate-800/50">
                      <td className="px-6 py-4 font-mono text-xs text-slate-500">{formatDateTime(log.timestamp)}</td>
                      <td className="px-6 py-4 text-white font-medium">{log.actor || log.user || 'system'}</td>
                      <td className="px-6 py-4 text-slate-300">{log.action}</td>
                      <td className="px-6 py-4 text-slate-400 font-mono text-xs">{log.resource}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </Card>
        )}
      </div>
    </PageLayout>
  );
};
