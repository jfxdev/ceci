// Package auth holds local username/password authentication, JWT
// access-token issuance and refresh-token session management, split out
// from internal/service so it can be maintained and tested independently of
// the rest of the service layer.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/argon2"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrEmailTaken         = errors.New("email already registered")
	ErrBootstrapAdminSSO  = errors.New("bootstrap administrator cannot use sso")
	ErrFederatedEmailUsed = errors.New("email is already used by a local account")
	ErrJITDisabled        = errors.New("just-in-time provisioning is disabled")
	ErrInvalidLocale      = errors.New("invalid locale")
)

type Service struct {
	users     repository.UserRepository
	jwtSecret []byte
}

func NewService(users repository.UserRepository, jwtSecret string) *Service {
	return &Service{users: users, jwtSecret: []byte(jwtSecret)}
}

// HashPassword returns a PHC-formatted argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, constants.Argon2SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, constants.Argon2Iterations, constants.Argon2Memory, constants.Argon2Parallelism, constants.Argon2KeyLength)
	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, constants.Argon2Memory, constants.Argon2Iterations, constants.Argon2Parallelism, b64Salt, b64Hash), nil
}

func VerifyPassword(password, encoded string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errors.New("invalid hash format")
	}
	var memory, iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false, err
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(expected, actual) == 1, nil
}

type Claims struct {
	jwt.RegisteredClaims
}

func (s *Service) issueAccessToken(userID uuid.UUID) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(constants.AccessTokenTTL)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *Service) ParseAccessToken(tokenStr string) (uuid.UUID, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		return s.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return uuid.Nil, ErrInvalidToken
	}
	return uuid.Parse(claims.Subject)
}

func newOpaqueToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// Login validates credentials and returns an access token + a raw refresh token to be set as an httpOnly cookie.
func (s *Service) Login(ctx context.Context, email, password string) (accessToken, refreshToken string, user *model.User, err error) {
	user, err = s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", "", nil, ErrInvalidCredentials
	}
	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		return "", "", nil, ErrInvalidCredentials
	}
	accessToken, refreshToken, err = s.CreateSession(ctx, user)
	if err != nil {
		return "", "", nil, err
	}
	return accessToken, refreshToken, user, nil
}

// CreateSession issues the application's own access and refresh tokens after
// any successful authentication method.
func (s *Service) CreateSession(ctx context.Context, user *model.User) (accessToken, refreshToken string, err error) {
	accessToken, err = s.issueAccessToken(user.ID)
	if err != nil {
		return "", "", err
	}
	rawRefresh, refreshHash, err := newOpaqueToken()
	if err != nil {
		return "", "", err
	}
	if err := s.users.CreateRefreshToken(ctx, &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(constants.RefreshTokenTTL),
	}); err != nil {
		return "", "", err
	}
	return accessToken, rawRefresh, nil
}

// LoginOIDC resolves a stable provider subject to a user. It never links an
// existing local account by email: that would allow an IdP email collision to
// take over a password-backed account. New OIDC users are created only when
// JIT provisioning is explicitly enabled.
func (s *Service) LoginOIDC(ctx context.Context, provider, subject, email, name string, jitEnabled bool) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if identity, err := s.users.FindIdentity(ctx, provider, subject); err == nil {
		user, err := s.users.FindByID(ctx, identity.UserID)
		if err != nil {
			return nil, err
		}
		if user.IsBootstrapAdmin {
			return nil, ErrBootstrapAdminSSO
		}
		return user, nil
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	if user, err := s.users.FindByEmail(ctx, email); err == nil {
		if user.IsBootstrapAdmin {
			return nil, ErrBootstrapAdminSSO
		}
		return nil, ErrFederatedEmailUsed
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if !jitEnabled {
		return nil, ErrJITDisabled
	}

	user := &model.User{Email: email, Name: strings.TrimSpace(name)}
	if err := s.users.Create(ctx, user); err != nil {
		return nil, err
	}
	if err := s.users.CreateIdentity(ctx, &model.AuthIdentity{UserID: user.ID, Provider: provider, Subject: subject}); err != nil {
		return nil, err
	}
	return user, nil
}

// Refresh rotates the refresh token and issues a new access token.
func (s *Service) Refresh(ctx context.Context, rawRefresh string) (accessToken, newRefreshToken string, err error) {
	sum := sha256.Sum256([]byte(rawRefresh))
	hash := hex.EncodeToString(sum[:])
	rt, err := s.users.FindRefreshToken(ctx, hash)
	if err != nil {
		return "", "", ErrInvalidToken
	}
	if err := s.users.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return "", "", err
	}
	accessToken, err = s.issueAccessToken(rt.UserID)
	if err != nil {
		return "", "", err
	}
	newRawRefresh, newHash, err := newOpaqueToken()
	if err != nil {
		return "", "", err
	}
	if err := s.users.CreateRefreshToken(ctx, &model.RefreshToken{
		UserID:    rt.UserID,
		TokenHash: newHash,
		ExpiresAt: time.Now().Add(constants.RefreshTokenTTL),
	}); err != nil {
		return "", "", err
	}
	return accessToken, newRawRefresh, nil
}

func (s *Service) Logout(ctx context.Context, rawRefresh string) error {
	sum := sha256.Sum256([]byte(rawRefresh))
	hash := hex.EncodeToString(sum[:])
	rt, err := s.users.FindRefreshToken(ctx, hash)
	if err != nil {
		return nil // already invalid/gone, nothing to do
	}
	return s.users.RevokeRefreshToken(ctx, rt.ID)
}

func (s *Service) Me(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.users.FindByID(ctx, userID)
}

func (s *Service) UpdateLocale(ctx context.Context, userID uuid.UUID, locale string) (*model.User, error) {
	if locale != "en" && locale != "pt-BR" {
		return nil, ErrInvalidLocale
	}
	if err := s.users.SetLocale(ctx, userID, locale); err != nil {
		return nil, err
	}
	return s.users.FindByID(ctx, userID)
}

func (s *Service) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return user.IsAdmin, nil
}

// Register creates a new user with a hashed password. Returns ErrEmailTaken
// if the email is already registered.
func (s *Service) Register(ctx context.Context, email, password, name string) (*model.User, error) {
	if _, err := s.users.FindByEmail(ctx, email); err == nil {
		return nil, ErrEmailTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := &model.User{Email: email, PasswordHash: hash, Name: name}
	if err := s.users.Create(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
