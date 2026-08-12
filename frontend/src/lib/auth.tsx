import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react"
import { api, setAccessToken } from "@/lib/api"

export interface CurrentUser {
  id: string
  email: string
  name: string
}

interface AuthContextValue {
  user: CurrentUser | null
  isLoading: boolean
  login: (email: string, password: string) => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [isLoading, setIsLoading] = useState(true)

  // On first load there's no access token in memory yet (it's never
  // persisted client-side), so try to restore the session from the
  // httpOnly refresh cookie before deciding whether to show the login page.
  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const refreshed = await api.post<{ accessToken: string }>("/auth/refresh")
        setAccessToken(refreshed.accessToken)
        const me = await api.get<CurrentUser>("/me")
        if (!cancelled) setUser(me)
      } catch {
        if (!cancelled) setUser(null)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  const login = useCallback(async (email: string, password: string) => {
    const data = await api.post<{ accessToken: string; user: CurrentUser }>("/auth/login", { email, password })
    setAccessToken(data.accessToken)
    setUser(data.user)
  }, [])

  const logout = useCallback(async () => {
    await api.post("/auth/logout")
    setAccessToken(null)
    setUser(null)
  }, [])

  return <AuthContext.Provider value={{ user, isLoading, login, logout }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
