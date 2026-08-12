package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
)

type fakeFlagRepository struct {
	byProjectAndKey map[string]*model.FeatureFlag
}

func newFakeFlagRepository() *fakeFlagRepository {
	return &fakeFlagRepository{byProjectAndKey: map[string]*model.FeatureFlag{}}
}

func flagKey(projectID uuid.UUID, key string) string { return projectID.String() + "|" + key }

func (f *fakeFlagRepository) List(ctx context.Context, projectID uuid.UUID) ([]model.FeatureFlag, error) {
	var out []model.FeatureFlag
	for _, fl := range f.byProjectAndKey {
		if fl.ProjectID == projectID {
			out = append(out, *fl)
		}
	}
	return out, nil
}

func (f *fakeFlagRepository) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.FeatureFlag, error) {
	fl, ok := f.byProjectAndKey[flagKey(projectID, key)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *fl
	return &cp, nil
}

func (f *fakeFlagRepository) Create(ctx context.Context, flag *model.FeatureFlag) error {
	if flag.ID == uuid.Nil {
		flag.ID = uuid.New()
	}
	f.byProjectAndKey[flagKey(flag.ProjectID, flag.Key)] = flag
	return nil
}

func (f *fakeFlagRepository) Update(ctx context.Context, flag *model.FeatureFlag) error {
	existing, ok := f.byProjectAndKey[flagKey(flag.ProjectID, flag.Key)]
	if !ok {
		return repository.ErrNotFound
	}
	existing.Name = flag.Name
	existing.Description = flag.Description
	existing.Enabled = flag.Enabled
	existing.DefaultVariant = flag.DefaultVariant
	return nil
}

func (f *fakeFlagRepository) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	delete(f.byProjectAndKey, flagKey(projectID, key))
	return nil
}

func (f *fakeFlagRepository) ReplaceVariantsAndRules(ctx context.Context, flagID uuid.UUID, variants []model.FlagVariant, rules []model.FlagRule) error {
	for _, fl := range f.byProjectAndKey {
		if fl.ID == flagID {
			fl.Variants = variants
			fl.Rules = rules
		}
	}
	return nil
}

func TestFlagService_CreateGetEvaluate(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	f, err := svc.Create(ctx, projectID, "new-checkout", "New checkout", "", "boolean", "off",
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}},
		nil)
	require.NoError(t, err)
	assert.Equal(t, "new-checkout", f.Key)

	got, err := svc.Get(ctx, projectID, "new-checkout")
	require.NoError(t, err)
	assert.Equal(t, "off", got.DefaultVariant)

	res := svc.Evaluate(ctx, projectID, "new-checkout", map[string]any{})
	assert.Equal(t, "off", res.Variant)

	_, err = svc.Get(ctx, projectID, "missing")
	assert.ErrorIs(t, err, ErrFlagNotFound)
}

func TestFlagService_Update(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.Create(ctx, projectID, "f1", "F1", "", "boolean", "off",
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil)
	require.NoError(t, err)

	disabled := false
	updated, err := svc.Update(ctx, projectID, "f1", UpdateInput{Enabled: &disabled})
	require.NoError(t, err)
	assert.False(t, updated.Enabled)

	_, err = svc.Update(ctx, projectID, "missing", UpdateInput{})
	assert.ErrorIs(t, err, ErrFlagNotFound)
}

func TestFlagService_EvaluateAll(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.Create(ctx, projectID, "f1", "F1", "", "boolean", "off",
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil)
	require.NoError(t, err)
	_, err = svc.Create(ctx, projectID, "f2", "F2", "", "boolean", "on",
		[]VariantInput{{Key: "on", Value: []byte("true")}, {Key: "off", Value: []byte("false")}}, nil)
	require.NoError(t, err)

	results, err := svc.EvaluateAll(ctx, projectID, map[string]any{})
	require.NoError(t, err)
	assert.Len(t, results, 2)
}

func TestFlagService_Evaluate_NotFound(t *testing.T) {
	svc := NewFlagService(newFakeFlagRepository())
	res := svc.Evaluate(context.Background(), uuid.New(), "missing", map[string]any{})
	assert.Equal(t, "FLAG_NOT_FOUND", res.ErrorCode)
}

func TestFlagService_Delete(t *testing.T) {
	repo := newFakeFlagRepository()
	svc := NewFlagService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.Create(ctx, projectID, "f1", "F1", "", "boolean", "off",
		[]VariantInput{{Key: "off", Value: []byte("false")}}, nil)
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, projectID, "f1"))
	_, err = svc.Get(ctx, projectID, "f1")
	assert.ErrorIs(t, err, ErrFlagNotFound)
}
