import React, { useState, useEffect, useMemo, useRef } from 'react';
import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  ShieldAlert,
  Shield,
  Layers,
  Network,
  Activity,
  Settings,
  LogOut,
  Menu,
  X,
  ScrollText,
  Search,
  Globe,
  ChevronDown,
  Check,
  Share2,
  Briefcase,
  FileText,
  UserRound,
} from 'lucide-react';
import { useAuthStore } from '../../store/authStore';
import { useClusterStore } from '../../store/clusterStore';
import { useClusters } from '../../hooks/useClusters';
import { DataControlBar } from '../DataControlBar';
import { NotificationBell } from '../NotificationBell';
import { getClusterDisplayName } from '../../lib/clusterDisplay';
import { can, P } from '../../lib/permissions';
import { fortunaRoleShortLabel } from '../../lib/fortunaRoles';
import type { MissionNavSection } from '../../lib/personaMissionNavigation';
import { useTimeWindowStore, TIME_WINDOW_OPTIONS } from '../../store/timeWindowStore';
import { ShellBanner } from './ShellBanner';

const NAV_ICONS: Record<string, React.ReactElement> = {
  dashboard: <LayoutDashboard size={18} />,
  clusters: <Globe size={18} />,
  resources: <Layers size={18} />,
  network: <Share2 size={18} />,
  risks: <ShieldAlert size={18} />,
  investigation: <Briefcase size={18} />,
  capabilities: <Shield size={18} />,
  rules: <ScrollText size={18} />,
  attackPaths: <Network size={18} />,
  monitoring: <Activity size={18} />,
  governance: <FileText size={18} />,
  reports: <FileText size={18} />,
  settings: <Settings size={18} />,
};

function routeMatches(pathname: string, itemPath: string): boolean {
  const path = (pathname.replace(/\/$/, '') || '/');
  const base = (itemPath.replace(/\/$/, '') || '/');
  if (base === '/') return path === '/';
  if (path === base) return true;
  return path.startsWith(`${base}/`);
}

function navItemMatchesLocation(item: MissionNavSection['items'][number], pathname: string, search: string): boolean {
  if (item.search) {
    return item.path === pathname && item.search === search;
  }
  if (item.path === pathname) return true;
  if (item.path === '/') return pathname === '/';
  return routeMatches(pathname, item.path);
}

