
import React, { useState, useEffect, useMemo } from 'react';
import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { useClusterStore } from '../store/clusterStore';
import { api } from '../lib/api';
import { useClusters } from '../hooks/useClusters';
import { DataControlBar } from './DataControlBar';
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
  Share2,
  Briefcase,
  FileText,
} from 'lucide-react';
import { usePersona } from '../hooks/usePersona';
import { useOperationalContext } from '../hooks/useOperationalContext';
import { isNavVisible } from '../lib/visibilityEngine';
import { IncidentModeBanner } from './IncidentModeBanner';
import { EscalationStatusBar } from './EscalationStatusBar';
import { useIncidentMode } from '../hooks/useIncidentMode';
import { MultiIncidentStrip } from './MultiIncidentStrip';
import { TelemetryHealthStrip } from './TelemetryHealthStrip';
import { OperationalFatigueStrip } from './OperationalFatigueStrip';
import { Cluster } from '../types';
import { getClusterDisplayName } from '../lib/clusterDisplay';
import { can, P } from '../lib/permissions';
import { fortunaRoleShortLabel, fortunaRoleTooltip } from '../lib/fortunaRoles';

type NavItem = {
  icon: React.ReactElement;
  label: string;
  path: string;
  search?: string;
  perm?: string;
  permAny?: string[];
  highlight?: boolean;
};
type NavSection = { title: string; items: NavItem[] };

