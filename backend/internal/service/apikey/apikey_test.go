package apikey

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

type fakeAPIKeyRepository struct {
	byHash map[string]*model.ProjectAPIKey
	byID   map[uuid.UUID]*model.ProjectAPIKey
}

func newFakeAPIKeyRepository() *fakeAPIKeyRepository {
	return &fakeAPIKeyRepository{byHash: map[string]*model.ProjectAPIKey{}, byID: map[uuid.UUID]*model.ProjectAPIKey{}}
}

func (f *fakeAPIKeyRepository) Create(ctx context.Context, key *model.ProjectAPIKey) error {
	if key.ID == uuid.Nil {
		key.ID = uuid.New()
	}
	f.byHash[key.KeyHash] = key
	f.byID[key.ID] = key
	return nil
}

func (f *fakeAPIKeyRepository) FindActiveByHash(ctx context.Context, keyHash string) (*model.ProjectAPIKey, error) {
	k, ok := f.byHash[keyHash]
	if !ok || k.RevokedAt != nil {
		return nil, repository.ErrNotFound
	}
	return k, nil
}

func (f *fakeAPIKeyRepository) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.ProjectAPIKey, error) {
	var out []model.ProjectAPIKey
	for _, k := range f.byID {
		if k.ProjectID == projectID && k.EnvironmentID == environmentID {
			out = append(out, *k)
		}
	}
	return out, nil
}

func (f *fakeAPIKeyRepository) Revoke(ctx context.Context, id uuid.UUID) error {
	if k, ok := f.byID[id]; ok {
		now := k.CreatedAt
		k.RevokedAt = &now
	}
	return nil
}

func TestAPIKeyService_CreateAndResolve(t *testing.T) {
	repo := newFakeAPIKeyRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	rawKey, key, err := svc.Create(ctx, projectID, envID, "CI")
	require.NoError(t, err)
	assert.NotEmpty(t, rawKey)
	assert.Contains(t, rawKey, "leaflag_sk_")

	sum := sha256.Sum256([]byte(rawKey))
	assert.Equal(t, hex.EncodeToString(sum[:]), key.KeyHash)

	resolvedProject, resolvedEnv, err := svc.ResolveEnvironment(ctx, rawKey)
	require.NoError(t, err)
	assert.Equal(t, projectID, resolvedProject)
	assert.Equal(t, envID, resolvedEnv)
}

func TestAPIKeyService_ResolveInvalidKey(t *testing.T) {
	svc := NewService(newFakeAPIKeyRepository())
	_, _, err := svc.ResolveEnvironment(context.Background(), "not-a-real-key")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAPIKeyService_RevokeThenResolveFails(t *testing.T) {
	repo := newFakeAPIKeyRepository()
	svc := NewService(repo)
	ctx := context.Background()

	rawKey, key, err := svc.Create(ctx, uuid.New(), uuid.New(), "CI")
	require.NoError(t, err)

	require.NoError(t, svc.Revoke(ctx, key.ID))

	_, _, err = svc.ResolveEnvironment(ctx, rawKey)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestAPIKeyService_List(t *testing.T) {
	repo := newFakeAPIKeyRepository()
	svc := NewService(repo)
	ctx := context.Background()
	projectID := uuid.New()
	envID := uuid.New()

	_, _, err := svc.Create(ctx, projectID, envID, "A")
	require.NoError(t, err)
	_, _, err = svc.Create(ctx, projectID, envID, "B")
	require.NoError(t, err)

	list, err := svc.List(ctx, projectID, envID)
	require.NoError(t, err)
	assert.Len(t, list, 2)
}
