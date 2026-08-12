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
import { api, ApiError } from "@/lib/api"

const OPERATORS = ["==", "!=", ">", "<", "in", "contains"] as const
type Operator = (typeof OPERATORS)[number]

const COMBINATORS = ["and", "or"] as const
type Combinator = (typeof COMBINATORS)[number]

interface VariantRow {
  key: string
  value: string
}

interface RolloutBucket {
  variant: string
  percentage: string
}

interface ConditionRow {
  attribute: string
  operator: Operator
  value: string
}

interface RuleRow {
  priority: number
  description: string
  combinator: Combinator
  conditions: ConditionRow[]
  variantKey: string
  rollout: RolloutBucket[]
}

interface FlagDTO {
  key: string
  name: string
  description: string
  flagType: string
  enabled: boolean
  defaultVariant: string
  variants: { key: string; value: unknown }[]
  rules: { priority: number; description: string; condition: unknown; variantKey: string; rollout?: unknown }[]
}

function parseVariantValue(flagType: string, raw: string): unknown {
  if (flagType === "boolean") return raw === "true"
  if (flagType === "number") return Number(raw)
  if (flagType === "object") return JSON.parse(raw)
  return raw
}

function stringifyVariantValue(value: unknown): string {
  return typeof value === "string" ? value : JSON.stringify(value)
}

/** Reconstructs a single {attribute, operator, value} leaf from a JSONLogic comparison node. */
function conditionToLeaf(condition: unknown): ConditionRow {
  if (condition && typeof condition === "object") {
    for (const op of OPERATORS) {
      const args = (condition as Record<string, unknown>)[op]
      if (Array.isArray(args) && args.length === 2) {
        const [left, right] = args
        const attribute = (left as { var?: string })?.var ?? ""
        const value = Array.isArray(right) ? right.join(",") : String(right)
        return { attribute, operator: op, value }
      }
    }
  }
  return { attribute: "", operator: "==", value: "" }
}

/**
 * Reconstructs {combinator, conditions[]} from a JSONLogic condition. Supports a single
 * leaf comparison, or a top-level and/or of leaf comparisons — anything nested deeper
 * (e.g. and-of-or) falls back to a single blank leaf, since this editor only exposes one
 * combinator level.
 */
