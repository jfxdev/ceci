package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"

	"ceci/backend/internal/model"
)

func TestFlagRepository_CreateFindListDelete(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "new-checkout", FlagType: "boolean", Enabled: true, DefaultVariant: "off"}
	require.NoError(t, repo.Create(ctx, f))
	require.NotEqual(t, uuid.Nil, f.ID)

	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID,
		[]model.FlagVariant{{FlagID: f.ID, Key: "on", Value: datatypes.JSON(`true`)}, {FlagID: f.ID, Key: "off", Value: datatypes.JSON(`false`)}},
		[]model.FlagRule{{FlagID: f.ID, Priority: 1, ConditionJSON: datatypes.JSON(`true`), VariantKey: "on"}},
	))

	found, err := repo.FindByKey(ctx, projectID, "new-checkout")
	require.NoError(t, err)
	assert.Len(t, found.Variants, 2)
	require.Len(t, found.Rules, 1)
	assert.Equal(t, "on", found.Rules[0].VariantKey)

	list, err := repo.List(ctx, projectID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Delete(ctx, projectID, "new-checkout"))
	_, err = repo.FindByKey(ctx, projectID, "new-checkout")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestFlagRepository_ReplaceVariantsAndRules_Overwrites(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean", Enabled: true, DefaultVariant: "off"}
	require.NoError(t, repo.Create(ctx, f))

	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID,
		[]model.FlagVariant{{FlagID: f.ID, Key: "on", Value: datatypes.JSON(`true`)}}, nil))
	found, err := repo.FindByKey(ctx, projectID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Variants, 1)

	// Replacing again should drop the old variant, not append to it.
	require.NoError(t, repo.ReplaceVariantsAndRules(ctx, f.ID,
		[]model.FlagVariant{{FlagID: f.ID, Key: "off", Value: datatypes.JSON(`false`)}}, nil))
	found, err = repo.FindByKey(ctx, projectID, "f1")
	require.NoError(t, err)
	require.Len(t, found.Variants, 1)
	assert.Equal(t, "off", found.Variants[0].Key)
}

func TestFlagRepository_Update(t *testing.T) {
	repo := NewFlagRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()

	f := &model.FeatureFlag{ProjectID: projectID, Key: "f1", FlagType: "boolean", Enabled: true, DefaultVariant: "off"}
	require.NoError(t, repo.Create(ctx, f))

	f.Enabled = false
	f.Name = "F1 renamed"
	require.NoError(t, repo.Update(ctx, f))

	found, err := repo.FindByKey(ctx, projectID, "f1")
	require.NoError(t, err)
	assert.False(t, found.Enabled)
	assert.Equal(t, "F1 renamed", found.Name)
}
