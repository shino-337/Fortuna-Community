
import React, { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Notification } from '../types';
import { Check, Info, AlertTriangle, XCircle, CheckCircle, ArrowLeft, ExternalLink } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty, PageError, PageLoading } from '../design-system/components/PageStatus';
import { formatDateTime } from '../lib/display';
import { PAGE_TITLES } from '../lib/pageTitles';
import { DataFreshness } from '../components/DataFreshness';

export const Notifications: React.FC = () => {
  const navigate = useNavigate();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const [markingId, setMarkingId] = useState<string | null>(null);

  const loadNotifications = () => {
    setLoading(true);
    api.getNotifications().then((data) => {
      setNotifications(data);
      setError(null);
      setUpdatedAt(new Date());
    }).catch(() => {
      setError('Notifications could not be refreshed.');
    }).finally(() => {
      setLoading(false);
    });
  };

  useEffect(() => {
    loadNotifications();
  }, []);

  const markNotificationRead = async (id: string) => {
    setMarkingId(id);
    try {
      await api.markNotificationRead(id);
      await api.getNotifications().then((data) => {
        setNotifications(data);
        setError(null);
        setUpdatedAt(new Date());
      });
    } catch {
      setError('Notification read state could not be updated.');
    } finally {
      setMarkingId(null);
    }
  };

  const markAllRead = async () => {
    setMarkingId('all');
    try {
      await api.markAllNotificationsRead();
      await api.getNotifications().then((data) => {
        setNotifications(data);
        setError(null);
        setUpdatedAt(new Date());
      });
    } catch {
      setError('Notification read state could not be updated.');
    } finally {
      setMarkingId(null);
    }
  };

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
      title={PAGE_TITLES.notifications}
      description="System events from database-backed notifications."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => navigate('/monitoring')}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back to Monitoring
          </Button>
          <DataFreshness updatedAt={updatedAt} loading={loading} error={error} />
          <Button variant="secondary" onClick={() => loadNotifications()} isLoading={loading}>
            Refresh
          </Button>
          <Button variant="secondary" onClick={markAllRead} isLoading={markingId === 'all'} disabled={notifications.every((n) => n.read)}>
            Mark all read
          </Button>
        </div>
      }
    >
      {loading && notifications.length === 0 ? (
        <PageLoading message="Loading notifications..." className="min-h-[30dvh]" />
      ) : error && notifications.length === 0 ? (
        <PageError
          title="Could not load notifications"
          description="Notification records are unavailable. Retry when the monitoring API is reachable."
          action={<Button variant="secondary" onClick={loadNotifications} isLoading={loading}>Retry notifications</Button>}
        />
      ) : notifications.length === 0 ? (
        <PageEmpty title="No notifications" description="No notification records in database." />
      ) : (
        <div className="space-y-4">
          {notifications.map((note) => (
            <div
              key={note.id}
              className={`p-4 rounded-lg border flex items-start space-x-4 transition-colors ${note.read ? 'bg-surface border-border' : 'bg-surface border-brand/30 shadow-lg shadow-black/20'}`}
            >
              <div className="mt-1 shrink-0" aria-hidden>{getIcon(note.type || note.severity || 'info')}</div>
              <div className="flex-1">
                <div className="flex justify-between items-start">
                  <h3 className={`text-body font-semibold ${note.read ? 'text-muted' : 'text-text'}`}>{note.title}</h3>
                  <span className="text-caption text-muted">{formatDateTime(note.timestamp)}</span>
                </div>
                {note.resourceName ? (
                  <p className="mt-1 text-caption font-semibold text-brand">{note.resourceName}</p>
                ) : null}
                <p className="text-body text-muted mt-1">{note.message}</p>
                <p className="text-caption text-muted mt-2">
                  status: {note.read ? 'read' : 'unread'} ·{' '}
                  severity: {(note.severity || 'info').toUpperCase()}
                  {note.source ? ` · source: ${note.source}` : ''}
                  {note.category ? ` · category: ${note.category}` : ''}
                </p>
                {note.route ? (
                  <button
                    type="button"
                    className="mt-2 inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-caption font-medium text-brand transition-colors hover:bg-brand/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                    onClick={() => navigate(note.route || '/dashboard')}
                  >
                    Open target <ExternalLink size={13} aria-hidden />
                  </button>
                ) : null}
              </div>
              {!note.read && (
                <button
                  type="button"
                  className="inline-flex min-h-10 min-w-10 items-center justify-center rounded-lg text-brand transition-colors hover:bg-brand/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 sm:min-h-8 sm:min-w-8"
                  aria-label={`Mark notification "${note.title}" as read`}
                  disabled={markingId === note.id}
                  onClick={() => void markNotificationRead(note.id)}
                >
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
