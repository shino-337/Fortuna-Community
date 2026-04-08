
import React, { useState, useEffect } from 'react';
import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { useClusterStore } from '../store/clusterStore';
import { api } from '../lib/api';
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
  Share2
} from 'lucide-react';
import { Cluster } from '../types';
import { getClusterDisplayName } from '../lib/clusterDisplay';

// Nav aligned to Dashboard-UX-Specification: Dashboard -> Clusters -> Resources -> Risk Operations -> Capability Knowledge -> Detection & Policy Catalog -> Attack Paths -> Monitoring. Settings at end.
export const Layout: React.FC = () => {
  const { user, logout } = useAuthStore();
  const { selectedClusterId, setSelectedClusterId } = useClusterStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [clusters, setClusters] = useState<Cluster[]>([]);
  const [clusterDropdownOpen, setClusterDropdownOpen] = useState(false);
  const [globalSearchQuery, setGlobalSearchQuery] = useState('');

  useEffect(() => {
    api.getClusters().then(setClusters).catch(() => setClusters([]));
  }, []);

  const handleGlobalSearch = () => {
    const q = globalSearchQuery.trim();
    if (q) {
      navigate(`/risks?search=${encodeURIComponent(q)}`);
      setGlobalSearchQuery('');
    }
  };

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const navSections = [
    {
      title: 'Overview',
      items: [
        { icon: <LayoutDashboard size={18} />, label: 'Dashboard', path: '/' },
        { icon: <Globe size={18} />, label: 'Clusters', path: '/clusters' },
        { icon: <Layers size={18} />, label: 'Resources', path: '/resources' },
        { icon: <Share2 size={18} />, label: 'Network activity', path: '/network-activity' },
      ],
    },
    {
      title: 'Security',
      items: [
        { icon: <ShieldAlert size={18} />, label: 'Risk Operations', path: '/risks' },
        { icon: <Shield size={18} />, label: 'Capability Knowledge', path: '/capabilities' },
        { icon: <ScrollText size={18} />, label: 'Detection & Policy Catalog', path: '/rules' },
        { icon: <Network size={18} />, label: 'Attack Paths', path: '/attack-paths', comingSoon: true },
      ],
    },
    {
      title: 'Operations',
      items: [
        { icon: <Activity size={18} />, label: 'Monitoring', path: '/monitoring' },
      ],
    },
    {
      title: 'Administration',
      items: [
        { icon: <Settings size={18} />, label: 'Settings', path: '/settings' },
      ],
    },
  ];

  const navItemsFlat = navSections.flatMap((s) => s.items);
  const currentTitle = navItemsFlat.find((i) => {
    if (i.path !== location.pathname) return false;
    if (i.search && location.search !== i.search) return false;
    return true;
  })?.label || navItemsFlat.find((i) => i.path === location.pathname)?.label || 'Dashboard';

  return (
    <div className="min-h-screen bg-base flex">
      {/* Mobile Sidebar Overlay */}
      {isMobileMenuOpen && (
        <div 
          className="fixed inset-0 bg-surface/80 backdrop-blur-sm z-40 lg:hidden"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside className={`
        fixed lg:static inset-y-0 left-0 z-50
        w-64 bg-surface text-muted border-r border-border transform transition-transform duration-300 ease-in-out
        ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
        flex flex-col
      `}>
        {/* Logo */}
        <div className="h-16 flex items-center px-6 border-b border-border shrink-0">
          <div className="flex items-center space-x-3">
            <img src="/logo.png" alt="Fortuna" className="w-8 h-8 shrink-0" />
            <span className="text-xl font-bold tracking-tight text-text">Fortuna</span>
          </div>
          <button 
            className="ml-auto lg:hidden text-muted hover:text-text transition-colors"
            onClick={() => setIsMobileMenuOpen(false)}
          >
            <X size={20} />
          </button>
        </div>

        {/* Nav Links */}
        <nav className="flex-1 px-4 py-6 space-y-4 overflow-y-auto scrollbar-hide">
          {navSections.map((section) => (
            <div key={section.title}>
              <p className="px-3 text-[10px] font-bold text-muted-2 uppercase tracking-widest mb-2">{section.title}</p>
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
                        flex items-center px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200
                        ${(linkActive || isActive)
                          ? 'bg-brand/10 text-brand border border-brand/30 shadow-sm'
                          : 'text-muted hover:text-text hover:bg-surface-2 border border-transparent'}
                      `}
                    >
                      <span className={`mr-3 ${isActive || location.pathname === item.path ? 'text-brand' : 'text-muted-2 group-hover:text-text'}`}>
                        {item.icon}
                      </span>
                      <span className="flex-1 truncate">{item.label}</span>
                      {(item as { comingSoon?: boolean }).comingSoon && (
                        <span className="shrink-0 text-[10px] font-medium px-1.5 py-0.5 rounded bg-muted-2/50 text-muted text-xs">Soon</span>
                      )}
                    </NavLink>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden relative">
        {/* Desktop Header — flex-wrap + min-w-0 so controls don’t overflow when the viewport is narrowed */}
        <header className="hidden lg:flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-2 min-h-16 bg-base/60 backdrop-blur-md border-b border-border px-4 xl:px-8 z-10">
          <div className="flex flex-wrap items-center gap-3 min-w-0 flex-1 basis-[min(100%,22rem)]">
            <h2 className="text-lg font-semibold text-text shrink-0 min-w-0 max-w-[10rem] xl:max-w-[14rem] 2xl:max-w-none truncate">
              {currentTitle}
            </h2>
            {/* Global Cluster Selector */}
            <div className="relative shrink-0 min-w-0">
              <button
                type="button"
                onClick={() => setClusterDropdownOpen((o) => !o)}
                className="flex items-center gap-2 px-3 py-1.5 bg-surface/70 border border-border rounded-lg text-sm text-muted hover:border-surface-2 transition-colors min-w-[10rem] max-w-[14rem] xl:min-w-[180px] xl:max-w-none"
              >
                <Globe className="w-4 h-4 text-brand shrink-0" />
                <span className="truncate">
                  {selectedClusterId
                    ? getClusterDisplayName(clusters.find((c) => c.id === selectedClusterId) ?? { id: selectedClusterId })
                    : 'All clusters'}
                </span>
                <ChevronDown className="w-4 h-4 shrink-0 ml-auto text-muted-2" />
              </button>
              {clusterDropdownOpen && (
                <>
                  <div className="fixed inset-0 z-20" onClick={() => setClusterDropdownOpen(false)} />
                  <div className="absolute top-full left-0 mt-1 w-64 max-h-72 overflow-y-auto bg-surface border border-border rounded-lg shadow-xl z-30 py-1">
                    <button
                      type="button"
                      onClick={() => { setSelectedClusterId(null); setClusterDropdownOpen(false); }}
                      className={`w-full text-left px-4 py-2 text-sm ${!selectedClusterId ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                    >
                      All clusters
                    </button>
                    {clusters.map((c) => (
                      <button
                        key={c.id}
                        type="button"
                        onClick={() => { setSelectedClusterId(c.id); setClusterDropdownOpen(false); }}
className={`w-full text-left px-4 py-2 text-sm truncate ${selectedClusterId === c.id ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                    >
                        {getClusterDisplayName(c)}
                      </button>
                    ))}
                    {clusters.length === 0 && (
                      <div className="px-4 py-2 text-muted-2 text-sm">No clusters</div>
                    )}
                  </div>
                </>
              )}
            </div>
            <div className="relative group flex items-center min-w-0 flex-1 basis-[12rem] max-w-xs 2xl:max-w-md">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-2 w-4 h-4 group-focus-within:text-brand transition-colors pointer-events-none" />
              <input
                type="text"
                value={globalSearchQuery}
                onChange={(e) => setGlobalSearchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleGlobalSearch()}
                placeholder="Search findings (Enter)"
                className="bg-surface/70 border border-border rounded-full pl-9 pr-4 py-1.5 text-sm text-text placeholder-muted-2 focus:outline-none focus:ring-1 focus:ring-brand focus:border-brand w-full min-w-0 transition-all"
                title="Search findings: type and press Enter to open Risk Operations with results."
              />
              <button
                type="button"
                onClick={handleGlobalSearch}
                className="ml-1 p-1.5 rounded-full text-muted-2 hover:text-brand hover:bg-brand/10 transition-colors"
                title="Search findings"
              >
                <Search size={16} />
              </button>
            </div>
          </div>
          <div className="flex flex-wrap items-center gap-3 min-w-0 justify-end shrink-0 w-full lg:w-auto lg:max-w-full">
            <DataControlBar />
            <div className="h-6 w-px bg-border shrink-0 hidden sm:block" aria-hidden />
            <div className="flex items-center gap-2 px-2 py-1 bg-surface border border-border rounded-lg min-w-0 max-w-full">
              <div className="w-7 h-7 rounded-full bg-brand/20 text-brand flex items-center justify-center text-[10px] font-bold shrink-0">
                {user?.name?.charAt(0) || 'A'}
              </div>
              <div className="flex flex-col min-w-0 max-w-[6rem] xl:max-w-[9rem]">
                <span className="text-xs font-medium text-text truncate">
                  {user?.name ?? 'User'}
                </span>
                <span className="text-[10px] text-muted-2 capitalize truncate">
                  {user?.role ?? 'admin'}
                </span>
              </div>
              <button
                type="button"
                onClick={handleLogout}
                className="inline-flex items-center shrink-0 px-2 py-1 rounded-md text-[11px] font-medium text-muted hover:text-critical hover:bg-critical/10 transition-colors"
                title="Sign out"
              >
                <LogOut size={14} className="sm:mr-1" />
                <span className="hidden sm:inline">Sign Out</span>
              </button>
            </div>
          </div>
        </header>

        {/* Mobile Header: row1 menu | logo | cluster; row2 search + data controls (no squeezed justify-between row) */}
        <header className="lg:hidden min-h-[3.5rem] bg-surface border-b border-border flex flex-col gap-2 px-4 py-2 shrink-0 z-10">
          <div className="flex items-center justify-between gap-2 min-w-0 h-12">
            <button
              onClick={() => setIsMobileMenuOpen(true)}
              className="p-2 text-muted hover:text-text shrink-0"
            >
              <Menu size={24} />
            </button>
            <div className="flex items-center space-x-2 min-w-0 justify-center flex-1">
              <div className="w-6 h-6 bg-brand rounded flex items-center justify-center shrink-0">
                <ShieldAlert className="text-white w-4 h-4" />
              </div>
              <span className="font-bold text-text tracking-tight truncate">Fortuna</span>
            </div>
            <div className="flex items-center gap-1.5 shrink-0">
              <div className="relative">
                <button
                  type="button"
                  onClick={() => setClusterDropdownOpen((o) => !o)}
                  className="flex items-center gap-2 px-2.5 py-1.5 bg-surface/80 border border-border rounded-md text-xs text-muted hover:border-surface-2 transition-colors max-w-[min(100vw-8rem,11rem)]"
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
                    <div className="fixed inset-0 z-20" onClick={() => setClusterDropdownOpen(false)} />
                    <div className="absolute right-0 top-full mt-1 w-56 max-h-64 overflow-y-auto bg-surface border border-border rounded-lg shadow-xl z-30 py-1">
                      <button
                        type="button"
                        onClick={() => { setSelectedClusterId(null); setClusterDropdownOpen(false); }}
                        className={`w-full text-left px-4 py-2 text-xs ${!selectedClusterId ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                      >
                        All clusters
                      </button>
                      {clusters.map((c) => (
                        <button
                          key={c.id}
                          type="button"
                          onClick={() => { setSelectedClusterId(c.id); setClusterDropdownOpen(false); }}
                          className={`w-full text-left px-4 py-2 text-xs truncate ${selectedClusterId === c.id ? 'bg-brand/20 text-brand' : 'text-muted hover:bg-surface-2'}`}
                        >
                          {getClusterDisplayName(c)}
                        </button>
                      ))}
                      {clusters.length === 0 && (
                        <div className="px-4 py-2 text-muted-2 text-xs">No clusters</div>
                      )}
                    </div>
                  </>
                )}
              </div>
            </div>
          </div>
          <div className="flex flex-col sm:flex-row sm:items-start gap-2 min-w-0">
            <div className="relative flex-1 min-w-0">
              <Search className="absolute left-2 top-1/2 -translate-y-1/2 text-muted-2 w-3.5 h-3.5 pointer-events-none" />
              <input
                type="text"
                value={globalSearchQuery}
                onChange={(e) => setGlobalSearchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && (handleGlobalSearch(), e.currentTarget.blur())}
                placeholder="Search findings"
                className="w-full min-w-0 pl-7 pr-2 py-1.5 bg-surface/80 border border-border rounded-md text-xs text-text placeholder-muted-2 focus:outline-none focus:ring-1 focus:ring-brand"
                title="Search findings (Enter)"
              />
            </div>
            <div className="min-w-0 w-full sm:w-auto sm:shrink-0">
              <DataControlBar />
            </div>
          </div>
        </header>

        {/* Page Content: consistent padding, max-width, and min-height for proper page break */}
        <main className="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8 relative scrollbar-thin min-h-0">
          <div className="max-w-7xl mx-auto pb-12 w-full">
            <Outlet />
          </div>
        </main>
        <footer className="shrink-0 px-4 py-2 border-t border-border bg-surface/50 text-center text-[10px] text-muted-2" title="Build time (UTC). Use this to confirm which dashboard image is running.">
          Fortuna Dashboard · build {typeof __BUILD_TIME__ !== 'undefined' ? __BUILD_TIME__ : 'dev'}
        </footer>
      </div>
    </div>
  );
};
