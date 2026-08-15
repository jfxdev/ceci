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
import { api, ApiError } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"

interface Flag {
  key: string
  name: string
  flagType: string
  enabled: boolean
  defaultVariant: string
}

interface VariantRow {
  key: string
  value: string
}

const FLAG_TYPES = ["boolean", "string", "number", "object"]

function parseVariantValue(flagType: string, raw: string): unknown {
  if (flagType === "boolean") return raw === "true"
  if (flagType === "number") return Number(raw)
  if (flagType === "object") return JSON.parse(raw)
  return raw
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

  const [open, setOpen] = useState(false)
  const [key, setKey] = useState("")
  const [name, setName] = useState("")
  const [flagType, setFlagType] = useState("boolean")
  const [defaultVariant, setDefaultVariant] = useState("off")
  const [variants, setVariants] = useState<VariantRow[]>([
    { key: "on", value: "true" },
    { key: "off", value: "false" },
  ])
  const [error, setError] = useState<string | null>(null)

  const createFlag = useMutation({
    mutationFn: () =>
      api.post(envPath("flags"), {
        key,
        name,
        flagType,
        defaultVariant,
        variants: variants.map((v) => ({ key: v.key, value: parseVariantValue(flagType, v.value) })),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["flags", projectId, envKey] })
      setOpen(false)
      setKey("")
      setName("")
      setFlagType("boolean")
      setDefaultVariant("off")
      setVariants([
        { key: "on", value: "true" },
        { key: "off", value: "false" },
      ])
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to create flag"),
  })

  function updateVariant(index: number, field: keyof VariantRow, value: string) {
    setVariants((prev) => prev.map((v, i) => (i === index ? { ...v, [field]: value } : v)))
  }

  function addVariant() {
    setVariants((prev) => [...prev, { key: "", value: "" }])
  }

  function removeVariant(index: number) {
    setVariants((prev) => prev.filter((_, i) => i !== index))
  }

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    try {
      variants.forEach((v) => parseVariantValue(flagType, v.value))
    } catch {
      setError("Variant values must be valid for the selected type")
      return
    }
    createFlag.mutate()
  }

  const columns: DataTableColumn<Flag>[] = [
    { key: "key", header: "Key", render: (f) => <code className="text-sm">{f.key}</code> },
    { key: "name", header: "Name", render: (f) => f.name },
    { key: "type", header: "Type", render: (f) => <Badge variant="secondary">{f.flagType}</Badge> },
    { key: "default", header: "Default", render: (f) => f.defaultVariant },
    {
      key: "enabled",
      header: "Enabled",
      render: (f) => (
        <Switch
          checked={f.enabled}
          onCheckedChange={() => toggleFlag.mutate(f)}
          onClick={(e) => e.stopPropagation()}
        />
      ),
    },
  ]

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
              <div className="flex gap-4">
                <div className="flex flex-1 flex-col gap-2">
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
                </div>
                <div className="flex flex-1 flex-col gap-2">
                  <Label>Default variant</Label>
                  <Select value={defaultVariant} onValueChange={setDefaultVariant}>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      {variants.filter((v) => v.key).map((v) => (
                        <SelectItem key={v.key} value={v.key}>
                          {v.key}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>
              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between">
                  <Label>Variants</Label>
                  <Button type="button" variant="ghost" size="sm" onClick={addVariant}>
                    Add variant
                  </Button>
                </div>
                {variants.map((v, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <Input
                      placeholder="key"
                      value={v.key}
                      onChange={(e) => updateVariant(i, "key", e.target.value)}
                      className="w-32"
                      required
                    />
                    <Input
                      placeholder="value"
                      value={v.value}
                      onChange={(e) => updateVariant(i, "value", e.target.value)}
                      required
                    />
                    <Button type="button" variant="ghost" size="sm" onClick={() => removeVariant(i)} disabled={variants.length <= 1}>
                      Remove
                    </Button>
                  </div>
                ))}
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
    </div>
  )
}
