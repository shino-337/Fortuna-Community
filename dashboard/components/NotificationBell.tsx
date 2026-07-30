import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Bell, CheckCheck, ShieldAlert, Route, PackageSearch, Bug, Info, Loader2 } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { api } from '../lib/api';
import type { Notification } from '../types';

function severityTone(note: Notification): string {
  const s = String(note.severity || note.type || '').toLowerCase();
  if (s === 'critical' || s === 'error') return 'border-red-500/35 bg-red-500/10 text-red-300';
  if (s === 'high' || s === 'warning') return 'border-amber-500/35 bg-amber-500/10 text-amber-200';
  return 'border-sky-500/25 bg-sky-500/10 text-sky-200';
}

function iconFor(note: Notification): React.ReactNode {
  const key = `${note.category || ''} ${note.source || ''} ${note.title || ''}`.toLowerCase();
  if (key.includes('attack')) return <Route className="h-3.5 w-3.5" aria-hidden />;
  if (key.includes('malware')) return <Bug className="h-3.5 w-3.5" aria-hidden />;
  if (key.includes('sbom') || key.includes('cve')) return <PackageSearch className="h-3.5 w-3.5" aria-hidden />;
  if (key.includes('risk') || key.includes('finding')) return <ShieldAlert className="h-3.5 w-3.5" aria-hidden />;
  return <Info className="h-3.5 w-3.5" aria-hidden />;
}

function notificationRoute(note: Notification): string {
  if (note.route) return note.route;
  const key = `${note.category || ''} ${note.source || ''} ${note.title || ''} ${note.message || ''}`.toLowerCase();
  if (key.includes('attack')) return '/attack-paths';
  if (key.includes('malware') || key.includes('sbom') || key.includes('cve')) return '/resources?tab=Pod';
  if (key.includes('risk') || key.includes('finding')) return '/risks/findings';
  return '/dashboard';
}

function shortTime(value?: string): string {
  if (!value) return 'recent';
  const t = new Date(value).getTime();
  if (!Number.isFinite(t)) return 'recent';
  const delta = Math.max(0, Date.now() - t);
  const min = Math.floor(delta / 60000);
  if (min < 1) return 'now';
  if (min < 60) return `${min}m`;
  const h = Math.floor(min / 60);
  if (h < 24) return `${h}h`;
  return `${Math.floor(h / 24)}d`;
}

export const NotificationBell: React.FC = () => {
  const navigate = useNavigate();
  const rootRef = useRef<HTMLDivElement | null>(null);
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [items, setItems] = useState<Notification[]>([]);
  const [unreadCount, setUnreadCount] = useState(0);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.getNotificationsSummary(20);
      setItems(res.notifications);
      setUnreadCount(res.unreadCount);
    } catch {
      setItems([]);
      setUnreadCount(0);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
    const id = window.setInterval(() => void load(), 30_000);
    return () => window.clearInterval(id);
  }, [load]);

  useEffect(() => {
    if (!open) return undefined;
    const onDown = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    window.addEventListener('mousedown', onDown);
    window.addEventListener('keydown', onKey);
    return () => {
      window.removeEventListener('mousedown', onDown);
      window.removeEventListener('keydown', onKey);
    };
  }, [open]);

  const highPriorityUnread = useMemo(
    () => items.some((n) => !n.read && ['critical', 'high', 'error', 'warning'].includes(String(n.severity || n.type || '').toLowerCase())),
    [items],
  );

  const markAll = async () => {
    await api.markAllNotificationsRead().catch(() => undefined);
    await load();
  };

  const openNotification = async (note: Notification) => {
    if (!note.read && note.id) {
      await api.markNotificationRead(note.id).catch(() => undefined);
    }
    setOpen(false);
    navigate(notificationRoute(note));
    void load();
  };

  return (
    <div ref={rootRef} className={`relative shrink-0 ${open ? 'z-toast' : 'z-0'}`}>
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="relative flex h-9 w-9 items-center justify-center rounded-lg border border-border bg-surface/70 text-muted transition-colors hover:border-brand/40 hover:bg-surface hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
        aria-label={unreadCount > 0 ? `${unreadCount} unread security notifications` : 'Security notifications'}
        aria-expanded={open}
      >
        <Bell className="h-4 w-4" aria-hidden />
        {unreadCount > 0 ? (
          <span
            className={`absolute -right-1 -top-1 min-w-4 rounded-full px-1 text-center text-[10px] font-bold leading-4 text-white ${
              highPriorityUnread ? 'bg-red-500' : 'bg-amber-500'
            }`}
          >
            {unreadCount > 9 ? '9+' : unreadCount}
          </span>
        ) : null}
      </button>

      {open ? (
        <div className="absolute right-0 top-[calc(100%+0.5rem)] z-toast w-[min(92vw,24rem)] overflow-hidden rounded-xl border border-border bg-surface shadow-2xl">
          <div className="flex items-center justify-between gap-3 border-b border-border px-3 py-2.5">
            <div>
              <p className="text-caption font-semibold text-text">Security notifications</p>
              <p className="text-meta text-muted-2">{unreadCount > 0 ? `${unreadCount} unread` : 'No unread alerts'}</p>
            </div>
            <button
              type="button"
              onClick={markAll}
              disabled={unreadCount === 0 || loading}
              className="inline-flex h-8 items-center gap-1.5 rounded-lg px-2 text-caption text-muted transition-colors hover:bg-surface-2 hover:text-text disabled:cursor-not-allowed disabled:opacity-50"
            >
              {loading ? <Loader2 className="h-3.5 w-3.5 animate-spin" aria-hidden /> : <CheckCheck className="h-3.5 w-3.5" aria-hidden />}
              Read all
            </button>
          </div>
          <div className="max-h-[26rem] overflow-y-auto p-2">
            {items.length === 0 ? (
              <p className="px-2 py-6 text-center text-caption text-muted-2">No notifications yet.</p>
            ) : (
              <div className="space-y-1.5">
                {items.map((note) => (
                  <button
                    key={note.id}
                    type="button"
                    onClick={() => void openNotification(note)}
                    className="grid w-full grid-cols-[auto_minmax(0,1fr)_auto] gap-2 rounded-lg px-2 py-2 text-left transition-colors hover:bg-surface-2/70 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
                  >
                    <span className={`mt-0.5 inline-flex h-7 w-7 items-center justify-center rounded-md border ${severityTone(note)}`}>
                      {iconFor(note)}
                    </span>
                    <span className="min-w-0">
                      <span className="block truncate text-caption font-semibold text-text">{note.title || 'Notification'}</span>
                      {note.resourceName ? (
                        <span className="mt-0.5 block truncate text-meta font-semibold text-brand">{note.resourceName}</span>
                      ) : null}
                      <span className="mt-0.5 line-clamp-2 text-caption leading-snug text-muted-2">{note.message || note.source || 'Security activity updated.'}</span>
                    </span>
                    <span className="flex flex-col items-end gap-1">
                      {!note.read ? <span className="h-2 w-2 rounded-full bg-brand" aria-label="Unread" /> : <span className="h-2 w-2" />}
                      <span className="text-meta text-muted-2">{shortTime(note.timestamp)}</span>
                    </span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
};
