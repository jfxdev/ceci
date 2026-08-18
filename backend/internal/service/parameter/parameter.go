// Package parameter holds the Consul-style key/value parameter store domain
// logic, split out from internal/service so it can be maintained and tested
// independently of the rest of the service layer. Models (model.Parameter)
// and persistence (repository.ParameterRepository) stay in their existing
// packages — only the business logic moved here.
package parameter

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var ErrNotFound = errors.New("parameter not found")

type Service struct {
	params repository.ParameterRepository
}

func NewService(params repository.ParameterRepository) *Service {
	return &Service{params: params}
}

func (s *Service) List(ctx context.Context, projectID, environmentID uuid.UUID, prefix string) ([]model.Parameter, error) {
	return s.params.List(ctx, projectID, environmentID, prefix)
}

func (s *Service) Get(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.Parameter, error) {
	p, err := s.params.Find(ctx, projectID, environmentID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// Upsert creates the parameter at version 1, or bumps the version on an existing one.
func (s *Service) Upsert(ctx context.Context, projectID, environmentID uuid.UUID, key, value string, changedBy uuid.UUID) (*model.Parameter, error) {
	changeType := "create"
	version := 1
	existing, err := s.params.Find(ctx, projectID, environmentID, key)
	if err == nil {
		changeType = "update"
		version = existing.Version + 1
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	param := &model.Parameter{
		ProjectID:     projectID,
		EnvironmentID: environmentID,
		Key:           key,
		Value:         value,
		Version:       version,
		UpdatedBy:     changedBy,
	}
	if existing != nil {
		param.ID = existing.ID
	}

	versionRow := &model.ParameterVersion{
		Version:    version,
		Value:      value,
		ChangedBy:  changedBy,
		ChangeType: changeType,
		ChangedAt:  time.Now(),
	}

	if err := s.params.Upsert(ctx, param, versionRow); err != nil {
		return nil, err
	}
	return param, nil
}

func (s *Service) Delete(ctx context.Context, projectID, environmentID uuid.UUID, key string, deletedBy uuid.UUID) error {
	existing, err := s.params.Find(ctx, projectID, environmentID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	versionRow := &model.ParameterVersion{
		Version:    existing.Version + 1,
		Value:      "",
		ChangedBy:  deletedBy,
		ChangeType: "delete",
		ChangedAt:  time.Now(),
	}
	err = s.params.Delete(ctx, projectID, environmentID, key, versionRow)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrNotFound
	}
	return err
}

func (s *Service) ListVersions(ctx context.Context, projectID, environmentID uuid.UUID, key string) ([]model.ParameterVersion, error) {
	p, err := s.params.Find(ctx, projectID, environmentID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.params.ListVersions(ctx, p.ID)
}
