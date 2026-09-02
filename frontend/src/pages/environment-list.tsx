import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Boxes, Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api } from "@/lib/api"
import { alertMessageFromError } from "@/lib/alerts"
import { useEnvironment, type Environment } from "@/lib/environment"
import { useTranslation } from "react-i18next"

export function EnvironmentListPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const queryClient = useQueryClient()
  const { environments, isLoading } = useEnvironment()
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [name, setName] = useState("")
  const [editing, setEditing] = useState<Record<string, string>>({})
  const [pendingDelete, setPendingDelete] = useState<Environment | null>(null)
  const [error, setError] = useState<string | null>(null)
  const { t } = useTranslation()

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["environments", projectId] })
  }

  const createEnvironment = useMutation({
    mutationFn: () => api.post(`/projects/${projectId}/environments`, { key, name }),
    onSuccess: () => {
      invalidate()
      setOpen(false)
      setKey("")
      setName("")
      setError(null)
    },
	    onError: (err) => setError(alertMessageFromError(err, t("pages.createEnvironmentFailed"))),
  })

  const renameEnvironment = useMutation({
    mutationFn: ({ envKey, newName }: { envKey: string; newName: string }) =>
      api.patch(`/projects/${projectId}/environments/${envKey}`, { name: newName }),
    onSuccess: (_data, { envKey }) => {
      invalidate()
      setEditing((current) => {
        const next = { ...current }
        delete next[envKey]
        return next
      })
    },
  })

  const deleteEnvironment = useMutation({
    mutationFn: (environment: Environment) => api.delete(`/projects/${projectId}/environments/${environment.key}`),
    onSuccess: () => {
      invalidate()
      setPendingDelete(null)
    },
  })

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    createEnvironment.mutate()
  }

  const columns: DataTableColumn<Environment>[] = [
    {
      key: "environment",
      header: t("pages.environment"),
      render: (environment) => (
        <div className="flex items-center gap-3">
          <div className="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <Boxes className="size-4" />
          </div>
          <div>
            <p className="font-medium">{environment.name}</p>
            <code className="text-xs text-muted-foreground">{environment.key}</code>
          </div>
        </div>
      ),
    },
    {
      key: "status",
      header: t("pages.status"),
      render: (environment) => (environment.isDefault ? t("pages.projectDefault") : t("pages.active")),
    },
    {
      key: "actions",
      header: "",
      className: "w-48 text-right",
      render: (environment) => (
        <div className="flex justify-end gap-1" onClick={(event) => event.stopPropagation()}>
          {editing[environment.key] === undefined ? (
            <Button variant="ghost" size="sm" onClick={() => setEditing((current) => ({ ...current, [environment.key]: environment.name }))}>
              {t("pages.rename")}
            </Button>
          ) : (
            <>
              <Input
                aria-label={`${t("pages.rename")} ${environment.name}`}
                value={editing[environment.key]}
                onChange={(event) => setEditing((current) => ({ ...current, [environment.key]: event.target.value }))}
                className="h-8 w-32"
              />
              <Button size="sm" onClick={() => renameEnvironment.mutate({ envKey: environment.key, newName: editing[environment.key] })}>
                {t("common.save")}
              </Button>
            </>
          )}
          {!environment.isDefault && editing[environment.key] === undefined && (
            <Button variant="ghost" size="sm" onClick={() => setPendingDelete(environment)}>
              {t("common.delete")}
            </Button>
          )}
        </div>
      ),
    },
  ]

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <p className="text-sm font-medium text-primary">{t("pages.projectConfiguration")}</p>
          <h1 className="mt-1 text-3xl font-semibold tracking-tight">{t("pages.environments")}</h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
	            {t("pages.environmentsIntro")}
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button>
              <Plus />
              {t("pages.newEnvironment")}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("pages.createEnvironment")}</DialogTitle>
              <p className="text-sm text-muted-foreground">{t("pages.environmentKeyLocked")}</p>
            </DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="environment-key">{t("overview.key")}</Label>
                <Input id="environment-key" placeholder="staging" value={key} onChange={(event) => setKey(event.target.value)} required />
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="environment-name">{t("overview.name")}</Label>
                <Input id="environment-name" placeholder="Staging" value={name} onChange={(event) => setName(event.target.value)} required />
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <DialogFooter>
                <Button type="submit" disabled={createEnvironment.isPending || !key.trim() || !name.trim()}>
                  {createEnvironment.isPending ? t("projects.creating") : t("pages.createEnvironment")}
                </Button>
              </DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t("pages.availableEnvironments")}</CardTitle>
          <CardDescription>{t("pages.environmentBaseline")}</CardDescription>
        </CardHeader>
        <CardContent>
          <DataTable columns={columns} rows={environments} rowKey={(environment) => environment.key} emptyMessage={isLoading ? t("common.loading") : t("pages.noEnvironments")} />
        </CardContent>
      </Card>

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(value) => !value && setPendingDelete(null)}
        title={`${t("common.delete")} "${pendingDelete?.name}"?`}
        description={t("pages.deleteEnvironmentDescription")}
        confirmLabel={t("common.delete")}
        loading={deleteEnvironment.isPending}
        onConfirm={() => pendingDelete && deleteEnvironment.mutate(pendingDelete)}
      />
    </div>
  )
}
