// Package flag holds feature-flag domain logic — CRUD, evaluation and
// change-notification — split out from internal/service so it can be
// maintained and tested independently of the rest of the service layer.
// Models (model.FeatureFlag) and persistence (repository.FlagRepository)
// stay in their existing packages — only the business logic moved here.
package flag

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
	"leaflag/backend/internal/service/strategy"
)

var ErrNotFound = errors.New("flag not found")

// Strategy validation/creation types and errors are aliased from the
// strategy package (see internal/service/strategy) so Service's public
// surface — and everything that already imports it (routes, tests) —
// doesn't need to change just because the targeting-strategy logic moved
// into its own, independently testable package.
type (
	StrategyInput        = strategy.Input
	StrategyVariantInput = strategy.VariantInput
)

var (
	ErrVariantTypeMismatch = strategy.ErrVariantTypeMismatch
	ErrUnknownFlagType     = strategy.ErrUnknownFlagType
	ErrDuplicatePriority   = strategy.ErrDuplicatePriority
)

type Service struct {
	flags       repository.FlagRepository
	broadcaster *Broadcaster
}

func NewService(flags repository.FlagRepository) *Service {
	return &Service{flags: flags, broadcaster: NewBroadcaster()}
}

func (s *Service) List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error) {
	return s.flags.List(ctx, projectID, environmentID)
}

func (s *Service) Get(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error) {
	f, err := s.flags.FindByKey(ctx, projectID, environmentID, key)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return f, nil
}

// Create defines a new flag (shared across all environments) and configures
// its targeting strategies for the environment it's being created in. Other
// environments start it disabled until explicitly configured (see
// strategy.Flatten).
func (s *Service) Create(ctx context.Context, projectID, environmentID uuid.UUID, key, name, description, flagType string, enabled bool, strategies []StrategyInput, prerequisiteFlagKey, prerequisiteVariant string, actors ...uuid.UUID) (*model.FeatureFlag, error) {
	if err := strategy.Validate(flagType, strategies); err != nil {
		return nil, err
	}
	flag := &model.FeatureFlag{
		ProjectID:           projectID,
		Key:                 key,
		Name:                name,
		Description:         description,
		FlagType:            flagType,
		PrerequisiteFlagKey: prerequisiteFlagKey,
		PrerequisiteVariant: prerequisiteVariant,
	}
	if len(actors) > 0 && actors[0] != uuid.Nil {
		flag.CreatedByID = &actors[0]
		flag.Collaborators = []model.FlagCollaborator{{UserID: actors[0]}}
	}
	if err := s.flags.Create(ctx, flag); err != nil {
		return nil, err
	}
	if err := s.flags.ReplaceStrategies(ctx, flag.ID, environmentID, strategy.ToModels(flag.ID, environmentID, strategies)); err != nil {
		return nil, err
	}
	if err := s.flags.UpsertEnvironmentConfig(ctx, flag.ID, environmentID, enabled); err != nil {
		return nil, err
	}
	s.broadcaster.Publish(projectID)
	return s.flags.FindByKey(ctx, projectID, environmentID, key)
}

type UpdateInput struct {
	Name                *string
	Description         *string
	Tags                *[]string
	Enabled             *bool
	Strategies          []StrategyInput
	PrerequisiteFlagKey *string
	PrerequisiteVariant *string
}

func (s *Service) Update(ctx context.Context, projectID, environmentID uuid.UUID, key string, in UpdateInput, actors ...uuid.UUID) (*model.FeatureFlag, error) {
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
	if in.Tags != nil {
		flag.Tags = datatypes.NewJSONSlice(normalizeTags(*in.Tags))
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
	changed := coreChanged
	if in.Strategies != nil {
		if err := strategy.Validate(flag.FlagType, in.Strategies); err != nil {
			return nil, err
		}
		if err := s.flags.ReplaceStrategies(ctx, flag.ID, environmentID, strategy.ToModels(flag.ID, environmentID, in.Strategies)); err != nil {
			return nil, err
		}
		changed = true
	}
	if in.Enabled != nil {
		if err := s.flags.UpsertEnvironmentConfig(ctx, flag.ID, environmentID, *in.Enabled); err != nil {
			return nil, err
		}
		changed = true
	}
	if changed && len(actors) > 0 {
		if err := s.flags.AddCollaborator(ctx, flag.ID, actors[0]); err != nil {
			return nil, err
		}
	}
	s.broadcaster.Publish(projectID)
	return s.flags.FindByKey(ctx, projectID, environmentID, key)
}

func normalizeTags(tags []string) []string {
	normalized := make([]string, 0, len(tags))
	seen := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, exists := seen[tag]; exists {
			continue
		}
		seen[tag] = struct{}{}
		normalized = append(normalized, tag)
	}
	return normalized
}

