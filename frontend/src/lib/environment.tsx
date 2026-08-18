import { createContext, useContext, useEffect, useMemo, useState, type ReactNode } from "react"
import { useQuery } from "@tanstack/react-query"
import { useParams } from "react-router-dom"
import { api } from "@/lib/api"

export interface Environment {
  id: string
  key: string
  name: string
  isDefault: boolean
}

interface EnvironmentContextValue {
  environments: Environment[]
  isLoading: boolean
  envKey: string
  setEnvKey: (key: string) => void
  /** Builds a project-and-environment-scoped API path, e.g. envPath("flags") -> "/projects/x/environments/production/flags". */
  envPath: (suffix: string) => string
}

const EnvironmentContext = createContext<EnvironmentContextValue | null>(null)

function storageKey(projectId: string) {
  return `leaflag:selectedEnv:${projectId}`
}

/**
 * Every project page needs an environment in scope (flags/parameters/API
 * keys are all per-environment). This provider fetches the project's
 * environments, remembers the last-selected one per project in
 * localStorage, and exposes envPath() so pages don't each re-derive the
 * "/projects/:id/environments/:envKey/..." prefix.
 */
export function EnvironmentProvider({ children }: { children: ReactNode }) {
  const { projectId } = useParams<{ projectId: string }>()
  const [envKey, setEnvKeyState] = useState<string>("")

  const { data: environments = [], isLoading } = useQuery({
    queryKey: ["environments", projectId],
    queryFn: () => api.get<Environment[]>(`/projects/${projectId}/environments`),
    enabled: !!projectId,
  })

  useEffect(() => {
    if (!projectId || environments.length === 0) return
    const stored = localStorage.getItem(storageKey(projectId))
    const valid = environments.some((e) => e.key === stored)
    setEnvKeyState(valid ? stored! : environments[0].key)
  }, [projectId, environments])

  function setEnvKey(key: string) {
    if (!projectId) return
    localStorage.setItem(storageKey(projectId), key)
    setEnvKeyState(key)
  }

  const envPath = useMemo(() => {
    return (suffix: string) => `/projects/${projectId}/environments/${envKey}${suffix.startsWith("/") ? "" : "/"}${suffix}`
  }, [projectId, envKey])

  return (
    <EnvironmentContext.Provider value={{ environments, isLoading, envKey, setEnvKey, envPath }}>
      {children}
    </EnvironmentContext.Provider>
  )
}

export function useEnvironment() {
  const ctx = useContext(EnvironmentContext)
  if (!ctx) throw new Error("useEnvironment must be used within EnvironmentProvider")
  return ctx
}
