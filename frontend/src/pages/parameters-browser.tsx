import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api, ApiError } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"
import { useTranslation } from "react-i18next"

interface Parameter {
  key: string
  value: string
  version: number
}

/** Escapes each segment but preserves Consul-style key hierarchy separators. */
export function parameterPath(parameterKey: string) {
  return parameterKey.split("/").map(encodeURIComponent).join("/")
}

export function ParametersBrowserPage() {
	const { t } = useTranslation()
  const { projectId } = useParams<{ projectId: string }>()
  const { envKey, envPath } = useEnvironment()
  const queryClient = useQueryClient()
  const [prefix, setPrefix] = useState("")
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [value, setValue] = useState("")
  const [editingParameter, setEditingParameter] = useState<Parameter | null>(null)
  const [pendingDelete, setPendingDelete] = useState<Parameter | null>(null)
  const [error, setError] = useState<string | null>(null)

  const { data: parameters = [], isLoading } = useQuery({
    queryKey: ["parameters", projectId, envKey, prefix],
    queryFn: () => api.get<Parameter[]>(`${envPath("parameters")}?prefix=${encodeURIComponent(prefix)}`),
    enabled: !!projectId && !!envKey,
  })

  function resetForm() {
    setKey("")
    setValue("")
    setEditingParameter(null)
    setError(null)
  }

  const upsert = useMutation({
    mutationFn: () => api.put(`${envPath("parameters/value")}/${parameterPath(key)}`, { value }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["parameters", projectId, envKey] })
      setOpen(false)
      resetForm()
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to save parameter"),
  })

  const remove = useMutation({
    mutationFn: (p: Parameter) => api.delete(`${envPath("parameters/value")}/${parameterPath(p.key)}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["parameters", projectId, envKey] })
      setPendingDelete(null)
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to delete parameter"),
  })

  const columns: DataTableColumn<Parameter>[] = [
    { key: "key", header: "Key", render: (p) => <code className="text-sm">{p.key}</code> },
    { key: "value", header: "Value", render: (p) => p.value },
    { key: "version", header: "Version", render: (p) => `v${p.version}` },
    {
      key: "actions",
      header: "",
      className: "w-40 text-right",
      render: (p) => (
        <div className="flex justify-end gap-1" onClick={(event) => event.stopPropagation()}>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setKey(p.key)
              setValue(p.value)
              setEditingParameter(p)
              setError(null)
              setOpen(true)
            }}
          >
            Edit
          </Button>
          <Button variant="ghost" size="sm" onClick={() => { setError(null); setPendingDelete(p) }}>
            Delete
          </Button>
        </div>
      ),
    },
  ]

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    upsert.mutate()
  }

  return (
    <div className="p-8">
      <div className="mb-6 flex items-start justify-between gap-4">
        <div>
	          <h1 className="text-2xl font-semibold">{t("pages.parametersTitle")}</h1>
	          <p className="mt-1 text-sm text-muted-foreground">{t("pages.parametersIntro")}</p>
        </div>
        <div className="flex items-center gap-2">
          <Input placeholder="Filter by prefix..." value={prefix} onChange={(e) => setPrefix(e.target.value)} className="w-64" />
          <Dialog
            open={open}
            onOpenChange={(nextOpen) => {
              setOpen(nextOpen)
              if (!nextOpen) resetForm()
            }}
          >
            <DialogTrigger asChild>
              <Button onClick={resetForm}>Set parameter</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{editingParameter ? `Edit "${editingParameter.key}"` : "Set parameter"}</DialogTitle>
              </DialogHeader>
              <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="key">Key</Label>
                  <Input
                    id="key"
                    placeholder="service/db/host"
                    value={key}
                    onChange={(e) => setKey(e.target.value)}
                    disabled={!!editingParameter}
                    required
                  />
                  {editingParameter && <p className="text-sm text-muted-foreground">Parameter keys cannot be changed.</p>}
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="value">Value</Label>
                  <Input id="value" value={value} onChange={(e) => setValue(e.target.value)} required />
                </div>
                {error && <p className="text-sm text-destructive">{error}</p>}
                <DialogFooter>
                  <Button type="submit" disabled={upsert.isPending || !key.trim() || !value.trim()}>
                    {upsert.isPending ? "Saving…" : editingParameter ? "Save changes" : "Save parameter"}
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>
      </div>
      {error && !open && <p className="mb-4 text-sm text-destructive">{error}</p>}
      <DataTable columns={columns} rows={parameters} rowKey={(p) => p.key} emptyMessage={isLoading ? "Loading..." : "No parameters yet"} />
      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(v) => !v && setPendingDelete(null)}
        title={`Delete "${pendingDelete?.key}"?`}
        description={error ?? "This cannot be undone."}
        confirmLabel="Delete"
        loading={remove.isPending}
        onConfirm={() => pendingDelete && remove.mutate(pendingDelete)}
      />
    </div>
  )
}
