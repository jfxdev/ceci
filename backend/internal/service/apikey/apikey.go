// Package apikey holds project API key domain logic — issuance and
// bearer-token resolution to a project/environment scope — split out from
// internal/service so it can be maintained and tested independently of the
// rest of the service layer.
package apikey

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var ErrNotFound = errors.New("api key not found")

type Service struct {
	keys repository.APIKeyRepository
}

func NewService(keys repository.APIKeyRepository) *Service {
	return &Service{keys: keys}
}

// Create generates a new project API key scoped to a single environment —
// the same convention SDKs from other feature-flag platforms use, one key
// per environment. The raw key is returned once and never stored — only its
// sha256 hash is persisted.
func (s *Service) Create(ctx context.Context, projectID, environmentID uuid.UUID, label string) (rawKey string, key *model.ProjectAPIKey, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", nil, err
	}
	secret := base64.RawURLEncoding.EncodeToString(b)
	rawKey = constants.ProjectAPIKeyPrefix + secret
	sum := sha256.Sum256([]byte(rawKey))

	key = &model.ProjectAPIKey{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		Label:         label,
		KeyHash:       hex.EncodeToString(sum[:]),
		Prefix:        rawKey[:len(constants.ProjectAPIKeyPrefix)+6],
	}
	if err := s.keys.Create(ctx, key); err != nil {
		return "", nil, err
	}
	return rawKey, key, nil
}

// ResolveEnvironment validates a raw bearer key and returns the project and
// environment it belongs to — this is how OFREP requests get their
// environment scope, without needing it spelled out in the URL.
func (s *Service) ResolveEnvironment(ctx context.Context, rawKey string) (projectID, environmentID uuid.UUID, err error) {
	sum := sha256.Sum256([]byte(rawKey))
	key, err := s.keys.FindActiveByHash(ctx, hex.EncodeToString(sum[:]))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return uuid.Nil, uuid.Nil, ErrNotFound
		}
		return uuid.Nil, uuid.Nil, err
	}
	return key.ProjectID, key.EnvironmentID, nil
}

func (s *Service) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.ProjectAPIKey, error) {
	return s.keys.List(ctx, projectID, environmentID)
}

func (s *Service) Revoke(ctx context.Context, id uuid.UUID) error {
	return s.keys.Revoke(ctx, id)
}
