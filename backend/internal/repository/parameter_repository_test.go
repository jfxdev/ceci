package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/model"
)

func TestParameterRepository_UpsertFindListDelete(t *testing.T) {
	repo := NewParameterRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	param := &model.Parameter{ProjectID: projectID, EnvironmentID: envID, Key: "service/db/host", Value: "localhost", Version: 1}
	version := &model.ParameterVersion{Version: 1, Value: "localhost", ChangeType: "create", ChangedAt: time.Now()}
	require.NoError(t, repo.Upsert(ctx, param, version))
	require.NotEqual(t, uuid.Nil, param.ID)

	found, err := repo.Find(ctx, projectID, envID, "service/db/host")
	require.NoError(t, err)
	assert.Equal(t, "localhost", found.Value)

	list, err := repo.List(ctx, projectID, envID, "")
	require.NoError(t, err)
	require.Len(t, list, 1)

	prefixed, err := repo.List(ctx, projectID, envID, "service/")
	require.NoError(t, err)
	assert.Len(t, prefixed, 1)

	notMatching, err := repo.List(ctx, projectID, envID, "other/")
	require.NoError(t, err)
	assert.Len(t, notMatching, 0)

	versions, err := repo.ListVersions(ctx, param.ID)
	require.NoError(t, err)
	require.Len(t, versions, 1)

	deleteVersion := &model.ParameterVersion{Version: 2, ChangeType: "delete", ChangedAt: time.Now()}
	require.NoError(t, repo.Delete(ctx, projectID, envID, "service/db/host", deleteVersion))

	_, err = repo.Find(ctx, projectID, envID, "service/db/host")
	assert.ErrorIs(t, err, ErrNotFound)

	versions, err = repo.ListVersions(ctx, param.ID)
	require.NoError(t, err)
	assert.Len(t, versions, 2, "delete should append a version row rather than removing history")
}

func TestParameterRepository_ScopedToEnvironment(t *testing.T) {
	repo := NewParameterRepository(newTestDB(t))
	ctx := context.Background()
	projectID := uuid.New()
	prodID := uuid.New()
	stagingID := uuid.New()

	prodParam := &model.Parameter{ProjectID: projectID, EnvironmentID: prodID, Key: "db/host", Value: "prod-host", Version: 1}
	require.NoError(t, repo.Upsert(ctx, prodParam, &model.ParameterVersion{Version: 1, Value: "prod-host", ChangeType: "create", ChangedAt: time.Now()}))
	stagingParam := &model.Parameter{ProjectID: projectID, EnvironmentID: stagingID, Key: "db/host", Value: "staging-host", Version: 1}
	require.NoError(t, repo.Upsert(ctx, stagingParam, &model.ParameterVersion{Version: 1, Value: "staging-host", ChangeType: "create", ChangedAt: time.Now()}))

	found, err := repo.Find(ctx, projectID, prodID, "db/host")
	require.NoError(t, err)
	assert.Equal(t, "prod-host", found.Value)

	list, err := repo.List(ctx, projectID, stagingID, "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "staging-host", list[0].Value)
}

func TestParameterRepository_DeleteMissing(t *testing.T) {
	repo := NewParameterRepository(newTestDB(t))
	err := repo.Delete(context.Background(), uuid.New(), uuid.New(), "missing", &model.ParameterVersion{ChangeType: "delete", ChangedAt: time.Now()})
	assert.ErrorIs(t, err, ErrNotFound)
}
