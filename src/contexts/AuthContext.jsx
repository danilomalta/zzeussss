import { createContext, useContext, useMemo, useState, useEffect, useCallback } from 'react'
import { authService } from '../services/authService.js'

const AuthContext = createContext({
  user: null,
  loading: true,
  isAuthenticated: false,
  login: async () => {},
  logout: () => {},
  register: async () => {},
})

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)
  const [loading, setLoading] = useState(true)

  // Initialize auth state from localStorage
  useEffect(() => {
    const initAuth = () => {
      try {
        const storedUser = authService.getCurrentUser()
        const token = authService.getToken()
        if (storedUser && token) {
          setUser(storedUser)
        }
      } catch (error) {
        console.error('Error initializing auth:', error)
        authService.logout()
      } finally {
        setLoading(false)
      }
    }

    initAuth()
  }, [])

  const login = useCallback(async (credentials) => {
    try {
      setLoading(true)
      const response = await authService.login(credentials)
      
      // Store token and user
      localStorage.setItem('auth_token', response.token)
      localStorage.setItem('user', JSON.stringify(response.user))
      
      setUser(response.user)
      return { success: true, user: response.user }
    } catch (error) {
      return {
        success: false,
        error: error.message || 'Login failed. Please check your credentials.',
      }
    } finally {
      setLoading(false)
    }
  }, [])

  const register = useCallback(async (data) => {
    try {
      setLoading(true)
      const response = await authService.register(data)
      return { success: true, message: response.message }
    } catch (error) {
      return {
        success: false,
        error: error.message || 'Registration failed. Please try again.',
      }
    } finally {
      setLoading(false)
    }
  }, [])

  const logout = useCallback(() => {
    authService.logout()
    setUser(null)
    // Navigation will be handled by the component using logout
    window.location.href = '/'
  }, [])

  const value = useMemo(
    () => ({
      user,
      loading,
      isAuthenticated: !!user && !!authService.getToken(),
      login,
      logout,
      register,
    }),
    [user, loading, login, logout, register]
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const context = useContext(AuthContext)
  if (!context) {
    throw new Error('useAuth must be used within AuthProvider')
  }
  return context
}
