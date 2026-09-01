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
import { useTranslation } from "react-i18next"

interface Flag {
  key: string
  name: string
	description: string
	tags: string[]
  flagType: string
  enabled: boolean
  archived: boolean
  strategies: unknown[]
}

const FLAG_TYPES = ["boolean", "string", "number", "object"]
type FlagAction = "archive" | "unarchive" | "delete"

export function FlagsListPage() {
  const { t } = useTranslation()
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
    onError: (err) => setError(err instanceof ApiError ? err.message : t("flags.createFailed")),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    createFlag.mutate()
  }

  const columns: DataTableColumn<Flag>[] = [
    { key: "key", header: t("flags.key"), render: (f) => <code className="text-sm">{f.key}</code> },
    { key: "name", header: t("flags.name"), render: (f) => f.name },
		{
			key: "tags",
		header: t("flags.tags"),
			render: (f) => f.tags.length ? <div className="flex max-w-48 flex-wrap gap-1">{f.tags.map((tag) => <Badge key={tag} variant="outline">{tag}</Badge>)}</div> : "—",
		},
    { key: "type", header: t("flags.type"), render: (f) => <Badge variant="secondary">{f.flagType}</Badge> },
    { key: "strategies", header: t("flags.strategies"), render: (f) => f.strategies.length },
    {
      key: "enabled",
      header: t("flags.enabled"),
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
      header: t("flags.status"),
      render: (f) => f.archived ? <Badge variant="secondary">{t("flags.archived")}</Badge> : <Badge variant="outline">{t("flags.active")}</Badge>,
    },
    {
      key: "actions",
      header: "",
      className: "w-44 text-right",
      render: (f) => (
        <div className="flex justify-end gap-1" onClick={(event) => event.stopPropagation()}>
          <Button variant="ghost" size="sm" onClick={() => setPendingAction({ flag: f, action: f.archived ? "unarchive" : "archive" })}>
            {f.archived ? t("flags.unarchive") : t("flags.archive")}
          </Button>
          <Button variant="ghost" size="sm" onClick={() => setPendingAction({ flag: f, action: "delete" })}>{t("flags.delete")}</Button>
        </div>
      ),
    },
  ]

  const actionCopy: Record<FlagAction, { title: string; description: string; label: string; variant: "default" | "destructive" }> = {
    archive: {
      title: t("flags.archiveTitle", { key: pendingAction?.flag.key ?? "" }),
      description: t("flags.archiveDescription"),
      label: t("flags.archive"),
      variant: "default",
    },
    unarchive: {
      title: t("flags.unarchiveTitle", { key: pendingAction?.flag.key ?? "" }),
      description: t("flags.unarchiveDescription"),
      label: t("flags.unarchive"),
      variant: "default",
    },
    delete: {
      title: t("flags.deleteTitle", { key: pendingAction?.flag.key ?? "" }),
      description: t("flags.deleteDescription"),
      label: t("flags.delete"),
      variant: "destructive",
    },
  }
  const confirmation = pendingAction ? actionCopy[pendingAction.action] : null

  return (
    <div className="p-8">
      <div className="mb-6 flex items-start justify-between">
        <div>
          <h1 className="text-2xl font-semibold">{t("flags.title")}</h1>
          <p className="mt-1 text-sm text-muted-foreground">{t("flags.intro")}</p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>{t("flags.new")}</Button>
          </DialogTrigger>
          <DialogContent className="max-w-lg">
            <DialogHeader>
              <DialogTitle>{t("flags.createTitle")}</DialogTitle>
            </DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="flag-key">{t("flags.key")}</Label>
                <Input id="flag-key" placeholder={t("flags.keyPlaceholder")} value={key} onChange={(e) => setKey(e.target.value)} required />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="flag-name">{t("flags.name")}</Label>
                <Input id="flag-name" value={name} onChange={(e) => setName(e.target.value)} />
              </div>
              <div className="flex flex-col gap-2">
                <Label>{t("flags.type")}</Label>
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
                <p className="text-xs text-muted-foreground">{t("flags.typeHint")}</p>
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <DialogFooter>
                <Button type="submit" disabled={createFlag.isPending}>
                  {t("common.create")}
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
        emptyMessage={isLoading ? t("common.loading") : t("flags.noFlags")}
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
