package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var (
	ErrEnvironmentNotFound = errors.New("environment not found")
	ErrEnvironmentKeyTaken = errors.New("environment key already exists")
	ErrLastEnvironment     = errors.New("cannot delete the last environment in a project")
	ErrDefaultEnvironment  = errors.New("cannot delete the default environment")
)

type EnvironmentService struct {
	envs repository.EnvironmentRepository
}

func NewEnvironmentService(envs repository.EnvironmentRepository) *EnvironmentService {
	return &EnvironmentService{envs: envs}
}

// EnsureDefault creates the project's first environment ("production") — a
// project always has at least one environment to evaluate flags against.
func (s *EnvironmentService) EnsureDefault(ctx context.Context, projectID uuid.UUID) (*model.Environment, error) {
	return s.Create(ctx, projectID, constants.DefaultEnvironmentKey, constants.DefaultEnvironmentName)
}

func (s *EnvironmentService) Create(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error) {
	if _, err := s.envs.FindByKey(ctx, projectID, key); err == nil {
		return nil, ErrEnvironmentKeyTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	count, err := s.envs.Count(ctx, projectID)
	if err != nil {
		return nil, err
	}
	env := &model.Environment{ProjectID: projectID, Key: key, Name: name, SortOrder: int(count)}
	if err := s.envs.Create(ctx, env); err != nil {
		return nil, err
	}
	return env, nil
}

func (s *EnvironmentService) List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error) {
	return s.envs.List(ctx, projectID)
}

func (s *EnvironmentService) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error) {
	e, err := s.envs.FindByKey(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrEnvironmentNotFound
		}
		return nil, err
	}
	return e, nil
}

func (s *EnvironmentService) Update(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error) {
	env, err := s.FindByKey(ctx, projectID, key)
	if err != nil {
		return nil, err
	}
	env.Name = name
	if err := s.envs.Update(ctx, env); err != nil {
		return nil, err
	}
	return env, nil
}

func (s *EnvironmentService) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	env, err := s.FindByKey(ctx, projectID, key)
	if err != nil {
		return err
	}
	if env.Key == constants.DefaultEnvironmentKey {
		return ErrDefaultEnvironment
	}
	count, err := s.envs.Count(ctx, projectID)
	if err != nil {
		return err
	}
	if count <= 1 {
		return ErrLastEnvironment
	}
	return s.envs.Delete(ctx, env.ID)
}
