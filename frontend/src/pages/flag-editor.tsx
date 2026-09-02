import { useEffect, useState, type FormEvent } from "react"
import { useNavigate, useParams } from "react-router-dom"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Button } from "@/components/ui/button"
import { ActionButton } from "@/components/ui/action-button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Switch } from "@/components/ui/switch"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { Slider } from "@/components/ui/slider"
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { MultiSelect } from "@/components/shared/multi-select"
import { AttributeInput } from "@/components/shared/attribute-input"
import { api } from "@/lib/api"
import { alerts } from "@/lib/alerts"
import { useEnvironment } from "@/lib/environment"
import { ArrowDown, ArrowUp, EllipsisVertical, ListPlus, Plus, Trash2, X } from "lucide-react"
import { useTranslation } from "react-i18next"

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
	in: "flags.inValuePlaceholder",
	"not in": "flags.inValuePlaceholder",
	matches: "flags.regexPlaceholder",
	"semver>": "flags.versionPlaceholder",
	"semver<": "flags.versionPlaceholder",
	"semver=": "flags.versionPlaceholder",
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
  /** The logical operator that joins this item to the preceding top-level item. */
  connector?: Combinator
}

/** A nested AND/OR group. Groups can contain leaves, but not other groups. */
export interface ConditionGroup {
  kind: "group"
  combinator: Combinator
  conditions: ConditionRow[]
  /** The logical operator that joins this group to the preceding top-level item. */
  connector?: Combinator
}

export type TargetingItem = ConditionRow | ConditionGroup

export interface RuleRow {
  priority: number
  description: string
  combinator: Combinator
  conditions: TargetingItem[]
  variantKey: string
  rollout: RolloutBucket[]
}

// StrategyRow is the editor's flat state for one targeting strategy. The
// default (catch-all) strategy has no meaningful conditions/combinator — it
// always matches — but keeps the fields so it round-trips through the same
// condition helpers as every other strategy.
interface StrategyRow {
  name: string
  combinator: Combinator
  conditions: TargetingItem[]
  defaultVariant: string
  rollout: RolloutBucket[]
	variants: VariantRow[]
	rawCondition?: unknown
}

interface StrategyDTO {
  order: number
  name: string
  condition: unknown
  defaultVariant: string
  rollout?: unknown
  variants: { key: string; value: unknown }[]
}

interface FlagDTO {
  key: string
  name: string
  description: string
	tags: string[]
  flagType: string
  enabled: boolean
	createdAt: string
	createdBy?: FlagUser
	collaborators: FlagUser[]
  strategies: StrategyDTO[]
  prerequisiteFlagKey?: string
  prerequisiteVariant?: string
}

