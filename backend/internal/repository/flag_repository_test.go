package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"leaflag/backend/internal/model"
)

func TestFlagRepository_CreateFindListDelete(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "new-checkout", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))
	require.NotEqual(t, uuid.Nil, f.ID)

	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID, envID,
		[]model.FlagVariant{{FlagID: f.ID, Key: "on", Value: datatypes.JSON(`true`)}, {FlagID: f.ID, Key: "off", Value: datatypes.JSON(`false`)}},
		[]model.FlagRule{{FlagID: f.ID, EnvironmentID: envID, Priority: 1, ConditionJSON: datatypes.JSON(`true`), VariantKey: "on"}},
	))
	require.NoError(t, repo.UpsertEnvironmentConfig(ctx, f.ID, envID, true, "off"))

	found, err := repo.FindByKey(ctx, projectID, envID, "new-checkout")
	require.NoError(t, err)
	assert.Len(t, found.Variants, 2)
	require.Len(t, found.Rules, 1)
	assert.Equal(t, "on", found.Rules[0].VariantKey)
	require.Len(t, found.Configs, 1)
	assert.True(t, found.Configs[0].Enabled)

	list, err := repo.List(ctx, projectID, envID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Delete(ctx, projectID, "new-checkout"))
	_, err = repo.FindByKey(ctx, projectID, envID, "new-checkout")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFlagRepository_ReplaceVariantsAndRules_Overwrites(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID, envID,
		[]model.FlagVariant{{FlagID: f.ID, Key: "on", Value: datatypes.JSON(`true`)}}, nil))
	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Variants, 1)

	// Replacing again should drop the old variant, not append to it.
	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID, envID,
		[]model.FlagVariant{{FlagID: f.ID, Key: "off", Value: datatypes.JSON(`false`)}}, nil))
	found, err = repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Variants, 1)
	assert.Equal(t, "off", found.Variants[0].Key)
}

func TestFlagRepository_ReplaceVariantsAndRules_ScopedToEnvironment(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	prodID := uuid.New()
	stagingID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID, prodID, nil,
		[]model.FlagRule{{FlagID: f.ID, EnvironmentID: prodID, Priority: 1, ConditionJSON: datatypes.JSON(`true`), VariantKey: "on"}}))
	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID, stagingID, nil,
		[]model.FlagRule{{FlagID: f.ID, EnvironmentID: stagingID, Priority: 1, ConditionJSON: datatypes.JSON(`true`), VariantKey: "off"}}))

	prodFlag, err := repo.FindByKey(ctx, projectID, prodID, "f1")
	require.NoError(t, err)
	require.Len(t, prodFlag.Rules, 1)
	assert.Equal(t, "on", prodFlag.Rules[0].VariantKey, "rules for one environment must not leak into another")
}

func TestFlagRepository_UpdateCore(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	f.Name = "F1 renamed"
	require.NoError(t, repo.UpdateCore(ctx, f))

	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	assert.Equal(t, "F1 renamed", found.Name)
}

func TestFlagRepository_UpsertEnvironmentConfig(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.UpsertEnvironmentConfig(ctx, f.ID, envID, true, "on"))
	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Configs, 1)
	assert.True(t, found.Configs[0].Enabled)
	assert.Equal(t, "on", found.Configs[0].DefaultVariant)

	// Upserting again should update the existing row, not duplicate it.
	require.NoError(t, repo.UpsertEnvironmentConfig(ctx, f.ID, envID, false, "off"))
	found, err = repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Configs, 1)
	assert.False(t, found.Configs[0].Enabled)
	assert.Equal(t, "off", found.Configs[0].DefaultVariant)
}

func TestFlagRepository_Version(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	count, _, err := repo.Version(ctx, projectID)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	count, maxUpdated, err := repo.Version(ctx, projectID)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
	assert.False(t, maxUpdated.IsZero())

	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID, envID,
		[]model.FlagVariant{{FlagID: f.ID, Key: "off", Value: datatypes.JSON(`false`)}}, nil))

	_, maxUpdatedAfterReplace, err := repo.Version(ctx, projectID)
	require.NoError(t, err)
	assert.True(t, !maxUpdatedAfterReplace.Before(maxUpdated), "replacing variants/rules should bump the flag's updated_at")
}
