package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

type fakeFlagRepository struct {
	byProjectAndKey map[string]*model.FeatureFlag
	configs         map[string]*model.FlagEnvironmentConfig // "flagID|envID"
}

func newFakeFlagRepository() *fakeFlagRepository {
	return &fakeFlagRepository{
		byProjectAndKey: map[string]*model.FeatureFlag{},
		configs:         map[string]*model.FlagEnvironmentConfig{},
	}
}

func flagKey(projectID uuid.UUID, key string) string { return projectID.String() + "|" + key }
func configKey(flagID, environmentID uuid.UUID) string {
	return flagID.String() + "|" + environmentID.String()
}

func (f *fakeFlagRepository) withEnv(fl *model.FeatureFlag, environmentID uuid.UUID) model.FeatureFlag {
	cp := *fl
	cp.Rules = nil
	for _, r := range fl.Rules {
		if r.EnvironmentID == environmentID {
			cp.Rules = append(cp.Rules, r)
		}
	}
	cp.Configs = nil
	if cfg, ok := f.configs[configKey(fl.ID, environmentID)]; ok {
		cp.Configs = []model.FlagEnvironmentConfig{*cfg}
	}
	return cp
}

func (f *fakeFlagRepository) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error) {
	var out []model.FeatureFlag
	for _, fl := range f.byProjectAndKey {
		if fl.ProjectID == projectID {
			out = append(out, f.withEnv(fl, environmentID))
		}
	}
	return out, nil
}

func (f *fakeFlagRepository) FindByKey(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error) {
	fl, ok := f.byProjectAndKey[flagKey(projectID, key)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := f.withEnv(fl, environmentID)
	return &cp, nil
}

func (f *fakeFlagRepository) Create(ctx context.Context, flag *model.FeatureFlag) error {
	if flag.ID == uuid.Nil {
		flag.ID = uuid.New()
	}
	stored := *flag
	f.byProjectAndKey[flagKey(flag.ProjectID, flag.Key)] = &stored
	return nil
}

func (f *fakeFlagRepository) UpdateCore(ctx context.Context, flag *model.FeatureFlag) error {
	existing, ok := f.byProjectAndKey[flagKey(flag.ProjectID, flag.Key)]
	if !ok {
		return repository.ErrNotFound
	}
	existing.Name = flag.Name
	existing.Description = flag.Description
	existing.PrerequisiteFlagKey = flag.PrerequisiteFlagKey
	existing.PrerequisiteVariant = flag.PrerequisiteVariant
	existing.UpdatedAt = time.Now()
	return nil
}

func (f *fakeFlagRepository) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	delete(f.byProjectAndKey, flagKey(projectID, key))
	return nil
}

