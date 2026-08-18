// Package strategy holds the flag targeting-strategy domain: the JSONLogic
// condition engine, rollout bucketing, and the strategy evaluation/validation
// logic used by the flag service. It's split out from internal/service so
// the eval engine can be unit-tested in isolation from FlagService/GORM.
package strategy

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
)

var errConditionMismatch = errors.New("condition operands mismatch")

var (
	ErrVariantTypeMismatch = errors.New("variant value does not match the flag type")
	ErrUnknownFlagType     = errors.New("unknown flag type")
	ErrDefaultCount        = errors.New("exactly one strategy must be marked as default")
)

// EvaluationResult is the outcome of evaluating a flag against a context.
type EvaluationResult struct {
	Key       string
	Value     any
	Reason    string
	Variant   string
	ErrorCode string
}

// Flag is a flag flattened against one environment: its enabled state plus
// that environment's ordered strategy list (env-scoped, see FlagRepository).
type Flag struct {
	Key        string
	Archived   bool
	Enabled    bool
	Strategies []model.FlagStrategy
}

// Flatten builds a Flag from a model.FeatureFlag's env-scoped
// FlagEnvironmentConfig (preloaded by the repository for exactly the
// requested environment). A flag with no config row for this environment is
// treated as disabled — new environments start every flag off until
// explicitly configured.
func Flatten(flag *model.FeatureFlag) Flag {
	f := Flag{Key: flag.Key, Archived: flag.ArchivedAt != nil, Strategies: flag.Strategies}
	if len(flag.Configs) > 0 {
		f.Enabled = flag.Configs[0].Enabled
	}
	return f
}

// sortedStrategies orders non-default strategies by Priority ahead of the
// (single) default strategy, which is always evaluated last as the catch-all.
func sortedStrategies(in []model.FlagStrategy) []model.FlagStrategy {
	out := make([]model.FlagStrategy, len(in))
	copy(out, in)
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDefault != out[j].IsDefault {
			return !out[i].IsDefault
		}
		return out[i].Priority < out[j].Priority
	})
	return out
}

// Evaluate runs the targeting engine for a single flag: disabled check, then
// ordered strategy matching (first match wins — any strategy whose
// ConditionJSON matches the context, with optional per-strategy rollout),
// falling back to the flag's default (catch-all) strategy.
func Evaluate(flag Flag, evalCtx map[string]any) EvaluationResult {
	strategies := sortedStrategies(flag.Strategies)

	if flag.Archived || !flag.Enabled {
		return DefaultResult(flag, constants.ReasonDisabled)
	}

	for _, st := range strategies {
		matched, err := evaluateCondition(st.ConditionJSON, evalCtx)
		if err != nil {
			return EvaluationResult{Key: flag.Key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeGeneral}
		}
		if !matched {
			continue
		}
		return resolveStrategyMatch(flag.Key, st, len(strategies), evalCtx)
	}

	// No strategy matched — shouldn't happen, since the default strategy has
	// an empty condition and always matches. Fail closed rather than panic.
	return EvaluationResult{Key: flag.Key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeGeneral}
}

// resolveStrategyMatch applies a matched strategy's rollout (if any) and
// resolves the resulting variant against that strategy's own catalog.
func resolveStrategyMatch(flagKey string, st model.FlagStrategy, totalStrategies int, evalCtx map[string]any) EvaluationResult {
	variantKey := st.DefaultVariant
	reason := constants.ReasonTargetingMatch
	if st.IsDefault {
		if totalStrategies == 1 {
			reason = constants.ReasonStatic
		} else {
			reason = constants.ReasonDefault
		}
	}
	if len(st.RolloutJSON) > 0 {
		// Salted with the strategy ID (not just the flag key) so different
		// strategies on the same flag bucket subjects independently.
		bucketed, err := pickRolloutVariant(st.RolloutJSON, flagKey+":"+st.ID.String(), evalCtx)
		if err == nil && bucketed != "" {
			variantKey = bucketed
			reason = constants.ReasonSplit
		}
	}
	return resultForStrategy(flagKey, st, variantKey, reason)
}

// DefaultResult resolves the flag's default (catch-all) strategy to its own
// DefaultVariant, ignoring targeting/rollout — used for the disabled/archived
// and prerequisite-failed paths. An environment that has never been
// configured for this flag has no strategies at all (see Flatten); that's
// still reported as the requested reason (e.g. DISABLED), just without a
// resolvable variant, rather than as an error.
func DefaultResult(flag Flag, reason string) EvaluationResult {
	for _, st := range flag.Strategies {
		if st.IsDefault {
			return resultForStrategy(flag.Key, st, st.DefaultVariant, reason)
		}
	}
	return EvaluationResult{Key: flag.Key, Reason: reason}
}

