import { useEffect, useState, type FormEvent } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Switch } from "@/components/ui/switch"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { MultiSelect } from "@/components/shared/multi-select"
import { api, ApiError } from "@/lib/api"
import { useEnvironment } from "@/lib/environment"

const OPERATORS = [
  "==",
  "!=",
  ">",
  "<",
  "in",
  "not in",
  "contains",
  "not contains",
  "semver>",
  "semver<",
  "semver=",
  "matches",
] as const
type Operator = (typeof OPERATORS)[number]

const OPERATOR_PLACEHOLDERS: Partial<Record<Operator, string>> = {
  in: "value (comma-separated for 'in')",
  "not in": "value (comma-separated)",
  matches: "regex pattern",
  "semver>": "1.2.3",
  "semver<": "1.2.3",
  "semver=": "1.2.3",
}

const COMBINATORS = ["and", "or"] as const
type Combinator = (typeof COMBINATORS)[number]

interface ContextField {
  key: string
  values: { value: string; description?: string }[]
}

interface VariantRow {
  key: string
  value: string
}

interface RolloutBucket {
  variant: string
  percentage: string
}

export interface ConditionRow {
  attribute: string
  operator: Operator
  value: string
  negate: boolean
}

export interface RuleRow {
  priority: number
  description: string
  combinator: Combinator
  conditions: ConditionRow[]
  variantKey: string
  rollout: RolloutBucket[]
}

// StrategyRow is the editor's flat state for one targeting strategy. The
// default (catch-all) strategy has no meaningful conditions/combinator — it
// always matches — but keeps the fields so it round-trips through the same
// condition helpers as every other strategy.
interface StrategyRow {
  name: string
  description: string
  isDefault: boolean
  combinator: Combinator
  conditions: ConditionRow[]
  defaultVariant: string
  rollout: RolloutBucket[]
  variants: VariantRow[]
}

interface StrategyDTO {
  order: number
  name: string
  description: string
  isDefault: boolean
  condition: unknown
  defaultVariant: string
  rollout?: unknown
  variants: { key: string; value: unknown }[]
}

interface FlagDTO {
  key: string
  name: string
  description: string
  flagType: string
  enabled: boolean
  strategies: StrategyDTO[]
  prerequisiteFlagKey?: string
  prerequisiteVariant?: string
}

interface FlagListItem {
  key: string
  strategies: { variants: { key: string }[] }[]
}

const NONE_PREREQUISITE = "__none__"

function parseVariantValue(flagType: string, raw: string): unknown {
  if (flagType === "boolean") return raw === "true"
  if (flagType === "number") return Number(raw)
  if (flagType === "object") return JSON.parse(raw)
  return raw
}

function stringifyVariantValue(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value)
}

/** Reconstructs a single {attribute, operator, value, negate} leaf from a JSONLogic comparison node. */
export function conditionToLeaf(condition: unknown): ConditionRow {
  if (condition && typeof condition === "object") {
    // A `{"!": <node>}` wrapper negates the inner comparison — unwrap it
    // and mark negate, rather than treating "!" as its own operator (it's
    // unary, unlike every entry in OPERATORS).
    const notArg = (condition as Record<string, unknown>)["!"]
    if (notArg !== undefined) {
      const inner = Array.isArray(notArg) && notArg.length === 1 ? notArg[0] : notArg
      return { ...conditionToLeaf(inner), negate: true }
    }
    for (const op of OPERATORS) {
      const args = (condition as Record<string, unknown>)[op]
      if (Array.isArray(args) && args.length === 2) {
        const [left, right] = args
        const attribute = (left as { var?: string })?.var ?? ""
        const value = Array.isArray(right) ? right.join(",") : String(right)
        return { attribute, operator: op, value, negate: false }
      }
    }
  }
  return { attribute: "", operator: "==", value: "", negate: false }
}

/**
 * Reconstructs {combinator, conditions[]} from a JSONLogic condition. Supports a single
 * leaf comparison, or a top-level and/or of leaf comparisons — anything nested deeper
 * (e.g. and-of-or) falls back to a single blank leaf, since this editor only exposes one
 * combinator level.
 */
