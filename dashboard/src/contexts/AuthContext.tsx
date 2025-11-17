import React, { createContext, useContext, useState, useEffect, ReactNode } from 'react'
import api, { authApi, User } from '../services/api'

interface AuthContextType {
  user: User | null
  token: string | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
  isAuthenticated: boolean
  loading: boolean
}

const AuthContext = createContext<AuthContextType | undefined>(undefined)

export const useAuth = () => {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}

interface AuthProviderProps {
  children: ReactNode
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null)
  const [token, setToken] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    // Load token from localStorage on mount
    const savedToken = localStorage.getItem('ksam_token')
    console.log('[AuthContext] Initial load, token:', savedToken ? 'exists' : 'not found')
    if (savedToken) {
      setToken(savedToken)
      // Set token in axios default headers
      api.defaults.headers.common['Authorization'] = `Bearer ${savedToken}`
      // Try to get current user
      fetchCurrentUser()
    } else {
      console.log('[AuthContext] No token found, setting loading to false')
      setLoading(false)
    }
    // Add timeout to ensure loading state is cleared (safety net)
    const timeout = setTimeout(() => {
      console.warn('[AuthContext] Loading timeout, forcing loading to false')
      setLoading(false)
    }, 5000) // 5 second timeout
    
    return () => clearTimeout(timeout)
  }, [])

  const fetchCurrentUser = async () => {
    try {
      console.log('[AuthContext] Fetching current user...')
      const response = await authApi.getCurrentUser()
      console.log('[AuthContext] Current user response:', response.data)
      // Backend returns {user: {...}}, extract user object
      const userData = (response.data as any).user || response.data
      setUser(userData as User)
      setToken(localStorage.getItem('ksam_token')) // Ensure token is set
      console.log('[AuthContext] User set:', userData)
    } catch (error: any) {
      console.error('[AuthContext] Failed to fetch current user:', error)
      // Token might be invalid, clear it
      localStorage.removeItem('ksam_token')
      setToken(null)
      setUser(null)
      delete api.defaults.headers.common['Authorization']
    } finally {
      setLoading(false)
      console.log('[AuthContext] Loading set to false')
    }
  }

  const login = async (username: string, password: string) => {
    try {
      console.log('[AuthContext] Attempting login for:', username)
      const response = await authApi.login(username, password)
      console.log('[AuthContext] Login response:', response.data)
      // Backend returns {token, user, expiresAt}
      const { token: newToken, user: userData } = response.data
      
      if (!newToken) {
        throw new Error('No token received from server')
      }
      
      console.log('[AuthContext] Setting token and user')
      setToken(newToken)
      setUser(userData)
      
      // Save token to localStorage
      localStorage.setItem('ksam_token', newToken)
      
      // Set token in axios default headers
      api.defaults.headers.common['Authorization'] = `Bearer ${newToken}`
      console.log('[AuthContext] Login successful')
    } catch (error: any) {
      console.error('[AuthContext] Login failed:', error)
      const errorMessage = error.response?.data?.error || error.message || 'Login failed'
      throw new Error(errorMessage)
    }
  }

  const logout = () => {
    setToken(null)
    setUser(null)
    localStorage.removeItem('ksam_token')
    delete api.defaults.headers.common['Authorization']
  }

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        login,
        logout,
        isAuthenticated: !!token,
        loading,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