const NAV_ICONS: Record<string, React.ReactElement> = {
  dashboard: <LayoutDashboard size={18} />,
  platformIntegrity: <LayoutDashboard size={18} />,
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

/** Shell layout with navigation — responsive sidebar on desktop, collapsible drawer on mobile. */
// Nav labels are sourced from the page title registry so shell, sidebar, and main titles stay aligned.
export const Layout: React.FC = () => {
  const { user, logout } = useAuthStore();
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const { clusters } = useClusters();
  const [clusterDropdownOpen, setClusterDropdownOpen] = useState(false);
  const [globalSearchQuery, setGlobalSearchQuery] = useState('');

  useEffect(() => {
    if (!isMobileMenuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setIsMobileMenuOpen(false);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [isMobileMenuOpen]);

  const handleGlobalSearch = () => {
    const q = globalSearchQuery.trim();
    if (!q) return;
    // Risk Center overview (/risks) hides the findings table; deep-link to Findings so search is visible and applied.
    const clusterQs =
      selectedClusterId != null && String(selectedClusterId).trim() !== ''
        ? `&clusterId=${encodeURIComponent(String(selectedClusterId).trim())}`
        : '';
    navigate(`/risks/findings?search=${encodeURIComponent(q)}${clusterQs}`);
    setGlobalSearchQuery('');
  };

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const navUser = useMemo(() => {
    if (!user) return user;
    if (user.permissions && user.permissions.length > 0) return user;
    if (String(user.role || '').toLowerCase() === 'admin') {
      return { ...user, permissions: Object.values(P) };
    }
    return user;
  }, [user]);

  const canSearchFindings = can(navUser, P.findingsRead);
  const { navigation: personaNavigation } = usePersona();
  const { user: opUser, personaId, ownership, telemetry } = useOperationalContext();
  const { navCondensed, active: incidentActive } = useIncidentMode();

  const navSections: NavSection[] = useMemo(() => {
    const incidentPriority = ['/investigation', '/risks', '/attack-paths', '/monitoring', '/'];
    const deprioritized = new Set(['/governance', '/reports', '/capabilities', '/rules', '/certificates']);

    return personaNavigation
      .map((section) => ({
        title: section.title,
        items: section.items
          .filter((item) =>
            isNavVisible({
              feature: item.feature,
              user: opUser,
              personaId,
              ownership,
              telemetry,
            }),
          )
          .filter((item) => !navCondensed || !deprioritized.has(item.path))
          .sort((a, b) => {
            if (!navCondensed) return 0;
            const ia = incidentPriority.indexOf(a.path);
            const ib = incidentPriority.indexOf(b.path);
            return (ia === -1 ? 99 : ia) - (ib === -1 ? 99 : ib);
          })
          .map((item) => ({
            icon: NAV_ICONS[item.iconKey] ?? <LayoutDashboard size={18} />,
            label: item.label,
            path: item.path,
            search: item.search,
            highlight: item.highlight || (incidentActive && item.path === '/investigation'),
          })),
      }))
      .filter((s) => s.items.length > 0);
  }, [personaNavigation, opUser, personaId, ownership, telemetry, navCondensed, incidentActive]);

  return (
    <div className="flex min-h-dvh min-w-0 bg-base">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:left-4 focus:top-4 focus:z-skip focus:rounded-md focus:bg-brand focus:px-4 focus:py-2.5 focus:text-sm focus:font-semibold focus:text-white focus:shadow-lg focus:outline-none focus:ring-2 focus:ring-white/40"
      >
        Skip to main content
      </a>
      {/* Mobile Sidebar Overlay */}
      {isMobileMenuOpen && (
        <div 
          className="fixed inset-0 bg-surface/80 backdrop-blur-sm z-40 lg:hidden"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`
        fixed lg:static inset-y-0 left-0 z-50
        w-64 bg-surface text-muted border-r border-border transform transition-transform duration-150 ease-out motion-reduce:transition-none
        ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
        flex flex-col
      `}
        aria-label="Main navigation"
      >
        {/* Logo */}
        <div className="h-16 flex items-center px-6 border-b border-border shrink-0">
          <div className="flex items-center space-x-3">
            <img src="/logo.png" alt="Fortuna" className="w-8 h-8 shrink-0" />
            <span className="text-xl font-bold text-text">Fortuna</span>
          </div>
          <button
            type="button"
            className="ml-auto rounded-lg p-1 text-muted transition-colors hover:bg-surface-2/50 hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 lg:hidden"
            onClick={() => setIsMobileMenuOpen(false)}
            aria-label="Close menu"
          >
            <X size={20} />
          </button>
        </div>

        {/* Nav Links */}
        <nav id="app-sidebar-nav" className="flex-1 min-h-0 px-4 py-6 space-y-4 overflow-y-auto overscroll-y-contain scrollbar-hide">
          {navSections.map((section) => (
            <div key={section.title}>
              <p className="mb-2 px-3 text-caption font-semibold text-muted">{section.title}</p>
              <div className="space-y-1">
                {section.items.map((item) => {
                  const to = (item as { path: string; search?: string }).search
                    ? { pathname: (item as { path: string; search?: string }).path, search: (item as { path: string; search?: string }).search }
                    : item.path;
                  const isActive = location.pathname === item.path && (!(item as { search?: string }).search || location.search === (item as { search?: string }).search);
                  return (
                    <NavLink
                      key={item.path + ((item as { search?: string }).search || '')}
                      to={to}
                      onClick={() => setIsMobileMenuOpen(false)}
                      className={({ isActive: linkActive }) => `
                        group flex items-center px-3 py-2.5 rounded-lg text-body font-medium transition-colors duration-150 motion-reduce:transition-none focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-offset-2 focus-visible:ring-offset-surface
                        ${(linkActive || isActive)
                          ? 'bg-brand/15 text-text border border-brand/40 shadow-md'
                          : (item as { highlight?: boolean }).highlight
                            ? 'text-text bg-amber-500/10 border border-amber-500/30 hover:bg-amber-500/15'
                            : 'text-muted hover:text-text hover:bg-surface/80 border border-transparent hover:border-border/50 hover:shadow-sm'}
                      `}
                    >
                      <span className={`mr-3 shrink-0 ${isActive || location.pathname === item.path ? 'text-brand' : 'text-muted-2 group-hover:text-text'}`}>
                        {item.icon}
                      </span>
                      <span className="flex-1 truncate">{item.label}</span>
                      {(item as { comingSoon?: boolean }).comingSoon && (
                        <span className="shrink-0 text-caption font-medium px-1.5 py-0.5 rounded bg-muted-2/50 text-muted">Soon</span>
                      )}
                    </NavLink>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>

        {/* Mobile / drawer: account + sign out (desktop uses header bar) */}
        <div className="shrink-0 border-t border-border bg-surface-2/30 px-4 py-3 lg:hidden">
          <div className="flex items-center gap-3 min-w-0">
            <div className="w-9 h-9 rounded-full bg-brand/20 text-brand flex items-center justify-center text-caption font-bold shrink-0">
              {user?.name?.charAt(0) || 'U'}
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-body font-medium text-text truncate">{user?.name ?? 'User'}</p>
              <p className="text-caption text-muted truncate" title={fortunaRoleTooltip(user?.role)}>
                {fortunaRoleShortLabel(user?.role)}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={() => {
              setIsMobileMenuOpen(false);
              handleLogout();
            }}
            className="mt-3 flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-surface/80 py-2.5 text-body font-medium text-muted transition-colors hover:border-critical/30 hover:bg-critical/5 hover:text-critical focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-critical/70"
          >
            <LogOut size={16} aria-hidden />
            Sign out
          </button>
        </div>
      </aside>

      {/* Main Content Area — overflow visible so cluster dropdown (absolute) is not clipped by header */}
      <div className="flex-1 flex flex-col min-w-0 relative">
        {/* Desktop Header — no overflow-y clip: cluster listbox opens below and must receive clicks */}
        <header className="relative z-30 hidden lg:flex h-16 shrink-0 items-center justify-end gap-x-3 gap-y-0 bg-base/60 backdrop-blur-md border-b border-border px-6">
          <div className="flex min-w-0 flex-1 items-center gap-3 flex-nowrap">
            {/* Global Cluster Selector */}
            <div className="relative shrink-0 min-w-0">
              <button
                type="button"
                onClick={() => setClusterDropdownOpen((o) => !o)}
                aria-expanded={clusterDropdownOpen}
                aria-haspopup="listbox"
                aria-label="Cluster scope"
                className="flex h-9 min-w-[10rem] max-w-[15rem] items-center gap-2 rounded-lg border border-border bg-surface/70 px-3 text-body text-text transition-colors hover:border-surface-2 hover:bg-surface focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 focus-visible:ring-offset-2 focus-visible:ring-offset-base xl:min-w-[12rem] xl:max-w-[min(22rem,32vw)]"
              >
                <Globe className="w-4 h-4 text-brand shrink-0" />
                <span className="truncate text-left font-medium">
                  {selectedClusterId
                    ? getClusterDisplayName(clusters.find((c) => c.id === selectedClusterId) ?? { id: selectedClusterId })
                    : 'All clusters'}
                </span>
                <ChevronDown className="w-4 h-4 shrink-0 ml-auto text-muted-2" />
              </button>
              {clusterDropdownOpen && (
                <>
                  <div
                    className="fixed inset-0 z-overlay"
                    aria-hidden
                    onClick={() => setClusterDropdownOpen(false)}
                  />
                  <div
                    role="listbox"
                    aria-label="Select cluster"
                    className="absolute left-0 top-full z-dropdown mt-1 max-h-72 w-64 overflow-y-auto rounded-lg border border-border bg-surface py-1 shadow-xl"
                  >
                    <button
                      type="button"
                      onClick={() => { setSelectedClusterId(null); setClusterDropdownOpen(false); }}
                      className={`w-full px-4 py-2 text-left text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 ${!selectedClusterId ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                    >
                      All clusters
                    </button>
                    {clusters.map((c) => (
                      <button
                        key={c.id}
                        type="button"
                        onClick={() => { setSelectedClusterId(c.id); setClusterDropdownOpen(false); }}
                        className={`w-full truncate px-4 py-2 text-left text-body focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 ${selectedClusterId === c.id ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                      >
                        {getClusterDisplayName(c)}
                      </button>
                    ))}
                    {clusters.length === 0 && (
                      <div className="px-4 py-2 text-muted-2 text-body">No clusters</div>
                    )}
                  </div>
                </>
              )}
            </div>
            {canSearchFindings ? (
              <div className="relative group flex items-center min-w-0 flex-1 basis-[12rem] max-w-xs 2xl:max-w-md">
                <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-2 w-4 h-4 group-focus-within:text-brand transition-colors pointer-events-none" />
                <input
                  type="text"
                  value={globalSearchQuery}
                  onChange={(e) => setGlobalSearchQuery(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && handleGlobalSearch()}
                  placeholder="Search findings (Enter)"
                  className="bg-surface/70 border border-border rounded-full pl-9 pr-3 py-1.5 text-body text-text placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-brand focus:border-brand w-full min-w-0 max-w-[16rem] xl:max-w-md transition-colors duration-150 motion-reduce:transition-none"
                  title="Open Risk Operations → Findings with this search (current cluster scope when selected)."
                />
                <button
                  type="button"
                  onClick={handleGlobalSearch}
                  className="ml-1 rounded-full p-1.5 text-muted-2 transition-colors hover:bg-brand/10 hover:text-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                  title="Search findings (opens Findings table)"
                >
                  <Search size={16} />
                </button>
              </div>
            ) : null}
          </div>
          <div className="flex items-center gap-2 sm:gap-3 min-w-0 justify-end shrink-0 flex-nowrap">
            <div className="h-6 w-px bg-border shrink-0 hidden lg:block" aria-hidden />
            <DataControlBar className="!py-1 !px-2 sm:!gap-2 [&_*]:flex-shrink-0" />
            <div className="h-6 w-px bg-border shrink-0 hidden sm:block" aria-hidden />
            <div className="flex items-center gap-2 sm:gap-2.5 px-2 py-1 sm:px-2.5 bg-surface border border-border rounded-lg min-w-0 max-w-full shrink-0">
              <div className="w-8 h-8 rounded-full bg-brand/20 text-brand flex items-center justify-center text-caption font-bold shrink-0">
                {user?.name?.charAt(0) || 'A'}
              </div>
              <div className="flex flex-col min-w-0 max-w-[7rem] xl:max-w-[10rem]">
                <span className="text-body font-medium text-text truncate">
                  {user?.name ?? 'User'}
                </span>
                <span className="text-caption text-muted truncate" title={fortunaRoleTooltip(user?.role)}>
                  {fortunaRoleShortLabel(user?.role)}
                </span>
              </div>
              <button
                type="button"
                onClick={handleLogout}
                className="inline-flex shrink-0 items-center rounded-md px-2 py-1.5 text-caption font-medium text-muted transition-colors hover:bg-critical/10 hover:text-critical focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-critical/70"
                title="Sign out"
              >
                <LogOut size={14} className="sm:mr-1" />
                <span className="hidden sm:inline">Sign Out</span>
              </button>
            </div>
          </div>
        </header>

        {/* Mobile Header: row1 menu | logo | cluster; row2 search + data controls (no squeezed justify-between row) */}
        <header className="relative z-30 lg:hidden min-h-[3.5rem] bg-surface border-b border-border flex flex-col gap-2 px-4 py-2 shrink-0">
          <div className="flex items-center justify-between gap-2 min-w-0 h-12">
            <button
              type="button"
              onClick={() => setIsMobileMenuOpen(true)}
              className="shrink-0 rounded-lg p-2 text-muted transition-colors hover:bg-surface-2/50 hover:text-text focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
              aria-expanded={isMobileMenuOpen}
              aria-controls="app-sidebar-nav"
              aria-label="Open menu"
            >
              <Menu size={24} />
            </button>
            <div className="flex items-center space-x-2 min-w-0 justify-center flex-1">
              <div className="w-6 h-6 bg-brand rounded flex items-center justify-center shrink-0">
                <ShieldAlert className="text-white w-4 h-4" />
              </div>
              <span className="font-bold text-text truncate">Fortuna</span>
            </div>
            <div className="flex items-center gap-1.5 shrink-0">
              <div className="relative">
                <button
                  type="button"
                  onClick={() => setClusterDropdownOpen((o) => !o)}
                  aria-expanded={clusterDropdownOpen}
                  aria-haspopup="listbox"
                  aria-label="Cluster scope"
                  className="flex max-w-[min(100vw-8rem,11rem)] items-center gap-2 rounded-md border border-border bg-surface/80 px-2.5 py-1.5 text-caption text-muted transition-colors hover:border-surface-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70"
                >
                  <Globe className="w-3.5 h-3.5 text-brand shrink-0" />
                  <span className="truncate min-w-0">
                    {selectedClusterId
                      ? getClusterDisplayName(clusters.find((c) => c.id === selectedClusterId) ?? { id: selectedClusterId })
                      : 'All clusters'}
                  </span>
                  <ChevronDown className="w-3.5 h-3.5 text-muted-2 shrink-0" />
                </button>
                {clusterDropdownOpen && (
                  <>
                    <div
                      className="fixed inset-0 z-overlay lg:hidden"
                      aria-hidden
                      onClick={() => setClusterDropdownOpen(false)}
                    />
                    <div
                      role="listbox"
                      aria-label="Select cluster"
                      className="absolute right-0 top-full z-dropdown mt-1 max-h-64 w-56 overflow-y-auto rounded-lg border border-border bg-surface py-1 shadow-xl"
                    >
                      <button
                        type="button"
                        onClick={() => { setSelectedClusterId(null); setClusterDropdownOpen(false); }}
                        className={`w-full px-4 py-2 text-left text-caption focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 ${!selectedClusterId ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                      >
                        All clusters
                      </button>
                      {clusters.map((c) => (
                        <button
                          key={c.id}
                          type="button"
                          onClick={() => { setSelectedClusterId(c.id); setClusterDropdownOpen(false); }}
                          className={`w-full truncate px-4 py-2 text-left text-caption focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/70 ${selectedClusterId === c.id ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                        >
                          {getClusterDisplayName(c)}
                        </button>
                      ))}
                      {clusters.length === 0 && (
                        <div className="px-4 py-2 text-muted-2 text-caption">No clusters</div>
                      )}
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
          <div className="flex flex-col sm:flex-row sm:items-start gap-2 min-w-0">
            {canSearchFindings ? (
              <div className="relative flex-1 min-w-0">
                <Search className="absolute left-2 top-1/2 -translate-y-1/2 text-muted-2 w-3.5 h-3.5 pointer-events-none" />
                <input
                  type="text"
                  value={globalSearchQuery}
                  onChange={(e) => setGlobalSearchQuery(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && (handleGlobalSearch(), e.currentTarget.blur())}
                  placeholder="Search findings"
                  className="w-full min-w-0 pl-7 pr-2 py-2 bg-surface/80 border border-border rounded-md text-body text-text placeholder:text-muted focus:outline-none focus:ring-1 focus:ring-brand"
                  title="Opens Findings with this search (Enter)"
                />
              </div>
            ) : null}
            <div className="min-w-0 w-full sm:w-auto sm:shrink-0">
              <DataControlBar />
            </div>
          </div>
        </header>

        {/* Page content: on /network-activity use flex column so topology flex-1 fills the frame; other pages follow content height */}
        <main
          id="main-content"
          tabIndex={-1}
          className={`flex-1 min-w-0 px-0 py-0 relative scrollbar-thin min-h-0 flex flex-col scroll-mt-4 outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-brand/50 ${
            location.pathname === '/network-activity'
              ? 'max-lg:overflow-y-auto lg:overflow-hidden'
              : 'overflow-y-auto'
          }`}
        >
          <div
            className={`w-full min-w-0 max-w-none min-h-0 ${location.pathname === '/network-activity' ? 'flex-1 flex flex-col' : ''}`}
          >
            <IncidentModeBanner />
            <EscalationStatusBar />
            <MultiIncidentStrip />
            <OperationalFatigueStrip />
            <TelemetryHealthStrip />
            <Outlet />
          </div>
        </main>
        <footer className="shrink-0 border-t border-border bg-surface/50 py-2">
          <div
            className="mx-auto w-full max-w-[90rem] px-4 sm:px-6 lg:px-8 text-center text-caption text-muted"
            title="Build time (UTC). Use this to confirm which dashboard image is running."
          >
            Fortuna Dashboard · build {typeof __BUILD_TIME__ !== 'undefined' ? __BUILD_TIME__ : 'dev'}
          </div>
        </footer>
      </div>
    </div>
  );
};
