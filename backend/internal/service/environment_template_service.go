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
	ErrEnvironmentTemplateNotFound    = errors.New("environment template not found")
	ErrEnvironmentTemplateKeyTaken    = errors.New("environment template key already exists")
	ErrReservedEnvironmentTemplateKey = errors.New("environment template key is reserved")
)

type EnvironmentTemplateService struct {
	templates repository.EnvironmentTemplateRepository
}

func NewEnvironmentTemplateService(templates repository.EnvironmentTemplateRepository) *EnvironmentTemplateService {
	return &EnvironmentTemplateService{templates: templates}
}

func (s *EnvironmentTemplateService) List(ctx context.Context) ([]model.EnvironmentTemplate, error) {
	return s.templates.List(ctx)
}

func (s *EnvironmentTemplateService) Create(ctx context.Context, key, name string, isRequired bool) (*model.EnvironmentTemplate, error) {
	if key == constants.DefaultEnvironmentKey {
		return nil, ErrReservedEnvironmentTemplateKey
	}
	if _, err := s.templates.FindByKey(ctx, key); err == nil {
		return nil, ErrEnvironmentTemplateKeyTaken
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

func (s *EnvironmentTemplateService) Update(ctx context.Context, id uuid.UUID, name string, isRequired bool) (*model.EnvironmentTemplate, error) {
	template, err := s.templates.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrEnvironmentTemplateNotFound
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

func (s *EnvironmentTemplateService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.templates.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrEnvironmentTemplateNotFound
		}
		return err
	}
	return s.templates.Delete(ctx, id)
}
