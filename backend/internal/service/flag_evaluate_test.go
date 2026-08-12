package service

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
)

func boolFlag(enabled bool) *model.FeatureFlag {
	return &model.FeatureFlag{
		Key:            "new-checkout",
		Enabled:        enabled,
		DefaultVariant: "off",
		Variants: []model.FlagVariant{
			{Key: "on", Value: datatypes.JSON(`true`)},
			{Key: "off", Value: datatypes.JSON(`false`)},
		},
	}
}

func TestEvaluateFlag_Disabled(t *testing.T) {
	f := boolFlag(false)
	res := evaluateFlag(f, map[string]any{})
	assert.Equal(t, constants.ReasonDisabled, res.Reason)
	assert.Equal(t, "off", res.Variant)
	assert.Equal(t, false, res.Value)
}

func TestEvaluateFlag_StaticNoRules(t *testing.T) {
	f := boolFlag(true)
	res := evaluateFlag(f, map[string]any{})
	assert.Equal(t, constants.ReasonStatic, res.Reason)
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluateFlag_TargetingMatch(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{
			Priority:      1,
			ConditionJSON: datatypes.JSON(`{"==": [{"var": "plan"}, "pro"]}`),
			VariantKey:    "on",
		},
	}
	res := evaluateFlag(f, map[string]any{"plan": "pro"})
	assert.Equal(t, constants.ReasonTargetingMatch, res.Reason)
	assert.Equal(t, "on", res.Variant)
	assert.Equal(t, true, res.Value)
}

func TestEvaluateFlag_DefaultWhenNoRuleMatches(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{Priority: 1, ConditionJSON: datatypes.JSON(`{"==": [{"var": "plan"}, "pro"]}`), VariantKey: "on"},
	}
	res := evaluateFlag(f, map[string]any{"plan": "free"})
	assert.Equal(t, constants.ReasonDefault, res.Reason)
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluateFlag_RulePriorityOrder(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{Priority: 2, ConditionJSON: datatypes.JSON(`true`), VariantKey: "off"},
		{Priority: 1, ConditionJSON: datatypes.JSON(`true`), VariantKey: "on"},
	}
	res := evaluateFlag(f, map[string]any{})
	assert.Equal(t, "on", res.Variant, "lower priority number should be evaluated first")
}

func TestEvaluateFlag_AndOrOperators(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{
			Priority: 1,
			ConditionJSON: datatypes.JSON(`{"and": [
				{"==": [{"var": "plan"}, "pro"]},
				{"or": [{"==": [{"var": "country"}, "US"]}, {"==": [{"var": "country"}, "BR"]}]}
			]}`),
			VariantKey: "on",
		},
	}
	res := evaluateFlag(f, map[string]any{"plan": "pro", "country": "BR"})
	assert.Equal(t, "on", res.Variant)

	res = evaluateFlag(f, map[string]any{"plan": "pro", "country": "FR"})
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluateFlag_InAndContains(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{Priority: 1, ConditionJSON: datatypes.JSON(`{"in": [{"var": "country"}, ["US", "BR"]]}`), VariantKey: "on"},
	}
	res := evaluateFlag(f, map[string]any{"country": "BR"})
	assert.Equal(t, "on", res.Variant)

	f.Rules[0].ConditionJSON = datatypes.JSON(`{"contains": [{"var": "email"}, "@acme.com"]}`)
	res = evaluateFlag(f, map[string]any{"email": "a@acme.com"})
	assert.Equal(t, "on", res.Variant)
}

func TestEvaluateFlag_NumberComparisons(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{Priority: 1, ConditionJSON: datatypes.JSON(`{">": [{"var": "age"}, 18]}`), VariantKey: "on"},
	}
	res := evaluateFlag(f, map[string]any{"age": float64(21)})
	assert.Equal(t, "on", res.Variant)

	res = evaluateFlag(f, map[string]any{"age": float64(10)})
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluateFlag_InvalidConditionReturnsError(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{Priority: 1, ConditionJSON: datatypes.JSON(`{"unsupported-op": [1, 2]}`), VariantKey: "on"},
	}
	res := evaluateFlag(f, map[string]any{})
	assert.Equal(t, constants.ReasonError, res.Reason)
}

func TestEvaluateFlag_RolloutDeterministic(t *testing.T) {
	f := boolFlag(true)
	f.Rules = []model.FlagRule{
		{
			Priority:      1,
			ConditionJSON: datatypes.JSON(`true`),
			VariantKey:    "off",
			RolloutJSON:   datatypes.JSON(`[{"variant":"on","percentage":100}]`),
		},
	}
	ctx := map[string]any{"targetingKey": uuid.NewString()}
	first := evaluateFlag(f, ctx)
	second := evaluateFlag(f, ctx)
	assert.Equal(t, first.Variant, second.Variant, "same targetingKey must bucket consistently")
	assert.Equal(t, "on", first.Variant, "100% rollout should always pick the target variant")
}

func TestEvaluateFlag_UnknownVariantIsError(t *testing.T) {
	f := boolFlag(true)
	f.DefaultVariant = "does-not-exist"
	res := evaluateFlag(f, map[string]any{})
	assert.Equal(t, constants.ReasonError, res.Reason)
}
