import { useState, type FormEvent } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Switch } from "@/components/ui/switch"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api, ApiError } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"

interface Flag {
  key: string
  name: string
  flagType: string
  enabled: boolean
  archived: boolean
  strategies: { isDefault: boolean; defaultVariant: string }[]
}

function defaultVariantOf(flag: Flag): string {
  return flag.strategies.find((s) => s.isDefault)?.defaultVariant ?? ""
}

const FLAG_TYPES = ["boolean", "string", "number", "object"]
type FlagAction = "archive" | "unarchive" | "delete"

const FLAG_TYPE_HINT: Record<string, string> = {
  boolean: "Starts with A/B variants.",
  string: "Starts with a single default variant — add the rest in the editor.",
  number: "Starts with a single default variant — add the rest in the editor.",
  object: "Starts with a single default variant — add the rest in the editor.",
}

export function FlagsListPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const { envKey, envPath } = useEnvironment()

  const { data: flags = [], isLoading } = useQuery({
    queryKey: ["flags", projectId, envKey],
    queryFn: () => api.get<Flag[]>(envPath("flags")),
    enabled: !!projectId && !!envKey,
  })

  const toggleFlag = useMutation({
    mutationFn: (flag: Flag) => api.patch(`${envPath("flags")}/${flag.key}`, { enabled: !flag.enabled }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["flags", projectId, envKey] }),
  })

  const [pendingAction, setPendingAction] = useState<{ flag: Flag; action: FlagAction } | null>(null)
  const flagAction = useMutation({
    mutationFn: ({ flag, action }: { flag: Flag; action: FlagAction }) => {
      if (action === "archive") return api.post(`${envPath("flags")}/${flag.key}/archive`)
      if (action === "unarchive") return api.post(`${envPath("flags")}/${flag.key}/unarchive`)
      return api.delete(`${envPath("flags")}/${flag.key}`)
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["flags", projectId, envKey] })
      setPendingAction(null)
    },
  })

  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [name, setName] = useState("")
  const [flagType, setFlagType] = useState("boolean")
  const [error, setError] = useState<string | null>(null)

  const createFlag = useMutation({
    mutationFn: () => api.post(envPath("flags"), { key, name, flagType }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["flags", projectId, envKey] })
      setOpen(false)
      setKey("")
      setName("")
      setFlagType("boolean")
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to create flag"),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    createFlag.mutate()
  }

  const columns: DataTableColumn<Flag>[] = [
    { key: "key", header: "Key", render: (f) => <code className="text-sm">{f.key}</code> },
    { key: "name", header: "Name", render: (f) => f.name },
    { key: "type", header: "Type", render: (f) => <Badge variant="secondary">{f.flagType}</Badge> },
    { key: "default", header: "Default", render: (f) => defaultVariantOf(f) },
    {
      key: "enabled",
      header: "Enabled",
      render: (f) => (
        <Switch
          checked={f.enabled && !f.archived}
          onCheckedChange={() => toggleFlag.mutate(f)}
          onClick={(e) => e.stopPropagation()}
          disabled={f.archived}
        />
      ),
    },
    {
      key: "status",
      header: "Status",
      render: (f) => f.archived ? <Badge variant="secondary">Archived</Badge> : <Badge variant="outline">Active</Badge>,
    },
    {
      key: "actions",
      header: "",
      className: "w-44 text-right",
      render: (f) => (
        <div className="flex justify-end gap-1" onClick={(event) => event.stopPropagation()}>
          <Button variant="ghost" size="sm" onClick={() => setPendingAction({ flag: f, action: f.archived ? "unarchive" : "archive" })}>
            {f.archived ? "Unarchive" : "Archive"}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setPendingAction({ flag: f, action: "delete" })}>Delete</Button>
        </div>
      ),
    },
  ]

  const actionCopy: Record<FlagAction, { title: string; description: string; label: string; variant: "default" | "destructive" }> = {
    archive: {
      title: `Archive "${pendingAction?.flag.key ?? ""}"?`,
      description: "The flag will stop serving targeted values until it is unarchived.",
      label: "Archive",
      variant: "default",
    },
    unarchive: {
      title: `Unarchive "${pendingAction?.flag.key ?? ""}"?`,
      description: "The flag will become available for evaluation again.",
      label: "Unarchive",
      variant: "default",
    },
    delete: {
      title: `Delete "${pendingAction?.flag.key ?? ""}"?`,
      description: "All variants, targeting rules, and environment configuration for this flag will be permanently removed.",
      label: "Delete",
      variant: "destructive",
    },
  }
  const confirmation = pendingAction ? actionCopy[pendingAction.action] : null

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Feature flags</h1>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>New flag</Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>Create flag</DialogTitle>
            </DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="flag-key">Key</Label>
                <Input id="flag-key" placeholder="new-checkout" value={key} onChange={(e) => setKey(e.target.value)} required />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="flag-name">Name</Label>
                <Input id="flag-name" value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div className="flex flex-col gap-2">
                <Label>Type</Label>
                <Select value={flagType} onValueChange={setFlagType}>
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {FLAG_TYPES.map((t) => (
                      <SelectItem key={t} value={t}>
                        {t}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-xs text-muted-foreground">{FLAG_TYPE_HINT[flagType]}</p>
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <DialogFooter>
                <Button type="submit" disabled={createFlag.isPending}>
                  Create
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      <DataTable
        columns={columns}
        rows={flags}
        rowKey={(f) => f.key}
        emptyMessage={isLoading ? "Loading..." : "No flags yet"}
        onRowClick={(f) => navigate(`/projects/${projectId}/flags/${f.key}`)}
      />
      <ConfirmDialog
        open={!!pendingAction}
        onOpenChange={(value) => !value && setPendingAction(null)}
        title={confirmation?.title ?? ""}
        description={confirmation?.description}
        confirmLabel={confirmation?.label}
        confirmVariant={confirmation?.variant}
        loading={flagAction.isPending}
        onConfirm={() => pendingAction && flagAction.mutate(pendingAction)}
      />
    </div>
  )
}
