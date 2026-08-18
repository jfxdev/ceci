package environment

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

type fakeEnvRepository struct {
	byID map[uuid.UUID]*model.Environment
}

func newFakeEnvRepository() *fakeEnvRepository {
	return &fakeEnvRepository{byID: map[uuid.UUID]*model.Environment{}}
}

func (f *fakeEnvRepository) Create(ctx context.Context, env *model.Environment) error {
	if env.ID == uuid.Nil {
		env.ID = uuid.New()
	}
	cp := *env
	f.byID[env.ID] = &cp
	return nil
}

func (f *fakeEnvRepository) List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error) {
	var out []model.Environment
	for _, e := range f.byID {
		if e.ProjectID == projectID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (f *fakeEnvRepository) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error) {
	for _, e := range f.byID {
		if e.ProjectID == projectID && e.Key == key {
			cp := *e
			return &cp, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeEnvRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	e, ok := f.byID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	cp := *e
	return &cp, nil
}

func (f *fakeEnvRepository) Update(ctx context.Context, env *model.Environment) error {
	cp := *env
	f.byID[env.ID] = &cp
	return nil
}

func (f *fakeEnvRepository) Delete(ctx context.Context, id uuid.UUID) error {
	delete(f.byID, id)
	return nil
}

func (f *fakeEnvRepository) Count(ctx context.Context, projectID uuid.UUID) (int64, error) {
	var count int64
	for _, e := range f.byID {
		if e.ProjectID == projectID {
			count++
		}
	}
	return count, nil
}

func TestEnvironmentService_EnsureDefault(t *testing.T) {
	repo := newFakeEnvRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	env, err := svc.EnsureDefault(ctx, projectID)
	require.NoError(t, err)
	assert.Equal(t, "all", env.Key)
	assert.Equal(t, "All", env.Name)
}

func TestEnvironmentService_CreateDuplicateKey(t *testing.T) {
	repo := newFakeEnvRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.Create(ctx, projectID, "staging", "Staging")
	require.NoError(t, err)

	_, err = svc.Create(ctx, projectID, "staging", "Staging again")
	assert.ErrorIs(t, err, ErrKeyTaken)
}

func TestEnvironmentService_UpdateNotFound(t *testing.T) {
	svc := NewService(newFakeEnvRepository())
	_, err := svc.Update(context.Background(), uuid.New(), "missing", "New name")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestEnvironmentService_Update(t *testing.T) {
	repo := newFakeEnvRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.Create(ctx, projectID, "staging", "Staging")
	require.NoError(t, err)

	updated, err := svc.Update(ctx, projectID, "staging", "Staging Env")
	require.NoError(t, err)
	assert.Equal(t, "Staging Env", updated.Name)
}

func TestEnvironmentService_Delete_DefaultEnvironmentProtected(t *testing.T) {
	repo := newFakeEnvRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.EnsureDefault(ctx, projectID)
	require.NoError(t, err)
	_, err = svc.Create(ctx, projectID, "staging", "Staging")
	require.NoError(t, err)

	err = svc.Delete(ctx, projectID, "all")
	assert.ErrorIs(t, err, ErrIsDefault, "the default env must never be deletable, even with other environments present")
}

func TestEnvironmentService_Delete_LastEnvironmentProtected(t *testing.T) {
	repo := newFakeEnvRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.Create(ctx, projectID, "staging", "Staging")
	require.NoError(t, err)

	err = svc.Delete(ctx, projectID, "staging")
	assert.ErrorIs(t, err, ErrLastEnvironment)
}

func TestEnvironmentService_Delete_Success(t *testing.T) {
	repo := newFakeEnvRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.EnsureDefault(ctx, projectID)
	require.NoError(t, err)
	_, err = svc.Create(ctx, projectID, "staging", "Staging")
	require.NoError(t, err)

	require.NoError(t, svc.Delete(ctx, projectID, "staging"))
	_, err = svc.FindByKey(ctx, projectID, "staging")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestEnvironmentService_List(t *testing.T) {
	repo := newFakeEnvRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()

	_, err := svc.EnsureDefault(ctx, projectID)
	require.NoError(t, err)
	_, err = svc.Create(ctx, projectID, "staging", "Staging")
	require.NoError(t, err)

	list, err := svc.List(ctx, projectID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
