package strategy

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/datatypes"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
)

func boolVariants() []model.FlagStrategyVariant {
	return []model.FlagStrategyVariant{
		{Key: "on", Value: datatypes.JSON(`true`)},
		{Key: "off", Value: datatypes.JSON(`false`)},
	}
}

// boolFlag returns a Flag with just the mandatory catch-all default strategy
// (serving "off"). Tests append additional, higher-priority strategies to
// exercise targeting/rollout ahead of the default.
func boolFlag(enabled bool) Flag {
	return Flag{
		Key:     "new-checkout",
		Enabled: enabled,
		Strategies: []model.FlagStrategy{
			{IsDefault: true, DefaultVariant: "off", Variants: boolVariants()},
		},
	}
}

// targetingStrategy builds a non-default strategy that serves variantKey
// when condition matches.
func targetingStrategy(priority int, condition string, variantKey string) model.FlagStrategy {
	return model.FlagStrategy{
		Priority:       priority,
		ConditionJSON:  datatypes.JSON(condition),
		DefaultVariant: variantKey,
		Variants:       boolVariants(),
	}
}

func TestEvaluate_Disabled(t *testing.T) {
	f := boolFlag(false)
	res := Evaluate(f, map[string]any{})
	assert.Equal(t, constants.ReasonDisabled, res.Reason)
	assert.Equal(t, "off", res.Variant)
	assert.Equal(t, false, res.Value)
}

func TestEvaluate_StaticNoRules(t *testing.T) {
	f := boolFlag(true)
	res := Evaluate(f, map[string]any{})
	assert.Equal(t, constants.ReasonStatic, res.Reason)
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluate_TargetingMatch(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"==": [{"var": "plan"}, "pro"]}`, "on"))
	res := Evaluate(f, map[string]any{"plan": "pro"})
	assert.Equal(t, constants.ReasonTargetingMatch, res.Reason)
	assert.Equal(t, "on", res.Variant)
	assert.Equal(t, true, res.Value)
}

func TestEvaluate_DefaultWhenNoRuleMatches(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"==": [{"var": "plan"}, "pro"]}`, "on"))
	res := Evaluate(f, map[string]any{"plan": "free"})
	assert.Equal(t, constants.ReasonDefault, res.Reason)
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluate_RulePriorityOrder(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies,
		targetingStrategy(2, `true`, "off"),
		targetingStrategy(1, `true`, "on"),
	)
	res := Evaluate(f, map[string]any{})
	assert.Equal(t, "on", res.Variant, "lower priority number should be evaluated first")
}

func TestEvaluate_AndOrOperators(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"and": [
		{"==": [{"var": "plan"}, "pro"]},
		{"or": [{"==": [{"var": "country"}, "US"]}, {"==": [{"var": "country"}, "BR"]}]}
	]}`, "on"))

	res := Evaluate(f, map[string]any{"plan": "pro", "country": "BR"})
	assert.Equal(t, "on", res.Variant)

	res = Evaluate(f, map[string]any{"plan": "pro", "country": "FR"})
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluate_InAndContains(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"in": [{"var": "country"}, ["US", "BR"]]}`, "on"))
	res := Evaluate(f, map[string]any{"country": "BR"})
	assert.Equal(t, "on", res.Variant)

	f.Strategies[1].ConditionJSON = datatypes.JSON(`{"contains": [{"var": "email"}, "@acme.com"]}`)
	res = Evaluate(f, map[string]any{"email": "a@acme.com"})
	assert.Equal(t, "on", res.Variant)
}

