import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { User } from '../types';
import { Card } from '../components/ui/Card';
import { Button } from '../components/ui/Button';
import { UserPlus, MoreVertical, Mail, Shield, Key, CheckCircle, XCircle } from 'lucide-react';

export const Users: React.FC = () => {
  const [users, setUsers] = useState<User[]>([]);
  const [activeTab, setActiveTab] = useState('users');

  useEffect(() => {
    api.getUsers().then(setUsers);
  }, []);

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-white">User Management</h1>
          <p className="text-slate-400">Manage access and RBAC roles for the dashboard.</p>
        </div>
        <Button>
          <UserPlus className="w-4 h-4 mr-2" />
          Invite User
        </Button>
      </div>

      <div className="border-b border-border bg-surface/50">
          <nav className="flex space-x-6">
              {['users', 'roles', 'api keys'].map(tab => (
                  <button
                      key={tab}
                      onClick={() => setActiveTab(tab)}
                      className={`pb-4 pt-1 text-sm font-medium border-b-2 transition-colors capitalize ${
                          activeTab === tab 
                          ? 'border-pink-500 text-pink-500' 
                          : 'border-transparent text-muted hover:text-text hover:border-border'
                      }`}
                  >
                      {tab}
                  </button>
              ))}
          </nav>
      </div>

      {activeTab === 'users' && (
      <Card className="p-0 overflow-hidden">
        <div className="overflow-x-auto max-h-[60vh] overflow-y-auto">
          <table className="w-full text-sm text-left">
            <thead className="text-xs text-muted uppercase bg-muted/50 border-b border-border sticky top-0 z-10">
              <tr>
                <th className="px-6 py-4 font-medium">User</th>
                <th className="px-6 py-4 font-medium">Role</th>
                <th className="px-6 py-4 font-medium">Status</th>
                <th className="px-6 py-4 font-medium">Last Login</th>
                <th className="px-6 py-4 font-medium text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-border">
              {users.map((user) => (
                <tr key={user.id} className="hover:bg-muted/30 transition-colors">
                  <td className="px-6 py-4">
                    <div className="flex items-center">
                      <div className="w-10 h-10 rounded-full bg-slate-800 flex items-center justify-center text-slate-400 mr-3 font-semibold border border-slate-700">
                        {user.name.charAt(0)}
                      </div>
                      <div>
                        <div className="font-medium text-white">{user.name}</div>
                        <div className="text-slate-500 flex items-center text-xs mt-0.5">
                          <Mail className="w-3 h-3 mr-1" /> {user.email}
                        </div>
                      </div>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <div className="flex items-center">
                      <Shield className={`w-4 h-4 mr-2 ${user.role === 'admin' ? 'text-pink-500' : 'text-slate-500'}`} />
                      <span className="capitalize text-slate-300">{user.role}</span>
                    </div>
                  </td>
                  <td className="px-6 py-4">
                    <span className="inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 capitalize">
                      {user.status || 'active'}
                    </span>
                  </td>
                  <td className="px-6 py-4 text-slate-400 font-mono text-xs">
                    {user.lastLogin}
                  </td>
                  <td className="px-6 py-4 text-right">
                    <button className="text-slate-500 hover:text-white transition-colors p-2 hover:bg-slate-800 rounded">
                      <MoreVertical className="w-5 h-5" />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Card>
      )}

      {activeTab === 'roles' && (
          <div className="grid md:grid-cols-2 lg:grid-cols-3 gap-6">
              {['Admin', 'User', 'Viewer'].map(role => (
                  <Card key={role} title={`${role} Role`} actions={<Button variant="ghost" className="text-xs">Edit</Button>}>
                      <ul className="space-y-3 mt-2">
                          <li className="flex items-center text-sm text-slate-300">
                              <CheckCircle className="w-4 h-4 text-emerald-500 mr-3" /> View all resources
                          </li>
                          <li className="flex items-center text-sm text-slate-300">
                              {role === 'Admin' ? <CheckCircle className="w-4 h-4 text-emerald-500 mr-3" /> : <XCircle className="w-4 h-4 text-slate-600 mr-3" />} 
                              Manage clusters
                          </li>
                          <li className="flex items-center text-sm text-slate-300">
                              {role === 'Admin' ? <CheckCircle className="w-4 h-4 text-emerald-500 mr-3" /> : <XCircle className="w-4 h-4 text-slate-600 mr-3" />}
                              Manage users
                          </li>
                      </ul>
                  </Card>
              ))}
          </div>
      )}

      {activeTab === 'api keys' && (
          <Card>
              <div className="text-center py-12 text-slate-500">
                  <Key className="w-12 h-12 mx-auto mb-4 opacity-20" />
                  <p>No active API keys found.</p>
                  <Button variant="secondary" className="mt-4">Generate New Key</Button>
              </div>
          </Card>
      )}
    </div>
  );
};