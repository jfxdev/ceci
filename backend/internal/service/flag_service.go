package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var ErrFlagNotFound = errors.New("flag not found")

type FlagService struct {
	flags       repository.FlagRepository
	broadcaster *FlagBroadcaster
}

func NewFlagService(flags repository.FlagRepository) *FlagService {
	return &FlagService{flags: flags, broadcaster: NewFlagBroadcaster()}
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

func (s *FlagService) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error) {
	return s.flags.List(ctx, projectID, environmentID)
}

func (s *FlagService) Get(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error) {
	f, err := s.flags.FindByKey(ctx, projectID, environmentID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrFlagNotFound
		}
		return nil, err
	}
	return f, nil
}

// Create defines a new flag (shared across all environments) and configures
// it — enabled state, default variant, targeting rules — for the
// environment it's being created in. Other environments start it disabled
// until explicitly configured (see envFlag in flag_evaluate.go).
func (s *FlagService) Create(ctx context.Context, projectID, environmentID uuid.UUID, key, name, description, flagType, defaultVariant string, enabled bool, variants []VariantInput, rules []RuleInput, prerequisiteFlagKey, prerequisiteVariant string) (*model.FeatureFlag, error) {
	flag := &model.FeatureFlag{
		ProjectID:           projectID,
		Key:                 key,
		Name:                name,
		Description:         description,
		FlagType:            flagType,
		PrerequisiteFlagKey: prerequisiteFlagKey,
		PrerequisiteVariant: prerequisiteVariant,
	}
	if err := s.flags.Create(ctx, flag); err != nil {
		return nil, err
	}
	if err := s.flags.ReplaceVariantsAndRules(ctx, flag.ID, environmentID, toVariantModels(flag.ID, variants), toRuleModels(flag.ID, environmentID, rules)); err != nil {
		return nil, err
	}
	if err := s.flags.UpsertEnvironmentConfig(ctx, flag.ID, environmentID, enabled, defaultVariant); err != nil {
		return nil, err
	}
	s.broadcaster.Publish(projectID)
	return s.flags.FindByKey(ctx, projectID, environmentID, key)
}

type UpdateInput struct {
	Name                *string
	Description         *string
	Enabled             *bool
	DefaultVariant      *string
	Variants            []VariantInput
	Rules               []RuleInput
	PrerequisiteFlagKey *string
	PrerequisiteVariant *string
}

func (s *FlagService) Update(ctx context.Context, projectID, environmentID uuid.UUID, key string, in UpdateInput) (*model.FeatureFlag, error) {
	flag, err := s.Get(ctx, projectID, environmentID, key)
	if err != nil {
		return nil, err
	}
	coreChanged := false
	if in.Name != nil {
		flag.Name = *in.Name
		coreChanged = true
	}
	if in.Description != nil {
		flag.Description = *in.Description
		coreChanged = true
	}
	if in.PrerequisiteFlagKey != nil {
		flag.PrerequisiteFlagKey = *in.PrerequisiteFlagKey
		coreChanged = true
	}
	if in.PrerequisiteVariant != nil {
		flag.PrerequisiteVariant = *in.PrerequisiteVariant
		coreChanged = true
	}
	if coreChanged {
		if err := s.flags.UpdateCore(ctx, flag); err != nil {
			return nil, err
		}
	}
	if in.Variants != nil || in.Rules != nil {
		if err := s.flags.ReplaceVariantsAndRules(ctx, flag.ID, environmentID, toVariantModels(flag.ID, in.Variants), toRuleModels(flag.ID, environmentID, in.Rules)); err != nil {
			return nil, err
		}
	}
	if in.Enabled != nil || in.DefaultVariant != nil {
		enabled := currentEnabled(flag)
		defaultVariant := currentDefaultVariant(flag)
		if in.Enabled != nil {
			enabled = *in.Enabled
		}
		if in.DefaultVariant != nil {
			defaultVariant = *in.DefaultVariant
		}
		if err := s.flags.UpsertEnvironmentConfig(ctx, flag.ID, environmentID, enabled, defaultVariant); err != nil {
			return nil, err
		}
	}
	s.broadcaster.Publish(projectID)
	return s.flags.FindByKey(ctx, projectID, environmentID, key)
}

func currentEnabled(flag *model.FeatureFlag) bool {
	if len(flag.Configs) > 0 {
		return flag.Configs[0].Enabled
	}
	return false
}

func currentDefaultVariant(flag *model.FeatureFlag) string {
	if len(flag.Configs) > 0 {
		return flag.Configs[0].DefaultVariant
	}
	if len(flag.Variants) > 0 {
		return flag.Variants[0].Key
	}
	return ""
}