export const ShellChrome: React.FC<{
  navSections: MissionNavSection[];
  allowedRoutes: string[];
  /** Cluster and time window controls; off for roles that never read cluster data. */
  showScope?: boolean;
  showFindingSearch?: boolean;
  showBanner?: boolean;
}> = ({
  navSections,
  allowedRoutes,
  showScope = true,
  showFindingSearch = true,
  showBanner = true,
}) => {
  const { user, logout } = useAuthStore();
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { clusters, loading: clustersLoading, availabilityIssue: clusterAvailabilityIssue } = useClusters();
  const [clusterDropdownOpen, setClusterDropdownOpen] = useState(false);
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [globalSearchQuery, setGlobalSearchQuery] = useState('');
  const mainRef = useRef<HTMLElement | null>(null);

  const navUser = useMemo(() => {
    if (!user) return user;
    if (user.permissions?.length) return user;
    if (String(user.role || '').toLowerCase() === 'admin') {
      return { ...user, permissions: Object.values(P) };
    }
    return user;
  }, [user]);

  const canSearchFindings = showFindingSearch && can(navUser, P.findingsRead);
  const canReadNotifications = can(navUser, P.observabilityMetricsRead);

  const handleGlobalSearch = () => {
    const q = globalSearchQuery.trim();
    if (!q) return;
    const clusterQs =
      selectedClusterId != null && String(selectedClusterId).trim() !== ''
        ? `&clusterId=${encodeURIComponent(String(selectedClusterId).trim())}`
        : '';
    navigate(`/risks/findings?search=${encodeURIComponent(q)}${clusterQs}`);
    setGlobalSearchQuery('');
  };

  const selectedClusterLabel = selectedClusterId
    ? getClusterDisplayName(clusters.find((c) => c.id === selectedClusterId) ?? { id: selectedClusterId })
    : 'All clusters';
  const timeWindowMinutes = useTimeWindowStore((s) => s.valueMinutes);
  const timeWindowLabel =
    TIME_WINDOW_OPTIONS.find((o) => o.valueMinutes === timeWindowMinutes)?.label ?? `Last ${timeWindowMinutes}m`;
  const accountName = user?.username || user?.email || 'Account';
  const accountInitials = accountName.slice(0, 2).toUpperCase();

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  useEffect(() => {
    if (!isMobileMenuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsMobileMenuOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [isMobileMenuOpen]);

  useEffect(() => {
    setClusterDropdownOpen(false);
    setAccountMenuOpen(false);
    setIsMobileMenuOpen(false);
  }, [location.pathname, location.search]);

  useEffect(() => {
    window.setTimeout(() => mainRef.current?.focus({ preventScroll: true }), 0);
  }, [location.pathname, location.search]);

  useEffect(() => {
    if (clusters.length === 0) return;
    if (selectedClusterId == null) {
      if (clusters.length === 1) setSelectedClusterId(String(clusters[0].id));
      return;
    }
    const selectedExists = clusters.some((cluster) => String(cluster.id) === String(selectedClusterId));
    if (!selectedExists) {
      setSelectedClusterId(clusters.length === 1 ? String(clusters[0].id) : null);
    }
  }, [clusters, selectedClusterId, setSelectedClusterId]);

  const renderClusterSelector = (className = '') => (
    <div className={`relative min-w-0 ${className}`.trim()}>
      <button
        type="button"
        onClick={() => setClusterDropdownOpen((o) => !o)}
        className="flex h-10 w-full min-w-0 items-center gap-2 rounded-lg border border-border bg-surface/80 px-3 text-body text-text transition-colors hover:border-muted hover:bg-surface-2/70 focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 lg:h-9"
        aria-haspopup="dialog"
        aria-expanded={clusterDropdownOpen}
        aria-label={`Scope: ${selectedClusterLabel}, ${timeWindowLabel}`}
      >
        <Globe className="h-4 w-4 shrink-0 text-brand" aria-hidden />
        <span className="min-w-0 flex-1 truncate text-left">
          {selectedClusterLabel}
          <span className="text-muted"> · {timeWindowLabel}</span>
        </span>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted" aria-hidden />
      </button>

      {clusterDropdownOpen ? (
        <div
            className="absolute left-0 top-[calc(100%+0.5rem)] z-popover w-[24rem] max-w-[calc(100vw-2rem)] overflow-hidden rounded-lg border border-border bg-surface shadow-xl shadow-black/30"
          role="dialog"
          aria-label="Scope"
        >
          <p className="px-3 pt-3 pb-1 text-micro font-semibold uppercase tracking-wide text-muted">Cluster</p>
          <div role="listbox" aria-label="Select cluster scope">
          <button
            type="button"
            role="option"
            aria-selected={selectedClusterId == null}
            onClick={() => {
              setSelectedClusterId(null);
              setClusterDropdownOpen(false);
            }}
              className="flex w-full items-center gap-2 px-3 py-2.5 text-left text-body text-text hover:bg-surface-2 focus:outline-none focus-visible:bg-surface-2 focus-visible:ring-2 focus-visible:ring-brand/70"
          >
            <Check className={`h-4 w-4 shrink-0 ${selectedClusterId == null ? 'text-brand' : 'text-transparent'}`} aria-hidden />
            <span className="min-w-0 flex-1 truncate">All clusters</span>
          </button>

          <div className="max-h-72 overflow-y-auto border-t border-border/70 py-1">
            {clusters.length > 0 ? (
              clusters.map((cluster) => (
                <button
                  key={cluster.id}
                  type="button"
                  role="option"
                  aria-selected={selectedClusterId === cluster.id}
                  onClick={() => {
                    setSelectedClusterId(cluster.id);
                    setClusterDropdownOpen(false);
                  }}
                    className="flex w-full items-center gap-2 px-3 py-2.5 text-left text-body text-text hover:bg-surface-2 focus:outline-none focus-visible:bg-surface-2 focus-visible:ring-2 focus-visible:ring-brand/70"
                >
                  <Check
                    className={`h-4 w-4 shrink-0 ${selectedClusterId === cluster.id ? 'text-brand' : 'text-transparent'}`}
                    aria-hidden
                  />
                  <span className="min-w-0 flex-1 truncate">{getClusterDisplayName(cluster)}</span>
                  {cluster.connectionStatus ? (
                    <span className="shrink-0 rounded border border-border bg-base/50 px-1.5 py-0.5 text-meta text-muted">
                      {cluster.connectionStatus}
                    </span>
                  ) : null}
                </button>
              ))
            ) : (
              <div className="px-3 py-3 text-caption text-muted">
                {clusterAvailabilityIssue ? 'Cluster inventory unavailable' : clustersLoading ? 'Loading clusters…' : 'No clusters discovered'}
              </div>
            )}
          </div>
          </div>
          <div className="border-t border-border/70 p-3">
            <p className="pb-2 text-micro font-semibold uppercase tracking-wide text-muted">Time window and refresh</p>
            <DataControlBar className="!border-0 !bg-transparent !p-0" />
          </div>
        </div>
      ) : null}
    </div>
  );

  const renderFindingSearch = (className = '') => (
    <form
      className={`flex min-w-0 items-center gap-2 ${className}`.trim()}
      onSubmit={(e) => {
        e.preventDefault();
        handleGlobalSearch();
      }}
      role="search"
      aria-label="Search findings"
    >
      <div className="relative min-w-0 flex-1">
        <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-2" aria-hidden />
        <input
          type="search"
          value={globalSearchQuery}
          onChange={(e) => setGlobalSearchQuery(e.target.value)}
          placeholder="Search findings"
          aria-label="Search findings"
          className="h-10 w-full rounded-lg border border-border bg-surface pl-9 pr-3 text-body text-text placeholder:text-muted focus:outline-none focus-visible:border-brand focus-visible:ring-2 focus-visible:ring-brand/50 lg:h-9"
        />
      </div>
    </form>
  );

  const renderAccountMenu = () => (
    <div className="relative shrink-0">
      <button
        type="button"
        onClick={() => setAccountMenuOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={accountMenuOpen}
        aria-label={`Account menu for ${accountName}`}
        className="flex h-10 w-10 items-center justify-center rounded-full border border-border bg-surface-2 text-caption font-semibold text-text transition-colors hover:border-muted focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 lg:h-9 lg:w-9"
      >
        {accountInitials}
      </button>
      {accountMenuOpen ? (
        <div
          role="menu"
          aria-label="Account"
          className="absolute right-0 top-[calc(100%+0.5rem)] z-popover w-60 overflow-hidden rounded-lg border border-border bg-surface shadow-xl shadow-black/30"
        >
          <div className="border-b border-border/70 px-3 py-3">
            <p className="truncate text-body font-semibold text-text">{accountName}</p>
            <p className="text-caption text-muted">{fortunaRoleShortLabel(user?.role)}</p>
          </div>
          {allowedRoutes.includes('/account') ? (
            <button
              type="button"
              role="menuitem"
              onClick={() => navigate('/account')}
              className="flex w-full items-center gap-2 px-3 py-2.5 text-left text-body text-text hover:bg-surface-2 focus:outline-none focus-visible:bg-surface-2"
            >
              <UserRound className="h-4 w-4 text-muted" aria-hidden /> Account
            </button>
          ) : null}
          <button
            type="button"
            role="menuitem"
            onClick={handleLogout}
            className="flex w-full items-center gap-2 px-3 py-2.5 text-left text-body text-text hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:bg-surface-2"
          >
            <LogOut className="h-4 w-4" aria-hidden /> Sign out
          </button>
        </div>
      ) : null}
    </div>
  );

  return (
    <div className="flex h-dvh min-w-0 overflow-hidden bg-base">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-skip focus:rounded-md focus:bg-brand focus:px-4 focus:py-2.5 focus:text-sm focus:font-semibold focus:text-white"
      >
        Skip to main content
      </a>

      {isMobileMenuOpen ? (
        <div
          className="fixed inset-0 z-overlay bg-surface/80 lg:hidden"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      ) : null}

      <aside
        className={`fixed lg:static inset-y-0 left-0 z-50 w-64 bg-surface border-r border-border flex flex-col transform transition-transform duration-150 motion-reduce:transition-none ${
          isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'
        }`}
        aria-label="Main navigation"
      >
        <div className="h-16 flex items-center px-6 border-b border-border shrink-0">
          <img src="/logo.png" alt="" className="w-8 h-8 shrink-0" />
          <span className="ml-3 min-w-0 truncate text-lg font-bold text-text">Fortuna</span>
          <button
            type="button"
            className="ml-auto rounded-lg p-1 text-muted transition-colors hover:bg-surface-2/50 hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 lg:hidden"
            onClick={() => setIsMobileMenuOpen(false)}
            aria-label="Close menu"
          >
            <X size={20} />
          </button>
        </div>

        <nav className="flex-1 min-h-0 px-4 pt-4 pb-4 space-y-4 overflow-y-auto">
          {navSections.map((section) => (
            <div key={section.title}>
              <p className="px-3 text-caption font-bold text-muted uppercase tracking-wide mb-2">{section.title}</p>
              <div className="space-y-1">
                {section.items.map((item) => {
                  const to = item.search ? { pathname: item.path, search: item.search } : item.path;
                  const itemActive = navItemMatchesLocation(item, location.pathname, location.search);
                  return (
                    <NavLink
                      key={item.id}
                      to={to}
                      end={item.path === '/'}
                      onClick={() => setIsMobileMenuOpen(false)}
                      className={() =>
                        `flex items-center px-3 py-2.5 rounded-lg text-body font-medium border transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 ${
                          itemActive
                            ? 'bg-brand/15 text-text border-brand/40'
                            : 'text-muted border-transparent hover:bg-surface/80'
                        }`
                      }
                    >
                      <span className="mr-3 shrink-0">{NAV_ICONS[item.iconKey] ?? NAV_ICONS.dashboard}</span>
                      <span className="truncate">{item.label}</span>
                    </NavLink>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>

      <div className="flex min-h-0 flex-1 flex-col min-w-0">
        <header className="relative z-dropdown hidden h-16 shrink-0 items-center gap-3 overflow-visible border-b border-border bg-base/70 px-6 backdrop-blur-md lg:flex">
          {showScope ? renderClusterSelector('w-[17rem] xl:w-[19rem]') : null}
          {canSearchFindings ? renderFindingSearch('w-full max-w-[26rem]') : null}
          <div className="ml-auto flex shrink-0 items-center gap-2">
            {canReadNotifications ? <NotificationBell /> : null}
            {renderAccountMenu()}
          </div>
        </header>

        <header className="relative z-dropdown flex h-14 shrink-0 items-center gap-2 overflow-visible border-b border-border px-4 lg:hidden">
          <button
            type="button"
            onClick={() => setIsMobileMenuOpen(true)}
            aria-label="Open menu"
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-border bg-surface/70 text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60"
          >
            <Menu size={22} aria-hidden />
          </button>
          <span className="min-w-0 flex-1 truncate font-semibold text-text">Fortuna</span>
          {canReadNotifications ? <NotificationBell /> : null}
          {renderAccountMenu()}
        </header>

        {(showScope || canSearchFindings) ? (
          <div className="grid gap-2 border-b border-border bg-base/80 px-4 py-3 sm:grid-cols-2 lg:hidden">
            {showScope ? renderClusterSelector() : null}
            {canSearchFindings ? renderFindingSearch() : null}
          </div>
        ) : null}

        <main
          id="main-content"
          ref={mainRef}
          tabIndex={-1}
          className={`flex-1 min-h-0 flex flex-col overflow-y-auto ${
            location.pathname === '/network-activity' ? 'lg:overflow-hidden' : ''
          } outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand/50`}
        >
          {showBanner ? <ShellBanner allowedRoutes={allowedRoutes} /> : null}
          <Outlet />
        </main>
      </div>

      {(showScope && clusterDropdownOpen) || accountMenuOpen ? (
        <div
          className="fixed inset-0 z-overlay"
          onClick={() => {
            setClusterDropdownOpen(false);
            setAccountMenuOpen(false);
          }}
        />
      ) : null}
    </div>
  );
};