func resultForStrategy(flagKey string, st model.FlagStrategy, variantKey, reason string) EvaluationResult {
	for _, v := range st.Variants {
		if v.Key == variantKey {
			var value any
			_ = json.Unmarshal(v.Value, &value)
			return EvaluationResult{Key: flagKey, Value: value, Reason: reason, Variant: variantKey}
		}
	}
	return EvaluationResult{Key: flagKey, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeGeneral}
}

type rolloutBucket struct {
	Variant    string  `json:"variant"`
	Percentage float64 `json:"percentage"`
}

// pickRolloutVariant deterministically buckets context.targetingKey into
// 0-100 using fnv hashing (salted with saltKey) so the same subject always
// lands in the same bucket for a given flag+strategy.
func pickRolloutVariant(rolloutJSON []byte, saltKey string, evalCtx map[string]any) (string, error) {
	var buckets []rolloutBucket
	if err := json.Unmarshal(rolloutJSON, &buckets); err != nil {
		return "", err
	}
	raw, ok := bucketFor(saltKey, evalCtx)
	if !ok {
		return "", nil
	}
	bucket := float64(raw)

	var cumulative float64
	for _, b := range buckets {
		cumulative += b.Percentage
		if bucket < cumulative {
			return b.Variant, nil
		}
	}
	return "", nil
}

// bucketFor deterministically maps context.targetingKey into 0-99, salted
// with saltKey. The bool is false when there's no targetingKey to bucket on.
func bucketFor(saltKey string, evalCtx map[string]any) (int, bool) {
	targetingKey, _ := evalCtx["targetingKey"].(string)
	if targetingKey == "" {
		return 0, false
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(saltKey + ":" + targetingKey))
	return int(h.Sum32() % 100), true
}

// evaluateCondition evaluates a JSONLogic-style condition tree, e.g.
// {"==": [{"var": "plan"}, "pro"]} or {"and": [...]}.
func evaluateCondition(conditionJSON []byte, evalCtx map[string]any) (bool, error) {
	if len(conditionJSON) == 0 {
		return true, nil
	}
	var node any
	if err := json.Unmarshal(conditionJSON, &node); err != nil {
		return false, err
	}
	result, err := evalNode(node, evalCtx)
	if err != nil {
		return false, err
	}
	b, ok := result.(bool)
	if !ok {
		return false, errConditionMismatch
	}
	return b, nil
}

func evalNode(node any, evalCtx map[string]any) (any, error) {
	obj, ok := node.(map[string]any)
	if !ok {
		return node, nil
	}
	if len(obj) != 1 {
		return nil, fmt.Errorf("condition node must have exactly one operator, got %d", len(obj))
	}
	for op, rawArgs := range obj {
		return applyOperator(op, rawArgs, evalCtx)
	}
	return nil, errConditionMismatch
}

func applyOperator(op string, rawArgs any, evalCtx map[string]any) (any, error) {
	switch op {
	case "var":
		name, ok := rawArgs.(string)
		if arr, isArr := rawArgs.([]any); isArr && len(arr) > 0 {
			name, ok = arr[0].(string)
		}
		if !ok {
			return nil, errConditionMismatch
		}
		return evalCtx[name], nil
	case "and", "or":
		args, ok := rawArgs.([]any)
		if !ok {
			return nil, errConditionMismatch
		}
		return evalBoolChain(op, args, evalCtx)
	case "!":
		// JSONLogic allows "!" with either a bare inner node or a
		// single-element array wrapping it.
		inner := rawArgs
		if arr, isArr := rawArgs.([]any); isArr && len(arr) == 1 {
			inner = arr[0]
		}
		result, err := evalNode(inner, evalCtx)
		if err != nil {
			return nil, err
		}
		b, ok := result.(bool)
		if !ok {
			return nil, errConditionMismatch
		}
		return !b, nil
	}

	args, ok := rawArgs.([]any)
	if !ok || len(args) != 2 {
		return nil, errConditionMismatch
	}
	left, err := evalNode(args[0], evalCtx)
	if err != nil {
		return nil, err
	}
	right, err := evalNode(args[1], evalCtx)
	if err != nil {
		return nil, err
	}

	switch op {
	case "==":
		return fmt.Sprint(left) == fmt.Sprint(right), nil
	case "!=":
		return fmt.Sprint(left) != fmt.Sprint(right), nil
	case ">":
		return compareNumbers(left, right, func(a, b float64) bool { return a > b })
	case "<":
		return compareNumbers(left, right, func(a, b float64) bool { return a < b })
	case "in":
		return membershipCheck(left, right)
	case "not in":
		return negatedMembershipCheck(left, right)
	case "contains":
		return membershipCheck(right, left)
	case "not contains":
		return negatedMembershipCheck(right, left)
	case "semver>":
		return compareSemverOp(left, right, func(c int) bool { return c > 0 })
	case "semver<":
		return compareSemverOp(left, right, func(c int) bool { return c < 0 })
	case "semver=":
		return compareSemverOp(left, right, func(c int) bool { return c == 0 })
	case "matches":
		return regexMatch(left, right)
	default:
		return nil, fmt.Errorf("unsupported operator %q", op)
	}
}

