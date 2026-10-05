
import React, { useCallback, useEffect, useRef, useState } from 'react';
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

const PAGE_SIZE = 50;

export const Notifications: React.FC = () => {
  const navigate = useNavigate();
  const [notifications, setNotifications] = useState<Notification[]>([]);
  const [total, setTotal] = useState(0);
  const [unreadCount, setUnreadCount] = useState(0);
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [updatedAt, setUpdatedAt] = useState<Date | null>(null);
  const [markingId, setMarkingId] = useState<string | null>(null);
  const requestRef = useRef(0);

  /** Reload the first page; a newer request supersedes an older one. */
  const loadNotifications = useCallback(async (onlyUnread: boolean) => {
    const requestId = ++requestRef.current;
    setLoading(true);
    try {
      const page = await api.getNotificationsPage({ limit: PAGE_SIZE, unreadOnly: onlyUnread });
      if (requestId !== requestRef.current) return;
      setNotifications(page.notifications);
      setTotal(page.total);
      setUnreadCount(page.unreadCount);
      setError(null);
      setUpdatedAt(new Date());
    } catch {
      if (requestId === requestRef.current) setError('Notifications could not be refreshed.');
    } finally {
      if (requestId === requestRef.current) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadNotifications(unreadOnly);
  }, [loadNotifications, unreadOnly]);

  const loadMore = async () => {
    const requestId = requestRef.current;
    setLoadingMore(true);
    try {
      const page = await api.getNotificationsPage({ limit: PAGE_SIZE, offset: notifications.length, unreadOnly });
      if (requestId !== requestRef.current) return;
      setNotifications((prev) => {
        const seen = new Set(prev.map((n) => n.id));
        return [...prev, ...page.notifications.filter((n) => !seen.has(n.id))];
      });
      setTotal(page.total);
      setUnreadCount(page.unreadCount);
    } catch {
      setError('More notifications could not be loaded.');
    } finally {
      setLoadingMore(false);
    }
  };

  const markNotificationRead = async (id: string) => {
    setMarkingId(id);
    try {
      await api.markNotificationRead(id);
      await loadNotifications(unreadOnly);
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
      await loadNotifications(unreadOnly);
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
      description="Security events for the clusters you can access. Read state is your own."
      actions={
        <div className="flex items-center gap-2 flex-wrap">
          <Button variant="secondary" size="sm" onClick={() => (window.history.length > 1 ? navigate(-1) : navigate('/'))}>
            <ArrowLeft className="w-4 h-4 mr-2" /> Back
          </Button>
          <div className="inline-flex rounded-lg border border-border p-0.5" role="group" aria-label="Filter notifications">
            {([false, true] as const).map((only) => (
              <button
                key={String(only)}
                type="button"
                aria-pressed={unreadOnly === only}
                onClick={() => setUnreadOnly(only)}
                className={`rounded-md px-2.5 py-1 text-caption font-medium transition-colors ${
                  unreadOnly === only ? 'bg-surface-2 text-text' : 'text-muted hover:text-text'
                }`}
              >
                {only ? `Unread (${unreadCount})` : 'All'}
              </button>
            ))}
          </div>
          <DataFreshness updatedAt={updatedAt} loading={loading} error={error} />
          <Button variant="secondary" onClick={() => void loadNotifications(unreadOnly)} isLoading={loading}>
            Refresh
          </Button>
          <Button variant="secondary" onClick={markAllRead} isLoading={markingId === 'all'} disabled={unreadCount === 0}>
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
          action={<Button variant="secondary" onClick={() => void loadNotifications(unreadOnly)} isLoading={loading}>Retry notifications</Button>}
        />
      ) : notifications.length === 0 ? (
        <PageEmpty
          title={unreadOnly ? 'No unread notifications' : 'No notifications'}
          description={unreadOnly ? 'You have read every notification in your scope.' : 'No security events in your scope yet.'}
        />
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
                    onClick={() => navigate(note.route || '/')}
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
          <div className="flex items-center justify-between gap-3 text-caption text-muted">
            <span>
              Showing {notifications.length} of {total}
            </span>
            {notifications.length < total ? (
              <Button variant="secondary" size="sm" onClick={() => void loadMore()} isLoading={loadingMore}>
                Load more
              </Button>
            ) : null}
          </div>
        </div>
      )}
    </PageLayout>
  );
};
