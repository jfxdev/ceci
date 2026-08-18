import { useEffect, useState, type FormEvent } from "react"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { LockKeyhole, Plus, ShieldCheck, TriangleAlert, Trash2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api } from "@/lib/api"
import { useAuth } from "@/lib/auth"
import { useMaintenance, type MaintenanceStatus } from "@/lib/maintenance"

interface EnvironmentTemplate {
  id: string
  key: string
  name: string
  isRequired: boolean
}

export function AdminSettingsPage() {
  const { user } = useAuth()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [name, setName] = useState("")
  const [isRequired, setIsRequired] = useState(false)
  const [pendingDelete, setPendingDelete] = useState<EnvironmentTemplate | null>(null)
  const { status: maintenance, setStatus: setMaintenance } = useMaintenance()
  const [maintenanceMessage, setMaintenanceMessage] = useState(maintenance.message)
  const [confirmEnableMaintenance, setConfirmEnableMaintenance] = useState(false)

  useEffect(() => setMaintenanceMessage(maintenance.message), [maintenance.message])

  const { data: templates = [], isLoading } = useQuery({
    queryKey: ["admin", "environment-templates"],
    queryFn: () => api.get<EnvironmentTemplate[]>("/admin/environment-templates"),
    enabled: !!user?.isAdmin,
  })

  const createTemplate = useMutation({
    mutationFn: () => api.post<EnvironmentTemplate>("/admin/environment-templates", { key, name, isRequired }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "environment-templates"] })
      setOpen(false)
      setKey("")
      setName("")
      setIsRequired(false)
    },
  })

  const updateTemplate = useMutation({
    mutationFn: ({ template, required }: { template: EnvironmentTemplate; required: boolean }) =>
      api.patch<EnvironmentTemplate>(`/admin/environment-templates/${template.id}`, { name: template.name, isRequired: required }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["admin", "environment-templates"] }),
  })

  const deleteTemplate = useMutation({
    mutationFn: (template: EnvironmentTemplate) => api.delete(`/admin/environment-templates/${template.id}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["admin", "environment-templates"] })
      setPendingDelete(null)
    },
  })

  const updateMaintenance = useMutation({
    mutationFn: ({ enabled, message }: { enabled: boolean; message: string }) =>
      api.patch<MaintenanceStatus>("/admin/maintenance", { enabled, message }),
    onSuccess: (status) => {
      setMaintenance(status)
      setMaintenanceMessage(status.message)
      setConfirmEnableMaintenance(false)
    },
  })

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    createTemplate.mutate()
  }

  if (!user?.isAdmin) {
    return (
      <div className="mx-auto flex w-full max-w-2xl p-4 sm:p-6 lg:p-8">
        <Card className="w-full items-center py-12 text-center">
          <LockKeyhole className="size-6 text-muted-foreground" />
          <div>
            <CardTitle>Admin access required</CardTitle>
            <CardDescription className="mt-2">Only workspace administrators can change project environment defaults.</CardDescription>
          </div>
        </Card>
      </div>
    )
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <p className="text-sm font-medium text-primary">Administration</p>
          <h1 className="mt-1 text-3xl font-semibold tracking-tight">Environment defaults</h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
            Define the environments offered when a project is created. Required templates are automatically included and cannot be deselected.
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button disabled={maintenance.enabled}><Plus /> New template</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader><DialogTitle>Add environment template</DialogTitle></DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="template-key">Key</Label>
                <Input id="template-key" placeholder="staging" value={key} onChange={(event) => setKey(event.target.value)} required />
                <p className="text-xs text-muted-foreground">Keys are used in APIs and stay unchanged after creation.</p>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="template-name">Name</Label>
                <Input id="template-name" placeholder="Staging" value={name} onChange={(event) => setName(event.target.value)} required />
              </div>
              <label className="flex cursor-pointer items-start gap-3 rounded-lg border p-3">
                <Checkbox checked={isRequired} onCheckedChange={(checked) => setIsRequired(checked === true)} />
                <span>
                  <span className="block text-sm font-medium">Required for every new project</span>
                  <span className="mt-1 block text-xs leading-5 text-muted-foreground">This environment will always be provisioned, even if it is not selected in the project form.</span>
                </span>
              </label>
              <DialogFooter><Button type="submit" disabled={maintenance.enabled || createTemplate.isPending || !key.trim() || !name.trim()}>{createTemplate.isPending ? "Adding…" : "Add template"}</Button></DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Card className={maintenance.enabled ? "border-amber-400" : ""}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><TriangleAlert className="size-5" /> Maintenance mode</CardTitle>
          <CardDescription>
            {maintenance.enabled
              ? "The Control Plane is read-only. Data Plane workloads continue to run normally."
              : "Temporarily prevent Control Plane configuration changes while maintenance is in progress."}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="maintenance-message">Message shown to users</Label>
            <Textarea
              id="maintenance-message"
              value={maintenanceMessage}
              onChange={(event) => setMaintenanceMessage(event.target.value)}
              maxLength={500}
              placeholder="We are upgrading project settings. Changes will be available shortly."
            />
            <p className="text-xs text-muted-foreground">{maintenanceMessage.length}/500 characters</p>
          </div>
          <div className="flex flex-wrap gap-3">
            {maintenance.enabled ? (
              <>
                <Button
                  variant="secondary"
                  disabled={updateMaintenance.isPending || !maintenanceMessage.trim()}
                  onClick={() => updateMaintenance.mutate({ enabled: true, message: maintenanceMessage })}
                >
                  {updateMaintenance.isPending ? "Saving…" : "Update message"}
                </Button>
                <Button
                  variant="outline"
                  disabled={updateMaintenance.isPending}
                  onClick={() => updateMaintenance.mutate({ enabled: false, message: "" })}
                >
                  End maintenance
                </Button>
              </>
            ) : (
              <Button disabled={!maintenanceMessage.trim()} onClick={() => setConfirmEnableMaintenance(true)}>
                Enable maintenance mode
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><ShieldCheck className="size-5" /> Project environment templates</CardTitle>
          <CardDescription>The shared baseline environment is always created separately for each project.</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">Loading templates…</p>
          ) : templates.length === 0 ? (
            <div className="rounded-lg border border-dashed p-8 text-center">
              <p className="font-medium">No templates yet</p>
              <p className="mt-1 text-sm text-muted-foreground">New projects will start with their shared baseline only.</p>
            </div>
          ) : (
            <div className="divide-y">
              {templates.map((template) => (
                <div key={template.id} className="flex flex-col gap-4 py-4 sm:flex-row sm:items-center sm:justify-between">
                  <div>
                    <p className="font-medium">{template.name}</p>
                    <code className="text-xs text-muted-foreground">{template.key}</code>
                  </div>
                  <div className="flex items-center gap-3">
                    <label className="flex cursor-pointer items-center gap-2 text-sm">
                      <Checkbox
                        checked={template.isRequired}
                        disabled={maintenance.enabled || updateTemplate.isPending}
                        onCheckedChange={(checked) => updateTemplate.mutate({ template, required: checked === true })}
                      />
                      Required
                    </label>
                    <Button variant="ghost" size="icon-sm" aria-label={`Delete ${template.name}`} disabled={maintenance.enabled} onClick={() => setPendingDelete(template)}>
                      <Trash2 />
                    </Button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(value) => !value && setPendingDelete(null)}
        title={`Delete "${pendingDelete?.name}"?`}
        description="This will not change existing projects. New projects will no longer be offered this environment."
        confirmLabel="Delete template"
        loading={deleteTemplate.isPending}
        onConfirm={() => pendingDelete && deleteTemplate.mutate(pendingDelete)}
      />
      <ConfirmDialog
        open={confirmEnableMaintenance}
        onOpenChange={setConfirmEnableMaintenance}
        title="Enable maintenance mode?"
        description="All Control Plane configuration changes will be blocked until an administrator ends maintenance. Data Plane workloads will remain available."
        confirmLabel="Enable maintenance"
        loading={updateMaintenance.isPending}
        onConfirm={() => updateMaintenance.mutate({ enabled: true, message: maintenanceMessage })}
      />
    </div>
  )
}
