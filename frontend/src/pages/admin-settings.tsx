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
import { useTranslation } from "react-i18next"

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
  const { t } = useTranslation()

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
            <CardTitle>{t("pages.adminAccessRequired")}</CardTitle>
            <CardDescription className="mt-2">{t("pages.adminAccessDescription")}</CardDescription>
          </div>
        </Card>
      </div>
    )
  }

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <p className="text-sm font-medium text-primary">{t("pages.administration")}</p>
          <h1 className="mt-1 text-3xl font-semibold tracking-tight">{t("pages.environmentDefaults")}</h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">
            {t("pages.templateIntro")}
          </p>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button disabled={maintenance.enabled}><Plus /> {t("pages.newTemplate")}</Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader><DialogTitle>{t("pages.addEnvironmentTemplate")}</DialogTitle></DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="template-key">{t("pages.key")}</Label>
                <Input id="template-key" placeholder="staging" value={key} onChange={(event) => setKey(event.target.value)} required />
                <p className="text-xs text-muted-foreground">{t("pages.keyLocked")}</p>
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="template-name">{t("pages.name")}</Label>
                <Input id="template-name" placeholder={t("pages.templateNamePlaceholder")} value={name} onChange={(event) => setName(event.target.value)} required />
              </div>
              <label className="flex cursor-pointer items-start gap-3 rounded-lg border p-3">
                <Checkbox checked={isRequired} onCheckedChange={(checked) => setIsRequired(checked === true)} />
                <span>
                  <span className="block text-sm font-medium">{t("pages.requiredForNewProject")}</span>
                  <span className="mt-1 block text-xs leading-5 text-muted-foreground">{t("pages.requiredTemplateDescription")}</span>
                </span>
              </label>
              <DialogFooter><Button type="submit" disabled={maintenance.enabled || createTemplate.isPending || !key.trim() || !name.trim()}>{createTemplate.isPending ? t("pages.adding") : t("pages.addTemplate")}</Button></DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>

      <Card className={maintenance.enabled ? "border-amber-400" : ""}>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><TriangleAlert className="size-5" /> {t("pages.maintenanceMode")}</CardTitle>
          <CardDescription>
            {maintenance.enabled
              ? t("pages.maintenanceEnabledDescription")
              : t("pages.maintenanceDisabledDescription")}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label htmlFor="maintenance-message">{t("pages.maintenanceMessage")}</Label>
            <Textarea
              id="maintenance-message"
              value={maintenanceMessage}
              onChange={(event) => setMaintenanceMessage(event.target.value)}
              maxLength={500}
              placeholder={t("pages.maintenancePlaceholder")}
            />
            <p className="text-xs text-muted-foreground">{t("pages.characters", { count: maintenanceMessage.length })}</p>
          </div>
          <div className="flex flex-wrap gap-3">
            {maintenance.enabled ? (
              <>
                <Button
                  variant="secondary"
                  disabled={updateMaintenance.isPending || !maintenanceMessage.trim()}
                  onClick={() => updateMaintenance.mutate({ enabled: true, message: maintenanceMessage })}
                >
                  {updateMaintenance.isPending ? t("common.loading") : t("pages.updateMessage")}
                </Button>
                <Button
                  variant="outline"
                  disabled={updateMaintenance.isPending}
                  onClick={() => updateMaintenance.mutate({ enabled: false, message: "" })}
                >
                  {t("pages.endMaintenance")}
                </Button>
              </>
            ) : (
              <Button disabled={!maintenanceMessage.trim()} onClick={() => setConfirmEnableMaintenance(true)}>
                {t("pages.enableMaintenance")}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2"><ShieldCheck className="size-5" /> {t("pages.templates")}</CardTitle>
          <CardDescription>{t("pages.templateBaselineDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <p className="text-sm text-muted-foreground">{t("common.loading")}</p>
          ) : templates.length === 0 ? (
            <div className="rounded-lg border border-dashed p-8 text-center">
              <p className="font-medium">{t("pages.noTemplates")}</p>
              <p className="mt-1 text-sm text-muted-foreground">{t("pages.templateEmptyDescription")}</p>
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
                      {t("projects.required")}
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
        description={t("pages.deleteTemplateDescription")}
        confirmLabel={t("pages.deleteTemplate")}
        loading={deleteTemplate.isPending}
        onConfirm={() => pendingDelete && deleteTemplate.mutate(pendingDelete)}
      />
      <ConfirmDialog
        open={confirmEnableMaintenance}
        onOpenChange={setConfirmEnableMaintenance}
        title={t("pages.confirmMaintenanceTitle")}
        description={t("pages.confirmMaintenanceDescription")}
        confirmLabel={t("pages.confirmEnableMaintenance")}
        loading={updateMaintenance.isPending}
        onConfirm={() => updateMaintenance.mutate({ enabled: true, message: maintenanceMessage })}
      />
    </div>
  )
}