func (s *Service) Delete(ctx context.Context, projectID uuid.UUID, key string) error {
	if err := s.flags.Delete(ctx, projectID, key); err != nil {
		return err
	}
	s.broadcaster.Publish(projectID)
	return nil
}

func (s *Service) Archive(ctx context.Context, projectID uuid.UUID, key string) error {
	if err := s.flags.SetArchived(ctx, projectID, key, true); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	s.broadcaster.Publish(projectID)
	return nil
}

func (s *Service) Unarchive(ctx context.Context, projectID uuid.UUID, key string) error {
	if err := s.flags.SetArchived(ctx, projectID, key, false); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrNotFound
		}
		return err
	}
	s.broadcaster.Publish(projectID)
	return nil
}

// Subscribe registers an OFREP SSE listener for a project's flag changes.
// The returned cancel func must be called when the subscriber disconnects.
func (s *Service) Subscribe(projectID uuid.UUID) (<-chan struct{}, func()) {
	return s.broadcaster.Subscribe(projectID)
}

// Evaluate resolves a single flag against an evaluation context (OFREP-shaped).
func (s *Service) Evaluate(ctx context.Context, projectID, environmentID uuid.UUID, key string, evalCtx map[string]any) EvaluationResult {
	flag, err := s.flags.FindByKey(ctx, projectID, environmentID, key)
	if err != nil {
		return EvaluationResult{Key: key, Reason: constants.ReasonError, ErrorCode: constants.ErrCodeFlagNotFound}
	}
	return s.resolveWithPrerequisites(ctx, projectID, environmentID, flag, evalCtx, map[string]bool{flag.Key: true})
}

// EvaluateAll resolves every flag in a project against an evaluation context (OFREP bulk).
func (s *Service) EvaluateAll(ctx context.Context, projectID, environmentID uuid.UUID, evalCtx map[string]any) ([]EvaluationResult, error) {
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
func (s *Service) resolveWithPrerequisites(ctx context.Context, projectID, environmentID uuid.UUID, flag *model.FeatureFlag, evalCtx map[string]any, visited map[string]bool) EvaluationResult {
	if flag.PrerequisiteFlagKey == "" {
		return strategy.Evaluate(strategy.Flatten(flag), evalCtx)
	}
	if visited[flag.PrerequisiteFlagKey] || len(visited) >= maxPrerequisiteDepth {
		return strategy.DefaultResult(strategy.Flatten(flag), constants.ReasonPrerequisiteFailed)
	}
	prereq, err := s.flags.FindByKey(ctx, projectID, environmentID, flag.PrerequisiteFlagKey)
	if err != nil {
		return strategy.DefaultResult(strategy.Flatten(flag), constants.ReasonPrerequisiteFailed)
	}
	visited[flag.PrerequisiteFlagKey] = true
	prereqResult := s.resolveWithPrerequisites(ctx, projectID, environmentID, prereq, evalCtx, visited)
	if prereqResult.Variant != flag.PrerequisiteVariant {
		return strategy.DefaultResult(strategy.Flatten(flag), constants.ReasonPrerequisiteFailed)
	}
	return strategy.Evaluate(strategy.Flatten(flag), evalCtx)
}

// Version returns an opaque, monotonically-changing token for a project's
// flag set, suitable for use as an HTTP ETag by OFREP polling clients.
func (s *Service) Version(ctx context.Context, projectID uuid.UUID) (string, error) {
	count, maxUpdated, err := s.flags.Version(ctx, projectID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`"%d-%d"`, count, maxUpdated.UnixNano()), nil
}
