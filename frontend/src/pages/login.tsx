import { useEffect, useState, type FormEvent } from "react"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useAuth } from "@/lib/auth"
import { ApiError } from "@/lib/api"

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const [searchParams] = useSearchParams()
  const [email, setEmail] = useState("")
  const [password, setPassword] = useState("")
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [oidcEnabled, setOIDCEnabled] = useState(false)

  useEffect(() => {
    fetch("/api/v1/auth/oidc/config", { credentials: "include" })
      .then((response) => response.ok ? response.json() : { enabled: false })
      .then((config: { enabled?: boolean }) => setOIDCEnabled(config.enabled === true))
      .catch(() => setOIDCEnabled(false))
  }, [])

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    setLoading(true)
    try {
      await login(email, password)
      navigate("/projects")
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Login failed")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex min-h-svh items-center justify-center bg-background p-4">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>Sign in to LeaFlag</CardTitle>
        </CardHeader>
        <CardContent>
          <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
            <div className="flex flex-col gap-2">
              <Label htmlFor="email">Email</Label>
              <Input id="email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password">Password</Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </div>
            {error && <p className="text-sm text-destructive">{error}</p>}
            {searchParams.get("sso_error") === "1" && <p className="text-sm text-destructive">Unable to sign in with your identity provider.</p>}
            <Button type="submit" disabled={loading}>
              {loading ? "Signing in..." : "Sign in"}
            </Button>
            {oidcEnabled && <>
              <div className="relative py-1 text-center text-xs text-muted-foreground before:absolute before:left-0 before:right-0 before:top-1/2 before:border-t"><span className="relative bg-card px-2">or</span></div>
              <Button type="button" variant="outline" onClick={() => { window.location.assign("/api/v1/auth/oidc/login") }}>Sign in with SSO</Button>
            </>}
            <p className="text-center text-sm text-muted-foreground">
              Don't have an account?{" "}
              <Link to="/register" className="underline">
                Create one
              </Link>
            </p>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}
