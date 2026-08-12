package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
)

var ErrParameterNotFound = errors.New("parameter not found")

type ParameterService struct {
	params repository.ParameterRepository
}

func NewParameterService(params repository.ParameterRepository) *ParameterService {
	return &ParameterService{params: params}
}

func (s *ParameterService) List(ctx context.Context, projectID uuid.UUID, prefix string) ([]model.Parameter, error) {
	return s.params.List(ctx, projectID, prefix)
}

func (s *ParameterService) Get(ctx context.Context, projectID uuid.UUID, key string) (*model.Parameter, error) {
	p, err := s.params.Find(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrParameterNotFound
		}
		return nil, err
	}
	return p, nil
}

// Upsert creates the parameter at version 1, or bumps the version on an existing one.
func (s *ParameterService) Upsert(ctx context.Context, projectID uuid.UUID, key, value string, changedBy uuid.UUID) (*model.Parameter, error) {
	changeType := "create"
	version := 1
	existing, err := s.params.Find(ctx, projectID, key)
	if err == nil {
		changeType = "update"
		version = existing.Version + 1
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	param := &model.Parameter{
		ProjectID: projectID,
		Key:       key,
		Value:     value,
		Version:   version,
		UpdatedBy: changedBy,
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

func (s *ParameterService) Delete(ctx context.Context, projectID uuid.UUID, key string, deletedBy uuid.UUID) error {
	existing, err := s.params.Find(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrParameterNotFound
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
	err = s.params.Delete(ctx, projectID, key, versionRow)
	if errors.Is(err, repository.ErrNotFound) {
		return ErrParameterNotFound
	}
	return err
}

func (s *ParameterService) ListVersions(ctx context.Context, projectID uuid.UUID, key string) ([]model.ParameterVersion, error) {
	p, err := s.params.Find(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrParameterNotFound
		}
		return nil, err
	}
	return s.params.ListVersions(ctx, p.ID)
}