func (f *fakeFlagRepository) ReplaceVariantsAndRules(ctx context.Context, flagID, environmentID uuid.UUID, variants []model.FlagVariant, rules []model.FlagRule) error {
	for _, fl := range f.byProjectAndKey {
		if fl.ID == flagID {
			fl.Variants = variants
			kept := make([]model.FlagRule, 0, len(fl.Rules))
			for _, r := range fl.Rules {
				if r.EnvironmentID != environmentID {
					kept = append(kept, r)
				}
			}
			fl.Rules = append(kept, rules...)
			fl.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (f *fakeFlagRepository) UpsertEnvironmentConfig(ctx context.Context, flagID, environmentID uuid.UUID, enabled bool, defaultVariant string) error {
	f.configs[configKey(flagID, environmentID)] = &model.FlagEnvironmentConfig{
		FlagID: flagID, EnvironmentID: environmentID, Enabled: enabled, DefaultVariant: defaultVariant,
	}
	for _, fl := range f.byProjectAndKey {
		if fl.ID == flagID {
			fl.UpdatedAt = time.Now()
		}
	}
	return nil
}

func (f *fakeFlagRepository) Version(ctx context.Context, projectID uuid.UUID) (int64, time.Time, error) {
	var count int64
	var maxUpdated time.Time
	for _, fl := range f.byProjectAndKey {
		if fl.ProjectID != projectID {
			continue
		}
		count++
		if fl.UpdatedAt.After(maxUpdated) {
			maxUpdated = fl.UpdatedAt
		}
	}
	return count, maxUpdated, nil
}

func TestFlagService_CreateGetEvaluate(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f, err := svc.Create(ctx, projectID, envID, "new-checkout", "New checkout", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}},
		nil, "", "")
	require.NoError(t, err)
	assert.Equal(t, "new-checkout", f.Key)

	got, err := svc.Get(ctx, projectID, envID, "new-checkout")
	require.NoError(t, err)
	assert.Equal(t, "off", got.Configs[0].DefaultVariant)

	res := svc.Evaluate(ctx, projectID, envID, "new-checkout", map[string]any{})
	assert.Equal(t, "off", res.Variant)

	_, err = svc.Get(ctx, projectID, envID, "missing")
	assert.ErrorIs(t, err, ErrFlagNotFound)
}

func TestFlagService_Create_OtherEnvironmentStartsDisabled(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	prodID := uuid.New()
	stagingID := uuid.New()

	_, err := svc.Create(ctx, projectID, prodID, "f1", "F1", "", "boolean", "on", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "", "")
	require.NoError(t, err)

	res := svc.Evaluate(ctx, projectID, stagingID, "f1", map[string]any{})
	assert.Equal(t, "DISABLED", res.Reason, "an environment with no config for a flag defaults to disabled")
}

func TestFlagService_Update(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	_, err := svc.Create(ctx, projectID, envID, "f1", "F1", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "", "")
	require.NoError(t, err)

	disabled := false
	updated, err := svc.Update(ctx, projectID, envID, "f1", UpdateInput{Enabled: &disabled})
	require.NoError(t, err)
	assert.False(t, updated.Configs[0].Enabled)

	_, err = svc.Update(ctx, projectID, envID, "missing", UpdateInput{})
	assert.ErrorIs(t, err, ErrFlagNotFound)
}

func TestFlagService_EvaluateAll(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	_, err := svc.Create(ctx, projectID, envID, "f1", "F1", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "", "")
	require.NoError(t, err)
	_, err = svc.Create(ctx, projectID, envID, "f2", "F2", "", "boolean", "on", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "", "")
	require.NoError(t, err)

	results, err := svc.EvaluateAll(ctx, projectID, envID, map[string]any{})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestFlagService_Evaluate_NotFound(t *testing.T) {
	svc := NewFlagService(newFakeFlagRepository())
	res := svc.Evaluate(context.Background(), uuid.New(), uuid.New(), "missing", map[string]any{})
	assert.Equal(t, "FLAG_NOT_FOUND", res.ErrorCode)
}

func TestFlagService_Delete(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	_, err := svc.Create(ctx, projectID, envID, "f1", "F1", "", "boolean", "off", true,
		[]VariantInput{{Key: "off", Value: []byte("false")}}, nil, "", "")
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, projectID, "f1"))
	_, err = svc.Get(ctx, projectID, envID, "f1")
	assert.ErrorIs(t, err, ErrFlagNotFound)
}

func TestFlagService_Prerequisite(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	_, err := svc.Create(ctx, projectID, envID, "base", "Base", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "", "")
	require.NoError(t, err)
	_, err = svc.Create(ctx, projectID, envID, "dependent", "Dependent", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}},
		[]RuleInput{{Priority: 1, ConditionJSON: []byte(`true`), VariantKey: "on"}}, "base", "on")
	require.NoError(t, err)

	res := svc.Evaluate(ctx, projectID, envID, "dependent", map[string]any{})
	assert.Equal(t, "off", res.Variant, "prerequisite defaults to off, so dependent should fall back to its default")
	assert.Equal(t, "PREREQUISITE_FAILED", res.Reason)

	_, err = svc.Update(ctx, projectID, envID, "base", UpdateInput{DefaultVariant: strPtr("on")})
	require.NoError(t, err)

	res = svc.Evaluate(ctx, projectID, envID, "dependent", map[string]any{})
	assert.Equal(t, "on", res.Variant, "prerequisite now resolves to 'on', so dependent's own rule should now apply normally")
	assert.Equal(t, "TARGETING_MATCH", res.Reason)
}

func TestFlagService_Prerequisite_MissingFlag(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	_, err := svc.Create(ctx, projectID, envID, "dependent", "Dependent", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "missing", "on")
	require.NoError(t, err)

	res := svc.Evaluate(ctx, projectID, envID, "dependent", map[string]any{})
	assert.Equal(t, "off", res.Variant)
	assert.Equal(t, "PREREQUISITE_FAILED", res.Reason)
}

func TestFlagService_Prerequisite_CycleGuard(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	_, err := svc.Create(ctx, projectID, envID, "a", "A", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "b", "on")
	require.NoError(t, err)
	_, err = svc.Create(ctx, projectID, envID, "b", "B", "", "boolean", "off", true,
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil, "a", "on")
	require.NoError(t, err)

	res := svc.Evaluate(ctx, projectID, envID, "a", map[string]any{})
	assert.Equal(t, "off", res.Variant, "cyclical prerequisites must not hang and should fail closed")
}

func TestFlagService_Version(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	v1, err := svc.Version(ctx, projectID)
	require.NoError(t, err)

	_, err = svc.Create(ctx, projectID, envID, "f1", "F1", "", "boolean", "off", true,
		[]VariantInput{{Key: "off", Value: []byte("false")}}, nil, "", "")
	require.NoError(t, err)

	v2, err := svc.Version(ctx, projectID)
	require.NoError(t, err)
	assert.NotEqual(t, v1, v2, "version must change once a flag is created")
}

func strPtr(s string) *string { return &s }
