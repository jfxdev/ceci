import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react"
import { api, setAccessToken } from "@/lib/api"
import i18n, { browserLocale, type AppLocale } from "@/lib/i18n"

export interface CurrentUser {
  id: string
  email: string
  name: string
  isAdmin: boolean
  locale: AppLocale
}

interface AuthContextValue {
  user: CurrentUser | null
  isLoading: boolean
  login: (email: string, password: string) => Promise<void>
  register: (email: string, password: string, name: string) => Promise<void>
  logout: () => Promise<void>
  setLocale: (locale: AppLocale) => Promise<void>
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
        if (!cancelled) {
          setUser(me)
          void i18n.changeLanguage(me.locale)
        }
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
    await i18n.changeLanguage(data.user.locale)
  }, [])

  const register = useCallback(async (email: string, password: string, name: string) => {
    const data = await api.post<{ accessToken: string; user: CurrentUser }>("/auth/register", {
      email,
      password,
      name,
    })
    setAccessToken(data.accessToken)
    setUser(data.user)
    const locale = browserLocale()
    if (data.user.locale !== locale) {
      void api.put<CurrentUser>("/me/preferences", { locale }).then(setUser).catch(() => undefined)
    }
    await i18n.changeLanguage(locale)
  }, [])

  const logout = useCallback(async () => {
    await api.post("/auth/logout")
    setAccessToken(null)
    setUser(null)
    await i18n.changeLanguage(browserLocale())
  }, [])

  const setLocale = useCallback(async (locale: AppLocale) => {
    const user = await api.put<CurrentUser>("/me/preferences", { locale })
    setUser(user)
    await i18n.changeLanguage(locale)
  }, [])

  return <AuthContext.Provider value={{ user, isLoading, login, register, logout, setLocale }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