export function conditionToRule(condition: unknown): { combinator: Combinator; conditions: ConditionRow[] } {
  if (condition && typeof condition === "object") {
    for (const combinator of COMBINATORS) {
      const args = (condition as Record<string, unknown>)[combinator]
      if (Array.isArray(args) && args.length > 0) {
        return { combinator, conditions: args.map(conditionToLeaf) }
      }
    }
  }
  return { combinator: "and", conditions: [conditionToLeaf(condition)] }
}

export function leafToCondition(leaf: ConditionRow): unknown {
  const value: unknown =
    leaf.operator === "in" || leaf.operator === "not in" ? leaf.value.split(",").map((s) => s.trim()) : leaf.value
  const node = { [leaf.operator]: [{ var: leaf.attribute }, value] }
  return leaf.negate ? { "!": node } : node
}

export function ruleToCondition(rule: RuleRow): unknown {
  if (rule.conditions.length <= 1) {
    return leafToCondition(rule.conditions[0] ?? { attribute: "", operator: "==", value: "", negate: false })
  }
  return { [rule.combinator]: rule.conditions.map(leafToCondition) }
}

function parseStrategy(s: StrategyDTO): StrategyRow {
  const { combinator, conditions } = conditionToRule(s.condition)
  const rollout = Array.isArray(s.rollout)
    ? (s.rollout as { variant: string; percentage: number }[]).map((b) => ({ variant: b.variant, percentage: String(b.percentage) }))
    : []
  return {
    name: s.name,
    description: s.description,
    isDefault: s.isDefault,
    combinator,
    conditions,
    defaultVariant: s.defaultVariant,
    rollout,
    variants: s.variants.map((v) => ({ key: v.key, value: stringifyVariantValue(v.value) })),
  }
}

function newStrategy(seedVariantKeys: string[]): StrategyRow {
  return {
    name: "",
    description: "",
    isDefault: false,
    combinator: "and",
    conditions: [{ attribute: "", operator: "==", value: "", negate: false }],
    defaultVariant: "",
    rollout: [],
    variants: seedVariantKeys.length ? seedVariantKeys.map((key) => ({ key, value: "" })) : [{ key: "", value: "" }],
  }
}

