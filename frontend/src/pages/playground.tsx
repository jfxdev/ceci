import { useEffect, useRef, useState, type FormEvent } from "react"
import { useParams } from "react-router-dom"
import { useMutation, useQuery } from "@tanstack/react-query"
import { Info } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import { DataTable, type DataTableColumn } from "@/components/shared/data-table"
import { MultiSelect } from "@/components/shared/multi-select"
import { api, ApiError } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"
import { Trans, useTranslation } from "react-i18next"

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

interface ContextField {
  id: string
  key: string
  description?: string
  values: { value: string; description?: string }[]
}

const DEFAULT_CONTEXT = `{
  "targetingKey": "user-123",
  "empresa": "inter",
  "environment": "prd",
  "user_group": "xyz"
}`

const STATUS_LEGEND = [
	{ status: "TARGETING_MATCH", labelKey: "pages.statusTargetingMatchLabel", descriptionKey: "pages.statusTargetingMatch" },
	{ status: "SPLIT", labelKey: "pages.statusSplitLabel", descriptionKey: "pages.statusSplit" },
	{ status: "NO_MATCH", labelKey: "pages.statusNoMatchLabel", descriptionKey: "pages.statusNoMatch" },
	{ status: "DISABLED", labelKey: "pages.statusDisabledLabel", descriptionKey: "pages.statusDisabled" },
	{ status: "PREREQUISITE_FAILED", labelKey: "pages.statusPrerequisiteFailedLabel", descriptionKey: "pages.statusPrerequisiteFailed" },
	{ status: "DEFAULT", labelKey: "pages.statusDefaultLabel", descriptionKey: "pages.statusDefault" },
	{ status: "STATIC", labelKey: "pages.statusStaticLabel", descriptionKey: "pages.statusStatic" },
	{ status: "ERROR", labelKey: "pages.statusErrorLabel", descriptionKey: "pages.statusError" },
] as const