interface FlagUser {
	id: string
	name: string
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

function tagsFromText(value: string): string[] {
	const tags = value.split(",").map((tag) => tag.trim()).filter(Boolean)
	return [...new Set(tags)]
}

function formatCreatedAt(value: string, locale: string): string {
	const date = new Date(value)
	return Number.isNaN(date.getTime()) ? "—" : new Intl.DateTimeFormat(locale).format(date)
}

function clampPercentage(value: string | number): number {
  const percentage = Number(value)
  if (!Number.isFinite(percentage)) return 0
  return Math.min(100, Math.max(0, percentage))
}

type StrategyType = "standard" | "gradual"

function strategyTypeOf(strategy: StrategyRow): StrategyType {
  return strategy.rollout.length > 0 ? "gradual" : "standard"
}

function booleanVariantKey(variants: VariantRow[], value: boolean): string {
  return variants.find((variant) => variant.value === String(value))?.key ?? ""
}

function starterVariants(flagType: string): VariantRow[] {
  switch (flagType) {
    case "boolean":
      return [{ key: "on", value: "true" }, { key: "off", value: "false" }]
    case "number":
      return [{ key: "default", value: "0" }]
    case "object":
      return [{ key: "default", value: "{}" }]
    default:
      return [{ key: "default", value: "" }]
  }
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

function isCombinatorGroup(condition: unknown): condition is Record<Combinator, unknown[]> {
  if (!condition || typeof condition !== "object") return false
  return COMBINATORS.some((combinator) => Array.isArray((condition as Record<string, unknown>)[combinator]))
}

function isEditableLeaf(condition: unknown): boolean {
	let leaf = condition
	let negationCount = 0
	while (leaf && typeof leaf === "object" && "!" in leaf) {
		const wrapper = leaf as Record<string, unknown>
		if (Object.keys(wrapper).length !== 1 || ++negationCount > 1) return false
		const argument = wrapper["!"]
		leaf = Array.isArray(argument) ? argument.length === 1 ? argument[0] : undefined : argument
	}
	if (!leaf || typeof leaf !== "object") return false

	const node = leaf as Record<string, unknown>
	for (const operator of OPERATORS) {
		if (!(operator in node) || Object.keys(node).length !== 1) continue
		const args = node[operator]
		if (!Array.isArray(args) || args.length !== 2) return false
		const [left, right] = args
		if (!left || typeof left !== "object" || Array.isArray(left) || Object.keys(left).length !== 1 || typeof (left as { var?: unknown }).var !== "string") return false
		if (operator === "in" || operator === "not in") {
			return Array.isArray(right) && right.length > 0 && right.every((value) => typeof value === "string" && value.trim() === value && !value.includes(","))
		}
		return typeof right === "string"
	}
	return false
}

export function isEditableCondition(condition: unknown): boolean {
	if (isEditableLeaf(condition)) return true
	if (!isCombinatorGroup(condition)) return false
	const combinator = COMBINATORS.find((candidate) => Array.isArray(condition[candidate]))!
	const items = condition[combinator]
	if (Object.keys(condition).length !== 1 || items.length === 0) return false
	let nestedGroups = 0
	return items.every((item) => {
		if (isEditableLeaf(item)) return true
		if (!isCombinatorGroup(item) || Object.keys(item).length !== 1 || ++nestedGroups > 1) return false
		const nestedCombinator = COMBINATORS.find((candidate) => Array.isArray(item[candidate]))!
		return item[nestedCombinator].length > 0 && item[nestedCombinator].every(isEditableLeaf)
	})
}

function conditionToItem(condition: unknown): TargetingItem {
  if (isCombinatorGroup(condition)) {
    const combinator = COMBINATORS.find((candidate) => Array.isArray(condition[candidate]))!
    const conditions = condition[combinator].map(conditionToLeaf)
    return { kind: "group", combinator, conditions: conditions.length ? conditions : [{ attribute: "", operator: "==", value: "", negate: false }] }
  }
  return conditionToLeaf(condition)
}

function withConnector(item: TargetingItem, connector?: Combinator): TargetingItem {
  return connector ? { ...item, connector } : item
}

/**
 * Reconstructs a rule from JSONLogic. The editor exposes a top-level AND/OR
 * plus one optional grouped AND/OR level, such as `A and (B or C)`.
 */
export function conditionToRule(condition: unknown): { combinator: Combinator; conditions: TargetingItem[] } {
  if (condition && typeof condition === "object") {
    for (const combinator of COMBINATORS) {
      const args = (condition as Record<string, unknown>)[combinator]
      if (Array.isArray(args) && args.length > 0) {
        const conditions: TargetingItem[] = []

        for (const item of args) {
          // ruleToCondition represents top-level OR logic as OR-separated
          // AND segments. Flatten those segments back into their individual
          // rows so reopening the editor keeps every line's own connector.
          const nestedAnd = isCombinatorGroup(item) ? item.and : undefined
          if (combinator === "or" && Array.isArray(nestedAnd)) {
            nestedAnd.forEach((nestedItem, index) => {
              conditions.push(withConnector(conditionToItem(nestedItem), conditions.length === 0 ? undefined : index === 0 ? "or" : "and"))
            })
            continue
          }

          // An AND inside an AND is equivalent to the surrounding group.
          // Flattening it prevents the UI from creating a third visible level.
          if (combinator === "and" && Array.isArray(nestedAnd)) {
            nestedAnd.forEach((nestedItem) => conditions.push(withConnector(conditionToItem(nestedItem), conditions.length === 0 ? undefined : "and")))
            continue
          }

          conditions.push(withConnector(conditionToItem(item), conditions.length === 0 ? undefined : combinator))
        }

        return { combinator, conditions }
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

function isConditionGroup(item: TargetingItem): item is ConditionGroup {
  return "kind" in item && item.kind === "group"
}

function itemToCondition(item: TargetingItem): unknown {
  if (isConditionGroup(item)) {
    return { [item.combinator]: item.conditions.map(leafToCondition) }
  }
  return leafToCondition(item)
}

export function ruleToCondition(rule: RuleRow): unknown {
  const [first, ...rest] = rule.conditions
  if (!first) {
    return leafToCondition({ attribute: "", operator: "==", value: "", negate: false })
  }

  // Consecutive AND conditions form a segment; OR starts a new segment.
  // This keeps individual line connectors independent while producing the
  // compact JSONLogic form: A AND B OR C becomes (A AND B) OR C.
  const segments: TargetingItem[][] = [[first]]
  for (const item of rest) {
    const connector = item.connector ?? rule.combinator
    if (connector === "or") segments.push([item])
    else segments.at(-1)!.push(item)
  }

  const conditionForSegment = (segment: TargetingItem[]) =>
    segment.length === 1 ? itemToCondition(segment[0]) : { and: segment.map(itemToCondition) }

  return segments.length === 1 ? conditionForSegment(segments[0]) : { or: segments.map(conditionForSegment) }
}

function parseStrategy(s: StrategyDTO): StrategyRow {
	const editableCondition = isEditableCondition(s.condition)
	const { combinator, conditions } = editableCondition
		? conditionToRule(s.condition)
		: { combinator: "and" as const, conditions: [{ attribute: "", operator: "==" as const, value: "", negate: false }] }
  const rollout = Array.isArray(s.rollout)
    ? (s.rollout as { variant: string; percentage: number }[]).map((b) => ({ variant: b.variant, percentage: String(b.percentage) }))
    : []
  return {
    name: s.name,
    combinator,
    conditions,
    defaultVariant: s.defaultVariant,
    rollout,
		variants: s.variants.map((v) => ({ key: v.key, value: stringifyVariantValue(v.value) })),
		rawCondition: editableCondition ? undefined : s.condition,
  }
}

function newStrategy(seedVariants: VariantRow[], type: StrategyType): StrategyRow {
  const variants = seedVariants.length ? seedVariants.map((variant) => ({ ...variant })) : [{ key: "", value: "" }]
  const onVariant = booleanVariantKey(variants, true)
  const offVariant = booleanVariantKey(variants, false)

  return {
		name: "",
    combinator: "and",
    conditions: [{ attribute: "", operator: "==", value: "", negate: false }],
    defaultVariant: offVariant || variants[0]?.key || "",
    rollout: type === "gradual" ? [{ variant: onVariant || variants[0]?.key || "", percentage: "0" }] : [],
    variants,
  }
}

function TargetingConditionFields({
  condition,
  contextFields,
  onChange,
  onAttributeChange,
}: {
  condition: ConditionRow
  contextFields: ContextField[]
  onChange: (patch: Partial<ConditionRow>) => void
  onAttributeChange: (attribute: string) => void
}) {
	const { t } = useTranslation()
	const field = contextFields.find((candidate) => candidate.key === condition.attribute)

  return (
    <>
      <AttributeInput
        attributes={contextFields.map((field) => field.key)}
        value={condition.attribute}
        onChange={onAttributeChange}
      />
	  <div className="flex items-center gap-1" title={t("flags.negateCondition")}>
        <Switch checked={condition.negate} onCheckedChange={(negate) => onChange({ negate })} />
		<Label className="text-xs text-muted-foreground">{t("flags.not")}</Label>
      </div>
      <Select value={condition.operator} onValueChange={(operator) => onChange({ operator: operator as Operator })}>
        <SelectTrigger className="w-28">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {OPERATORS.map((operator) => (
            <SelectItem key={operator} value={operator}>
              {operator}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {field && field.values.length > 0 ? (
        condition.operator === "in" || condition.operator === "not in" ? (
          <MultiSelect
            options={field.values.map((value) => ({ value: value.value, label: value.value }))}
            selected={condition.value ? condition.value.split(",").filter(Boolean) : []}
            onChange={(values) => onChange({ value: values.join(",") })}
			placeholder={t("flags.selectValues")}
          />
        ) : (
          <Select value={condition.value} onValueChange={(value) => onChange({ value })}>
			<SelectTrigger><SelectValue placeholder={t("flags.selectValue")} /></SelectTrigger>
            <SelectContent>{field.values.map((value) => <SelectItem key={value.value} value={value.value}>{value.value}</SelectItem>)}</SelectContent>
          </Select>
        )
      ) : (
        <Input
			placeholder={t(OPERATOR_PLACEHOLDERS[condition.operator] ?? "flags.valuePlaceholder")}
          value={condition.value}
          onChange={(event) => onChange({ value: event.target.value })}
        />
      )}
    </>
  )
}

export function FlagEditorPage() {
	const { t, i18n } = useTranslation()
  const { projectId, key } = useParams<{ projectId: string; key: string }>()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
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
	const [description, setDescription] = useState("")
	const [tagsText, setTagsText] = useState("")
  const [enabled, setEnabled] = useState(true)
  const [strategies, setStrategies] = useState<StrategyRow[]>([])
  const [prerequisiteFlagKey, setPrerequisiteFlagKey] = useState("")
  const [prerequisiteVariant, setPrerequisiteVariant] = useState("")

  useEffect(() => {
    if (!flag) return
    setName(flag.name)
		setDescription(flag.description)
		setTagsText((flag.tags ?? []).join(", "))
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
		description,
		tags: tagsFromText(tagsText),
        enabled,
        prerequisiteFlagKey,
        prerequisiteVariant: prerequisiteFlagKey ? prerequisiteVariant : "",
        strategies: strategies.map((s, index) => ({
          order: index,
          name: s.name,
          isDefault: false,
		  condition: s.rawCondition ?? ruleToCondition({ priority: 0, description: "", combinator: s.combinator, conditions: s.conditions, variantKey: "", rollout: [] }),
          defaultVariant: s.defaultVariant,
          rollout: s.rollout.length ? s.rollout.map((b) => ({ variant: b.variant, percentage: Number(b.percentage) })) : undefined,
          variants: s.variants.map((v) => ({ key: v.key, value: parseVariantValue(flag.flagType, v.value) })),
        })),
      })
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["flags", projectId, envKey] })
      queryClient.invalidateQueries({ queryKey: ["flag", projectId, envKey, key] })
      alerts.success(t("flags.saveChanges"))
    },
    onError: (err) => alerts.errorFrom(err, t("errors.generic")),
  })

  function handleSubmit(e: FormEvent) {
    e.preventDefault()
    save.mutate()
  }

  function updateStrategy(i: number, patch: Partial<StrategyRow>) {
    setStrategies((prev) => prev.map((s, idx) => (idx === i ? { ...s, ...patch } : s)))
  }

  function addStrategy(type: StrategyType) {
    const variants = strategies[0]?.variants ?? starterVariants(flag?.flagType ?? "string")
    setStrategies((prev) => [...prev, { ...newStrategy(variants, type), name: t(type === "gradual" ? "flags.gradual" : "flags.standard") }])
  }

  function removeStrategy(i: number) {
    setStrategies((prev) => prev.filter((_, idx) => idx !== i))
  }

  function moveStrategy(index: number, direction: -1 | 1) {
    setStrategies((prev) => {
      const destination = index + direction
      if (destination < 0 || destination >= prev.length) return prev
      const next = [...prev]
      const current = next[index]
      next[index] = next[destination]
      next[destination] = current
      return next
    })
  }

  function setStrategyType(strategyIndex: number, type: StrategyType) {
    setStrategies((prev) =>
      prev.map((strategy, index) => {
        if (index !== strategyIndex || strategyTypeOf(strategy) === type) return strategy
        const onVariant = booleanVariantKey(strategy.variants, true)
        const offVariant = booleanVariantKey(strategy.variants, false)

        if (type === "standard") {
          return { ...strategy, defaultVariant: offVariant || strategy.defaultVariant, rollout: [] }
        }

        return {
          ...strategy,
          defaultVariant: offVariant || strategy.defaultVariant,
          rollout: [{ variant: onVariant || strategy.variants[0]?.key || "", percentage: "0" }],
        }
      }),
    )
  }

  function setBooleanStandardValue(strategyIndex: number, enabled: boolean) {
    setStrategies((prev) =>
      prev.map((strategy, index) => {
        if (index !== strategyIndex) return strategy
        return { ...strategy, defaultVariant: booleanVariantKey(strategy.variants, enabled) || strategy.defaultVariant }
      }),
    )
  }

  function setBooleanRolloutPercentage(strategyIndex: number, percentage: string) {
    setStrategies((prev) =>
      prev.map((strategy, index) => {
        if (index !== strategyIndex) return strategy
        const onVariant = booleanVariantKey(strategy.variants, true)
        const offVariant = booleanVariantKey(strategy.variants, false)
        return {
          ...strategy,
          defaultVariant: offVariant || strategy.defaultVariant,
          rollout: [{ variant: onVariant || strategy.variants[0]?.key || "", percentage }],
        }
      }),
    )
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
        idx === strategyIndex
          ? { ...s, conditions: [...s.conditions, { attribute: "", operator: "==", value: "", negate: false, connector: s.combinator }] }
          : s,
      ),
    )
  }

  function addConditionGroup(strategyIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? {
              ...s,
              conditions: [...s.conditions, { kind: "group", combinator: "and", conditions: [{ attribute: "", operator: "==", value: "", negate: false }], connector: s.combinator }],
            }
          : s,
      ),
    )
  }

