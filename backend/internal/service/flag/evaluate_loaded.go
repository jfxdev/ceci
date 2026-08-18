package flag

import (
	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/service/strategy"
)

// EvaluateLoaded evaluates flags that were already loaded for exactly one
// project/environment. It is used by the data plane, which intentionally
// has no database connection.
func EvaluateLoaded(flags []model.FeatureFlag, key string, evalCtx map[string]any) EvaluationResult {
	byKey := make(map[string]*model.FeatureFlag, len(flags))
	for i := range flags {
		byKey[flags[i].Key] = &flags[i]
	}
	flag, ok := byKey[key]
	if !ok {
		return EvaluationResult{Key: key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeFlagNotFound}
	}
	return resolveLoadedFlag(flag, byKey, evalCtx, map[string]bool{key: true})
}

// EvaluateAllLoaded evaluates every flag in the supplied loaded scope.
func EvaluateAllLoaded(flags []model.FeatureFlag, evalCtx map[string]any) []EvaluationResult {
	results := make([]EvaluationResult, 0, len(flags))
	byKey := make(map[string]*model.FeatureFlag, len(flags))
	for i := range flags {
		byKey[flags[i].Key] = &flags[i]
	}
	for i := range flags {
		results = append(results, resolveLoadedFlag(&flags[i], byKey, evalCtx, map[string]bool{flags[i].Key: true}))
	}
	return results
}

func resolveLoadedFlag(flag *model.FeatureFlag, byKey map[string]*model.FeatureFlag, evalCtx map[string]any, visited map[string]bool) EvaluationResult {
	if flag.PrerequisiteFlagKey == "" {
		return strategy.Evaluate(strategy.Flatten(flag), evalCtx)
	}
	if visited[flag.PrerequisiteFlagKey] || len(visited) >= maxPrerequisiteDepth {
		return strategy.DefaultResult(strategy.Flatten(flag), constants.ReasonPrerequisiteFailed)
	}
	prereq, ok := byKey[flag.PrerequisiteFlagKey]
	if !ok {
		return strategy.DefaultResult(strategy.Flatten(flag), constants.ReasonPrerequisiteFailed)
	}
	visited[flag.PrerequisiteFlagKey] = true
	prereqResult := resolveLoadedFlag(prereq, byKey, evalCtx, visited)
	if prereqResult.Variant != flag.PrerequisiteVariant {
		return strategy.DefaultResult(strategy.Flatten(flag), constants.ReasonPrerequisiteFailed)
	}
	return strategy.Evaluate(strategy.Flatten(flag), evalCtx)
}
