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

func defaultStrategy(flagID, envID uuid.UUID, variantKey string) model.FlagStrategy {
	return model.FlagStrategy{
		FlagID: flagID, EnvironmentID: envID, IsDefault: true, DefaultVariant: variantKey,
		Variants: []model.FlagStrategyVariant{
			{Key: "on", Value: datatypes.JSON(`true`)},
			{Key: "off", Value: datatypes.JSON(`false`)},
		},
	}
}

func TestFlagRepository_CreateFindListDelete(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "new-checkout", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))
	require.NotEqual(t, uuid.Nil, f.ID)

	require.NoError(t, repo.ReplaceStrategies(ctx, f.ID, envID, []model.FlagStrategy{
		{
			FlagID: f.ID, EnvironmentID: envID, Priority: 1,
			ConditionJSON: datatypes.JSON(`true`), DefaultVariant: "on",
			Variants: []model.FlagStrategyVariant{{Key: "on", Value: datatypes.JSON(`true`)}},
		},
		defaultStrategy(f.ID, envID, "off"),
	}))
	require.NoError(t, repo.UpsertEnvironmentConfig(ctx, f.ID, envID, true))

	found, err := repo.FindByKey(ctx, projectID, envID, "new-checkout")
	require.NoError(t, err)
	require.Len(t, found.Strategies, 2)
	assert.Equal(t, "off", found.Strategies[0].DefaultVariant, "strategies sort by priority")
	assert.Equal(t, 0, found.Strategies[0].Priority)
	require.Len(t, found.Configs, 1)
	assert.True(t, found.Configs[0].Enabled)

	list, err := repo.List(ctx, projectID, envID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Delete(ctx, projectID, "new-checkout"))
	_, err = repo.FindByKey(ctx, projectID, envID, "new-checkout")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFlagRepository_ReplaceStrategies_Overwrites(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.ReplaceStrategies(ctx, f.ID, envID, []model.FlagStrategy{defaultStrategy(f.ID, envID, "on")}))
	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Strategies, 1)

	// Replacing again should drop the old strategy (and its variants), not append to it.
	require.NoError(t, repo.ReplaceStrategies(ctx, f.ID, envID, []model.FlagStrategy{defaultStrategy(f.ID, envID, "off")}))
	found, err = repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Strategies, 1)
	assert.Equal(t, "off", found.Strategies[0].DefaultVariant)
	assert.Len(t, found.Strategies[0].Variants, 2)
}

func TestFlagRepository_ReplaceStrategies_RejectsDuplicatePriorities(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	err := repo.ReplaceStrategies(ctx, f.ID, envID, []model.FlagStrategy{
		defaultStrategy(f.ID, envID, "on"),
		defaultStrategy(f.ID, envID, "off"),
	})
	assert.ErrorIs(t, err, ErrDuplicateStrategyPriority)
}

func TestFlagStrategyPriorityHasDatabaseConstraint(t *testing.T) {
	db := newTestDB(t)
	strategy := model.FlagStrategy{FlagID: uuid.New(), EnvironmentID: uuid.New(), Priority: 1, DefaultVariant: "on"}
	require.NoError(t, db.Create(&strategy).Error)

	duplicate := model.FlagStrategy{FlagID: strategy.FlagID, EnvironmentID: strategy.EnvironmentID, Priority: strategy.Priority, DefaultVariant: "off"}
	assert.Error(t, db.Create(&duplicate).Error)
}

func TestFlagRepository_ReplaceStrategies_ScopedToEnvironment(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	prodID := uuid.New()
	stagingID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.ReplaceStrategies(ctx, f.ID, prodID, []model.FlagStrategy{defaultStrategy(f.ID, prodID, "on")}))
	require.NoError(t, repo.ReplaceStrategies(ctx, f.ID, stagingID, []model.FlagStrategy{defaultStrategy(f.ID, stagingID, "off")}))

	prodFlag, err := repo.FindByKey(ctx, projectID, prodID, "f1")
	require.NoError(t, err)
	require.Len(t, prodFlag.Strategies, 1)
	assert.Equal(t, "on", prodFlag.Strategies[0].DefaultVariant, "strategies for one environment must not leak into another")
}

func TestFlagRepository_UpdateCore(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean", Tags: datatypes.NewJSONSlice([]string{"checkout"})}
	require.NoError(t, repo.Create(ctx, f))

	f.Name = "F1 renamed"
	f.Tags = datatypes.NewJSONSlice([]string{"checkout", "growth"})
	require.NoError(t, repo.UpdateCore(ctx, f))

	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	assert.Equal(t, "F1 renamed", found.Name)
	assert.Equal(t, []string{"checkout", "growth"}, []string(found.Tags))
}

func TestFlagRepository_Collaborators(t *testing.T) {
	db := newTestDB(t)
	repo := NewFlagRepository(db)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()
	creator := &model.User{Email: "joao@example.com", Name: "João"}
	require.NoError(t, db.Create(creator).Error)

	f := &model.FeatureFlag{
		ProjectID:     projectID,
		Key:           "f1",
		FlagType:      "boolean",
		CreatedByID:   &creator.ID,
		Collaborators: []model.FlagCollaborator{{UserID: creator.ID}},
	}
	require.NoError(t, repo.Create(ctx, f))

	other := &model.User{Email: "david@example.com", Name: "David"}
	require.NoError(t, db.Create(other).Error)
	require.NoError(t, repo.AddCollaborator(ctx, f.ID, other.ID))
	require.NoError(t, repo.AddCollaborator(ctx, f.ID, other.ID), "adding the same collaborator twice is idempotent")

	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.NotNil(t, found.CreatedBy)
	assert.Equal(t, "João", found.CreatedBy.Name)
	require.Len(t, found.Collaborators, 2)
	assert.ElementsMatch(t, []string{"João", "David"}, []string{found.Collaborators[0].User.Name, found.Collaborators[1].User.Name})
}

func TestFlagRepository_SetArchived(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()
	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.SetArchived(ctx, projectID, "f1", true))
	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.NotNil(t, found.ArchivedAt)

	require.NoError(t, repo.SetArchived(ctx, projectID, "f1", false))
	found, err = repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	assert.Nil(t, found.ArchivedAt)
}

func TestFlagRepository_UpsertEnvironmentConfig(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.UpsertEnvironmentConfig(ctx, f.ID, envID, true))
	found, err := repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Configs, 1)
	assert.True(t, found.Configs[0].Enabled)

	// Upserting again should update the existing row, not duplicate it.
	require.NoError(t, repo.UpsertEnvironmentConfig(ctx, f.ID, envID, false))
	found, err = repo.FindByKey(ctx, projectID, envID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Configs, 1)
	assert.False(t, found.Configs[0].Enabled)
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

	require.NoError(t, repo.ReplaceStrategies(ctx, f.ID, envID, []model.FlagStrategy{defaultStrategy(f.ID, envID, "off")}))

	_, maxUpdatedAfterReplace, err := repo.Version(ctx, projectID)
	require.NoError(t, err)
	assert.True(t, !maxUpdatedAfterReplace.Before(maxUpdated), "replacing strategies should bump the flag's updated_at")
}
