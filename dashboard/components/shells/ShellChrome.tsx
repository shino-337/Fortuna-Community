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
} from 'lucide-react';
import { useAuthStore } from '../../store/authStore';
import { useClusterStore } from '../../store/clusterStore';
import { useClusters } from '../../hooks/useClusters';
import { DataControlBar } from '../DataControlBar';
import { NotificationBell } from '../NotificationBell';
import { getClusterDisplayName } from '../../lib/clusterDisplay';
import { can, P } from '../../lib/permissions';
import { fortunaRoleShortLabel, fortunaRoleTooltip } from '../../lib/fortunaRoles';
import type { MissionNavSection } from '../../lib/personaMissionNavigation';

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
  governance: <ScrollText size={18} />,
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
  identityLabel: string;
  identityDescription: string;
  navSections: MissionNavSection[];
  globalStrips?: React.ReactNode;
  showClusterSelector?: boolean;
  showFindingSearch?: boolean;
  showDataControl?: boolean;
}> = ({
  identityLabel,
  identityDescription,
  navSections,
  globalStrips,
  showClusterSelector = true,
  showFindingSearch = true,
  showDataControl = true,
}) => {
  const { user, logout } = useAuthStore();
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { clusters, loading: clustersLoading } = useClusters();
  const [clusterDropdownOpen, setClusterDropdownOpen] = useState(false);
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
        aria-haspopup="listbox"
        aria-expanded={clusterDropdownOpen}
      >
        <Globe className="h-4 w-4 shrink-0 text-brand" aria-hidden />
        <span className="min-w-0 flex-1 truncate text-left">{selectedClusterLabel}</span>
        <ChevronDown className="h-4 w-4 shrink-0 text-muted" aria-hidden />
      </button>

      {clusterDropdownOpen ? (
        <div
            className="absolute right-0 top-[calc(100%+0.5rem)] z-popover w-full min-w-[18rem] overflow-hidden rounded-lg border border-border bg-surface shadow-xl shadow-black/30"
          role="listbox"
          aria-label="Select cluster scope"
        >
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
                {clustersLoading ? 'Loading clusters…' : 'No clusters discovered'}
              </div>
            )}
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
          className="h-10 w-full rounded-lg border border-border bg-surface px-9 text-body text-text placeholder:text-muted focus:outline-none focus-visible:border-brand focus-visible:ring-2 focus-visible:ring-brand/50 lg:h-9"
        />
      </div>
      <button
        type="submit"
        className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg border border-border bg-surface/80 text-muted transition-colors hover:border-muted hover:bg-surface-2 hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/60 disabled:cursor-not-allowed disabled:opacity-50 lg:h-9 lg:w-9"
        disabled={!globalSearchQuery.trim()}
        aria-label="Run findings search"
      >
        <Search className="h-4 w-4" aria-hidden />
      </button>
    </form>
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
        aria-label="Operational navigation"
      >
        <div className="h-16 flex items-center px-6 border-b border-border shrink-0">
          <img src="/logo.png" alt="" className="w-8 h-8 shrink-0" />
          <div className="ml-3 min-w-0">
            <span className="text-lg font-bold text-text block truncate">Fortuna</span>
            <span className="text-meta text-muted block truncate">{identityLabel}</span>
          </div>
            <button
              type="button"
              className="ml-auto rounded-lg p-1 text-muted transition-colors hover:bg-surface-2/50 hover:text-text focus:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 lg:hidden"
              onClick={() => setIsMobileMenuOpen(false)}
              aria-label="Close menu"
            >
            <X size={20} />
          </button>
        </div>

        <p className="shrink-0 px-4 pt-3 pb-2 text-meta text-muted-2 leading-snug">
          {identityDescription}
        </p>

        <nav className="flex-1 min-h-0 px-4 pt-2 pb-4 space-y-4 overflow-y-auto">
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
                            : item.highlight
                              ? 'text-text bg-amber-500/10 border-amber-500/30'
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

        <div className="shrink-0 border-t border-border p-4 lg:hidden">
            <button
              type="button"
              onClick={handleLogout}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-surface/80 py-2.5 text-body font-medium text-muted transition-colors hover:border-critical/30 hover:bg-critical/5 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-critical/70"
            >
            <LogOut size={16} /> Sign out
          </button>
        </div>
      </aside>

      <div className="flex min-h-0 flex-1 flex-col min-w-0">
        <header className="relative z-dropdown hidden h-16 shrink-0 items-center justify-end gap-2 overflow-visible border-b border-border bg-base/70 px-6 backdrop-blur-md lg:flex">
          <div className="flex min-w-0 flex-1 items-center justify-end gap-2 overflow-visible">
            {showClusterSelector ? renderClusterSelector('w-[13rem] xl:w-[14rem]') : null}
            {canSearchFindings ? renderFindingSearch('w-[14rem] xl:w-[17rem]') : null}
            {showDataControl ? <DataControlBar compact className="h-9 !flex-nowrap overflow-hidden !py-0" /> : null}
            {canReadNotifications ? <NotificationBell /> : null}
            <button
              type="button"
              onClick={handleLogout}
              className="flex h-9 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-caption text-muted transition-colors hover:bg-critical/10 hover:text-critical focus:outline-none focus-visible:ring-2 focus-visible:ring-critical/60"
            >
              <LogOut size={14} aria-hidden />
              Sign out
            </button>
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
        </header>

        {(showClusterSelector || canSearchFindings || showDataControl) ? (
          <div className="space-y-2 border-b border-border bg-base/80 px-4 py-3 lg:hidden">
            <div className="grid gap-2 sm:grid-cols-2">
              {showClusterSelector ? renderClusterSelector() : null}
              {canSearchFindings ? renderFindingSearch() : null}
            </div>
            {showDataControl ? <DataControlBar /> : null}
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
          {globalStrips}
          <Outlet />
        </main>

        <footer className="shrink-0 border-t border-border py-2 text-center text-caption text-muted">
          Fortuna · {identityLabel}
        </footer>
      </div>

      {showClusterSelector && clusterDropdownOpen ? (
          <div className="fixed inset-0 z-overlay" onClick={() => setClusterDropdownOpen(false)} />
      ) : null}
    </div>
  );
};
