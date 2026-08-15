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

func TestUserRepository_CreateAndFindByEmail(t *testing.T) {
	repo := NewUserRepository(newTestDB(t))
	ctx := context.Background()

	u := &model.User{Email: "a@b.com", PasswordHash: "hash", Name: "Ana"}
	require.NoError(t, repo.Create(ctx, u))

	found, err := repo.FindByEmail(ctx, "a@b.com")
	require.NoError(t, err)
	assert.Equal(t, u.ID, found.ID)

	_, err = repo.FindByEmail(ctx, "missing@b.com")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUserRepository_FindByID(t *testing.T) {
	repo := NewUserRepository(newTestDB(t))
	ctx := context.Background()

	u := &model.User{ID: uuid.New(), Email: "a@b.com", PasswordHash: "hash"}
	require.NoError(t, repo.Create(ctx, u))

	found, err := repo.FindByID(ctx, u.ID)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", found.Email)

	_, err = repo.FindByID(ctx, uuid.New())
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestUserRepository_RefreshTokenLifecycle(t *testing.T) {
	repo := NewUserRepository(newTestDB(t))
	ctx := context.Background()

	u := &model.User{ID: uuid.New(), Email: "a@b.com", PasswordHash: "hash"}
	require.NoError(t, repo.Create(ctx, u))

	rt := &model.RefreshToken{ID: uuid.New(), UserID: u.ID, TokenHash: "hash123", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, repo.CreateRefreshToken(ctx, rt))

	found, err := repo.FindRefreshToken(ctx, "hash123")
	require.NoError(t, err)
	assert.Equal(t, u.ID, found.UserID)

	require.NoError(t, repo.RevokeRefreshToken(ctx, rt.ID))
	_, err = repo.FindRefreshToken(ctx, "hash123")
	assert.ErrorIs(t, err, ErrNotFound, "revoked token should no longer be found")
}

func TestUserRepository_FindRefreshToken_Expired(t *testing.T) {
	repo := NewUserRepository(newTestDB(t))
	ctx := context.Background()

	u := &model.User{ID: uuid.New(), Email: "a@b.com", PasswordHash: "hash"}
	require.NoError(t, repo.Create(ctx, u))

	rt := &model.RefreshToken{ID: uuid.New(), UserID: u.ID, TokenHash: "expiredhash", ExpiresAt: time.Now().Add(-time.Hour)}
	require.NoError(t, repo.CreateRefreshToken(ctx, rt))

	_, err := repo.FindRefreshToken(ctx, "expiredhash")
	assert.ErrorIs(t, err, ErrNotFound)
}