func (s *FlagService) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	if err := s.flags.Delete(ctx, projectID, key); err != nil {
		return err
	}
	s.broadcaster.Publish(projectID)
	return nil
}

// Subscribe registers an OFREP SSE listener for a project's flag changes.
// The returned cancel func must be called when the subscriber disconnects.
func (s *FlagService) Subscribe(projectID uuid.UUID) (<-chan struct{}, func()) {
	return s.broadcaster.Subscribe(projectID)
}

// Evaluate resolves a single flag against an evaluation context (OFREP-shaped).
func (s *FlagService) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx map[string]any) EvaluationResult {
	flag, err := s.flags.FindByKey(ctx, projectID, environmentID, key)
	if err != nil {
		return EvaluationResult{Key: key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeFlagNotFound}
	}
	return s.resolveWithPrerequisites(ctx, projectID, environmentID, flag, evalCtx, map[string]bool{flag.Key: true})
}

// EvaluateAll resolves every flag in a project against an evaluation context (OFREP bulk).
func (s *FlagService) EvaluateAll(ctx context.Context, projectID, environmentID uuid.UUID, evalCtx map[string]any) ([]EvaluationResult, error) {
	flags, err := s.flags.List(ctx, projectID, environmentID)
	if err != nil {
		return nil, err
	}
	results := make([]EvaluationResult, 0, len(flags))
	for i := range flags {
		results = append(results, s.resolveWithPrerequisites(ctx, projectID, environmentID, &flags[i], evalCtx, map[string]bool{flags[i].Key: true}))
	}
	return results, nil
}

// maxPrerequisiteDepth bounds prerequisite-chain resolution so a
// misconfigured cycle (A depends on B depends on A) can't hang evaluation.
const maxPrerequisiteDepth = 5

// resolveWithPrerequisites evaluates flag normally unless it declares a
// prerequisite, in which case the prerequisite flag is resolved first (with
// cycle/depth guarding via visited) and flag falls back to its default
// variant unless the prerequisite resolves to the required variant.
func (s *FlagService) resolveWithPrerequisites(ctx context.Context, projectID, environmentID uuid.UUID, flag *model.FeatureFlag, evalCtx map[string]any, visited map[string]bool) EvaluationResult {
	if flag.PrerequisiteFlagKey == "" {
		return evaluateFlag(toEnvFlag(flag), evalCtx)
	}
	if visited[flag.PrerequisiteFlagKey] || len(visited) >= maxPrerequisiteDepth {
		return resultFor(toEnvFlag(flag), currentDefaultVariant(flag), constants.ReasonPrerequisiteFailed)
	}
	prereq, err := s.flags.FindByKey(ctx, projectID, environmentID, flag.PrerequisiteFlagKey)
	if err != nil {
		return resultFor(toEnvFlag(flag), currentDefaultVariant(flag), constants.ReasonPrerequisiteFailed)
	}
	visited[flag.PrerequisiteFlagKey] = true
	prereqResult := s.resolveWithPrerequisites(ctx, projectID, environmentID, prereq, evalCtx, visited)
	if prereqResult.Variant != flag.PrerequisiteVariant {
		return resultFor(toEnvFlag(flag), currentDefaultVariant(flag), constants.ReasonPrerequisiteFailed)
	}
	return evaluateFlag(toEnvFlag(flag), evalCtx)
}

// Version returns an opaque, monotonically-changing token for a project's
// flag set, suitable for use as an HTTP ETag by OFREP polling clients.
func (s *FlagService) Version(ctx context.Context, projectID uuid.UUID) (string, error) {
	count, maxUpdated, err := s.flags.Version(ctx, projectID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`"%d-%d"`, count, maxUpdated.UnixNano()), nil
}

func toVariantModels(flagID uuid.UUID, in []VariantInput) []model.FlagVariant {
	out := make([]model.FlagVariant, 0, len(in))
	for _, v := range in {
		out = append(out, model.FlagVariant{FlagID: flagID, Key: v.Key, Value: datatypes.JSON(v.Value)})
	}
	return out
}

func toRuleModels(flagID, environmentID uuid.UUID, in []RuleInput) []model.FlagRule {
	out := make([]model.FlagRule, 0, len(in))
	for _, r := range in {
		out = append(out, model.FlagRule{
			FlagID:        flagID,
			EnvironmentID: environmentID,
			Priority:      r.Priority,
			Description:   r.Description,
			ConditionJSON: datatypes.JSON(r.ConditionJSON),
			VariantKey:    r.VariantKey,
			RolloutJSON:   datatypes.JSON(r.RolloutJSON),
		})
	}
	return out
}
