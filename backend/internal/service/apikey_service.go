package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
)

var ErrAPIKeyNotFound = errors.New("api key not found")

type APIKeyService struct {
	keys repository.APIKeyRepository
}

func NewAPIKeyService(keys repository.APIKeyRepository) *APIKeyService {
	return &APIKeyService{keys: keys}
}

// Create generates a new project API key. The raw key is returned once and
// never stored — only its sha256 hash is persisted.
func (s *APIKeyService) Create(ctx context.Context, projectID uuid.UUID, label string) (rawKey string, key *model.ProjectAPIKey, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	rawKey = constants.ProjectAPIKeyPrefix + secret
	sum := sha256.Sum256([]byte(rawKey))

	key = &model.ProjectAPIKey{
		ProjectID: projectID,
		Label:     label,
		KeyHash:   hex.EncodeToString(sum[:]),
		Prefix:    rawKey[:len(constants.ProjectAPIKeyPrefix)+6],
	}
	if err := s.keys.Create(ctx, key); err != nil {
		return "", nil, err
	}
	return rawKey, key, nil
}

// ResolveProjectID validates a raw bearer key and returns the project it belongs to.
func (s *APIKeyService) ResolveProjectID(ctx context.Context, rawKey string) (uuid.UUID, error) {
	sum := sha256.Sum256([]byte(rawKey))
	key, err := s.keys.FindActiveByHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return uuid.Nil, ErrAPIKeyNotFound
		}
		return uuid.Nil, err
	}
	return key.ProjectID, nil
}

func (s *APIKeyService) List(ctx context.Context, projectID uuid.UUID) ([]model.ProjectAPIKey, error) {
	return s.keys.List(ctx, projectID)
}

func (s *APIKeyService) Revoke(ctx context.Context, id uuid.UUID) error {
	return s.keys.Revoke(ctx, id)
}
