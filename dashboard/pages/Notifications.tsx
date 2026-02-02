
import React, { useEffect, useState } from 'react';
import { api } from '../lib/api';
import { Notification } from '../types';
import { Card } from '../components/ui/Card';
import { Bell, Check, Info, AlertTriangle, XCircle, CheckCircle, Sliders, Mail } from 'lucide-react';
import { Button } from '../components/ui/Button';

export const Notifications: React.FC = () => {
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [activeTab, setActiveTab] = useState('history');

  useEffect(() => {
    api.getNotifications().then(setNotifications);
  }, []);

  const getIcon = (type: string) => {
    switch (type) {
      case 'error': return <XCircle className="w-5 h-5 text-red-500" />;
      case 'warning': return <AlertTriangle className="w-5 h-5 text-amber-500" />;
      case 'success': return <CheckCircle className="w-5 h-5 text-emerald-500" />;
      default: return <Info className="w-5 h-5 text-blue-500" />;
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <div>
          <h1 className="text-2xl font-bold text-white">Notifications</h1>
          <p className="text-slate-400">System alerts and messages.</p>
        </div>
        <div className="space-x-2">
             <Button variant="secondary" onClick={() => setNotifications(prev => prev.map(n => ({...n, read: true})))}>
                Mark all read
            </Button>
        </div>
      </div>

      <div className="border-b border-slate-800">
          <nav className="flex space-x-6">
              {['history', 'channels', 'rules'].map(tab => (
                  <button
                      key={tab}
                      onClick={() => setActiveTab(tab)}
                      className={`pb-4 text-sm font-medium border-b-2 transition-colors capitalize ${
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

      {activeTab === 'history' && (
      <div className="space-y-4">
        {notifications.map((note) => (
          <div 
            key={note.id} 
            className={`
              p-4 rounded-lg border flex items-start space-x-4 transition-colors
              ${note.read 
                ? 'bg-slate-900 border-slate-800' 
                : 'bg-slate-900 border-pink-500/30 shadow-lg shadow-pink-900/10'
              }
            `}
          >
            <div className="mt-1 shrink-0">
              {getIcon(note.type)}
            </div>
            <div className="flex-1">
              <div className="flex justify-between items-start">
                <h3 className={`text-sm font-semibold ${note.read ? 'text-slate-300' : 'text-white'}`}>
                    {note.title}
                </h3>
                <span className="text-xs text-slate-500">{note.timestamp}</span>
              </div>
              <p className="text-sm text-slate-400 mt-1">{note.message}</p>
            </div>
            {!note.read && (
                <button className="text-pink-500 hover:text-pink-400" title="Mark as read">
                    <Check size={16} />
                </button>
            )}
          </div>
        ))}

        {notifications.length === 0 && (
            <div className="text-center py-12 text-slate-500">
                <Bell className="w-12 h-12 mx-auto mb-4 opacity-20" />
                No notifications to display.
            </div>
        )}
      </div>
      )}

      {activeTab === 'channels' && (
          <div className="grid md:grid-cols-2 gap-6">
              <Card title="Slack Integration" actions={<Button variant="secondary" className="text-xs">Configure</Button>}>
                  <div className="flex items-center mt-2">
                       <span className="w-2 h-2 rounded-full bg-emerald-500 mr-2"></span>
                       <span className="text-sm text-slate-300">Connected to <strong>#ksam-alerts</strong></span>
                  </div>
              </Card>
              <Card title="Email" actions={<Button variant="secondary" className="text-xs">Configure</Button>}>
                  <div className="flex items-center mt-2">
                       <span className="w-2 h-2 rounded-full bg-emerald-500 mr-2"></span>
                       <span className="text-sm text-slate-300">Sending to <strong>admin@fortuna.io</strong></span>
                  </div>
              </Card>
               <Card title="PagerDuty" actions={<Button variant="secondary" className="text-xs">Connect</Button>}>
                  <div className="flex items-center mt-2">
                       <span className="w-2 h-2 rounded-full bg-slate-600 mr-2"></span>
                       <span className="text-sm text-slate-400">Not connected</span>
                  </div>
              </Card>
          </div>
      )}

      {activeTab === 'rules' && (
           <Card className="overflow-hidden">
               <table className="w-full text-sm text-left">
                   <thead className="text-xs text-slate-400 uppercase bg-slate-950/30 border-b border-slate-800">
                       <tr>
                           <th className="px-6 py-4">Rule Name</th>
                           <th className="px-6 py-4">Trigger</th>
                           <th className="px-6 py-4">Channels</th>
                           <th className="px-6 py-4">Status</th>
                       </tr>
                   </thead>
                   <tbody className="divide-y divide-slate-800">
                       <tr>
                           <td className="px-6 py-4 font-medium text-white">Critical Insight Detected</td>
                           <td className="px-6 py-4 text-slate-400">Severity = CRITICAL</td>
                           <td className="px-6 py-4 text-slate-300">Slack, Email</td>
                           <td className="px-6 py-4"><span className="text-emerald-400 text-xs font-medium bg-emerald-500/10 px-2 py-0.5 rounded">Enabled</span></td>
                       </tr>
                       <tr>
                           <td className="px-6 py-4 font-medium text-white">Certificate Expiring</td>
                           <td className="px-6 py-4 text-slate-400">Expiry &lt; 30 days</td>
                           <td className="px-6 py-4 text-slate-300">Email</td>
                           <td className="px-6 py-4"><span className="text-emerald-400 text-xs font-medium bg-emerald-500/10 px-2 py-0.5 rounded">Enabled</span></td>
                       </tr>
                   </tbody>
               </table>
           </Card>
      )}
    </div>
  );
};
