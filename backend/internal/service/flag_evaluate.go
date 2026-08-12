package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
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

// evaluate runs the targeting engine for a single flag: disabled check, then
// ordered rule matching (first match wins, with optional percentage
// rollout), falling back to the flag's default variant.
func evaluateFlag(flag *model.FeatureFlag, evalCtx map[string]any) EvaluationResult {
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

func resultFor(flag *model.FeatureFlag, variantKey, reason string) EvaluationResult {
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
	case "contains":
		return membershipCheck(right, left)
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