func evalBoolChain(op string, args []any, evalCtx map[string]any) (bool, error) {
	for _, a := range args {
		v, err := evalNode(a, evalCtx)
		if err != nil {
			return false, err
		}
		b, ok := v.(bool)
		if !ok {
			return false, errConditionMismatch
		}
		if op == "and" && !b {
			return false, nil
		}
		if op == "or" && b {
			return true, nil
		}
	}
	return op == "and", nil
}

func compareNumbers(left, right any, cmp func(a, b float64) bool) (bool, error) {
	lf, lok := toFloat(left)
	rf, rok := toFloat(right)
	if !lok || !rok {
		return false, errConditionMismatch
	}
	return cmp(lf, rf), nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	default:
		return 0, false
	}
}

// compareSemverOp compares two "major.minor.patch"-shaped version strings
// (pre-release/build suffixes are ignored) and applies cmp to the 3-way
// comparison result (-1, 0, 1).
func compareSemverOp(left, right any, cmp func(c int) bool) (bool, error) {
	c, err := compareSemver(fmt.Sprint(left), fmt.Sprint(right))
	if err != nil {
		return false, err
	}
	return cmp(c), nil
}

func compareSemver(a, b string) (int, error) {
	av, err := parseSemver(a)
	if err != nil {
		return 0, err
	}
	bv, err := parseSemver(b)
	if err != nil {
		return 0, err
	}
	for i := 0; i < 3; i++ {
		if av[i] != bv[i] {
			if av[i] < bv[i] {
				return -1, nil
			}
			return 1, nil
		}
	}
	return 0, nil
}

func parseSemver(v string) ([3]int, error) {
	var out [3]int
	// Strip any pre-release/build metadata (e.g. "1.2.3-beta.1").
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	parts := strings.SplitN(v, ".", 3)
	for i := 0; i < 3; i++ {
		if i >= len(parts) {
			break
		}
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			return out, errConditionMismatch
		}
		out[i] = n
	}
	return out, nil
}

var regexCache sync.Map // string -> *regexp.Regexp

func regexMatch(value, pattern any) (bool, error) {
	pat := fmt.Sprint(pattern)
	var re *regexp.Regexp
	if cached, ok := regexCache.Load(pat); ok {
		re = cached.(*regexp.Regexp)
	} else {
		compiled, err := regexp.Compile(pat)
		if err != nil {
			return false, errConditionMismatch
		}
		regexCache.Store(pat, compiled)
		re = compiled
	}
	return re.MatchString(fmt.Sprint(value)), nil
}

func membershipCheck(needle, haystack any) (bool, error) {
	list, ok := haystack.([]any)
	if !ok {
		if s, ok := haystack.(string); ok {
			return strings.Contains(s, fmt.Sprint(needle)), nil
		}
		return false, errConditionMismatch
	}
	for _, item := range list {
		if fmt.Sprint(item) == fmt.Sprint(needle) {
			return true, nil
		}
	}
	return false, nil
}

