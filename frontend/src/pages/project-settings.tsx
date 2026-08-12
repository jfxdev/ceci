import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api } from "@/lib/api"

interface APIKey {
  id: string
  label: string
  prefix: string
  revoked: boolean
}

export function ProjectSettingsPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [pendingRevoke, setPendingRevoke] = useState<APIKey | null>(null)

  const { data: keys = [], isLoading } = useQuery({
    queryKey: ["api-keys", projectId],
    queryFn: () => api.get<APIKey[]>(`/projects/${projectId}/api-keys`),
    enabled: !!projectId,
  })

  const createKey = useMutation({
    mutationFn: () => api.post<{ rawKey: string }>(`/projects/${projectId}/api-keys`, { label }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["api-keys", projectId] })
      setCreatedKey(data.rawKey)
      setLabel("")
    },
  })

  const revokeKey = useMutation({
    mutationFn: (key: APIKey) => api.delete(`/projects/${projectId}/api-keys/${key.id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["api-keys", projectId] })
      setPendingRevoke(null)
    },
  })

  const columns: DataTableColumn<APIKey>[] = [
    { key: "label", header: "Label", render: (k) => k.label },
    { key: "prefix", header: "Key", render: (k) => <code className="text-sm">{k.prefix}...</code> },
    { key: "status", header: "Status", render: (k) => (k.revoked ? "Revoked" : "Active") },
    {
      key: "actions",
      header: "",
      render: (k) =>
        !k.revoked && (
          <Button variant="ghost" size="sm" onClick={(e) => { e.stopPropagation(); setPendingRevoke(k) }}>
            Revoke
          </Button>
        ),
    },
  ]

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    createKey.mutate()
  }

  return (
    <div className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Settings</h1>
      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>OFREP API keys</CardTitle>
          <Dialog
            open={open}
            onOpenChange={(v) => {
              setOpen(v)
              if (!v) setCreatedKey(null)
            }}
          >
            <DialogTrigger asChild>
              <Button size="sm">New key</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Create API key</DialogTitle>
              </DialogHeader>
              {createdKey ? (
                <div className="flex flex-col gap-3">
                  <p className="text-sm text-muted-foreground">
                    Copy this key now — it won't be shown again.
                  </p>
                  <code className="break-all rounded bg-muted p-2 text-sm">{createdKey}</code>
                  <DialogFooter>
                    <Button onClick={() => setOpen(false)}>Done</Button>
                  </DialogFooter>
                </div>
              ) : (
                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="key-label">Label</Label>
                    <Input id="key-label" placeholder="CI" value={label} onChange={(e) => setLabel(e.target.value)} required />
                  </div>
                  <DialogFooter>
                    <Button type="submit" disabled={createKey.isPending}>
                      Create
                    </Button>
                  </DialogFooter>
                </form>
              )}
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent>
          <DataTable columns={columns} rows={keys} rowKey={(k) => k.id} emptyMessage={isLoading ? "Loading..." : "No API keys yet"} />
        </CardContent>
      </Card>
      <ConfirmDialog
        open={!!pendingRevoke}
        onOpenChange={(v) => !v && setPendingRevoke(null)}
        title={`Revoke "${pendingRevoke?.label}"?`}
        description="OFREP clients using this key will stop being able to evaluate flags."
        confirmLabel="Revoke"
        loading={revokeKey.isPending}
        onConfirm={() => pendingRevoke && revokeKey.mutate(pendingRevoke)}
      />
    </div>
  )
}
