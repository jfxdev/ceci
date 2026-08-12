import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { RoleBadge, type ProjectRole } from "@/components/shared/role-badge"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { api, ApiError } from "@/lib/api"

interface Member {
  userId: string
  email: string
  name: string
  role: ProjectRole
}

const ROLES: ProjectRole[] = ["viewer", "editor", "admin", "owner"]

export function MembersListPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [email, setEmail] = useState("")
  const [role, setRole] = useState<ProjectRole>("viewer")
  const [error, setError] = useState<string | null>(null)

  const { data: members = [], isLoading } = useQuery({
    queryKey: ["members", projectId],
    queryFn: () => api.get<Member[]>(`/projects/${projectId}/members`),
    enabled: !!projectId,
  })

  const addMember = useMutation({
    mutationFn: () => api.post(`/projects/${projectId}/members`, { email, role }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["members", projectId] })
      setOpen(false)
      setEmail("")
      setRole("viewer")
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to add member"),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    addMember.mutate()
  }

  const columns: DataTableColumn<Member>[] = [
    { key: "name", header: "Name", render: (m) => m.name },
    { key: "email", header: "Email", render: (m) => m.email },
    { key: "role", header: "Role", render: (m) => <RoleBadge role={m.role} /> },
  ]

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Members</h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>Add member</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Add member</DialogTitle>
            </DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="member-email">Email</Label>
                <Input id="member-email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
                <p className="text-xs text-muted-foreground">The user must already have a ceci account.</p>
              </div>
              <div className="flex flex-col gap-2">
                <Label>Role</Label>
                <Select value={role} onValueChange={(v) => setRole(v as ProjectRole)}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ROLES.map((r) => (
                      <SelectItem key={r} value={r}>
                        {r}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <DialogFooter>
                <Button type="submit" disabled={addMember.isPending}>
                  Add
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      <DataTable columns={columns} rows={members} rowKey={(m) => m.userId} emptyMessage={isLoading ? "Loading..." : "No members yet"} />
    </div>
  )
}
