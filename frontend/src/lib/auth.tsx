import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react"
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
  const authSessionGeneration = useRef(0)

  const applyUserForSession = useCallback(async (nextUser: CurrentUser, generation: number) => {
    if (generation !== authSessionGeneration.current) return
    setUser(nextUser)
    if (generation !== authSessionGeneration.current) return
    await i18n.changeLanguage(nextUser.locale)
  }, [])

  // On first load there's no access token in memory yet (it's never
  // persisted client-side), so try to restore the session from the
  // httpOnly refresh cookie before deciding whether to show the login page.
  useEffect(() => {
    let cancelled = false
		const generation = authSessionGeneration.current
    ;(async () => {
      try {
        const refreshed = await api.post<{ accessToken: string }>("/auth/refresh")
			if (cancelled || generation !== authSessionGeneration.current) return
        setAccessToken(refreshed.accessToken)
        const me = await api.get<CurrentUser>("/me")
			if (!cancelled) await applyUserForSession(me, generation)
      } catch {
        if (!cancelled) setUser(null)
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
	}, [applyUserForSession])

  const login = useCallback(async (email: string, password: string) => {
    const data = await api.post<{ accessToken: string; user: CurrentUser }>("/auth/login", { email, password })
		const generation = ++authSessionGeneration.current
    setAccessToken(data.accessToken)
		await applyUserForSession(data.user, generation)
  }, [applyUserForSession])

  const register = useCallback(async (email: string, password: string, name: string) => {
    const data = await api.post<{ accessToken: string; user: CurrentUser }>("/auth/register", {
      email,
      password,
      name,
    })
		const generation = ++authSessionGeneration.current
    setAccessToken(data.accessToken)
		await applyUserForSession(data.user, generation)
    const locale = browserLocale()
    if (data.user.locale !== locale) {
		void api.put<CurrentUser>("/me/preferences", { locale }).then((user) => applyUserForSession(user, generation)).catch(() => undefined)
    }
		if (generation === authSessionGeneration.current) await i18n.changeLanguage(locale)
  }, [applyUserForSession])

  const logout = useCallback(async () => {
    await api.post("/auth/logout")
		++authSessionGeneration.current
    setAccessToken(null)
    setUser(null)
    await i18n.changeLanguage(browserLocale())
  }, [])

  const setLocale = useCallback(async (locale: AppLocale) => {
		const generation = authSessionGeneration.current
    const user = await api.put<CurrentUser>("/me/preferences", { locale })
		await applyUserForSession(user, generation)
  }, [applyUserForSession])

  return <AuthContext.Provider value={{ user, isLoading, login, register, logout, setLocale }}>{children}</AuthContext.Provider>
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error("useAuth must be used within AuthProvider")
  return ctx
}
