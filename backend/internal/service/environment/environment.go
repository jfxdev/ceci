// Package environment holds the environment and environment-template domain
// logic, split out from internal/service so it can be maintained and tested
// independently of the rest of the service layer. Models
// (model.Environment/EnvironmentTemplate) and persistence
// (repository.EnvironmentRepository/EnvironmentTemplateRepository) stay in
// their existing packages — only the business logic moved here.
package environment

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var (
	ErrNotFound        = errors.New("environment not found")
	ErrKeyTaken        = errors.New("environment key already exists")
	ErrLastEnvironment = errors.New("cannot delete the last environment in a project")
	ErrIsDefault       = errors.New("cannot delete the default environment")
)

type Service struct {
	envs repository.EnvironmentRepository
}

func NewService(envs repository.EnvironmentRepository) *Service {
	return &Service{envs: envs}
}

// EnsureDefault creates the project's first environment ("production") — a
// project always has at least one environment to evaluate flags against.
func (s *Service) EnsureDefault(ctx context.Context, projectID uuid.UUID) (*model.Environment, error) {
	return s.Create(ctx, projectID, constants.DefaultEnvironmentKey, constants.DefaultEnvironmentName)
}

func (s *Service) Create(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error) {
	if _, err := s.envs.FindByKey(ctx, projectID, key); err == nil {
		return nil, ErrKeyTaken
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

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error) {
	return s.envs.List(ctx, projectID)
}

func (s *Service) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error) {
	e, err := s.envs.FindByKey(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return e, nil
}

func (s *Service) Update(ctx context.Context, projectID uuid.UUID, key, name string) (*model.Environment, error) {
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

func (s *Service) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	env, err := s.FindByKey(ctx, projectID, key)
	if err != nil {
		return err
	}
	if env.Key == constants.DefaultEnvironmentKey {
		return ErrIsDefault
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

var (
	ErrTemplateNotFound    = errors.New("environment template not found")
	ErrTemplateKeyTaken    = errors.New("environment template key already exists")
	ErrReservedTemplateKey = errors.New("environment template key is reserved")
)

type TemplateService struct {
	templates repository.EnvironmentTemplateRepository
}

func NewTemplateService(templates repository.EnvironmentTemplateRepository) *TemplateService {
	return &TemplateService{templates: templates}
}

func (s *TemplateService) List(ctx context.Context) ([]model.EnvironmentTemplate, error) {
	return s.templates.List(ctx)
}

func (s *TemplateService) Create(ctx context.Context, key, name string, isRequired bool) (*model.EnvironmentTemplate, error) {
	if key == constants.DefaultEnvironmentKey {
		return nil, ErrReservedTemplateKey
	}
	if _, err := s.templates.FindByKey(ctx, key); err == nil {
		return nil, ErrTemplateKeyTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}

	list, err := s.templates.List(ctx)
	if err != nil {
		return nil, err
	}
	template := &model.EnvironmentTemplate{Key: key, Name: name, IsRequired: isRequired, SortOrder: len(list)}
	if err := s.templates.Create(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *TemplateService) Update(ctx context.Context, id uuid.UUID, name string, isRequired bool) (*model.EnvironmentTemplate, error) {
	template, err := s.templates.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	template.Name = name
	template.IsRequired = isRequired
	if err := s.templates.Update(ctx, template); err != nil {
		return nil, err
	}
	return template, nil
}

func (s *TemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.templates.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTemplateNotFound
		}
		return err
	}
	return s.templates.Delete(ctx, id)
}