function conditionToRule(condition: unknown): { combinator: Combinator; conditions: ConditionRow[] } {
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

function leafToCondition(leaf: ConditionRow): unknown {
  const value: unknown = leaf.operator === "in" ? leaf.value.split(",").map((s) => s.trim()) : leaf.value
  return { [leaf.operator]: [{ var: leaf.attribute }, value] }
}

function ruleToCondition(rule: RuleRow): unknown {
  if (rule.conditions.length <= 1) {
    return leafToCondition(rule.conditions[0] ?? { attribute: "", operator: "==", value: "" })
  }
  return { [rule.combinator]: rule.conditions.map(leafToCondition) }
}

export function FlagEditorPage() {
  const { projectId, key } = useParams<{ projectId: string; key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [error, setError] = useState<string | null>(null)

  const { data: flag, isLoading } = useQuery({
    queryKey: ["flag", projectId, key],
    queryFn: () => api.get<FlagDTO>(`/projects/${projectId}/flags/${key}`),
    enabled: !!projectId && !!key,
  })

  const [name, setName] = useState("")
  const [enabled, setEnabled] = useState(true)
  const [defaultVariant, setDefaultVariant] = useState("")
  const [variants, setVariants] = useState<VariantRow[]>([])
  const [rules, setRules] = useState<RuleRow[]>([])

  useEffect(() => {
    if (!flag) return
    setName(flag.name)
    setEnabled(flag.enabled)
    setDefaultVariant(flag.defaultVariant)
    setVariants(flag.variants.map((v) => ({ key: v.key, value: stringifyVariantValue(v.value) })))
    setRules(
      [...flag.rules]
        .sort((a, b) => a.priority - b.priority)
        .map((r) => {
          const { combinator, conditions } = conditionToRule(r.condition)
          const rollout = Array.isArray(r.rollout)
            ? (r.rollout as { variant: string; percentage: number }[]).map((b) => ({
                variant: b.variant,
                percentage: String(b.percentage),
              }))
            : []
          return { priority: r.priority, description: r.description, combinator, conditions, variantKey: r.variantKey, rollout }
        }),
    )
  }, [flag])

  const save = useMutation({
    mutationFn: () => {
      if (!flag) throw new Error("flag not loaded")
      return api.patch(`/projects/${projectId}/flags/${key}`, {
        name,
        enabled,
        defaultVariant,
        variants: variants.map((v) => ({ key: v.key, value: parseVariantValue(flag.flagType, v.value) })),
        rules: rules.map((r) => ({
          priority: r.priority,
          description: r.description,
          condition: ruleToCondition(r),
          variantKey: r.variantKey,
          rollout: r.rollout.length
            ? r.rollout.map((b) => ({ variant: b.variant, percentage: Number(b.percentage) }))
            : undefined,
        })),
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["flags", projectId] })
      queryClient.invalidateQueries({ queryKey: ["flag", projectId, key] })
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Failed to save flag"),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError(null)
    save.mutate()
  }

  function updateVariant(i: number, field: keyof VariantRow, value: string) {
    setVariants((prev) => prev.map((v, idx) => (idx === i ? { ...v, [field]: value } : v)))
  }

  function addRule() {
    setRules((prev) => [
      ...prev,
      {
        priority: prev.length + 1,
        description: "",
        combinator: "and",
        conditions: [{ attribute: "", operator: "==", value: "" }],
        variantKey: defaultVariant,
        rollout: [],
      },
    ])
  }

  function updateRule(i: number, patch: Partial<RuleRow>) {
    setRules((prev) => prev.map((r, idx) => (idx === i ? { ...r, ...patch } : r)))
  }

  function removeRule(i: number) {
    setRules((prev) => prev.filter((_, idx) => idx !== i))
  }

  function addCondition(ruleIndex: number) {
    setRules((prev) =>
      prev.map((r, idx) =>
        idx === ruleIndex ? { ...r, conditions: [...r.conditions, { attribute: "", operator: "==", value: "" }] } : r,
      ),
    )
  }

  function updateCondition(ruleIndex: number, condIndex: number, patch: Partial<ConditionRow>) {
    setRules((prev) =>
      prev.map((r, idx) =>
        idx === ruleIndex
          ? { ...r, conditions: r.conditions.map((c, ci) => (ci === condIndex ? { ...c, ...patch } : c)) }
          : r,
      ),
    )
  }

  function removeCondition(ruleIndex: number, condIndex: number) {
    setRules((prev) =>
      prev.map((r, idx) => (idx === ruleIndex ? { ...r, conditions: r.conditions.filter((_, ci) => ci !== condIndex) } : r)),
    )
  }

  function addRolloutBucket(ruleIndex: number) {
    setRules((prev) =>
      prev.map((r, idx) => (idx === ruleIndex ? { ...r, rollout: [...r.rollout, { variant: "", percentage: "" }] } : r)),
    )
  }

  function updateRolloutBucket(ruleIndex: number, bucketIndex: number, field: keyof RolloutBucket, value: string) {
    setRules((prev) =>
      prev.map((r, idx) =>
        idx === ruleIndex
          ? { ...r, rollout: r.rollout.map((b, bi) => (bi === bucketIndex ? { ...b, [field]: value } : b)) }
          : r,
      ),
    )
  }

  function removeRolloutBucket(ruleIndex: number, bucketIndex: number) {
    setRules((prev) =>
      prev.map((r, idx) => (idx === ruleIndex ? { ...r, rollout: r.rollout.filter((_, bi) => bi !== bucketIndex) } : r)),
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
              <Label>{enabled ? "Enabled" : "Disabled (kill switch — always serves default)"}</Label>
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="flag-name">Name</Label>
              <Input id="flag-name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
            <div className="flex flex-col gap-2 max-w-xs">
              <Label>Default variant</Label>
              {/* Keyed on variants readiness: Radix Select only registers an item's
                  display label once it has mounted, so remount once real data
                  arrives instead of the empty initial state, or the trigger renders blank. */}
              <Select key={variants.length ? "ready" : "loading"} value={defaultVariant} onValueChange={setDefaultVariant}>
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
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Variants</CardTitle>
            <Button type="button" variant="ghost" size="sm" onClick={() => setVariants((prev) => [...prev, { key: "", value: "" }])}>
              Add variant
            </Button>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {variants.map((v, i) => (
              <div key={i} className="flex items-center gap-2">
                <Input placeholder="key" value={v.key} onChange={(e) => updateVariant(i, "key", e.target.value)} className="w-32" />
                <Input placeholder="value" value={v.value} onChange={(e) => updateVariant(i, "value", e.target.value)} />
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => setVariants((prev) => prev.filter((_, idx) => idx !== i))}
                  disabled={variants.length <= 1}
                >
                  Remove
                </Button>
              </div>
            ))}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>Targeting rules</CardTitle>
            <Button type="button" variant="ghost" size="sm" onClick={addRule}>
              Add rule
            </Button>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {rules.length === 0 && <p className="text-sm text-muted-foreground">No rules — always serves the default variant.</p>}
            {rules.map((r, i) => (
              <div key={i} className="flex flex-col gap-3 rounded-md border p-3">
                <div className="flex items-center gap-2">
                  <Input
                    type="number"
                    value={r.priority}
                    onChange={(e) => updateRule(i, { priority: Number(e.target.value) })}
                    className="w-20"
                    title="Priority (lower evaluated first)"
                  />
                  <Input
                    placeholder="rule description"
                    value={r.description}
                    onChange={(e) => updateRule(i, { description: e.target.value })}
                    className="flex-1"
                  />
                  <Select value={r.variantKey} onValueChange={(v) => updateRule(i, { variantKey: v })}>
                    <SelectTrigger className="w-32">
                      <SelectValue placeholder="serve..." />
                    </SelectTrigger>
                    <SelectContent>
                      {variants.filter((v) => v.key).map((v) => (
                        <SelectItem key={v.key} value={v.key}>
                          {v.key}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <Button type="button" variant="ghost" size="sm" onClick={() => removeRule(i)}>
                    Remove rule
                  </Button>
                </div>

                <div className="flex flex-col gap-2">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs text-muted-foreground">Conditions (all must reference the evaluation context)</Label>
                    <Button type="button" variant="ghost" size="sm" onClick={() => addCondition(i)}>
                      Add condition
                    </Button>
                  </div>
                  {r.conditions.map((c, ci) => (
                    <div key={ci} className="flex items-center gap-2">
                      {ci === 0 ? (
                        <span className="w-16 text-xs text-muted-foreground">where</span>
                      ) : (
                        <Select
                          value={r.combinator}
                          onValueChange={(v) => updateRule(i, { combinator: v as Combinator })}
                        >
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
                        onChange={(e) => updateCondition(i, ci, { attribute: e.target.value })}
                        className="w-40"
                      />
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
                      <Input
                        placeholder="value (comma-separated for 'in')"
                        value={c.value}
                        onChange={(e) => updateCondition(i, ci, { value: e.target.value })}
                      />
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => removeCondition(i, ci)}
                        disabled={r.conditions.length <= 1}
                      >
                        Remove
                      </Button>
                    </div>
                  ))}
                </div>

                <Separator />

                <div className="flex flex-col gap-2">
                  <div className="flex items-center justify-between">
                    <Label className="text-xs text-muted-foreground">Rollout % (optional — splits matched traffic across variants)</Label>
                    <Button type="button" variant="ghost" size="sm" onClick={() => addRolloutBucket(i)}>
                      Add bucket
                    </Button>
                  </div>
                  {r.rollout.map((b, bi) => (
                    <div key={bi} className="flex items-center gap-2">
                      <Select value={b.variant} onValueChange={(v) => updateRolloutBucket(i, bi, "variant", v)}>
                        <SelectTrigger className="w-32">
                          <SelectValue placeholder="variant" />
                        </SelectTrigger>
                        <SelectContent>
                          {variants.filter((v) => v.key).map((v) => (
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
    </div>
  )
}
