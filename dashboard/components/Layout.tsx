
import React, { useState, useEffect } from 'react';
import { NavLink, Outlet, useNavigate, useLocation } from 'react-router-dom';
import { useAuthStore } from '../store/authStore';
import { api } from '../lib/api';
import { RefreshIntervalSelector } from './RefreshIntervalSelector';
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
  UserCircle,
  ScrollText,
  FileText,
  History,
  Lock,
  Bell,
  Search,
  PackageSearch,
  Globe
} from 'lucide-react';

export const Layout: React.FC = () => {
  const { user, logout } = useAuthStore();
  const navigate = useNavigate();
  const location = useLocation();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [unreadCount, setUnreadCount] = useState(0);

  useEffect(() => {
    api.getNotifications().then(notes => {
      setUnreadCount(notes.filter(n => !n.read).length);
    });
  }, []);

  const handleLogout = () => {
    logout();
    navigate('/login');
  };

  const navItems = [
    { icon: <LayoutDashboard size={18} />, label: 'Dashboard', path: '/' },
    { icon: <ShieldAlert size={18} />, label: 'Risk Center', path: '/risks' },
    { icon: <Shield size={18} />, label: 'Capabilities', path: '/capabilities' },
    { icon: <PackageSearch size={18} />, label: 'SBOM Analysis', path: '/sbom' },
    { icon: <Network size={18} />, label: 'Attack Paths', path: '/attack-paths' },
    { icon: <Globe size={18} />, label: 'Clusters', path: '/clusters' },
    { icon: <Layers size={18} />, label: 'Resources', path: '/resources' },
    { icon: <ScrollText size={18} />, label: 'Rules', path: '/rules' },
    { icon: <Lock size={18} />, label: 'Certificates', path: '/certificates' },
    { icon: <FileText size={18} />, label: 'Reports', path: '/reports' },
    { icon: <Activity size={18} />, label: 'Monitoring', path: '/monitoring' },
    { icon: <History size={18} />, label: 'Audit Logs', path: '/audit' },
    { icon: <Settings size={18} />, label: 'Settings', path: '/settings' },
  ];

  const currentTitle = navItems.find(i => i.path === location.pathname)?.label || 'Dashboard';

  return (
    <div className="min-h-screen bg-slate-950 flex">
      {/* Mobile Sidebar Overlay */}
      {isMobileMenuOpen && (
        <div 
          className="fixed inset-0 bg-slate-900/80 backdrop-blur-sm z-40 lg:hidden"
          onClick={() => setIsMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside className={`
        fixed lg:static inset-y-0 left-0 z-50
        w-64 bg-slate-900 text-slate-300 border-r border-slate-800 transform transition-transform duration-300 ease-in-out
        ${isMobileMenuOpen ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'}
        flex flex-col
      `}>
        {/* Logo */}
        <div className="h-16 flex items-center px-6 border-b border-slate-800 shrink-0">
          <div className="flex items-center space-x-3">
            <div className="w-8 h-8 bg-pink-600 rounded-lg flex items-center justify-center shadow-lg shadow-pink-600/20">
              <ShieldAlert className="text-white w-5 h-5" />
            </div>
            <span className="text-xl font-bold tracking-tight text-white">Fortuna</span>
          </div>
          <button 
            className="ml-auto lg:hidden text-slate-400 hover:text-white transition-colors"
            onClick={() => setIsMobileMenuOpen(false)}
          >
            <X size={20} />
          </button>
        </div>

        {/* Nav Links */}
        <nav className="flex-1 px-4 py-6 space-y-1 overflow-y-auto scrollbar-hide">
          <p className="px-3 text-[10px] font-bold text-slate-500 uppercase tracking-widest mb-2">Main Menu</p>
          {navItems.map((item) => (
            <NavLink
              key={item.path}
              to={item.path}
              onClick={() => setIsMobileMenuOpen(false)}
              className={({ isActive }) => `
                flex items-center px-3 py-2 rounded-lg text-sm font-medium transition-all duration-200
                ${isActive 
                  ? 'bg-pink-600/10 text-pink-500 border border-pink-500/20 shadow-sm' 
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800 border border-transparent'}
              `}
            >
              <span className={`mr-3 ${location.pathname === item.path ? 'text-pink-500' : 'text-slate-500 group-hover:text-slate-300'}`}>
                {item.icon}
              </span>
              {item.label}
            </NavLink>
          ))}
        </nav>

        {/* User Profile */}
        <div className="p-4 border-t border-slate-800 shrink-0 bg-slate-900/50">
          <div className="flex items-center mb-4 px-2">
            <div className="relative">
              <UserCircle className="w-9 h-9 text-slate-400" />
              <div className="absolute bottom-0 right-0 w-2.5 h-2.5 bg-emerald-500 border-2 border-slate-900 rounded-full"></div>
            </div>
            <div className="ml-3 min-w-0">
              <p className="text-sm font-semibold text-white truncate">{user?.name}</p>
              <p className="text-xs text-slate-500 truncate capitalize">{user?.role}</p>
            </div>
          </div>
          <button
            onClick={handleLogout}
            className="w-full flex items-center px-3 py-2 rounded-lg text-sm font-medium text-slate-400 hover:text-red-400 hover:bg-red-400/10 transition-all duration-200"
          >
            <LogOut size={16} className="mr-3" />
            Sign Out
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden relative">
        {/* Desktop Header */}
        <header className="hidden lg:flex h-16 bg-slate-950/50 backdrop-blur-md border-b border-slate-800 items-center px-8 justify-between z-10">
          <div className="flex items-center">
            <h2 className="text-lg font-semibold text-white mr-8">{currentTitle}</h2>
            <div className="relative group">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-500 w-4 h-4 group-focus-within:text-pink-500 transition-colors" />
              <input 
                type="text" 
                placeholder="Global search..." 
                className="bg-slate-900/50 border border-slate-800 rounded-full pl-9 pr-4 py-1.5 text-sm text-slate-300 focus:outline-none focus:ring-1 focus:ring-pink-500 focus:border-pink-500 w-64 transition-all"
              />
            </div>
          </div>
          <div className="flex items-center space-x-4">
            <RefreshIntervalSelector />
            <button 
              onClick={() => navigate('/notifications')}
              className="relative p-2 text-slate-400 hover:text-white hover:bg-slate-800 rounded-full transition-all"
            >
              <Bell size={20} />
              {unreadCount > 0 && (
                <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-pink-600 text-white text-[10px] font-bold flex items-center justify-center rounded-full border-2 border-slate-950">
                  {unreadCount}
                </span>
              )}
            </button>
            <div className="h-6 w-px bg-slate-800"></div>
            <div className="flex items-center space-x-2 px-2 py-1 bg-slate-900 border border-slate-800 rounded-lg cursor-pointer hover:border-slate-700 transition-colors">
              <div className="w-6 h-6 rounded-full bg-pink-600/20 text-pink-500 flex items-center justify-center text-[10px] font-bold">
                {user?.name?.charAt(0) || 'A'}
              </div>
              <span className="text-xs font-medium text-slate-300">Production</span>
            </div>
          </div>
        </header>

        {/* Mobile Header */}
        <header className="lg:hidden h-16 bg-slate-900 border-b border-slate-800 flex items-center px-4 justify-between shrink-0 z-10">
          <button
            onClick={() => setIsMobileMenuOpen(true)}
            className="p-2 text-slate-400 hover:text-slate-200"
          >
            <Menu size={24} />
          </button>
          <div className="flex items-center space-x-2">
            <div className="w-6 h-6 bg-pink-600 rounded flex items-center justify-center">
               <ShieldAlert className="text-white w-4 h-4" />
            </div>
            <span className="font-bold text-white tracking-tight">Fortuna</span>
          </div>
          <div className="flex items-center gap-2">
            <RefreshIntervalSelector />
            <button 
              onClick={() => navigate('/notifications')}
              className="relative p-2 text-slate-400"
            >
            <Bell size={22} />
            {unreadCount > 0 && (
              <span className="absolute top-1.5 right-1.5 w-4 h-4 bg-pink-600 text-white text-[10px] font-bold flex items-center justify-center rounded-full">
                {unreadCount}
              </span>
            )}
          </button>
          </div>
        </header>

        {/* Page Content: consistent padding, max-width, and min-height for proper page break */}
        <main className="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8 relative scrollbar-thin min-h-0">
          <div className="max-w-7xl mx-auto pb-12 w-full">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  );
};
