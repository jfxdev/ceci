// Package contextfield holds the per-project targeting context-field
// registry domain logic (allowed attribute keys/values for flag targeting
// rules), split out from internal/service so it can be maintained and
// tested independently of the rest of the service layer.
package contextfield

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var (
	ErrNotFound = errors.New("context field not found")
	ErrKeyTaken = errors.New("context field key already exists")
	ErrInvalid  = errors.New("invalid context field")
)

type ValueInput struct {
	Value       string
	Description string
}

type Service struct {
	fields repository.ContextFieldRepository
}

func NewService(fields repository.ContextFieldRepository) *Service {
	return &Service{fields: fields}
}

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]model.ContextField, error) {
	return s.fields.List(ctx, projectID)
}

func (s *Service) Create(ctx context.Context, projectID uuid.UUID, key, description string, values []ValueInput) (*model.ContextField, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, ErrInvalid
	}
	if _, err := s.fields.FindByKey(ctx, projectID, key); err == nil {
		return nil, ErrKeyTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	prepared, err := prepareValues(values)
	if err != nil {
		return nil, err
	}
	field := &model.ContextField{ID: uuid.New(), ProjectID: projectID, Key: key, Description: strings.TrimSpace(description)}
	if err := s.fields.Create(ctx, field); err != nil {
		return nil, err
	}
	if err := s.fields.ReplaceValues(ctx, field.ID, prepared); err != nil {
		return nil, err
	}
	return s.fields.FindByKey(ctx, projectID, key)
}

func (s *Service) Update(ctx context.Context, projectID uuid.UUID, key string, description *string, values *[]ValueInput) (*model.ContextField, error) {
	field, err := s.fields.FindByKey(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if description != nil {
		field.Description = strings.TrimSpace(*description)
	}
	if values != nil {
		prepared, err := prepareValues(*values)
		if err != nil {
			return nil, err
		}
		if err := s.fields.ReplaceValues(ctx, field.ID, prepared); err != nil {
			return nil, err
		}
	}
	if err := s.fields.Update(ctx, field); err != nil {
		return nil, err
	}
	return s.fields.FindByKey(ctx, projectID, key)
}

func (s *Service) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	field, err := s.fields.FindByKey(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	return s.fields.Delete(ctx, field.ID)
}

func prepareValues(input []ValueInput) ([]model.ContextFieldValue, error) {
	seen := make(map[string]bool, len(input))
	values := make([]model.ContextFieldValue, 0, len(input))
	for i, item := range input {
		value := strings.TrimSpace(item.Value)
		if value == "" || seen[value] {
			return nil, fmt.Errorf("%w: values must be non-empty and unique", ErrInvalid)
		}
		seen[value] = true
		values = append(values, model.ContextFieldValue{ID: uuid.New(), Value: value, Description: strings.TrimSpace(item.Description), SortOrder: i})
	}
	return values, nil
}