export function FlagEditorPage() {
  const { projectId, key } = useParams<{ projectId: string; key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [error, setError] = useState<string | null>(null)
  const { envKey, envPath } = useEnvironment()

  const { data: flag, isLoading } = useQuery({
    queryKey: ["flag", projectId, envKey, key],
    queryFn: () => api.get<FlagDTO>(`${envPath("flags")}/${key}`),
    enabled: !!projectId && !!envKey && !!key,
  })

  const { data: allFlags } = useQuery({
    queryKey: ["flags", projectId, envKey],
    queryFn: () => api.get<FlagListItem[]>(envPath("flags")),
    enabled: !!projectId && !!envKey,
  })
  const { data: contextFields = [] } = useQuery({
    queryKey: ["context-fields", projectId],
    queryFn: () => api.get<ContextField[]>(`/projects/${projectId}/context-fields`),
    enabled: !!projectId,
  })
  const prerequisiteCandidates = (allFlags ?? []).filter((f) => f.key !== key)

  const [name, setName] = useState("")
  const [enabled, setEnabled] = useState(true)
  const [strategies, setStrategies] = useState<StrategyRow[]>([])
  const [prerequisiteFlagKey, setPrerequisiteFlagKey] = useState("")
  const [prerequisiteVariant, setPrerequisiteVariant] = useState("")

  useEffect(() => {
    if (!flag) return
    setName(flag.name)
    setEnabled(flag.enabled)
    setPrerequisiteFlagKey(flag.prerequisiteFlagKey ?? "")
    setPrerequisiteVariant(flag.prerequisiteVariant ?? "")
    setStrategies([...flag.strategies].sort((a, b) => a.order - b.order).map(parseStrategy))
  }, [flag])

  const save = useMutation({
    mutationFn: () => {
      if (!flag) throw new Error("flag not loaded")
      return api.patch(`${envPath("flags")}/${key}`, {
        name,
        enabled,
        prerequisiteFlagKey,
        prerequisiteVariant: prerequisiteFlagKey ? prerequisiteVariant : "",
        strategies: strategies.map((s, index) => ({
          order: index,
          name: s.name,
          description: s.description,
          isDefault: s.isDefault,
          condition: s.isDefault ? undefined : ruleToCondition({ priority: 0, description: "", combinator: s.combinator, conditions: s.conditions, variantKey: "", rollout: [] }),
          defaultVariant: s.defaultVariant,
          rollout: s.rollout.length ? s.rollout.map((b) => ({ variant: b.variant, percentage: Number(b.percentage) })) : undefined,
          variants: s.variants.map((v) => ({ key: v.key, value: parseVariantValue(flag.flagType, v.value) })),
        })),
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["flags", projectId, envKey] })
      queryClient.invalidateQueries({ queryKey: ["flag", projectId, envKey, key] })
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to save flag"),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    save.mutate()
  }

  function updateStrategy(i: number, patch: Partial<StrategyRow>) {
    setStrategies((prev) => prev.map((s, idx) => (idx === i ? { ...s, ...patch } : s)))
  }

  function addStrategy() {
    const defaultVariants = strategies.find((s) => s.isDefault)?.variants.map((v) => v.key).filter(Boolean) ?? []
    setStrategies((prev) => [...prev, newStrategy(defaultVariants)])
  }

  function removeStrategy(i: number) {
    setStrategies((prev) => prev.filter((_, idx) => idx !== i))
  }

  function makeDefault(i: number) {
    setStrategies((prev) => prev.map((s, idx) => ({ ...s, isDefault: idx === i })))
  }

  function updateVariant(strategyIndex: number, i: number, field: keyof VariantRow, value: string) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex ? { ...s, variants: s.variants.map((v, vi) => (vi === i ? { ...v, [field]: value } : v)) } : s,
      ),
    )
  }

  function addVariant(strategyIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) => (idx === strategyIndex ? { ...s, variants: [...s.variants, { key: "", value: "" }] } : s)),
    )
  }

  function removeVariant(strategyIndex: number, i: number) {
    setStrategies((prev) =>
      prev.map((s, idx) => (idx === strategyIndex ? { ...s, variants: s.variants.filter((_, vi) => vi !== i) } : s)),
    )
  }

  function addCondition(strategyIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex ? { ...s, conditions: [...s.conditions, { attribute: "", operator: "==", value: "", negate: false }] } : s,
      ),
    )
  }

  function updateCondition(strategyIndex: number, condIndex: number, patch: Partial<ConditionRow>) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? { ...s, conditions: s.conditions.map((c, ci) => (ci === condIndex ? { ...c, ...patch } : c)) }
          : s,
      ),
    )
  }

  function selectAttribute(strategyIndex: number, condIndex: number, attribute: string) {
    updateCondition(strategyIndex, condIndex, { attribute, value: "", operator: "==" })
  }

  function removeCondition(strategyIndex: number, condIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) => (idx === strategyIndex ? { ...s, conditions: s.conditions.filter((_, ci) => ci !== condIndex) } : s)),
    )
  }

  function addRolloutBucket(strategyIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) => (idx === strategyIndex ? { ...s, rollout: [...s.rollout, { variant: "", percentage: "" }] } : s)),
    )
  }

  function updateRolloutBucket(strategyIndex: number, bucketIndex: number, field: keyof RolloutBucket, value: string) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? { ...s, rollout: s.rollout.map((b, bi) => (bi === bucketIndex ? { ...b, [field]: value } : b)) }
          : s,
      ),
    )
  }

  function removeRolloutBucket(strategyIndex: number, bucketIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) => (idx === strategyIndex ? { ...s, rollout: s.rollout.filter((_, bi) => bi !== bucketIndex) } : s)),
    )
  }

  if (isLoading || !flag) {
    return <div className="p-8 text-muted-foreground">Loading...</div>
  }

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold">
          <code>{flag.key}</code>
        </h1>
        <Button variant="outline" onClick={() => navigate(`/projects/${projectId}/flags`)}>
          Back to flags
        </Button>
      </div>

      <form className="flex flex-col gap-6" onSubmit={handleSubmit}>
        <Card>
          <CardHeader>
            <CardTitle>Details</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div className="flex items-center gap-3">
              <Switch checked={enabled} onCheckedChange={setEnabled} />
              <Label>{enabled ? "Enabled" : "Disabled (kill switch — always serves the default strategy)"}</Label>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="flag-name">Name</Label>
              <Input id="flag-name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2 max-w-xs">
              <Label>Prerequisite flag (optional — this flag only serves non-default when the prerequisite resolves to the chosen variant)</Label>
              <Select
                value={prerequisiteFlagKey || NONE_PREREQUISITE}
                onValueChange={(v) => {
                  setPrerequisiteFlagKey(v === NONE_PREREQUISITE ? "" : v)
                  setPrerequisiteVariant("")
                }}
              >
                <SelectTrigger>
                  <SelectValue placeholder="none" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE_PREREQUISITE}>None</SelectItem>
                  {prerequisiteCandidates.map((f) => (
                    <SelectItem key={f.key} value={f.key}>
                      {f.key}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            {prerequisiteFlagKey && (
              <div className="flex flex-col gap-2 max-w-xs">
                <Label>Required prerequisite variant</Label>
                <Select value={prerequisiteVariant} onValueChange={setPrerequisiteVariant}>
                  <SelectTrigger>
                    <SelectValue placeholder="variant..." />
                  </SelectTrigger>
                  <SelectContent>
                    {Array.from(
                      new Set(
                        (prerequisiteCandidates.find((f) => f.key === prerequisiteFlagKey)?.strategies ?? []).flatMap((s) =>
                          s.variants.map((v) => v.key),
                        ),
                      ),
                    ).map((k) => (
                      <SelectItem key={k} value={k}>
                        {k}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Strategies</CardTitle>
            <Button type="button" variant="ghost" size="sm" onClick={addStrategy}>
              Add strategy
            </Button>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <p className="text-sm text-muted-foreground">
              Each strategy has its own targeting, rollout, and variant catalog. The first strategy (in order below)
              whose targeting matches the context wins. The default strategy always matches and is evaluated last —
              it's the flag's guaranteed fallback.
            </p>
            {strategies.map((s, i) => (
              <div key={i} className="flex flex-col gap-3 rounded-md border p-3">
                <div className="flex items-center gap-2">
                  <Input
                    placeholder="strategy name"
                    value={s.name}
                    onChange={(e) => updateStrategy(i, { name: e.target.value })}
                    className="flex-1"
                  />
                  {s.isDefault ? (
                    <Badge>Default</Badge>
                  ) : (
                    <Button type="button" variant="ghost" size="sm" onClick={() => makeDefault(i)}>
                      Make default
                    </Button>
                  )}
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => removeStrategy(i)}
                    disabled={s.isDefault || strategies.length <= 1}
                  >
                    Remove strategy
                  </Button>
                </div>
                <Input
                  placeholder="description"
                  value={s.description}
                  onChange={(e) => updateStrategy(i, { description: e.target.value })}
                />

                <div className="flex flex-col gap-2">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs text-muted-foreground">Variants</Label>
                    <Button type="button" variant="ghost" size="sm" onClick={() => addVariant(i)}>
                      Add variant
                    </Button>
                  </div>
                  {s.variants.map((v, vi) => (
                    <div key={vi} className="flex items-center gap-2">
                      <Input placeholder="key" value={v.key} onChange={(e) => updateVariant(i, vi, "key", e.target.value)} className="w-32" />
                      <Input placeholder="value" value={v.value} onChange={(e) => updateVariant(i, vi, "value", e.target.value)} />
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => removeVariant(i, vi)}
                        disabled={s.variants.length <= 1}
                      >
                        Remove
                      </Button>
                    </div>
                  ))}
                  <div className="flex flex-col gap-2 max-w-xs">
                    <Label className="text-xs text-muted-foreground">Default variant (served on match without a rollout hit)</Label>
                    <Select key={s.variants.length ? "ready" : "loading"} value={s.defaultVariant} onValueChange={(v) => updateStrategy(i, { defaultVariant: v })}>
                      <SelectTrigger>
                        <SelectValue placeholder="variant..." />
                      </SelectTrigger>
                      <SelectContent>
                        {s.variants.filter((v) => v.key).map((v) => (
                          <SelectItem key={v.key} value={v.key}>
                            {v.key}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>
                </div>

                <Separator />

                {!s.isDefault && (
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center justify-between">
                      <Label className="text-xs text-muted-foreground">Targeting (all must reference the evaluation context)</Label>
                      <Button type="button" variant="ghost" size="sm" onClick={() => addCondition(i)}>
                        Add condition
                      </Button>
                    </div>
                    {s.conditions.map((c, ci) => (
                      <div key={ci} className="flex items-center gap-2">
                        {ci === 0 ? (
                          <span className="w-16 text-xs text-muted-foreground">where</span>
                        ) : (
                          <Select value={s.combinator} onValueChange={(v) => updateStrategy(i, { combinator: v as Combinator })}>
                            <SelectTrigger className="w-16">
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                              {COMBINATORS.map((c) => (
                                <SelectItem key={c} value={c}>
                                  {c}
                                </SelectItem>
                              ))}
                            </SelectContent>
                          </Select>
                        )}
                        <Input
                          placeholder="attribute (e.g. plan)"
                          value={c.attribute}
                          onChange={(e) => selectAttribute(i, ci, e.target.value)}
                          className="w-40"
                          list="project-context-fields"
                        />
                        <div className="flex items-center gap-1" title="Negate this condition">
                          <Switch checked={c.negate} onCheckedChange={(v) => updateCondition(i, ci, { negate: v })} />
                          <Label className="text-xs text-muted-foreground">not</Label>
                        </div>
                        <Select value={c.operator} onValueChange={(v) => updateCondition(i, ci, { operator: v as Operator })}>
                          <SelectTrigger className="w-28">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {OPERATORS.map((op) => (
                              <SelectItem key={op} value={op}>
                                {op}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        {(() => {
                          const field = contextFields.find((candidate) => candidate.key === c.attribute)
                          if (field && field.values.length > 0) {
                            if (c.operator === "in" || c.operator === "not in") {
                              return (
                                <MultiSelect
                                  options={field.values.map((value) => ({ value: value.value, label: value.value }))}
                                  selected={c.value ? c.value.split(",").filter(Boolean) : []}
                                  onChange={(values) => updateCondition(i, ci, { value: values.join(",") })}
                                  placeholder="Select values"
                                />
                              )
                            }
                            return (
                              <Select value={c.value} onValueChange={(value) => updateCondition(i, ci, { value })}>
                                <SelectTrigger><SelectValue placeholder="Select value" /></SelectTrigger>
                                <SelectContent>{field.values.map((value) => <SelectItem key={value.value} value={value.value}>{value.value}</SelectItem>)}</SelectContent>
                              </Select>
                            )
                          }
                          return <Input placeholder={OPERATOR_PLACEHOLDERS[c.operator] ?? "value"} value={c.value} onChange={(e) => updateCondition(i, ci, { value: e.target.value })} />
                        })()}
                        <Button
                          type="button"
                          variant="ghost"
                          size="sm"
                          onClick={() => removeCondition(i, ci)}
                          disabled={s.conditions.length <= 1}
                        >
                          Remove
                        </Button>
                      </div>
                    ))}
                    <Separator />
                  </div>
                )}

                <div className="flex flex-col gap-2">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs text-muted-foreground">Rollout % (optional — splits matched traffic across this strategy's variants)</Label>
                    <Button type="button" variant="ghost" size="sm" onClick={() => addRolloutBucket(i)}>
                      Add bucket
                    </Button>
                  </div>
                  {s.rollout.map((b, bi) => (
                    <div key={bi} className="flex items-center gap-2">
                      <Select value={b.variant} onValueChange={(v) => updateRolloutBucket(i, bi, "variant", v)}>
                        <SelectTrigger className="w-32">
                          <SelectValue placeholder="variant" />
                        </SelectTrigger>
                        <SelectContent>
                          {s.variants.filter((v) => v.key).map((v) => (
                            <SelectItem key={v.key} value={v.key}>
                              {v.key}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <Input
                        type="number"
                        placeholder="%"
                        value={b.percentage}
                        onChange={(e) => updateRolloutBucket(i, bi, "percentage", e.target.value)}
                        className="w-24"
                      />
                      <Button type="button" variant="ghost" size="sm" onClick={() => removeRolloutBucket(i, bi)}>
                        Remove
                      </Button>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        {error && <p className="text-sm text-destructive">{error}</p>}
        <div>
          <Button type="submit" disabled={save.isPending}>
            Save changes
          </Button>
        </div>
      </form>
      <datalist id="project-context-fields">
        {contextFields.map((field) => <option key={field.key} value={field.key}>{field.key}</option>)}
      </datalist>
    </div>
  )
}