export function PlaygroundPage() {
  const { t } = useTranslation()
  const { projectId } = useParams<{ projectId: string }>()
  const { environments, envKey } = useEnvironment()
  const [contextText, setContextText] = useState(DEFAULT_CONTEXT)
  const [evaluationEnvKey, setEvaluationEnvKey] = useState("")
  const [selectedContextKey, setSelectedContextKey] = useState("")
  const [selectedContextValue, setSelectedContextValue] = useState("")
  const [selectedFlags, setSelectedFlags] = useState<string[]>([])
  const [error, setError] = useState<string | null>(null)
  const initializedFlagsEnvironment = useRef<string | null>(null)

  const evaluationEnvPath = (suffix: string) =>
    `/projects/${projectId}/environments/${evaluationEnvKey}${suffix.startsWith("/") ? "" : "/"}${suffix}`

  useEffect(() => {
    if (envKey) setEvaluationEnvKey(envKey)
  }, [envKey])

  const { data: flags = [], isLoading: flagsLoading } = useQuery({
    queryKey: ["flags", projectId, evaluationEnvKey],
    queryFn: () => api.get<Flag[]>(evaluationEnvPath("flags")),
    enabled: !!projectId && !!evaluationEnvKey,
  })
  const { data: contextFields = [], isLoading: contextFieldsLoading } = useQuery({
    queryKey: ["context-fields", projectId],
    queryFn: () => api.get<ContextField[]>(`/projects/${projectId}/context-fields`),
    enabled: !!projectId,
  })

  useEffect(() => {
    if (!evaluationEnvKey || flagsLoading || initializedFlagsEnvironment.current === evaluationEnvKey) return
    setSelectedFlags(flags.map((f) => f.key))
    initializedFlagsEnvironment.current = evaluationEnvKey
  }, [evaluationEnvKey, flags, flagsLoading])

  const evaluate = useMutation({
    mutationFn: (context: unknown) =>
      api.post<{ flags: PlaygroundResult[] }>(evaluationEnvPath("playground/evaluate"), { context }),
    onError: (err) => setError(err instanceof ApiError ? err.message : t("pages.evaluationFailed")),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    let context: unknown
    try {
      context = contextText.trim() ? JSON.parse(contextText) : {}
    } catch {
      setError(t("pages.invalidJSON"))
      return
    }
    evaluate.mutate(context)
  }

  function addContextValue(field: ContextField, value: string) {
    let context: unknown
    try {
      context = contextText.trim() ? JSON.parse(contextText) : {}
    } catch {
      setError(t("pages.invalidJSONBeforeAdd"))
      return
    }

    if (!context || typeof context !== "object" || Array.isArray(context)) {
      setError(t("pages.invalidJSONObject"))
      return
    }

	setContextText(JSON.stringify({ ...context, [field.key]: value }, null, 2))
	setError(null)
	evaluate.reset()
  }

  function selectEvaluationEnvironment(key: string) {
    setEvaluationEnvKey(key)
    setSelectedFlags([])
    setError(null)
    evaluate.reset()
  }

  const selectedContext = contextFields.find((field) => field.key === selectedContextKey)
	const statusLabel = (status: string) => {
		const knownStatus = STATUS_LEGEND.find((entry) => entry.status === status)
		return knownStatus ? t(knownStatus.labelKey) : status
	}

  function selectContext(key: string) {
    setSelectedContextKey(key)
    setSelectedContextValue("")
  }

  function selectContextValue(value: string) {
    if (!selectedContext) return
    addContextValue(selectedContext, value)
    setSelectedContextValue("")
  }

  const columns: DataTableColumn<PlaygroundResult>[] = [
    { key: "key", header: t("nav.flags"), render: (r) => <code className="text-sm">{r.key}</code> },
    {
      key: "value",
      header: t("pages.possibleValues"),
      render: (r) => <code className="text-sm">{r.value === undefined ? "-" : JSON.stringify(r.value)}</code>,
    },
	{ key: "variant", header: t("pages.variant"), render: (r) => r.variant ?? "-" },
	{ key: "reason", header: t("pages.reason"), render: (r) => <Badge variant="secondary">{statusLabel(r.reason)}</Badge> },
    {
      key: "error",
		 header: t("pages.error"),
      render: (r) => (r.errorCode ? <span className="text-sm text-destructive">{r.errorCode}</span> : "-"),
    },
  ]

  return (
    <div className="p-8">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold">{t("nav.playground")}</h1>
        <p className="mt-1 text-sm text-muted-foreground">{t("pages.playgroundIntro")}</p>
        <Card className="mt-2 border-primary/20 bg-primary/5">
          <CardContent className="flex items-center gap-2 py-1.5 text-sm text-muted-foreground">
            <Info className="size-4 shrink-0 text-primary" aria-hidden="true" />
	            <p><Trans i18nKey="pages.playgroundDeterministic" components={{ strong: <span className="font-medium text-foreground" />, code: <code /> }} /></p>
          </CardContent>
        </Card>
      </div>
      <div className="grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_20rem]">
        <main>
          <form className="mb-6 flex flex-col gap-3" onSubmit={handleSubmit}>
            <label htmlFor="playground-environment" className="text-sm font-medium">{t("pages.environmentToEvaluate")}</label>
            <Select value={evaluationEnvKey} onValueChange={selectEvaluationEnvironment}>
              <SelectTrigger id="playground-environment" className="max-w-md">
                <SelectValue placeholder={t("pages.selectEnvironment")} />
              </SelectTrigger>
              <SelectContent>
                {environments.map((environment) => (
                  <SelectItem key={environment.key} value={environment.key}>{environment.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>
            <label className="text-sm font-medium">{t("pages.featuresToTest")}</label>
            <MultiSelect
              className="max-w-md"
              options={flags.map((f) => ({ value: f.key, label: f.name || f.key }))}
              selected={selectedFlags}
              onChange={setSelectedFlags}
              placeholder={t("pages.noFlagsSelected")}
            />
            <label htmlFor="playground-context" className="text-sm font-medium">
              {t("pages.evaluationContext")}
            </label>
            <Textarea
              id="playground-context"
              rows={8}
              value={contextText}
	              onChange={(e) => { setContextText(e.target.value); evaluate.reset() }}
              spellCheck={false}
            />
            {error && <p className="text-sm text-destructive">{error}</p>}
            <div>
              <Button type="submit" disabled={evaluate.isPending || !evaluationEnvKey}>
                {t("pages.evaluate")}
              </Button>
            </div>
          </form>
          {evaluate.isSuccess && (
            <DataTable
              columns={columns}
              rows={evaluate.data.flags.filter((r) => selectedFlags.includes(r.key))}
              rowKey={(r) => r.key}
              emptyMessage={t("pages.noFlagsSelected")}
            />
          )}
        </main>

        <aside className="lg:sticky lg:top-6">
          <div className="flex flex-col gap-4">
            <Card>
              <CardHeader>
                <CardTitle className="text-base">{t("pages.projectContexts")}</CardTitle>
                <CardDescription>{t("pages.contextsDescription")}</CardDescription>
              </CardHeader>
              <CardContent>
                {contextFieldsLoading ? (
                  <p className="text-sm text-muted-foreground">{t("pages.loadingContexts")}</p>
                ) : contextFields.length === 0 ? (
                  <p className="text-sm text-muted-foreground">{t("pages.noContextsConfigured")}</p>
                ) : (
                  <div className="flex flex-col gap-3">
                    <div className="flex flex-col gap-1.5">
                      <label htmlFor="playground-context-field" className="text-xs font-medium">{t("pages.contextField")}</label>
                      <Select value={selectedContextKey} onValueChange={selectContext}>
                        <SelectTrigger id="playground-context-field">
                          <SelectValue placeholder={t("pages.selectContext")} />
                        </SelectTrigger>
                        <SelectContent>
                          {contextFields.map((field) => (
                            <SelectItem key={field.id} value={field.key}>{field.key}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    <div className="flex flex-col gap-1.5">
	                      <label htmlFor="playground-context-value" className="text-xs font-medium">{t("pages.contextValue")}</label>
                      <Select value={selectedContextValue} onValueChange={selectContextValue} disabled={!selectedContext || selectedContext.values.length === 0}>
                        <SelectTrigger id="playground-context-value">
                          <SelectValue placeholder={selectedContext ? selectedContext.values.length ? t("pages.selectValue") : t("pages.noPredefinedValues") : t("pages.selectContextFirst")} />
                        </SelectTrigger>
                        <SelectContent>
                          {selectedContext?.values.map((value) => (
                            <SelectItem key={value.value} value={value.value}>{value.value}</SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>

                    {selectedContext?.description && <p className="text-xs leading-5 text-muted-foreground">{selectedContext.description}</p>}
                  </div>
                )}
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">{t("pages.statusLegend")}</CardTitle>
                <CardDescription>{t("pages.statusLegendDescription")}</CardDescription>
              </CardHeader>
              <CardContent>
                <dl className="flex flex-col gap-3">
	                  {STATUS_LEGEND.map(({ status, labelKey, descriptionKey }) => (
                    <div key={status} className="flex flex-col gap-1">
	                      <dt><Badge variant="secondary">{t(labelKey)}</Badge></dt>
	                      <dd className="text-xs leading-5 text-muted-foreground">{t(descriptionKey)}</dd>
                    </div>
                  ))}
                </dl>
              </CardContent>
            </Card>
          </div>
        </aside>
      </div>
    </div>
  )
}
