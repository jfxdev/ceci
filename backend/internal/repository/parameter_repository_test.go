package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ceci/backend/internal/model"
)

func TestParameterRepository_UpsertFindListDelete(t *testing.T) {
	repo := NewParameterRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()

	param := &model.Parameter{ProjectID: projectID, Key: "service/db/host", Value: "localhost", Version: 1}
	version := &model.ParameterVersion{Version: 1, Value: "localhost", ChangeType: "create", ChangedAt: time.Now()}
	require.NoError(t, repo.Upsert(ctx, param, version))
	require.NotEqual(t, uuid.Nil, param.ID)

	found, err := repo.Find(ctx, projectID, "service/db/host")
	require.NoError(t, err)
	assert.Equal(t, "localhost", found.Value)

	list, err := repo.List(ctx, projectID, "")
	require.NoError(t, err)
	require.Len(t, list, 1)

	prefixed, err := repo.List(ctx, projectID, "service/")
	require.NoError(t, err)
	assert.Len(t, prefixed, 1)

	notMatching, err := repo.List(ctx, projectID, "other/")
	require.NoError(t, err)
	assert.Len(t, notMatching, 0)

	versions, err := repo.ListVersions(ctx, param.ID)
	require.NoError(t, err)
	require.Len(t, versions, 1)

	deleteVersion := &model.ParameterVersion{Version: 2, ChangeType: "delete", ChangedAt: time.Now()}
	require.NoError(t, repo.Delete(ctx, projectID, "service/db/host", deleteVersion))

	_, err = repo.Find(ctx, projectID, "service/db/host")
	assert.ErrorIs(t, err, ErrNotFound)

	versions, err = repo.ListVersions(ctx, param.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 2, "delete should append a version row rather than removing history")
}

func TestParameterRepository_DeleteMissing(t *testing.T) {
	repo := NewParameterRepository(newTestDB(t))
	err := repo.Delete(context.Background(), uuid.New(), "missing", &model.ParameterVersion{ChangeType: "delete", ChangedAt: time.Now()})
	assert.ErrorIs(t, err, ErrNotFound)
}
