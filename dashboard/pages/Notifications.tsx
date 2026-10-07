import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import { Notification } from '../types';
import { Check, Info, AlertTriangle, XCircle, CheckCircle } from 'lucide-react';
import { Button } from '../components/ui/Button';
import { PageLayout } from '../design-system/layouts/PageLayout';
import { PageEmpty, PageError, PageLoading } from '../design-system/components/PageStatus';
import { When } from '../components/When';
import { PAGE_TITLES } from '../lib/pageTitles';
import { DataFreshness } from '../components/DataFreshness';

const PAGE_SIZE = 50;

/** "Today", "Yesterday", then the date, newest first (the API already sorts by time). */
function groupByDay(notes: Notification[]): Array<[string, Notification[]]> {
  const groups = new Map<string, Notification[]>();
  const today = new Date();
  const yesterday = new Date(today);
  yesterday.setDate(today.getDate() - 1);
  for (const note of notes) {
    const t = note.timestamp ? new Date(note.timestamp) : null;
    const label = !t || Number.isNaN(t.getTime())
      ? 'Earlier'
      : t.toDateString() === today.toDateString()
        ? 'Today'
        : t.toDateString() === yesterday.toDateString()
          ? 'Yesterday'
          : t.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
    const list = groups.get(label) ?? [];
    list.push(note);
    groups.set(label, list);
  }
  return Array.from(groups.entries());
}

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

  /** Opening a notification marks it read; the page refreshes when you come back. */
  const openNotification = (note: Notification) => {
    if (!note.route) return;
    if (!note.read) void api.markNotificationRead(note.id).catch(() => undefined);
    navigate(note.route);
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
        <div className="space-y-6">
          {groupByDay(notifications).map(([day, notes]) => (
            <section key={day}>
              <h2 className="mb-2 text-caption font-semibold uppercase tracking-wide text-muted">{day}</h2>
              <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-surface">
                {notes.map((note) => (
                  <li key={note.id} className="flex items-center gap-3 px-4 py-3 hover:bg-surface-2/60">
                    <span className={`h-2 w-2 shrink-0 rounded-full ${note.read ? 'bg-transparent' : 'bg-brand'}`} aria-hidden />
                    <span className="shrink-0" aria-hidden>{getIcon(note.type || 'info')}</span>
                    <button
                      type="button"
                      className="min-w-0 flex-1 text-left focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 rounded"
                      onClick={() => openNotification(note)}
                      disabled={!note.route}
                      title={note.route ? 'Open' : undefined}
                    >
                      <span className="flex flex-wrap items-baseline gap-x-2">
                        <span className={`text-body ${note.read ? 'text-muted' : 'font-semibold text-text'}`}>{note.title}</span>
                        {!note.read ? <span className="sr-only">(unread)</span> : null}
                        {note.resourceName ? <span className="text-caption text-muted">{note.resourceName}</span> : null}
                      </span>
                      {note.message ? <span className="block truncate text-caption text-muted">{note.message}</span> : null}
                    </button>
                    <When iso={note.timestamp} className="shrink-0 text-caption text-muted" />
                    {!note.read ? (
                      <button
                        type="button"
                        className="inline-flex min-h-10 min-w-10 shrink-0 items-center justify-center rounded-lg text-brand transition-colors hover:bg-brand/10 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 sm:min-h-8 sm:min-w-8"
                        aria-label={`Mark notification "${note.title}" as read`}
                        title="Mark as read"
                        disabled={markingId === note.id}
                        onClick={() => void markNotificationRead(note.id)}
                      >
                        <Check size={16} />
                      </button>
                    ) : (
                      <span className="min-w-10 sm:min-w-8" aria-hidden />
                    )}
                  </li>
                ))}
              </ul>
            </section>
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
