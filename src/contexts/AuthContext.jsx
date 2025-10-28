import { createContext, useContext, useMemo, useState } from 'react'

const AuthContext = createContext({ user: null, login: async () => {}, logout: () => {} })

export function AuthProvider({ children }) {
  const [user, setUser] = useState(null)

  const login = async (payload) => {
    // Stub for now; integrate API later
    setUser({ id: '1', name: payload?.identifier || 'User' })
  }

  const logout = () => setUser(null)

  const value = useMemo(() => ({ user, login, logout }), [user])
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  return useContext(AuthContext)
}
