package service

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

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
)

type AuthService struct {
	users     repository.UserRepository
	jwtSecret []byte
}

func NewAuthService(users repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{users: users, jwtSecret: []byte(jwtSecret)}
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

func (s *AuthService) issueAccessToken(userID uuid.UUID) (string, error) {
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

func (s *AuthService) ParseAccessToken(tokenStr string) (uuid.UUID, error) {
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
func (s *AuthService) Login(ctx context.Context, email, password string) (accessToken, refreshToken string, user *model.User, err error) {
	user, err = s.users.FindByEmail(ctx, email)
	if err != nil {
		return "", "", nil, ErrInvalidCredentials
	}
	ok, err := VerifyPassword(password, user.PasswordHash)
	if err != nil || !ok {
		return "", "", nil, ErrInvalidCredentials
	}
	accessToken, err = s.issueAccessToken(user.ID)
	if err != nil {
		return "", "", nil, err
	}
	rawRefresh, refreshHash, err := newOpaqueToken()
	if err != nil {
		return "", "", nil, err
	}
	if err := s.users.CreateRefreshToken(ctx, &model.RefreshToken{
		UserID:    user.ID,
		TokenHash: refreshHash,
		ExpiresAt: time.Now().Add(constants.RefreshTokenTTL),
	}); err != nil {
		return "", "", nil, err
	}
	return accessToken, rawRefresh, user, nil
}

// Refresh rotates the refresh token and issues a new access token.
func (s *AuthService) Refresh(ctx context.Context, rawRefresh string) (accessToken, newRefreshToken string, err error) {
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

func (s *AuthService) Logout(ctx context.Context, rawRefresh string) error {
	sum := sha256.Sum256([]byte(rawRefresh))
	hash := hex.EncodeToString(sum[:])
	rt, err := s.users.FindRefreshToken(ctx, hash)
	if err != nil {
		return nil // already invalid/gone, nothing to do
	}
	return s.users.RevokeRefreshToken(ctx, rt.ID)
}

func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (*model.User, error) {
	return s.users.FindByID(ctx, userID)
}

// Register creates a new user with a hashed password.
func (s *AuthService) Register(ctx context.Context, email, password, name string) (*model.User, error) {
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