func TestEvaluate_NotInAndNotContains(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"not in": [{"var": "country"}, ["US", "BR"]]}`, "on"))
	res := Evaluate(f, map[string]any{"country": "FR"})
	assert.Equal(t, "on", res.Variant, "FR is not in [US, BR]")

	res = Evaluate(f, map[string]any{"country": "BR"})
	assert.Equal(t, "off", res.Variant, "BR is in [US, BR], so 'not in' should not match")

	f.Strategies[1].ConditionJSON = datatypes.JSON(`{"not contains": [{"var": "email"}, "@acme.com"]}`)
	res = Evaluate(f, map[string]any{"email": "a@other.com"})
	assert.Equal(t, "on", res.Variant, "email doesn't contain @acme.com")

	res = Evaluate(f, map[string]any{"email": "a@acme.com"})
	assert.Equal(t, "off", res.Variant, "email contains @acme.com, so 'not contains' should not match")
}

func TestEvaluate_NumberComparisons(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{">": [{"var": "age"}, 18]}`, "on"))
	res := Evaluate(f, map[string]any{"age": float64(21)})
	assert.Equal(t, "on", res.Variant)

	res = Evaluate(f, map[string]any{"age": float64(10)})
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluate_InvalidConditionReturnsError(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"unsupported-op": [1, 2]}`, "on"))
	res := Evaluate(f, map[string]any{})
	assert.Equal(t, constants.ReasonError, res.Reason)
}

func TestEvaluate_RolloutDeterministic(t *testing.T) {
	f := boolFlag(true)
	st := targetingStrategy(1, `true`, "off")
	st.RolloutJSON = datatypes.JSON(`[{"variant":"on","percentage":100}]`)
	f.Strategies = append(f.Strategies, st)

	ctx := map[string]any{"targetingKey": uuid.NewString()}
	first := Evaluate(f, ctx)
	second := Evaluate(f, ctx)
	assert.Equal(t, first.Variant, second.Variant, "same targetingKey must bucket consistently")
	assert.Equal(t, "on", first.Variant, "100% rollout should always pick the target variant")
}

func TestEvaluate_UnknownVariantIsError(t *testing.T) {
	f := boolFlag(true)
	f.Strategies[0].DefaultVariant = "does-not-exist"
	res := Evaluate(f, map[string]any{})
	assert.Equal(t, constants.ReasonError, res.Reason)
}

func TestEvaluate_NotOperator(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"!": {"==": [{"var": "plan"}, "free"]}}`, "on"))
	res := Evaluate(f, map[string]any{"plan": "pro"})
	assert.Equal(t, "on", res.Variant, "not-equal-free should match for plan=pro")

	res = Evaluate(f, map[string]any{"plan": "free"})
	assert.Equal(t, "off", res.Variant, "not-equal-free should not match for plan=free")
}

func TestEvaluate_NotOperator_ArrayWrapped(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"!": [{"==": [{"var": "plan"}, "free"]}]}`, "on"))
	res := Evaluate(f, map[string]any{"plan": "pro"})
	assert.Equal(t, "on", res.Variant)
}

func TestEvaluate_SemverOperators(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"semver>": [{"var": "appVersion"}, "1.2.0"]}`, "on"))
	res := Evaluate(f, map[string]any{"appVersion": "1.10.0"})
	assert.Equal(t, "on", res.Variant, "1.10.0 > 1.2.0 numerically, not lexically")

	res = Evaluate(f, map[string]any{"appVersion": "1.1.0"})
	assert.Equal(t, "off", res.Variant)

	f.Strategies[1].ConditionJSON = datatypes.JSON(`{"semver=": [{"var": "appVersion"}, "2.0.0"]}`)
	res = Evaluate(f, map[string]any{"appVersion": "2.0.0-beta.1"})
	assert.Equal(t, "on", res.Variant, "pre-release suffix is ignored for equality")

	f.Strategies[1].ConditionJSON = datatypes.JSON(`{"semver<": [{"var": "appVersion"}, "1.0.0"]}`)
	res = Evaluate(f, map[string]any{"appVersion": "0.9.9"})
	assert.Equal(t, "on", res.Variant)
}

