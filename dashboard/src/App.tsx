import { BrowserRouter, Routes, Route, Link, useLocation, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './contexts/AuthContext'
import { ThemeProvider, useTheme } from './contexts/ThemeContext'
import LandingPage from './pages/LandingPage'
import Dashboard from './pages/Dashboard'
import GraphView from './pages/GraphView'
import ServiceAccounts from './pages/ServiceAccounts'
import AuditLogs from './pages/AuditLogs'
import Login from './pages/Login'
import ProtectedRoute from './components/ProtectedRoute'
import './App.css'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 5 * 60 * 1000,
    },
  },
})

function Navigation() {
  const location = useLocation()
  const { user, logout, isAuthenticated } = useAuth()
  const { theme, toggleTheme } = useTheme()

  const isActive = (path: string) => {
    return location.pathname === path
  }

  if (!isAuthenticated) {
    return null
  }

  return (
    <nav className="bg-white dark:bg-gray-900 shadow-lg border-b border-gray-200 dark:border-gray-700">
      <div className="w-full px-6">
        {/* 3-Column Layout: Left | Center | Right */}
        <div className="grid grid-cols-3 items-center h-16">
          {/* LEFT: Logo + Project Name */}
          <div className="flex items-center justify-start">
            <Link to="/dashboard" className="flex items-center space-x-3 group">
              <div className="w-10 h-10 bg-gradient-to-br from-pink-600 to-pink-800 rounded-lg flex items-center justify-center transition-transform group-hover:scale-105">
                <svg className="w-6 h-6 text-white" viewBox="0 0 200 200" fill="none" xmlns="http://www.w3.org/2000/svg">
                  <g stroke="currentColor" strokeWidth="14" strokeLinecap="round">
                    <line x1="100" y1="20" x2="100" y2="180" />
                    <line x1="20" y1="100" x2="180" y2="100" />
                    <line x1="43" y1="43" x2="157" y2="157" />
                    <line x1="157" y1="43" x2="43" y2="157" />
                  </g>
                  <circle cx="100" cy="100" r="52" stroke="currentColor" strokeWidth="12" fill="none" />
                  <circle cx="100" cy="100" r="24" fill="currentColor" />
                </svg>
              </div>
              <div>
                <div className="text-base font-bold text-gray-900 dark:text-white leading-tight">K8s<span className="text-pink-600 dark:text-pink-500">Fortuna</span></div>
                <div className="text-xs text-gray-500 dark:text-gray-400 leading-tight">Management Platform</div>
              </div>
            </Link>
          </div>

          {/* CENTER: Navigation Links */}
          <div className="flex items-center justify-center space-x-1">
            <Link
              to="/dashboard"
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                isActive('/dashboard')
                  ? 'bg-pink-600 text-white shadow-sm'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              Dashboard
            </Link>
            <Link
              to="/graph"
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                isActive('/graph')
                  ? 'bg-pink-600 text-white shadow-sm'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              Graph View
            </Link>
            <Link
              to="/serviceaccounts"
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                isActive('/serviceaccounts')
                  ? 'bg-pink-600 text-white shadow-sm'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              ServiceAccounts
            </Link>
            <Link
              to="/audit"
              className={`px-4 py-2 rounded-lg text-sm font-medium transition-all ${
                isActive('/audit')
                  ? 'bg-pink-600 text-white shadow-sm'
                  : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800'
              }`}
            >
              Audit Logs
            </Link>
          </div>

          {/* RIGHT: Theme Toggle + User Account */}
          <div className="flex items-center justify-end space-x-3">
            {/* Theme Toggle */}
            <button
              onClick={toggleTheme}
              className="p-2 rounded-lg text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 transition-colors"
              title={theme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'}
            >
              {theme === 'light' ? (
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                </svg>
              ) : (
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                </svg>
              )}
            </button>

            {/* User Account */}
            <div className="flex items-center space-x-2 pl-3 border-l border-gray-200 dark:border-gray-700">
              <div className="w-8 h-8 bg-blue-600 rounded-full flex items-center justify-center">
                <span className="text-sm font-semibold text-white">
                  {user?.username?.charAt(0).toUpperCase()}
                </span>
              </div>
              <div className="text-left">
                <div className="text-sm font-medium text-gray-900 dark:text-white leading-tight">{user?.username}</div>
                <div className="text-xs text-gray-500 dark:text-gray-400 capitalize leading-tight">{user?.role}</div>
              </div>
              <button
                onClick={logout}
                className="ml-2 px-3 py-1.5 text-xs font-medium text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-800 rounded-lg transition-colors"
              >
                Logout
              </button>
            </div>
          </div>
        </div>
      </div>
    </nav>
  )
}

function AppRoutes() {
  const { loading, isAuthenticated } = useAuth()
  const location = useLocation()

  const isLoginPage = location.pathname === '/login'

  if (loading && !isLoginPage) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-12 w-12 border-4 border-blue-600 border-t-transparent"></div>
          <p className="mt-4 text-gray-600 dark:text-gray-400">Loading...</p>
        </div>
      </div>
    )
  }

  return (
    <>
      {isAuthenticated && <Navigation />}
      <div className={isAuthenticated ? "flex-1 overflow-hidden" : ""}>
        <Routes>
          {/* Public routes */}
          <Route path="/login" element={<Login />} />
          
          {/* Landing page for unauthenticated users */}
          <Route 
            path="/" 
            element={
              isAuthenticated ? (
                <Navigate to="/dashboard" replace />
              ) : (
                <LandingPage />
              )
            } 
          />

          {/* Protected routes */}
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <Dashboard />
              </ProtectedRoute>
            }
          />
          <Route
            path="/graph"
            element={
              <ProtectedRoute>
                <GraphView />
              </ProtectedRoute>
            }
          />
          <Route
            path="/serviceaccounts"
            element={
              <ProtectedRoute>
                <ServiceAccounts />
              </ProtectedRoute>
            }
          />
          <Route
            path="/audit"
            element={
              <ProtectedRoute>
                <AuditLogs />
              </ProtectedRoute>
            }
          />
          
          {/* Catch all */}
          <Route path="*" element={<Navigate to={isAuthenticated ? "/dashboard" : "/"} replace />} />
        </Routes>
      </div>
    </>
  )
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AuthProvider>
          <BrowserRouter>
            <AppContent />
          </BrowserRouter>
        </AuthProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}

function AppContent() {
  const { isAuthenticated } = useAuth()
  
  return (
    <div className={isAuthenticated ? "flex flex-col h-screen overflow-hidden" : ""}>
      <AppRoutes />
    </div>
  )
}

export default App