  function updateCondition(strategyIndex: number, condIndex: number, patch: Partial<ConditionRow>) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? {
              ...s,
              conditions: s.conditions.map((c, ci) => (ci === condIndex && !isConditionGroup(c) ? { ...c, ...patch } : c)),
            }
          : s,
      ),
    )
  }

  function updateConditionConnector(strategyIndex: number, conditionIndex: number, connector: Combinator) {
    setStrategies((prev) =>
      prev.map((strategy, strategyIndex_) =>
        strategyIndex_ === strategyIndex
          ? {
              ...strategy,
              conditions: strategy.conditions.map((item, itemIndex) =>
                itemIndex === conditionIndex ? { ...item, connector } : item,
              ),
            }
          : strategy,
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

  function updateGroup(strategyIndex: number, groupIndex: number, patch: Partial<ConditionGroup>) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? {
              ...s,
              conditions: s.conditions.map((item, itemIndex) =>
                itemIndex === groupIndex && isConditionGroup(item) ? { ...item, ...patch } : item,
              ),
            }
          : s,
      ),
    )
  }

  function addGroupCondition(strategyIndex: number, groupIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? {
              ...s,
              conditions: s.conditions.map((item, itemIndex) =>
                itemIndex === groupIndex && isConditionGroup(item)
                  ? { ...item, conditions: [...item.conditions, { attribute: "", operator: "==", value: "", negate: false }] }
                  : item,
              ),
            }
          : s,
      ),
    )
  }

  function updateGroupCondition(strategyIndex: number, groupIndex: number, conditionIndex: number, patch: Partial<ConditionRow>) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? {
              ...s,
              conditions: s.conditions.map((item, itemIndex) =>
                itemIndex === groupIndex && isConditionGroup(item)
                  ? { ...item, conditions: item.conditions.map((condition, index) => (index === conditionIndex ? { ...condition, ...patch } : condition)) }
                  : item,
              ),
            }
          : s,
      ),
    )
  }

  function removeGroupCondition(strategyIndex: number, groupIndex: number, conditionIndex: number) {
    setStrategies((prev) =>
      prev.map((s, idx) =>
        idx === strategyIndex
          ? {
              ...s,
              conditions: s.conditions.map((item, itemIndex) =>
                itemIndex === groupIndex && isConditionGroup(item)
                  ? { ...item, conditions: item.conditions.filter((_, index) => index !== conditionIndex) }
                  : item,
              ),
            }
          : s,
      ),
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
    return <div className="p-8 text-muted-foreground">{t("common.loading")}</div>
  }

  const isBooleanFlag = flag.flagType === "boolean"

  return (
    <div className="p-8">
      <div className="mb-6 flex items-center justify-between">
        <h1 className="text-2xl font-semibold">
          <code>{flag.key}</code>
        </h1>
        <Button variant="outline" onClick={() => navigate(`/projects/${projectId}/flags`)}>
          {t("flags.backToFlags")}
        </Button>
      </div>

      <form className="flex flex-col gap-6" onSubmit={handleSubmit}>
        <div className="grid gap-6 lg:grid-cols-[minmax(16rem,0.7fr)_minmax(0,1.8fr)] lg:items-start">
        <Card>
          <CardHeader>
            <CardTitle>{t("flags.details")}</CardTitle>
            <CardAction>
              <div className="flex items-center gap-3">
                <Label htmlFor="flag-enabled">{t("flags.enabled")}</Label>
                <Switch id="flag-enabled" checked={enabled} onCheckedChange={setEnabled} />
              </div>
            </CardAction>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="flag-name">{t("flags.name")}</Label>
              <Input id="flag-name" value={name} onChange={(e) => setName(e.target.value)} />
            </div>
			<div className="flex flex-col gap-2">
				<Label htmlFor="flag-description">{t("flags.description")}</Label>
				<Textarea
					id="flag-description"
					value={description}
					onChange={(e) => setDescription(e.target.value)}
					className="min-h-24 font-sans"
				/>
			</div>
			<div className="flex flex-col gap-2">
				<Label htmlFor="flag-tags">{t("flags.tags")}</Label>
				<Input
					id="flag-tags"
					placeholder={t("flags.tagsPlaceholder")}
					value={tagsText}
					onChange={(e) => setTagsText(e.target.value)}
				/>
				<p className="text-xs text-muted-foreground">{t("flags.tagsHint")}</p>
			</div>
			<div className="border-t pt-4">
				<dl className="grid gap-3 text-sm">
					<div className="grid gap-1">
						<dt className="text-muted-foreground">{t("flags.created")}</dt>
					<dd>{formatCreatedAt(flag.createdAt, i18n.language)}</dd>
					</div>
					<div className="grid gap-1">
						<dt className="text-muted-foreground">{t("flags.createdBy")}</dt>
						<dd>{flag.createdBy?.name || "—"}</dd>
					</div>
					<div className="grid gap-1">
						<dt className="text-muted-foreground">{t("flags.collaborators")}</dt>
						<dd>{flag.collaborators.map((collaborator) => collaborator.name).filter(Boolean).join(", ") || "—"}</dd>
					</div>
				</dl>
			</div>
          </CardContent>
        </Card>

        <Tabs defaultValue="strategies" className="min-w-0">
          <TabsList aria-label={t("flags.configurationSections")}>
            <TabsTrigger value="strategies">{t("flags.strategies")}</TabsTrigger>
            <TabsTrigger value="prerequisites">{t("flags.prerequisites")}</TabsTrigger>
          </TabsList>
          <TabsContent value="strategies">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>{t("flags.strategies")}</CardTitle>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <ActionButton type="button" variant="outline" size="sm">{t("flags.addStrategy")}</ActionButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem onSelect={() => addStrategy("standard")}>{t("flags.standard")}</DropdownMenuItem>
                <DropdownMenuItem onSelect={() => addStrategy("gradual")}>{t("flags.gradual")}</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <p className="text-sm text-muted-foreground">
              {t("flags.strategyIntro")}
            </p>
            {strategies.length === 0 && <p className="rounded-md border border-dashed p-4 text-sm text-muted-foreground">{t("flags.noStrategies")}</p>}
            {strategies.map((s, i) => {
              const type = strategyTypeOf(s)
              const onVariant = booleanVariantKey(s.variants, true)
              const offVariant = booleanVariantKey(s.variants, false)
              const rolloutPercentage = s.rollout.find((bucket) => bucket.variant === onVariant)?.percentage ?? s.rollout[0]?.percentage ?? "0"

              return (
              <div key={i} className="flex flex-col gap-3 rounded-md border border-primary/20 bg-primary/5 p-3">
                <div className="flex items-center gap-2">
                  <Input
                    placeholder={t("flags.strategyNamePlaceholder")}
                    value={s.name}
                    onChange={(e) => updateStrategy(i, { name: e.target.value })}
                    className="flex-1"
                  />
                  <Badge variant="secondary">{t(type === "gradual" ? "flags.gradual" : "flags.standard")}</Badge>
				  <Button type="button" variant="ghost" size="icon-sm" aria-label={t("flags.moveStrategyUp", { strategy: s.name || t("flags.strategyNumber", { index: i + 1 }) })} onClick={() => moveStrategy(i, -1)} disabled={i === 0}>
                    <ArrowUp />
                  </Button>
				  <Button type="button" variant="ghost" size="icon-sm" aria-label={t("flags.moveStrategyDown", { strategy: s.name || t("flags.strategyNumber", { index: i + 1 }) })} onClick={() => moveStrategy(i, 1)} disabled={i === strategies.length - 1}>
                    <ArrowDown />
                  </Button>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
					  <Button type="button" variant="ghost" size="icon-sm" aria-label={t("flags.strategyActionsAria", { strategy: s.name || t("flags.strategyNumber", { index: i + 1 }) })}>
                        <EllipsisVertical />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      <DropdownMenuItem
                        variant="destructive"
                        onSelect={() => removeStrategy(i)}
                      >
                        {t("flags.removeStrategy")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>

                <div className="flex w-fit rounded-md border p-1">
                  <Button type="button" size="sm" variant={type === "standard" ? "secondary" : "ghost"} onClick={() => setStrategyType(i, "standard")}>
                    {t("flags.standard")}
                  </Button>
                  <Button type="button" size="sm" variant={type === "gradual" ? "secondary" : "ghost"} onClick={() => setStrategyType(i, "gradual")}>
                    {t("flags.gradual")}
                  </Button>
                </div>

                {type === "gradual" && (
                  <div className="rounded-md border border-primary/20 bg-muted/50 p-3 text-sm">
                    <p className="font-medium">{t("flags.gradualDeterministic")}</p>
                    <p className="mt-1 text-xs text-muted-foreground">{t("flags.gradualDeterministicDescription")}</p>
                  </div>
                )}
                {isBooleanFlag ? (
                  type === "standard" ? (
                    <div className="flex items-center gap-3 rounded-md bg-muted/50 p-3">
                      <Switch
                        id={`strategy-${i}-standard-value`}
                        checked={Boolean(onVariant) && s.defaultVariant === onVariant}
                        onCheckedChange={(checked) => setBooleanStandardValue(i, checked)}
                        disabled={!onVariant || !offVariant}
                      />
                      <Label htmlFor={`strategy-${i}-standard-value`} className="font-medium">
					{Boolean(onVariant) && s.defaultVariant === onVariant ? t("flags.on") : t("flags.off")}
                      </Label>
                    </div>
                  ) : (
                    <div className="flex flex-col gap-2 rounded-md bg-muted/50 p-3">
                      <Label className="text-sm font-medium">{t("flags.percentageReceiveOn")}</Label>
                      <div className="flex items-center gap-3">
                        <Slider
                          aria-label={`Gradual rollout percentage for ${s.name || `strategy ${i + 1}`}`}
                          value={clampPercentage(rolloutPercentage)}
                          onValueChange={(value) => setBooleanRolloutPercentage(i, String(value))}
                          min={0}
                          max={100}
                          step={1}
                        />
                        <div className="relative w-20 shrink-0">
                          <Input
                            type="number"
                            min={0}
                            max={100}
                            step={1}
                            inputMode="numeric"
                            aria-label={`Gradual rollout percentage for ${s.name || `strategy ${i + 1}`}`}
                            value={rolloutPercentage}
                            onChange={(event) => setBooleanRolloutPercentage(i, event.target.value === "" ? "" : String(clampPercentage(event.target.value)))}
                            className="pr-7 text-right tabular-nums"
                          />
                          <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-muted-foreground">%</span>
                        </div>
                      </div>
                      <p className="text-xs text-muted-foreground">{t("flags.remainingReceiveOff")}</p>
                    </div>
                  )
                ) : null}
                {!isBooleanFlag && (
                  <div className="flex flex-col gap-2">
                    <div className="flex items-center justify-between">
					  <Label className="text-xs text-muted-foreground">{t("flags.variants")}</Label>
                      <Button type="button" variant="ghost" size="sm" onClick={() => addVariant(i)}>
						{t("flags.addVariant")}
                      </Button>
                    </div>
                  {s.variants.map((v, vi) => (
                    <div key={vi} className="flex items-center gap-2">
					  <Input placeholder={t("flags.variantKeyPlaceholder")} value={v.key} onChange={(e) => updateVariant(i, vi, "key", e.target.value)} className="w-32" />
					  <Input placeholder={t("flags.variantValuePlaceholder")} value={v.value} onChange={(e) => updateVariant(i, vi, "value", e.target.value)} />
                      <ActionButton
                        type="button"
                        variant="destructive"
                        size="sm"
                        icon={<Trash2 />}
                        onClick={() => removeVariant(i, vi)}
                        disabled={s.variants.length <= 1}
                      >
						{t("pages.remove")}
                      </ActionButton>
                    </div>
                  ))}
                  <div className="flex flex-col gap-2 max-w-xs">
					<Label className="text-xs text-muted-foreground">{t("flags.defaultVariantDescription")}</Label>
                    <Select key={s.variants.length ? "ready" : "loading"} value={s.defaultVariant} onValueChange={(v) => updateStrategy(i, { defaultVariant: v })}>
                      <SelectTrigger>
					  <SelectValue placeholder={t("flags.selectVariant")} />
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
                )}

                <Separator />

                <div className="flex flex-col gap-3">
                    <div className="flex flex-wrap items-center justify-between gap-2">
                      <div>
						<Label className="text-sm font-medium">{t("flags.targeting")}</Label>
						<p className="text-xs text-muted-foreground">{t("flags.targetingHint")}</p>
						{s.rawCondition !== undefined && <p role="alert" className="mt-2 text-xs text-amber-700 dark:text-amber-300">{t("flags.unsupportedCondition")}</p>}
                      </div>
                      <div className="flex items-center gap-1">
						<ActionButton type="button" variant="outline" size="sm" icon={<ListPlus />} onClick={() => addConditionGroup(i)} disabled={s.rawCondition !== undefined}>
						{t("flags.addGroup")}
                        </ActionButton>
                        <ActionButton
                          type="button"
                          variant="outline"
                          size="sm"
                          icon={<Plus />}
						  className="border-blue-500/60 text-blue-600 hover:bg-blue-500/10 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
						  onClick={() => addCondition(i)}
						  disabled={s.rawCondition !== undefined}
                        >
						{t("flags.addCondition")}
                        </ActionButton>
                      </div>
                    </div>
					<fieldset disabled={s.rawCondition !== undefined} className="contents">
					<div className="flex flex-col gap-2">
                      {s.conditions.map((item, itemIndex) => (
                        <div key={itemIndex} className="grid gap-2 sm:grid-cols-[5.5rem_minmax(0,1fr)] sm:items-center">
                          {itemIndex === 0 ? (
							<span className="inline-flex h-8 w-20 items-center justify-center rounded-full border border-primary/40 bg-primary/10 px-3 text-xs font-bold tracking-wide text-primary uppercase">{t("flags.where")}</span>
                          ) : (
                            <Select value={item.connector ?? s.combinator} onValueChange={(value) => updateConditionConnector(i, itemIndex, value as Combinator)}>
							  <SelectTrigger aria-label={t("flags.targetingOperatorAria", { index: itemIndex + 1 })} className="w-20 rounded-full border-primary bg-primary px-3 font-semibold text-primary-foreground [&_svg]:text-primary-foreground">
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent>
                                {COMBINATORS.map((combinator) => (
                                  <SelectItem key={combinator} value={combinator}>{combinator.toUpperCase()}</SelectItem>
                                ))}
                              </SelectContent>
                            </Select>
                          )}
                          {isConditionGroup(item) ? (
                            <div className="rounded-lg border border-primary/30 bg-muted/30 p-3">
                              <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
                                <div className="flex items-center gap-2">
								  <span className="text-xs font-semibold text-foreground">{t("flags.group")}</span>
                                  <Select value={item.combinator} onValueChange={(value) => updateGroup(i, itemIndex, { combinator: value as Combinator })}>
									<SelectTrigger aria-label={t("flags.groupOperatorAria")} className="w-24 border-primary/50 bg-background font-semibold text-foreground">
                                      <SelectValue />
                                    </SelectTrigger>
                                    <SelectContent>
									  <SelectItem value="and">{t("flags.allConditions")}</SelectItem>
									  <SelectItem value="or">{t("flags.anyConditions")}</SelectItem>
                                    </SelectContent>
                                  </Select>
								  <span className="text-xs text-muted-foreground">{t("flags.groupConditions")}</span>
                                </div>
                                <ActionButton
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  icon={<Plus />}
                                  className="border-blue-500/60 text-blue-600 hover:bg-blue-500/10 hover:text-blue-700 dark:text-blue-400 dark:hover:text-blue-300"
                                  onClick={() => addGroupCondition(i, itemIndex)}
                                >
									{t("flags.addCondition")}
                                </ActionButton>
                              </div>
                              <div className="flex flex-col gap-2 border-l-2 border-primary/30 pl-3">
                                {item.conditions.map((condition, conditionIndex) => (
                                  <div key={conditionIndex} className="grid gap-2 sm:grid-cols-[5.5rem_minmax(0,1fr)] sm:items-center">
                                    {conditionIndex === 0 ? (
									<span className="inline-flex h-8 w-20 items-center justify-center rounded-full border border-primary/40 bg-primary/10 px-3 text-xs font-bold tracking-wide text-primary uppercase">{t("flags.where")}</span>
                                    ) : (
                                      <span className="inline-flex h-8 w-20 items-center justify-center rounded-full bg-secondary px-3 text-xs font-semibold text-secondary-foreground">
                                        {item.combinator.toUpperCase()}
                                      </span>
                                    )}
                                    <div className="relative flex flex-wrap items-center gap-2 rounded-md border bg-background p-2 pr-10">
                                      <TargetingConditionFields
                                        condition={condition}
                                        contextFields={contextFields}
                                        onChange={(patch) => updateGroupCondition(i, itemIndex, conditionIndex, patch)}
                                        onAttributeChange={(attribute) => updateGroupCondition(i, itemIndex, conditionIndex, { attribute, value: "", operator: "==" })}
                                      />
                                      <ActionButton
                                        type="button"
                                        variant="ghost"
                                        size="icon-sm"
                                        icon={<X />}
									aria-label={t("flags.removeCondition")}
                                        className="absolute top-2 right-2 border border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive"
                                        onClick={() => removeGroupCondition(i, itemIndex, conditionIndex)}
                                        disabled={item.conditions.length <= 1}
                                      >
									<span className="sr-only">{t("flags.removeCondition")}</span>
                                      </ActionButton>
                                    </div>
                                  </div>
                                ))}
                              </div>
                            </div>
                          ) : (
                            <div className="relative flex flex-wrap items-center gap-2 rounded-lg border bg-muted/20 p-3 pr-11 shadow-xs">
                              <TargetingConditionFields
                                condition={item}
                                contextFields={contextFields}
                                onChange={(patch) => updateCondition(i, itemIndex, patch)}
                                onAttributeChange={(attribute) => selectAttribute(i, itemIndex, attribute)}
                              />
                              <ActionButton
                                type="button"
                                variant="ghost"
                                size="icon-sm"
                                icon={<X />}
								aria-label={t("flags.removeCondition")}
                                className="absolute top-3 right-3 border border-destructive/30 text-destructive hover:bg-destructive/10 hover:text-destructive"
                                onClick={() => removeCondition(i, itemIndex)}
                                disabled={s.conditions.length <= 1}
                              >
								<span className="sr-only">{t("flags.removeCondition")}</span>
                              </ActionButton>
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
					<p className="text-xs text-muted-foreground">{t("flags.groupLimitHint", { example: "plan = pro AND (country = BR OR country = US)" })}</p>
					<Separator />
					</fieldset>
                </div>

                {!isBooleanFlag && type === "gradual" && (
                <div className="flex flex-col gap-2">
                  <div className="flex items-center justify-between">
					<Label className="text-xs text-muted-foreground">{t("flags.rolloutVariants")}</Label>
                    <Button type="button" variant="ghost" size="sm" onClick={() => addRolloutBucket(i)}>
						{t("flags.addBucket")}
                    </Button>
                  </div>
                  {s.rollout.map((b, bi) => (
                    <div key={bi} className="flex items-center gap-2">
                      <Select value={b.variant} onValueChange={(v) => updateRolloutBucket(i, bi, "variant", v)}>
                        <SelectTrigger className="w-32">
					  <SelectValue placeholder={t("flags.variant")} />
                        </SelectTrigger>
                        <SelectContent>
                          {s.variants.filter((v) => v.key).map((v) => (
                            <SelectItem key={v.key} value={v.key}>
                              {v.key}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <div className="flex min-w-0 flex-1 items-center gap-2">
                        <Slider
						aria-label={t("flags.rolloutPercentageAria", { variant: b.variant || t("flags.bucketNumber", { index: bi + 1 }) })}
                          value={clampPercentage(b.percentage)}
                          onValueChange={(value) => updateRolloutBucket(i, bi, "percentage", String(value))}
                          min={0}
                          max={100}
                          step={1}
                        />
                        <div className="relative w-20 shrink-0">
                          <Input
                            type="number"
                            min={0}
                            max={100}
                            step={1}
                            inputMode="numeric"
							aria-label={t("flags.rolloutPercentageAria", { variant: b.variant || t("flags.bucketNumber", { index: bi + 1 }) })}
                            value={b.percentage}
                            onChange={(e) => {
                              const value = e.target.value
                              updateRolloutBucket(i, bi, "percentage", value === "" ? "" : String(clampPercentage(value)))
                            }}
                            className="pr-7 text-right tabular-nums"
                          />
                          <span className="pointer-events-none absolute inset-y-0 right-3 flex items-center text-sm text-muted-foreground">%</span>
                        </div>
                      </div>
                      <ActionButton type="button" variant="destructive" size="sm" icon={<Trash2 />} onClick={() => removeRolloutBucket(i, bi)}>
						{t("pages.remove")}
                      </ActionButton>
                    </div>
                  ))}
                </div>
                )}
              </div>
              )
            })}
          </CardContent>
        </Card>

          </TabsContent>
          <TabsContent value="prerequisites">
        <Card>
          <CardHeader>
			<CardTitle>{t("flags.prerequisitesTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <p className="text-sm text-muted-foreground">
			{t("flags.prerequisitesIntro")}
            </p>
            <div className="flex flex-col gap-2 max-w-xs">
			<Label>{t("flags.prerequisiteFlag")}</Label>
              <Select
                value={prerequisiteFlagKey || NONE_PREREQUISITE}
                onValueChange={(v) => {
                  setPrerequisiteFlagKey(v === NONE_PREREQUISITE ? "" : v)
                  setPrerequisiteVariant("")
                }}
              >
                <SelectTrigger>
			<SelectValue placeholder={t("flags.none")} />
                </SelectTrigger>
                <SelectContent>
			<SelectItem value={NONE_PREREQUISITE}>{t("flags.none")}</SelectItem>
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
			<Label>{t("flags.requiredPrerequisiteVariant")}</Label>
                <Select value={prerequisiteVariant} onValueChange={setPrerequisiteVariant}>
                  <SelectTrigger>
			<SelectValue placeholder={t("flags.selectVariant")} />
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
          </TabsContent>
        </Tabs>
        </div>

        <div>
          <Button type="submit" disabled={save.isPending}>
			{t("flags.saveChanges")}
          </Button>
        </div>
      </form>
    </div>
  )
}
