import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Braces, Plus } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { api, ApiError } from "@/lib/api"
import { useTranslation } from "react-i18next"

interface ContextFieldValue {
  value: string
  description?: string
}

export interface ContextField {
  id: string
  key: string
  description?: string
  values: ContextFieldValue[]
}

function emptyForm() {
  return { key: "", description: "", values: [] as ContextFieldValue[] }
}

export function ContextFieldsPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ContextField | null>(null)
  const [pendingDelete, setPendingDelete] = useState<ContextField | null>(null)
  const [form, setForm] = useState(emptyForm)
  const [error, setError] = useState<string | null>(null)
  const { t } = useTranslation()

  const { data: fields = [], isLoading } = useQuery({
    queryKey: ["context-fields", projectId],
    queryFn: () => api.get<ContextField[]>(`/projects/${projectId}/context-fields`),
    enabled: !!projectId,
  })

  function invalidate() {
    queryClient.invalidateQueries({ queryKey: ["context-fields", projectId] })
  }

  function closeDialog() {
    setOpen(false)
    setEditing(null)
    setForm(emptyForm())
    setError(null)
  }

  function startEdit(field: ContextField) {
    setEditing(field)
    setForm({ key: field.key, description: field.description ?? "", values: field.values })
    setError(null)
    setOpen(true)
  }

  const saveField = useMutation({
    mutationFn: () => {
      const body = { description: form.description, values: form.values }
      return editing
        ? api.patch(`/projects/${projectId}/context-fields/${encodeURIComponent(editing.key)}`, body)
        : api.post(`/projects/${projectId}/context-fields`, { ...body, key: form.key })
    },
    onSuccess: () => {
      invalidate()
      closeDialog()
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : t("pages.createContextField")),
  })

  const deleteField = useMutation({
    mutationFn: (field: ContextField) => api.delete(`/projects/${projectId}/context-fields/${encodeURIComponent(field.key)}`),
    onSuccess: () => {
      invalidate()
      setPendingDelete(null)
    },
  })

  function submit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    saveField.mutate()
  }

  function updateValue(index: number, patch: Partial<ContextFieldValue>) {
    setForm((current) => ({ ...current, values: current.values.map((value, i) => (i === index ? { ...value, ...patch } : value)) }))
  }

  const columns: DataTableColumn<ContextField>[] = [
    {
      key: "field",
      header: t("pages.contextField"),
      render: (field) => (
        <div className="flex items-center gap-3">
          <div className="flex size-9 items-center justify-center rounded-lg bg-primary/10 text-primary"><Braces className="size-4" /></div>
          <div>
            <code className="font-medium">{field.key}</code>
            {field.description && <p className="mt-1 text-xs text-muted-foreground">{field.description}</p>}
          </div>
        </div>
      ),
    },
    {
      key: "values",
      header: t("pages.possibleValues"),
      render: (field) => field.values.length ? <span className="text-sm">{field.values.map((value) => value.value).join(", ")}</span> : <span className="text-sm text-muted-foreground">{t("pages.noPredefinedValues")}</span>,
    },
    {
      key: "actions",
      header: "",
      className: "w-36 text-right",
      render: (field) => (
        <div className="flex justify-end gap-1" onClick={(event) => event.stopPropagation()}>
          <Button variant="ghost" size="sm" onClick={() => startEdit(field)}>{t("pages.rename")}</Button>
          <Button variant="ghost" size="sm" onClick={() => setPendingDelete(field)}>{t("common.delete")}</Button>
        </div>
      ),
    },
  ]

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 p-4 sm:p-6 lg:p-8">
      <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <p className="text-sm font-medium text-primary">{t("pages.projectConfiguration")}</p>
          <h1 className="mt-1 text-3xl font-semibold tracking-tight">{t("nav.contexts")}</h1>
          <p className="mt-2 max-w-2xl text-sm leading-6 text-muted-foreground">{t("pages.contextsIntro")}</p>
        </div>
        <Dialog open={open} onOpenChange={(value) => (value ? setOpen(true) : closeDialog())}>
          <DialogTrigger asChild><Button onClick={() => { setEditing(null); setForm(emptyForm()); setError(null) }}><Plus /> {t("pages.newContextField")}</Button></DialogTrigger>
          <DialogContent className="max-h-[85vh] overflow-y-auto">
            <DialogHeader><DialogTitle>{editing ? `${t("pages.rename")} ${editing.key}` : t("pages.createContextField")}</DialogTitle></DialogHeader>
            <form className="flex flex-col gap-4" onSubmit={submit}>
              <div className="flex flex-col gap-2">
                <Label htmlFor="context-key">{t("pages.contextFieldName")}</Label>
                <Input id="context-key" placeholder="region" value={form.key} onChange={(event) => setForm((current) => ({ ...current, key: event.target.value }))} disabled={!!editing} required />
                {editing && <p className="text-xs text-muted-foreground">{t("pages.contextNameLocked")}</p>}
              </div>
              <div className="flex flex-col gap-2">
                <Label htmlFor="context-description">{t("pages.description")}</Label>
                <Textarea id="context-description" value={form.description} onChange={(event) => setForm((current) => ({ ...current, description: event.target.value }))} placeholder="Where this attribute comes from and how it is used." />
              </div>
              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between"><Label>{t("pages.possibleValues")}</Label><Button type="button" variant="ghost" size="sm" onClick={() => setForm((current) => ({ ...current, values: [...current.values, { value: "", description: "" }] }))}>{t("pages.addValue")}</Button></div>
                <p className="text-xs text-muted-foreground">{t("pages.optionalValues")}</p>
                {form.values.map((value, index) => (
                  <div key={index} className="flex gap-2">
                    <Input aria-label={`Value ${index + 1}`} value={value.value} placeholder="Value" onChange={(event) => updateValue(index, { value: event.target.value })} />
                    <Input aria-label={`Value ${index + 1} description`} value={value.description ?? ""} placeholder="Description" onChange={(event) => updateValue(index, { description: event.target.value })} />
                    <Button type="button" variant="ghost" size="sm" onClick={() => setForm((current) => ({ ...current, values: current.values.filter((_, i) => i !== index) }))}>{t("pages.remove")}</Button>
                  </div>
                ))}
              </div>
              {error && <p className="text-sm text-destructive">{error}</p>}
              <DialogFooter><Button type="submit" disabled={saveField.isPending || !form.key.trim() || form.values.some((value) => !value.value.trim())}>{saveField.isPending ? t("projects.creating") : editing ? t("pages.saveChanges") : t("pages.createContextField")}</Button></DialogFooter>
            </form>
          </DialogContent>
        </Dialog>
      </div>
      <Card>
        <CardHeader><CardTitle>{t("pages.contextFields")}</CardTitle><CardDescription>{t("pages.contextShared")}</CardDescription></CardHeader>
        <CardContent><DataTable columns={columns} rows={fields} rowKey={(field) => field.id} emptyMessage={isLoading ? t("common.loading") : t("pages.noContextFields")} /></CardContent>
      </Card>
      <ConfirmDialog open={!!pendingDelete} onOpenChange={(value) => !value && setPendingDelete(null)} title={`${t("common.delete")} "${pendingDelete?.key}"?`} description={t("pages.deleteContextDescription")} confirmLabel={t("common.delete")} loading={deleteField.isPending} onConfirm={() => pendingDelete && deleteField.mutate(pendingDelete)} />
    </div>
  )
}
