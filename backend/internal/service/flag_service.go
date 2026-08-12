package service

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
)

var ErrFlagNotFound = errors.New("flag not found")

type FlagService struct {
	flags repository.FlagRepository
}

func NewFlagService(flags repository.FlagRepository) *FlagService {
	return &FlagService{flags: flags}
}

type VariantInput struct {
	Key   string
	Value []byte
}

type RuleInput struct {
	Priority      int
	Description   string
	ConditionJSON []byte
	VariantKey    string
	RolloutJSON   []byte
}

func (s *FlagService) List(ctx context.Context, projectID uuid.UUID) ([]model.FeatureFlag, error) {
	return s.flags.List(ctx, projectID)
}

func (s *FlagService) Get(ctx context.Context, projectID uuid.UUID, key string) (*model.FeatureFlag, error) {
	f, err := s.flags.FindByKey(ctx, projectID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFlagNotFound
		}
		return nil, err
	}
	return f, nil
}

func (s *FlagService) Create(ctx context.Context, projectID uuid.UUID, key, name, description, flagType, defaultVariant string, variants []VariantInput, rules []RuleInput) (*model.FeatureFlag, error) {
	flag := &model.FeatureFlag{
		ProjectID:      projectID,
		Key:            key,
		Name:           name,
		Description:    description,
		FlagType:       flagType,
		Enabled:        true,
		DefaultVariant: defaultVariant,
	}
	if err := s.flags.Create(ctx, flag); err != nil {
		return nil, err
	}
	if err := s.flags.ReplaceVariantsAndRules(ctx, flag.ID, toVariantModels(flag.ID, variants), toRuleModels(flag.ID, rules)); err != nil {
		return nil, err
	}
	return s.flags.FindByKey(ctx, projectID, key)
}

type UpdateInput struct {
	Name           *string
	Description    *string
	Enabled        *bool
	DefaultVariant *string
	Variants       []VariantInput
	Rules          []RuleInput
}

func (s *FlagService) Update(ctx context.Context, projectID uuid.UUID, key string, in UpdateInput) (*model.FeatureFlag, error) {
	flag, err := s.Get(ctx, projectID, key)
	if err != nil {
		return nil, err
	}
	if in.Name != nil {
		flag.Name = *in.Name
	}
	if in.Description != nil {
		flag.Description = *in.Description
	}
	if in.Enabled != nil {
		flag.Enabled = *in.Enabled
	}
	if in.DefaultVariant != nil {
		flag.DefaultVariant = *in.DefaultVariant
	}
	if err := s.flags.Update(ctx, flag); err != nil {
		return nil, err
	}
	if in.Variants != nil || in.Rules != nil {
		if err := s.flags.ReplaceVariantsAndRules(ctx, flag.ID, toVariantModels(flag.ID, in.Variants), toRuleModels(flag.ID, in.Rules)); err != nil {
			return nil, err
		}
	}
	return s.flags.FindByKey(ctx, projectID, key)
}

func (s *FlagService) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	return s.flags.Delete(ctx, projectID, key)
}

// Evaluate resolves a single flag against an evaluation context (OFREP-shaped).
func (s *FlagService) Evaluate(ctx context.Context, projectID uuid.UUID, key string, evalCtx map[string]any) EvaluationResult {
	flag, err := s.flags.FindByKey(ctx, projectID, key)
	if err != nil {
		return EvaluationResult{Key: key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeFlagNotFound}
	}
	return evaluateFlag(flag, evalCtx)
}

// EvaluateAll resolves every flag in a project against an evaluation context (OFREP bulk).
func (s *FlagService) EvaluateAll(ctx context.Context, projectID uuid.UUID, evalCtx map[string]any) ([]EvaluationResult, error) {
	flags, err := s.flags.List(ctx, projectID)
	if err != nil {
		return nil, err
	}
	results := make([]EvaluationResult, 0, len(flags))
	for i := range flags {
		results = append(results, evaluateFlag(&flags[i], evalCtx))
	}
	return results, nil
}

func toVariantModels(flagID uuid.UUID, in []VariantInput) []model.FlagVariant {
	out := make([]model.FlagVariant, 0, len(in))
	for _, v := range in {
		out = append(out, model.FlagVariant{FlagID: flagID, Key: v.Key, Value: datatypes.JSON(v.Value)})
	}
	return out
}

func toRuleModels(flagID uuid.UUID, in []RuleInput) []model.FlagRule {
	out := make([]model.FlagRule, 0, len(in))
	for _, r := range in {
		out = append(out, model.FlagRule{
			FlagID:        flagID,
			Priority:      r.Priority,
			Description:   r.Description,
			ConditionJSON: datatypes.JSON(r.ConditionJSON),
			VariantKey:    r.VariantKey,
			RolloutJSON:   datatypes.JSON(r.RolloutJSON),
		})
	}
	return out
}
