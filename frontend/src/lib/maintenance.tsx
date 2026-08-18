import { createContext, useCallback, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"

export interface MaintenanceStatus {
  enabled: boolean
  message: string
  startedAt?: string
  startedBy?: string
}

interface MaintenanceContextValue {
  status: MaintenanceStatus
  setStatus: (status: MaintenanceStatus) => void
  refresh: () => Promise<void>
}

const inactive: MaintenanceStatus = { enabled: false, message: "" }
const MaintenanceContext = createContext<MaintenanceContextValue | null>(null)

export function MaintenanceProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  const [status, setStatus] = useState<MaintenanceStatus>(inactive)

  const refresh = useCallback(async () => {
    if (!user) {
      setStatus(inactive)
      return
    }
    try {
      setStatus(await api.get<MaintenanceStatus>("/maintenance-status"))
    } catch {
      // A transient failure must not hide an already displayed maintenance
      // notice; the next scheduled refresh will try again.
    }
  }, [user])

  useEffect(() => {
    void refresh()
    const interval = window.setInterval(() => void refresh(), 30_000)
    return () => window.clearInterval(interval)
  }, [refresh])

  const value = useMemo(() => ({ status, setStatus, refresh }), [status, refresh])
  return <MaintenanceContext.Provider value={value}>{children}</MaintenanceContext.Provider>
}

export function useMaintenance() {
  const context = useContext(MaintenanceContext)
  if (!context) throw new Error("useMaintenance must be used within MaintenanceProvider")
  return context
}