// negatedMembershipCheck backs "not in"/"not contains" — same argument
// shape as membershipCheck, inverted result, error propagated unchanged.
func negatedMembershipCheck(needle, haystack any) (bool, error) {
	ok, err := membershipCheck(needle, haystack)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

// VariantInput is one variant key/value pair within a strategy's catalog.
type VariantInput struct {
	Key   string
	Value []byte
}

// Input is the shape used to create/replace a single strategy.
type Input struct {
	Order          int
	Name           string
	Description    string
	IsDefault      bool
	ConditionJSON  []byte
	DefaultVariant string
	RolloutJSON    []byte
	Variants       []VariantInput
}

// DefaultFor builds the starting catch-all strategy for a newly created flag
// of the given type: an always-matching default strategy with a
// single-variant catalog appropriate to the type.
func DefaultFor(flagType string) Input {
	variants, defaultVariant := DefaultVariantsFor(flagType)
	return Input{Name: "Default", IsDefault: true, DefaultVariant: defaultVariant, Variants: variants}
}

// DefaultVariantsFor builds the starting variant catalog for a newly created
// flag of the given type, plus the default variant key within that catalog.
func DefaultVariantsFor(flagType string) ([]VariantInput, string) {
	switch flagType {
	case constants.FlagTypeBoolean:
		return []VariantInput{{Key: "A", Value: []byte("true")}, {Key: "B", Value: []byte("false")}}, "B"
	case constants.FlagTypeNumber:
		return []VariantInput{{Key: "default", Value: []byte("0")}}, "default"
	case constants.FlagTypeObject:
		return []VariantInput{{Key: "default", Value: []byte("{}")}}, "default"
	default: // string
		return []VariantInput{{Key: "default", Value: []byte(`""`)}}, "default"
	}
}

// Validate enforces the flag-wide type on every strategy's variant catalog
// and requires exactly one strategy to be marked default (the catch-all
// every flag needs a guaranteed fallback result from).
func Validate(flagType string, strategies []Input) error {
	defaults := 0
	for _, st := range strategies {
		if st.IsDefault {
			defaults++
		}
		if err := ValidateVariantValues(flagType, st.Variants); err != nil {
			return err
		}
	}
	if defaults != 1 {
		return ErrDefaultCount
	}
	return nil
}

// ValidateVariantValues enforces that each variant's JSON value matches the
// flag's declared type, so OFREP consumers calling a typed getter never hit a
// TYPE_MISMATCH they couldn't have predicted from the flag's type.
func ValidateVariantValues(flagType string, variants []VariantInput) error {
	for _, v := range variants {
		var decoded any
		if err := json.Unmarshal(v.Value, &decoded); err != nil {
			return fmt.Errorf("%w: variant %q is not valid JSON", ErrVariantTypeMismatch, v.Key)
		}
		ok := false
		switch flagType {
		case constants.FlagTypeBoolean:
			_, ok = decoded.(bool)
		case constants.FlagTypeNumber:
			_, ok = decoded.(float64)
		case constants.FlagTypeString:
			_, ok = decoded.(string)
		case constants.FlagTypeObject:
			switch decoded.(type) {
			case map[string]any, []any:
				ok = true
			}
		default:
			return fmt.Errorf("%w: %q", ErrUnknownFlagType, flagType)
		}
		if !ok {
			return fmt.Errorf("%w: variant %q is not a %s", ErrVariantTypeMismatch, v.Key, flagType)
		}
	}
	return nil
}

// ToModels converts a strategy input list (as accepted by FlagService) into
// persistence models scoped to one flag+environment.
func ToModels(flagID, environmentID uuid.UUID, in []Input) []model.FlagStrategy {
	out := make([]model.FlagStrategy, 0, len(in))
	for _, st := range in {
		// The default strategy is a catch-all: force its condition empty
		// regardless of what was supplied, so it always matches.
		condition := st.ConditionJSON
		if st.IsDefault {
			condition = nil
		}
		out = append(out, model.FlagStrategy{
			FlagID:         flagID,
			EnvironmentID:  environmentID,
			Priority:       st.Order,
			Name:           st.Name,
			Description:    st.Description,
			IsDefault:      st.IsDefault,
			ConditionJSON:  datatypes.JSON(condition),
			DefaultVariant: st.DefaultVariant,
			RolloutJSON:    datatypes.JSON(st.RolloutJSON),
			Variants:       toVariantModels(st.Variants),
		})
	}
	return out
}

func toVariantModels(in []VariantInput) []model.FlagStrategyVariant {
	out := make([]model.FlagStrategyVariant, 0, len(in))
	for _, v := range in {
		out = append(out, model.FlagStrategyVariant{Key: v.Key, Value: datatypes.JSON(v.Value)})
	}
	return out
}
