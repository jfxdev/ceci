package routes

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/service"
)

// flagService is the subset of FlagService behavior routes depend on.
type flagService interface {
	List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error)
	Get(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error)
	Create(ctx context.Context, projectID, environmentID uuid.UUID, key, name, description, flagType, defaultVariant string, enabled bool, variants []service.VariantInput, rules []service.RuleInput, prerequisiteFlagKey, prerequisiteVariant string) (*model.FeatureFlag, error)
	Update(ctx context.Context, projectID, environmentID uuid.UUID, key string, in service.UpdateInput) (*model.FeatureFlag, error)
	Delete(ctx context.Context, projectID uuid.UUID, key string) error
}

func RegisterFlagRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envResolver middleware.EnvironmentResolver, flags *service.FlagService) {
	registerFlagRoutes(rg, auth, roleResolver, envResolver, flags)
}

func registerFlagRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envResolver middleware.EnvironmentResolver, flags flagService) {
	scoped := rg.Group("/projects/:projectID/environments/:envKey/flags")
	scoped.Use(middleware.RequireAuth(auth))

	scoped.GET("", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		list, err := flags.List(c.Request.Context(), projectID, environmentID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list flags"})
			return
		}
		out := make([]dto.FlagDTO, 0, len(list))
		for _, f := range list {
			out = append(out, toFlagDTO(&f))
		}
		c.JSON(http.StatusOK, out)
	})

	scoped.POST("", middleware.RequireProjectRole(roleResolver, constants.RoleEditor), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		var req dto.CreateFlagRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		f, err := flags.Create(c.Request.Context(), projectID, environmentID, req.Key, req.Name, req.Description, req.FlagType, req.DefaultVariant, enabled,
			toVariantInputs(req.Variants), toRuleInputs(req.Rules), req.PrerequisiteFlagKey, req.PrerequisiteVariant)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create flag"})
			return
		}
		c.JSON(http.StatusCreated, toFlagDTO(f))
	})

	scoped.GET("/:key", middleware.RequireProjectRole(roleResolver, constants.RoleViewer), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		f, err := flags.Get(c.Request.Context(), projectID, environmentID, c.Param("key"))
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "flag not found"})
			return
		}
		c.JSON(http.StatusOK, toFlagDTO(f))
	})

	scoped.PATCH("/:key", middleware.RequireProjectRole(roleResolver, constants.RoleEditor), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		var req dto.UpdateFlagRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		environmentID := c.MustGet(middleware.ContextEnvironmentIDKey).(uuid.UUID)
		in := service.UpdateInput{Enabled: req.Enabled}
		if req.Name != "" {
			in.Name = &req.Name
		}
		if req.Description != "" {
			in.Description = &req.Description
		}
		if req.DefaultVariant != "" {
			in.DefaultVariant = &req.DefaultVariant
		}
		if req.Variants != nil {
			in.Variants = toVariantInputs(req.Variants)
		}
		if req.Rules != nil {
			in.Rules = toRuleInputs(req.Rules)
		}
		in.PrerequisiteFlagKey = req.PrerequisiteFlagKey
		in.PrerequisiteVariant = req.PrerequisiteVariant
		f, err := flags.Update(c.Request.Context(), projectID, environmentID, c.Param("key"), in)
		if err != nil {
			if errors.Is(err, service.ErrFlagNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "flag not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update flag"})
			return
		}
		c.JSON(http.StatusOK, toFlagDTO(f))
	})

	scoped.DELETE("/:key", middleware.RequireProjectRole(roleResolver, constants.RoleAdmin), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := flags.Delete(c.Request.Context(), projectID, c.Param("key")); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete flag"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func toFlagDTO(f *model.FeatureFlag) dto.FlagDTO {
	variants := make([]dto.FlagVariantDTO, 0, len(f.Variants))
	for _, v := range f.Variants {
		variants = append(variants, dto.FlagVariantDTO{Key: v.Key, Value: []byte(v.Value)})
	}
	rules := make([]dto.FlagRuleDTO, 0, len(f.Rules))
	for _, r := range f.Rules {
		rules = append(rules, dto.FlagRuleDTO{
			Priority:    r.Priority,
			Description: r.Description,
			Condition:   []byte(r.ConditionJSON),
			VariantKey:  r.VariantKey,
			Rollout:     []byte(r.RolloutJSON),
		})
	}
	var enabled bool
	var defaultVariant string
	if len(f.Configs) > 0 {
		enabled = f.Configs[0].Enabled
		defaultVariant = f.Configs[0].DefaultVariant
	}
	return dto.FlagDTO{
		ID:                  f.ID.String(),
		Key:                 f.Key,
		Name:                f.Name,
		Description:         f.Description,
		FlagType:            f.FlagType,
		Enabled:             enabled,
		DefaultVariant:      defaultVariant,
		Variants:            variants,
		Rules:               rules,
		PrerequisiteFlagKey: f.PrerequisiteFlagKey,
		PrerequisiteVariant: f.PrerequisiteVariant,
	}
}

func toVariantInputs(in []dto.FlagVariantInput) []service.VariantInput {
	out := make([]service.VariantInput, 0, len(in))
	for _, v := range in {
		out = append(out, service.VariantInput{Key: v.Key, Value: v.Value})
	}
	return out
}

func toRuleInputs(in []dto.FlagRuleInput) []service.RuleInput {
	out := make([]service.RuleInput, 0, len(in))
	for _, r := range in {
		out = append(out, service.RuleInput{
			Priority:      r.Priority,
			Description:   r.Description,
			ConditionJSON: r.ConditionJSON,
			VariantKey:    r.VariantKey,
			RolloutJSON:   r.RolloutJSON,
		})
	}
	return out
}
