
import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Notification } from '../types';
import { Bell, Check, Info, AlertTriangle, XCircle, CheckCircle, ArrowLeft } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty } from '../components/PageEmpty';
import { formatDateTime } from '../lib/display';

export const Notifications: React.FC = () => {
  const navigate = useNavigate();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    api.getNotifications().then((data) => {
      setNotifications(data);
      setLoading(false);
    });
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
    <PageLayout
      title="Notifications"
      description="System events from database-backed notifications."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Monitoring
          </Button>
          <Button variant="secondary" onClick={() => setNotifications((prev) => prev.map((n) => ({ ...n, read: true })))}>
            Mark all read
          </Button>
        </div>
      }
    >
      {loading ? (
        <div className="text-slate-500 py-8">Loading notifications...</div>
      ) : notifications.length === 0 ? (
        <PageEmpty title="No notifications" description="No notification records in database." />
      ) : (
        <div className="space-y-4">
          {notifications.map((note) => (
            <div
              key={note.id}
              className={`p-4 rounded-lg border flex items-start space-x-4 transition-colors ${note.read ? 'bg-slate-900 border-slate-800' : 'bg-slate-900 border-pink-500/30 shadow-lg shadow-pink-900/10'}`}
            >
              <div className="mt-1 shrink-0">{getIcon(note.type || note.severity || 'info')}</div>
              <div className="flex-1">
                <div className="flex justify-between items-start">
                  <h3 className={`text-sm font-semibold ${note.read ? 'text-slate-300' : 'text-white'}`}>{note.title}</h3>
                  <span className="text-xs text-slate-500">{formatDateTime(note.timestamp)}</span>
                </div>
                <p className="text-sm text-slate-400 mt-1">{note.message}</p>
                <p className="text-xs text-slate-500 mt-2">
                  severity: {(note.severity || 'info').toUpperCase()}
                  {note.source ? ` · source: ${note.source}` : ''}
                </p>
              </div>
              {!note.read && (
                <button className="text-pink-500 hover:text-pink-400" title="Mark as read" onClick={() => setNotifications((prev) => prev.map((n) => (n.id === note.id ? { ...n, read: true } : n)))}>
                  <Check size={16} />
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </PageLayout>
  );
};
