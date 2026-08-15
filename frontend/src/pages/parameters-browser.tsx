import { useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Dialog, DialogContent, DialogFooter, DialogHeader, DialogTitle, DialogTrigger } from "@/components/ui/dialog"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { ConfirmDialog } from "@/components/shared/confirm-dialog"
import { api } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"

interface Parameter {
  key: string
  value: string
  version: number
}

export function ParametersBrowserPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const { envKey, envPath } = useEnvironment()
  const queryClient = useQueryClient()
  const [prefix, setPrefix] = useState("")
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [value, setValue] = useState("")
  const [pendingDelete, setPendingDelete] = useState<Parameter | null>(null)

  const { data: parameters = [], isLoading } = useQuery({
    queryKey: ["parameters", projectId, envKey, prefix],
    queryFn: () => api.get<Parameter[]>(`${envPath("parameters")}?prefix=${encodeURIComponent(prefix)}`),
    enabled: !!projectId && !!envKey,
  })

  const upsert = useMutation({
    mutationFn: () => api.put(`${envPath("parameters/value")}/${key}`, { value }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["parameters", projectId, envKey] })
      setOpen(false)
      setKey("")
      setValue("")
    },
  })

  const remove = useMutation({
    mutationFn: (p: Parameter) => api.delete(`${envPath("parameters/value")}/${p.key}`),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["parameters", projectId, envKey] })
      setPendingDelete(null)
    },
  })

  const columns: DataTableColumn<Parameter>[] = [
    { key: "key", header: "Key", render: (p) => <code className="text-sm">{p.key}</code> },
    { key: "value", header: "Value", render: (p) => p.value },
    { key: "version", header: "Version", render: (p) => `v${p.version}` },
    {
      key: "actions",
      header: "",
      render: (p) => (
        <Button variant="ghost" size="sm" onClick={(e) => { e.stopPropagation(); setPendingDelete(p) }}>
          Delete
        </Button>
      ),
    },
  ]

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    upsert.mutate()
  }

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold">Parameters</h1>
        <div className="flex items-center gap-2">
          <Input placeholder="Filter by prefix..." value={prefix} onChange={(e) => setPrefix(e.target.value)} className="w-64" />
          <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
              <Button>Set parameter</Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Set parameter</DialogTitle>
              </DialogHeader>
              <form className="flex flex-col gap-4" onSubmit={handleSubmit}>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="key">Key</Label>
                  <Input id="key" placeholder="service/db/host" value={key} onChange={(e) => setKey(e.target.value)} required />
                </div>
                <div className="flex flex-col gap-2">
                  <Label htmlFor="value">Value</Label>
                  <Input id="value" value={value} onChange={(e) => setValue(e.target.value)} required />
                </div>
                <DialogFooter>
                  <Button type="submit" disabled={upsert.isPending}>
                    Save
                  </Button>
                </DialogFooter>
              </form>
            </DialogContent>
          </Dialog>
        </div>
      </div>
      <DataTable columns={columns} rows={parameters} rowKey={(p) => p.key} emptyMessage={isLoading ? "Loading..." : "No parameters yet"} />
      <ConfirmDialog
        open={!!pendingDelete}
        onOpenChange={(v) => !v && setPendingDelete(null)}
        title={`Delete "${pendingDelete?.key}"?`}
        description="This cannot be undone."
        confirmLabel="Delete"
        loading={remove.isPending}
        onConfirm={() => pendingDelete && remove.mutate(pendingDelete)}
      />
    </div>
  )
}
