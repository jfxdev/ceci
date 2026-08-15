import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { KeyRound } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"

interface APIKey {
  id: string
  label: string
  prefix: string
  revoked: boolean
}

export function ProjectSettingsPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const { envKey, envPath } = useEnvironment()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [label, setLabel] = useState("")
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [pendingRevoke, setPendingRevoke] = useState<APIKey | null>(null)

  const { data: keys = [], isLoading } = useQuery({
    queryKey: ["api-keys", projectId, envKey],
    queryFn: () => api.get<APIKey[]>(envPath("api-keys")),
    enabled: !!projectId && !!envKey,
  })

  const createKey = useMutation({
    mutationFn: () => api.post<{ rawKey: string }>(envPath("api-keys"), { label }),
    onSuccess: (data) => {
      queryClient.invalidateQueries({ queryKey: ["api-keys", projectId, envKey] })
      setCreatedKey(data.rawKey)
      setLabel("")
    },
  })

  const revokeKey = useMutation({
    mutationFn: (key: APIKey) => api.delete(`${envPath("api-keys")}/${key.id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["api-keys", projectId, envKey] })
      setPendingRevoke(null)
    },
  })

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    createKey.mutate()
  }

  const columns: DataTableColumn<APIKey>[] = [
    { key: "label", header: "Label", render: (key) => key.label },
    { key: "prefix", header: "Key", render: (key) => <code className="text-sm">{key.prefix}…</code> },
    { key: "status", header: "Status", render: (key) => (key.revoked ? "Revoked" : "Active") },
    {
      key: "actions",
      header: "",
      className: "w-24 text-right",
      render: (key) =>
        !key.revoked && (
          <Button
            variant="ghost"
            size="sm"
            onClick={(event) => {
              event.stopPropagation()
              setPendingRevoke(key)
            }}
          >
            Revoke
          </Button>
        ),
    },
  ]

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div>
        <p className="text-sm font-medium text-primary">Project settings</p>
        <h1 className="mt-1 text-3xl font-semibold tracking-tight">API access</h1>
        <p className="mt-2 text-sm leading-6 text-muted-foreground">Create and revoke OFREP keys for the selected environment.</p>
      </div>
      <Card>
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <CardTitle className="flex items-center gap-2"><KeyRound className="size-5" /> OFREP API keys</CardTitle>
            <CardDescription className="mt-2">Keys are scoped to the environment selected in the header.</CardDescription>
          </div>
          <Dialog
            open={open}
            onOpenChange={(value) => {
              setOpen(value)
              if (!value) setCreatedKey(null)
            }}
          >
            <DialogTrigger asChild><Button size="sm">New key</Button></DialogTrigger>
            <DialogContent>
              <DialogHeader><DialogTitle>Create API key</DialogTitle></DialogHeader>
              {createdKey ? (
                <div className="flex flex-col gap-3">
                  <p className="text-sm text-muted-foreground">Copy this key now — it won't be shown again.</p>
                  <code className="break-all rounded bg-muted p-2 text-sm">{createdKey}</code>
                  <DialogFooter><Button onClick={() => setOpen(false)}>Done</Button></DialogFooter>
                </div>
              ) : (
                <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                  <div className="flex flex-col gap-2">
                    <Label htmlFor="key-label">Label</Label>
                    <Input id="key-label" placeholder="CI" value={label} onChange={(event) => setLabel(event.target.value)} required />
                  </div>
                  <DialogFooter><Button type="submit" disabled={createKey.isPending || !label.trim()}>{createKey.isPending ? "Creating…" : "Create key"}</Button></DialogFooter>
                </form>
              )}
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent>
          <DataTable columns={columns} rows={keys} rowKey={(key) => key.id} emptyMessage={isLoading ? "Loading…" : "No API keys yet"} />
        </CardContent>
      </Card>
      <ConfirmDialog
        open={!!pendingRevoke}
        onOpenChange={(value) => !value && setPendingRevoke(null)}
        title={`Revoke "${pendingRevoke?.label}"?`}
        description="OFREP clients using this key will stop being able to evaluate flags."
        confirmLabel="Revoke"
        loading={revokeKey.isPending}
        onConfirm={() => pendingRevoke && revokeKey.mutate(pendingRevoke)}
      />
    </div>
  )
}
