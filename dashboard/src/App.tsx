import { BrowserRouter, Routes, Route, Link, useLocation, Navigate } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './contexts/AuthContext'
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
      staleTime: 5 * 60 * 1000, // 5 minutes
    },
  },
})

function Navigation() {
  const location = useLocation()
  const { user, logout, isAuthenticated } = useAuth()

  const isActive = (path: string) => {
    return location.pathname === path
  }

  if (!isAuthenticated) {
    return null
  }

  return (
    <nav className="bg-white shadow-lg">
      <div className="container mx-auto px-4">
        <div className="flex justify-between items-center py-4">
          <Link to="/" className="text-2xl font-bold text-blue-600">
            KSAM
          </Link>
          <div className="flex items-center space-x-4">
            <Link
              to="/"
              className={`px-4 py-2 rounded-md transition-colors ${
                isActive('/')
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              Dashboard
            </Link>
            <Link
              to="/graph"
              className={`px-4 py-2 rounded-md transition-colors ${
                isActive('/graph')
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              Graph View
            </Link>
            <Link
              to="/serviceaccounts"
              className={`px-4 py-2 rounded-md transition-colors ${
                isActive('/serviceaccounts')
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              ServiceAccounts
            </Link>
            <Link
              to="/audit"
              className={`px-4 py-2 rounded-md transition-colors ${
                isActive('/audit')
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              Audit Logs
            </Link>
            <div className="flex items-center space-x-2">
              <span className="text-sm text-gray-600">{user?.username}</span>
              <button
                onClick={logout}
                className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-md"
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

  // Always allow access to login page, even during loading
  const isLoginPage = location.pathname === '/login'

  // Show loading while checking authentication (but not on login page)
  if (loading && !isLoginPage) {
    return (
      <div className="min-h-screen flex items-center justify-center">
        <div className="text-center">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          <p className="mt-4 text-gray-600">Loading...</p>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-gray-100">
      {isAuthenticated && <Navigation />}
      <Routes>
        <Route path="/login" element={<Login />} />
        <Route
          path="/"
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
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </div>
  )
}

function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <BrowserRouter>
          <AppRoutes />
        </BrowserRouter>
      </AuthProvider>
    </QueryClientProvider>
  )
}

export default App

