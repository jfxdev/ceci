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

type fakeParameterRepository struct {
	byProjectAndKey map[string]*model.Parameter
	versions        map[uuid.UUID][]model.ParameterVersion
}

func newFakeParameterRepository() *fakeParameterRepository {
	return &fakeParameterRepository{
		byProjectAndKey: map[string]*model.Parameter{},
		versions:        map[uuid.UUID][]model.ParameterVersion{},
	}
}

func paramKey(projectID uuid.UUID, key string) string { return projectID.String() + "|" + key }

func (f *fakeParameterRepository) List(ctx context.Context, projectID uuid.UUID, prefix string) ([]model.Parameter, error) {
	var out []model.Parameter
	for _, p := range f.byProjectAndKey {
		if p.ProjectID == projectID {
			out = append(out, *p)
		}
	}
	return out, nil
}

func (f *fakeParameterRepository) Find(ctx context.Context, projectID uuid.UUID, key string) (*model.Parameter, error) {
	p, ok := f.byProjectAndKey[paramKey(projectID, key)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}

func (f *fakeParameterRepository) Upsert(ctx context.Context, param *model.Parameter, version *model.ParameterVersion) error {
	if param.ID == uuid.Nil {
		param.ID = uuid.New()
	}
	f.byProjectAndKey[paramKey(param.ProjectID, param.Key)] = param
	version.ParameterID = param.ID
	f.versions[param.ID] = append(f.versions[param.ID], *version)
	return nil
}

func (f *fakeParameterRepository) Delete(ctx context.Context, projectID uuid.UUID, key string, version *model.ParameterVersion) error {
	p, ok := f.byProjectAndKey[paramKey(projectID, key)]
	if !ok {
		return repository.ErrNotFound
	}
	version.ParameterID = p.ID
	f.versions[p.ID] = append(f.versions[p.ID], *version)
	delete(f.byProjectAndKey, paramKey(projectID, key))
	return nil
}

func (f *fakeParameterRepository) ListVersions(ctx context.Context, parameterID uuid.UUID) ([]model.ParameterVersion, error) {
	return f.versions[parameterID], nil
}

func TestParameterService_UpsertCreateThenUpdate(t *testing.T) {
	repo := newFakeParameterRepository()
	svc := NewParameterService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	p, err := svc.Upsert(ctx, projectID, "service/db/host", "localhost", userID)
	require.NoError(t, err)
	assert.Equal(t, 1, p.Version)

	p, err = svc.Upsert(ctx, projectID, "service/db/host", "remotehost", userID)
	require.NoError(t, err)
	assert.Equal(t, 2, p.Version)
	assert.Equal(t, "remotehost", p.Value)

	versions, err := svc.ListVersions(ctx, projectID, "service/db/host")
	require.NoError(t, err)
	require.Len(t, versions, 2)
	assert.Equal(t, "create", versions[0].ChangeType)
	assert.Equal(t, "update", versions[1].ChangeType)
}

func TestParameterService_GetNotFound(t *testing.T) {
	svc := NewParameterService(newFakeParameterRepository())
	_, err := svc.Get(context.Background(), uuid.New(), "missing")
	assert.ErrorIs(t, err, ErrParameterNotFound)
}

func TestParameterService_DeleteNotFound(t *testing.T) {
	svc := NewParameterService(newFakeParameterRepository())
	err := svc.Delete(context.Background(), uuid.New(), "missing", uuid.New())
	assert.ErrorIs(t, err, ErrParameterNotFound)
}

func TestParameterService_DeleteThenGetFails(t *testing.T) {
	repo := newFakeParameterRepository()
	svc := NewParameterService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	_, err := svc.Upsert(ctx, projectID, "db/host", "localhost", userID)
	require.NoError(t, err)

	err = svc.Delete(ctx, projectID, "db/host", userID)
	require.NoError(t, err)

	_, err = svc.Get(ctx, projectID, "db/host")
	assert.ErrorIs(t, err, ErrParameterNotFound)
}

func TestParameterService_List(t *testing.T) {
	repo := newFakeParameterRepository()
	svc := NewParameterService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.Upsert(ctx, projectID, "a", "1", uuid.New())
	require.NoError(t, err)
	_, err = svc.Upsert(ctx, projectID, "b", "2", uuid.New())
	require.NoError(t, err)

	list, err := svc.List(ctx, projectID, "")
	require.NoError(t, err)
	assert.Len(t, list, 2)
}

func TestParameterService_ListVersions_NotFound(t *testing.T) {
	svc := NewParameterService(newFakeParameterRepository())
	_, err := svc.ListVersions(context.Background(), uuid.New(), "missing")
	assert.ErrorIs(t, err, ErrParameterNotFound)
}