func TestEvaluate_MatchesRegexOperator(t *testing.T) {
	f := boolFlag(true)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"matches": [{"var": "email"}, "^[a-z]+@acme\\.com$"]}`, "on"))
	res := Evaluate(f, map[string]any{"email": "jane@acme.com"})
	assert.Equal(t, "on", res.Variant)

	res = Evaluate(f, map[string]any{"email": "jane@other.com"})
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluate_DefaultStrategyRollout(t *testing.T) {
	f := boolFlag(true)
	f.Strategies[0].RolloutJSON = datatypes.JSON(`[{"variant":"on","percentage":100}]`)
	res := Evaluate(f, map[string]any{"targetingKey": "user-1"})
	assert.Equal(t, constants.ReasonSplit, res.Reason)
	assert.Equal(t, "on", res.Variant)

	// No rollout falls through to the default strategy's DefaultVariant.
	f.Strategies[0].RolloutJSON = nil
	res = Evaluate(f, map[string]any{"targetingKey": "user-1"})
	assert.Equal(t, "off", res.Variant)
	assert.NotEqual(t, constants.ReasonSplit, res.Reason)

	// No targetingKey means the subject can't be bucketed: default, not split.
	f.Strategies[0].RolloutJSON = datatypes.JSON(`[{"variant":"on","percentage":100}]`)
	res = Evaluate(f, map[string]any{})
	assert.Equal(t, "off", res.Variant)
}

func TestEvaluate_DefaultStrategyRolloutIsDeterministic(t *testing.T) {
	f := boolFlag(true)
	f.Strategies[0].RolloutJSON = datatypes.JSON(`[{"variant":"on","percentage":50}]`)
	ctx := map[string]any{"targetingKey": "stable-subject"}
	first := Evaluate(f, ctx)
	second := Evaluate(f, ctx)
	assert.Equal(t, first.Variant, second.Variant)
}

func TestEvaluate_TargetingStrategyOverridesDefaultRollout(t *testing.T) {
	f := boolFlag(true)
	f.Strategies[0].RolloutJSON = datatypes.JSON(`[{"variant":"on","percentage":100}]`)
	f.Strategies = append(f.Strategies, targetingStrategy(1, `{"==": [{"var": "plan"}, "free"]}`, "off"))
	res := Evaluate(f, map[string]any{"plan": "free", "targetingKey": "user-1"})
	assert.Equal(t, constants.ReasonTargetingMatch, res.Reason)
	assert.Equal(t, "off", res.Variant)
}

func TestValidateVariantValues(t *testing.T) {
	ok := []VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}
	assert.NoError(t, ValidateVariantValues(constants.FlagTypeBoolean, ok))

	assert.NoError(t, ValidateVariantValues(constants.FlagTypeNumber, []VariantInput{{Key: "a", Value: []byte("3.5")}}))
	assert.NoError(t, ValidateVariantValues(constants.FlagTypeString, []VariantInput{{Key: "a", Value: []byte(`"hi"`)}}))
	assert.NoError(t, ValidateVariantValues(constants.FlagTypeObject, []VariantInput{{Key: "a", Value: []byte(`{"x":1}`)}}))

	// Mismatches are rejected.
	assert.ErrorIs(t, ValidateVariantValues(constants.FlagTypeBoolean, []VariantInput{{Key: "a", Value: []byte(`"true"`)}}), ErrVariantTypeMismatch)
	assert.ErrorIs(t, ValidateVariantValues(constants.FlagTypeObject, []VariantInput{{Key: "a", Value: []byte("42")}}), ErrVariantTypeMismatch)
	assert.ErrorIs(t, ValidateVariantValues("nonsense", ok), ErrUnknownFlagType)
}

func TestDefaultVariantsFor(t *testing.T) {
	variants, def := DefaultVariantsFor(constants.FlagTypeBoolean)
	assert.Equal(t, "B", def)
	assert.Len(t, variants, 2)
	assert.NoError(t, ValidateVariantValues(constants.FlagTypeBoolean, variants))

	variants, def = DefaultVariantsFor(constants.FlagTypeObject)
	assert.Equal(t, "default", def)
	assert.NoError(t, ValidateVariantValues(constants.FlagTypeObject, variants))
}

func TestValidateStrategies_RequiresExactlyOneDefault(t *testing.T) {
	oneDefault := []Input{{IsDefault: true, DefaultVariant: "off", Variants: []VariantInput{{Key: "off", Value: []byte("false")}}}}
	assert.NoError(t, Validate(constants.FlagTypeBoolean, oneDefault))

	noDefault := []Input{{DefaultVariant: "off", Variants: []VariantInput{{Key: "off", Value: []byte("false")}}}}
	assert.ErrorIs(t, Validate(constants.FlagTypeBoolean, noDefault), ErrDefaultCount)

	twoDefaults := append(oneDefault, oneDefault[0])
	assert.ErrorIs(t, Validate(constants.FlagTypeBoolean, twoDefaults), ErrDefaultCount)
}
