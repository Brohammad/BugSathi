import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { api, configureAuthStore } from '../api/client'
import type { Tokens, User } from '../api/types'

type AuthState = {
  user: User | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

function readTokens(): Tokens | null {
  const access = localStorage.getItem('bs_access')
  const refresh = localStorage.getItem('bs_refresh')
  if (!access || !refresh) return null
  return { access_token: access, refresh_token: refresh }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    configureAuthStore({
      getAccess: () => localStorage.getItem('bs_access'),
      getRefresh: () => localStorage.getItem('bs_refresh'),
      setTokens: (t) => {
        localStorage.setItem('bs_access', t.access_token)
        localStorage.setItem('bs_refresh', t.refresh_token)
      },
      clear: () => {
        localStorage.removeItem('bs_access')
        localStorage.removeItem('bs_refresh')
      },
    })

    const boot = async () => {
      if (!readTokens()) {
        setLoading(false)
        return
      }
      try {
        const { user: me } = await api.me()
        setUser(me)
      } catch {
        localStorage.removeItem('bs_access')
        localStorage.removeItem('bs_refresh')
      } finally {
        setLoading(false)
      }
    }
    void boot()
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const res = await api.login(email, password)
    localStorage.setItem('bs_access', res.tokens.access_token)
    localStorage.setItem('bs_refresh', res.tokens.refresh_token)
    setUser(res.user)
  }, [])

  const register = useCallback(async (email: string, password: string, name: string) => {
    const res = await api.register(email, password, name)
    localStorage.setItem('bs_access', res.tokens.access_token)
    localStorage.setItem('bs_refresh', res.tokens.refresh_token)
    setUser(res.user)
  }, [])

  const logout = useCallback(async () => {
    const refresh = localStorage.getItem('bs_refresh')
    if (refresh) await api.logout(refresh)
    localStorage.removeItem('bs_access')
    localStorage.removeItem('bs_refresh')
    setUser(null)
  }, [])

  const value = useMemo(
    () => ({ user, loading, login, register, logout }),
    [user, loading, login, register, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth outside AuthProvider')
  return ctx
}
