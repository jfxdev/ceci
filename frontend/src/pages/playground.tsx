import { useEffect, useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery } from "@tanstack/react-query"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { MultiSelect } from "@/components/shared/multi-select"
import { api, ApiError } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"

interface Flag {
  key: string
  name: string
}

interface PlaygroundResult {
  key: string
  value?: unknown
  reason: string
  variant?: string
  errorCode?: string
  errorDetails?: string
}

const DEFAULT_CONTEXT = `{
  "targetingKey": "user-123",
  "empresa": "inter",
  "environment": "prd",
  "user_group": "xyz"
}`

export function PlaygroundPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const { envKey, envPath } = useEnvironment()
  const [contextText, setContextText] = useState(DEFAULT_CONTEXT)
  const [selectedFlags, setSelectedFlags] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)

  const { data: flags = [] } = useQuery({
    queryKey: ["flags", projectId, envKey],
    queryFn: () => api.get<Flag[]>(envPath("flags")),
    enabled: !!projectId && !!envKey,
  })

  useEffect(() => {
    setSelectedFlags(flags.map((f) => f.key))
  }, [flags])

  const evaluate = useMutation({
    mutationFn: (context: unknown) =>
      api.post<{ flags: PlaygroundResult[] }>(envPath("playground/evaluate"), { context }),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to evaluate flags"),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    let context: unknown
    try {
      context = contextText.trim() ? JSON.parse(contextText) : {}
    } catch {
      setError("Context must be valid JSON")
      return
    }
    evaluate.mutate(context)
  }

  const columns: DataTableColumn<PlaygroundResult>[] = [
    { key: "key", header: "Flag", render: (r) => <code className="text-sm">{r.key}</code> },
    {
      key: "value",
      header: "Value",
      render: (r) => <code className="text-sm">{r.value === undefined ? "-" : JSON.stringify(r.value)}</code>,
    },
    { key: "variant", header: "Variant", render: (r) => r.variant ?? "-" },
    { key: "reason", header: "Reason", render: (r) => <Badge variant="secondary">{r.reason}</Badge> },
    {
      key: "error",
      header: "Error",
      render: (r) => (r.errorCode ? <span className="text-sm text-destructive">{r.errorCode}</span> : "-"),
    },
  ]

  return (
    <div className="p-8">
      <h1 className="mb-6 text-2xl font-semibold">Playground</h1>
      <form className="mb-6 flex flex-col gap-3" onSubmit={handleSubmit}>
        <label className="text-sm font-medium">Features to test</label>
        <MultiSelect
          className="max-w-md"
          options={flags.map((f) => ({ value: f.key, label: f.name || f.key }))}
          selected={selectedFlags}
          onChange={setSelectedFlags}
          placeholder="No flags selected"
        />
        <label htmlFor="playground-context" className="text-sm font-medium">
          Evaluation context
        </label>
        <Textarea
          id="playground-context"
          rows={8}
          value={contextText}
          onChange={(e) => setContextText(e.target.value)}
          spellCheck={false}
        />
        {error && <p className="text-sm text-destructive">{error}</p>}
        <div>
          <Button type="submit" disabled={evaluate.isPending}>
            Evaluate
          </Button>
        </div>
      </form>
      {evaluate.isSuccess && (
        <DataTable
          columns={columns}
          rows={evaluate.data.flags.filter((r) => selectedFlags.includes(r.key))}
          rowKey={(r) => r.key}
          emptyMessage="No flags selected"
        />
      )}
    </div>
  )
}
