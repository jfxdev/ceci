import { Navigate, Outlet } from "react-router-dom"
import { useAuth } from "@/lib/auth"

/** Redirects to /login when there's no authenticated session. */
export function ProtectedRoute() {
  const { user, isLoading } = useAuth()

  if (isLoading) {
    return <div className="flex min-h-svh items-center justify-center text-muted-foreground">Loading...</div>
  }
  if (!user) {
    return <Navigate to="/login" replace />
  }
  return <Outlet />
}
