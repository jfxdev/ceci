package service

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

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
)

var errConditionMismatch = errors.New("condition operands mismatch")

// EvaluationResult is the outcome of evaluating a flag against a context.
type EvaluationResult struct {
	Key       string
	Value     any
	Reason    string
	Variant   string
	ErrorCode string
}

// envFlag is a flag flattened against one environment: its shared variant
// catalog plus that environment's enabled/default-variant config and
// targeting rules (both env-scoped, see FlagRepository).
type envFlag struct {
	Key            string
	Enabled        bool
	DefaultVariant string
	Variants       []model.FlagVariant
	Rules          []model.FlagRule
}

// toEnvFlag flattens a flag's env-scoped FlagEnvironmentConfig (preloaded by
// the repository for exactly the requested environment) onto the flag. A
// flag with no config row for this environment is treated as disabled — new
// environments start every flag off until explicitly configured.
func toEnvFlag(flag *model.FeatureFlag) envFlag {
	ef := envFlag{Key: flag.Key, Variants: flag.Variants, Rules: flag.Rules}
	if len(flag.Configs) > 0 {
		ef.Enabled = flag.Configs[0].Enabled
		ef.DefaultVariant = flag.Configs[0].DefaultVariant
	} else if len(flag.Variants) > 0 {
		// No config row for this environment: fail closed to the first
		// variant rather than erroring, since "" wouldn't match any variant.
		ef.DefaultVariant = flag.Variants[0].Key
	}
	return ef
}

// evaluate runs the targeting engine for a single flag: disabled check, then
// ordered rule matching (first match wins, with optional percentage
// rollout), falling back to the flag's default variant.
func evaluateFlag(flag envFlag, evalCtx map[string]any) EvaluationResult {
	if !flag.Enabled {
		return resultFor(flag, flag.DefaultVariant, constants.ReasonDisabled)
	}

	rules := make([]model.FlagRule, len(flag.Rules))
	copy(rules, flag.Rules)
	sort.Slice(rules, func(i, j int) bool { return rules[i].Priority < rules[j].Priority })

	for _, rule := range rules {
		matched, err := evaluateCondition(rule.ConditionJSON, evalCtx)
		if err != nil {
			return EvaluationResult{Key: flag.Key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeGeneral, Value: nil}
		}
		if !matched {
			continue
		}
		variantKey := rule.VariantKey
		if len(rule.RolloutJSON) > 0 {
			bucketed, err := pickRolloutVariant(rule.RolloutJSON, flag.Key, evalCtx)
			if err == nil && bucketed != "" {
				variantKey = bucketed
			}
		}
		return resultFor(flag, variantKey, constants.ReasonTargetingMatch)
	}

	if len(flag.Rules) == 0 {
		return resultFor(flag, flag.DefaultVariant, constants.ReasonStatic)
	}
	return resultFor(flag, flag.DefaultVariant, constants.ReasonDefault)
}

func resultFor(flag envFlag, variantKey, reason string) EvaluationResult {
	for _, v := range flag.Variants {
		if v.Key == variantKey {
			var value any
			_ = json.Unmarshal(v.Value, &value)
			return EvaluationResult{Key: flag.Key, Value: value, Reason: reason, Variant: variantKey}
		}
	}
	return EvaluationResult{Key: flag.Key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeGeneral}
}

type rolloutBucket struct {
	Variant    string  `json:"variant"`
	Percentage float64 `json:"percentage"`
}

// pickRolloutVariant deterministically buckets context.targetingKey into
// 0-100 using fnv hashing (salted with the flag key) so the same subject
// always lands in the same bucket for a given flag.
func pickRolloutVariant(rolloutJSON []byte, flagKey string, evalCtx map[string]any) (string, error) {
	var buckets []rolloutBucket
	if err := json.Unmarshal(rolloutJSON, &buckets); err != nil {
		return "", err
	}
	targetingKey, _ := evalCtx["targetingKey"].(string)
	if targetingKey == "" {
		return "", nil
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(flagKey + ":" + targetingKey))
	bucket := float64(h.Sum32() % 100)

	var cumulative float64
	for _, b := range buckets {
		cumulative += b.Percentage
		if bucket < cumulative {
			return b.Variant, nil
		}
	}
	return "", nil
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
