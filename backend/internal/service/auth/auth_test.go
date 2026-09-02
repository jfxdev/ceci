package auth

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

type fakeUserRepository struct {
	usersByEmail map[string]*model.User
	usersByID    map[uuid.UUID]*model.User
	tokens       map[string]*model.RefreshToken
	identities   map[string]*model.AuthIdentity
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{
		usersByEmail: map[string]*model.User{},
		usersByID:    map[uuid.UUID]*model.User{},
		tokens:       map[string]*model.RefreshToken{},
		identities:   map[string]*model.AuthIdentity{},
	}
}

func identityKey(provider, subject string) string { return provider + "|" + subject }

func (f *fakeUserRepository) FindIdentity(ctx context.Context, provider, subject string) (*model.AuthIdentity, error) {
	identity, ok := f.identities[identityKey(provider, subject)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return identity, nil
}

func (f *fakeUserRepository) CreateIdentity(ctx context.Context, identity *model.AuthIdentity) error {
	if identity.ID == uuid.Nil {
		identity.ID = uuid.New()
	}
	f.identities[identityKey(identity.Provider, identity.Subject)] = identity
	return nil
}

func (f *fakeUserRepository) Create(ctx context.Context, u *model.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	f.usersByEmail[u.Email] = u
	f.usersByID[u.ID] = u
	return nil
}

func (f *fakeUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := f.usersByID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) SetAdmin(ctx context.Context, id uuid.UUID, isAdmin bool) error {
	u, ok := f.usersByID[id]
	if !ok {
		return repository.ErrNotFound
	}
	u.IsAdmin = isAdmin
	return nil
}

func (f *fakeUserRepository) SetLocale(ctx context.Context, id uuid.UUID, locale string) error {
	u, ok := f.usersByID[id]
	if !ok {
		return repository.ErrNotFound
	}
	u.Locale = locale
	return nil
}

func (f *fakeUserRepository) CreateRefreshToken(ctx context.Context, rt *model.RefreshToken) error {
	if rt.ID == uuid.Nil {
		rt.ID = uuid.New()
	}
	f.tokens[rt.TokenHash] = rt
	return nil
}

func (f *fakeUserRepository) FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	rt, ok := f.tokens[tokenHash]
	if !ok || rt.RevokedAt != nil {
		return nil, repository.ErrNotFound
	}
	return rt, nil
}

func (f *fakeUserRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	for _, rt := range f.tokens {
		if rt.ID == id {
			now := rt.ExpiresAt
			rt.RevokedAt = &now
		}
	}
	return nil
}

func TestHashAndVerifyPassword(t *testing.T) {
	hash, err := HashPassword("s3cret!")
	require.NoError(t, err)
	assert.Contains(t, hash, "$argon2id$")

	ok, err := VerifyPassword("s3cret!", hash)
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = VerifyPassword("wrong", hash)
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestAuthService_RegisterLoginRefreshLogout(t *testing.T) {
	repo := newFakeUserRepository()
	auth := NewService(repo, "test-secret")
	ctx := context.Background()

	user, err := auth.Register(ctx, "a@b.com", "s3cret!", "Ana")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, user.ID)

	accessToken, refreshToken, loggedUser, err := auth.Login(ctx, "a@b.com", "s3cret!")
	require.NoError(t, err)
	assert.NotEmpty(t, accessToken)
	assert.NotEmpty(t, refreshToken)
	assert.Equal(t, user.ID, loggedUser.ID)

	_, _, _, err = auth.Login(ctx, "a@b.com", "wrongpass")
	assert.ErrorIs(t, err, ErrInvalidCredentials)

	parsedID, err := auth.ParseAccessToken(accessToken)
	require.NoError(t, err)
	assert.Equal(t, user.ID, parsedID)

	newAccessToken, newRefreshToken, err := auth.Refresh(ctx, refreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, newAccessToken)
	assert.NotEmpty(t, newRefreshToken)

	// old refresh token should now be revoked, replay must fail
	_, _, err = auth.Refresh(ctx, refreshToken)
	assert.ErrorIs(t, err, ErrInvalidToken)

	err = auth.Logout(ctx, newRefreshToken)
	require.NoError(t, err)

	_, _, err = auth.Refresh(ctx, newRefreshToken)
	assert.ErrorIs(t, err, ErrInvalidToken)

	fetched, err := auth.Me(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "a@b.com", fetched.Email)
}

func TestAuthService_LoginOIDC_CreatesIdentityAndProtectsBootstrapAdmin(t *testing.T) {
	repo := newFakeUserRepository()
	auth := NewService(repo, "test-secret")
	ctx := context.Background()
	user, err := auth.LoginOIDC(ctx, "oidc", "subject-1", "ana@example.com", "Ana", true)
	require.NoError(t, err)
	assert.Equal(t, "ana@example.com", user.Email)
	assert.Empty(t, user.PasswordHash)
	_, err = auth.LoginOIDC(ctx, "oidc", "subject-1", "changed@example.com", "Ana Updated", true)
	require.NoError(t, err, "stable provider subject must resolve the original user")

	bootstrap := &model.User{Email: "admin@example.com", IsBootstrapAdmin: true}
	require.NoError(t, repo.Create(ctx, bootstrap))
	_, err = auth.LoginOIDC(ctx, "oidc", "bootstrap-subject", bootstrap.Email, "Admin", true)
	assert.ErrorIs(t, err, ErrBootstrapAdminSSO)
}

func TestAuthService_LoginOIDC_RejectsLocalCollisionAndDisabledJIT(t *testing.T) {
	repo := newFakeUserRepository()
	auth := NewService(repo, "test-secret")
	ctx := context.Background()
	local := &model.User{Email: "local@example.com", PasswordHash: "hash"}
	require.NoError(t, repo.Create(ctx, local))
	_, err := auth.LoginOIDC(ctx, "oidc", "subject-local", local.Email, "Local", true)
	assert.ErrorIs(t, err, ErrFederatedEmailUsed)
	_, err = auth.LoginOIDC(ctx, "oidc", "subject-new", "new@example.com", "New", false)
	assert.ErrorIs(t, err, ErrJITDisabled)
}

func TestAuthService_Register_EmailTaken(t *testing.T) {
	repo := newFakeUserRepository()
	auth := NewService(repo, "test-secret")
	ctx := context.Background()

	_, err := auth.Register(ctx, "a@b.com", "s3cret!", "Ana")
	require.NoError(t, err)

	_, err = auth.Register(ctx, "a@b.com", "other!", "Ana2")
	assert.ErrorIs(t, err, ErrEmailTaken)
}

func TestAuthService_ParseAccessToken_Invalid(t *testing.T) {
	auth := NewService(newFakeUserRepository(), "test-secret")
	_, err := auth.ParseAccessToken("not-a-jwt")
	assert.ErrorIs(t, err, ErrInvalidToken)
}
