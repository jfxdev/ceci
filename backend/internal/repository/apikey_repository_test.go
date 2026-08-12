package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ceci/backend/internal/model"
)

func TestAPIKeyRepository_CreateFindListRevoke(t *testing.T) {
	repo := NewAPIKeyRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()

	key := &model.ProjectAPIKey{ProjectID: projectID, Label: "CI", KeyHash: "hash1", Prefix: "ceci_sk_ab"}
	require.NoError(t, repo.Create(ctx, key))
	require.NotEqual(t, uuid.Nil, key.ID)

	found, err := repo.FindActiveByHash(ctx, "hash1")
	require.NoError(t, err)
	assert.Equal(t, projectID, found.ProjectID)

	list, err := repo.List(ctx, projectID)
	require.NoError(t, err)
	assert.Len(t, list, 1)

	require.NoError(t, repo.Revoke(ctx, key.ID))
	_, err = repo.FindActiveByHash(ctx, "hash1")
	assert.ErrorIs(t, err, ErrNotFound, "revoked key should not resolve")
}

func TestAPIKeyRepository_FindActiveByHash_NotFound(t *testing.T) {
	repo := NewAPIKeyRepository(newTestDB(t))
	_, err := repo.FindActiveByHash(context.Background(), "missing")
	assert.ErrorIs(t, err, ErrNotFound)
}
