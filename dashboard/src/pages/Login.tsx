import { useState, useEffect } from 'react'
import { useNavigate, useLocation, Link } from 'react-router-dom'
import { useAuth } from '../contexts/AuthContext'
import HelmLogo from '../components/HelmLogo'
import { ArrowLeft, User, Lock, ChevronRight } from 'lucide-react'

export default function Login() {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const { login, isAuthenticated } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

  // Redirect if already authenticated
  useEffect(() => {
    if (isAuthenticated) {
      const from = (location.state as any)?.from?.pathname || '/dashboard'
      navigate(from, { replace: true })
    }
  }, [isAuthenticated, navigate, location])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      await login(username, password)
      // Navigate to the page user was trying to access, or dashboard
      const from = (location.state as any)?.from?.pathname || '/dashboard'
      navigate(from, { replace: true })
    } catch (err: any) {
      setError(err.message || 'Login failed. Please check your credentials.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="min-h-screen w-full bg-black flex items-center justify-center relative overflow-hidden">
      {/* Background Texture - Consistent with Landing */}
      <div className="absolute inset-0 z-0 opacity-10" 
           style={{ backgroundImage: 'radial-gradient(#333 1px, transparent 1px)', backgroundSize: '30px 30px' }}>
      </div>

      <div className="w-full max-w-md z-10 px-6">
        
        {/* Back Button */}
        <Link 
          to="/"
          className="mb-8 flex items-center gap-2 text-gray-500 hover:text-pink-500 transition-colors text-sm font-mono uppercase tracking-wider"
        >
          <ArrowLeft className="w-4 h-4" /> Back to Platform
        </Link>

        <div className="bg-gray-950 border border-gray-800 p-8 md:p-10 shadow-2xl relative overflow-hidden group">
          {/* Decorative Top Line */}
          <div className="absolute top-0 left-0 w-full h-1 bg-gradient-to-r from-pink-900 via-pink-600 to-pink-900"></div>

          <div className="flex flex-col items-center text-center mb-10">
            <div className="mb-6 transform hover:scale-105 transition-transform duration-500">
              <HelmLogo size={100} />
            </div>
            
            <h2 className="text-2xl font-bold text-white uppercase tracking-wider mb-2">
              K8s Fortuna
            </h2>
            
            <p className="text-pink-600 text-xs font-mono uppercase tracking-[0.15em] font-bold">
              Visibility First. Security Always.
            </p>
          </div>

          {error && (
            <div className="mb-6 p-4 bg-red-900/20 border border-red-900/50 rounded-sm">
              <p className="text-sm text-red-400 text-center">{error}</p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-6">
            <div className="space-y-2">
              <label className="text-xs text-gray-400 uppercase font-bold tracking-wider ml-1">Username</label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <User className="h-5 w-5 text-gray-600" />
                </div>
                <input
                  type="text"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  className="block w-full pl-10 pr-3 py-3 bg-black border border-gray-800 text-white placeholder-gray-600 focus:outline-none focus:border-pink-600 focus:ring-1 focus:ring-pink-600 transition-all text-sm"
                  placeholder="admin"
                  required
                  disabled={loading}
                />
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex justify-between items-center ml-1">
                <label className="text-xs text-gray-400 uppercase font-bold tracking-wider">Password</label>
              </div>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none">
                  <Lock className="h-5 w-5 text-gray-600" />
                </div>
                <input
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="block w-full pl-10 pr-3 py-3 bg-black border border-gray-800 text-white placeholder-gray-600 focus:outline-none focus:border-pink-600 focus:ring-1 focus:ring-pink-600 transition-all text-sm"
                  placeholder="••••••••"
                  required
                  disabled={loading}
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full group relative px-8 py-3 bg-pink-700 hover:bg-pink-600 text-white font-bold uppercase tracking-wider transition-all duration-200 ease-in-out overflow-hidden rounded-sm mt-4 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span className="relative z-10 flex items-center justify-center gap-2">
                {loading ? 'Authenticating...' : 'Authenticate'} 
                {!loading && <ChevronRight className="w-4 h-4 group-hover:translate-x-1 transition-transform" />}
              </span>
              {!loading && (
                <div className="absolute inset-0 -translate-x-full group-hover:translate-x-0 bg-pink-500 transition-transform duration-300 ease-out skew-x-12 origin-left"></div>
              )}
            </button>
          </form>

          <div className="mt-8 text-center">
            <p className="text-gray-600 text-xs">
              Default credentials: <span className="text-gray-400 font-mono">admin / admin123</span>
            </p>
          </div>
        </div>
        
        <div className="mt-8 text-center">
          <p className="text-gray-600 text-[10px] font-mono uppercase">
            Protected by K8s Fortuna Identity Guard v2.4
          </p>
        </div>
      </div>
    </div>
  )
}

