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
	"leaflag/backend/internal/service/flag"
)

// flagService is the subset of FlagService behavior routes depend on.
type flagService interface {
	List(ctx context.Context, projectID, environmentID uuid.UUID) ([]model.FeatureFlag, error)
	Get(ctx context.Context, projectID, environmentID uuid.UUID, key string) (*model.FeatureFlag, error)
	Create(ctx context.Context, projectID, environmentID uuid.UUID, key, name, description, flagType string, enabled bool, strategies []flag.StrategyInput, prerequisiteFlagKey, prerequisiteVariant string, actors ...uuid.UUID) (*model.FeatureFlag, error)
	Update(ctx context.Context, projectID, environmentID uuid.UUID, key string, in flag.UpdateInput, actors ...uuid.UUID) (*model.FeatureFlag, error)
	Archive(ctx context.Context, projectID uuid.UUID, key string) error
	Unarchive(ctx context.Context, projectID uuid.UUID, key string) error
	Delete(ctx context.Context, projectID uuid.UUID, key string) error
}

func RegisterFlagRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, roleResolver middleware.ProjectRoleResolver, envResolver middleware.EnvironmentResolver, flags *flag.Service) {
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
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		enabled := true
		if req.Enabled != nil {
			enabled = *req.Enabled
		}
		f, err := flags.Create(c.Request.Context(), projectID, environmentID, req.Key, req.Name, req.Description, req.FlagType, enabled,
			toStrategyInputs(req.Strategies), req.PrerequisiteFlagKey, req.PrerequisiteVariant, userID)
		if err != nil {
			if errors.Is(err, flag.ErrVariantTypeMismatch) || errors.Is(err, flag.ErrUnknownFlagType) || errors.Is(err, flag.ErrDuplicatePriority) {
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
				return
			}
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
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		in := flag.UpdateInput{Enabled: req.Enabled, Name: req.Name, Description: req.Description, Tags: req.Tags}
		if req.Strategies != nil {
			in.Strategies = toStrategyInputs(req.Strategies)
		}
		in.PrerequisiteFlagKey = req.PrerequisiteFlagKey
		in.PrerequisiteVariant = req.PrerequisiteVariant
		f, err := flags.Update(c.Request.Context(), projectID, environmentID, c.Param("key"), in, userID)
		if err != nil {
			if errors.Is(err, flag.ErrNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "flag not found"})
				return
			}
			if errors.Is(err, flag.ErrVariantTypeMismatch) || errors.Is(err, flag.ErrUnknownFlagType) || errors.Is(err, flag.ErrDuplicatePriority) {
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
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

	scoped.POST("/:key/archive", middleware.RequireProjectRole(roleResolver, constants.RoleEditor), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := flags.Archive(c.Request.Context(), projectID, c.Param("key")); err != nil {
			if errors.Is(err, flag.ErrNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "flag not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to archive flag"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	scoped.POST("/:key/unarchive", middleware.RequireProjectRole(roleResolver, constants.RoleEditor), middleware.RequireProjectEnvironment(envResolver), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := flags.Unarchive(c.Request.Context(), projectID, c.Param("key")); err != nil {
			if errors.Is(err, flag.ErrNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "flag not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to unarchive flag"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func toFlagDTO(f *model.FeatureFlag) dto.FlagDTO {
	tags := []string(f.Tags)
	if tags == nil {
		tags = []string{}
	}
	collaborators := make([]dto.FlagUserDTO, 0, len(f.Collaborators))
	for _, collaborator := range f.Collaborators {
		collaborators = append(collaborators, dto.FlagUserDTO{
			ID:   collaborator.UserID.String(),
			Name: collaborator.User.Name,
		})
	}
	var createdBy *dto.FlagUserDTO
	if f.CreatedBy != nil {
		createdBy = &dto.FlagUserDTO{ID: f.CreatedBy.ID.String(), Name: f.CreatedBy.Name}
	}
	strategies := make([]dto.StrategyDTO, 0, len(f.Strategies))
	for _, st := range f.Strategies {
		variants := make([]dto.StrategyVariantDTO, 0, len(st.Variants))
		for _, v := range st.Variants {
			variants = append(variants, dto.StrategyVariantDTO{Key: v.Key, Value: []byte(v.Value)})
		}
		strategies = append(strategies, dto.StrategyDTO{
			Order:          st.Priority,
			Name:           st.Name,
			Description:    st.Description,
			IsDefault:      st.IsDefault,
			Condition:      []byte(st.ConditionJSON),
			DefaultVariant: st.DefaultVariant,
			Rollout:        []byte(st.RolloutJSON),
			Variants:       variants,
		})
	}
	var enabled bool
	if len(f.Configs) > 0 {
		enabled = f.Configs[0].Enabled
	}
	return dto.FlagDTO{
		ID:                  f.ID.String(),
		Key:                 f.Key,
		Name:                f.Name,
		Description:         f.Description,
		Tags:                tags,
		FlagType:            f.FlagType,
		Enabled:             enabled,
		Archived:            f.ArchivedAt != nil,
		Strategies:          strategies,
		PrerequisiteFlagKey: f.PrerequisiteFlagKey,
		PrerequisiteVariant: f.PrerequisiteVariant,
		CreatedAt:           f.CreatedAt,
		CreatedBy:           createdBy,
		Collaborators:       collaborators,
	}
}

func toStrategyInputs(in []dto.StrategyInput) []flag.StrategyInput {
	out := make([]flag.StrategyInput, 0, len(in))
	for _, st := range in {
		variants := make([]flag.StrategyVariantInput, 0, len(st.Variants))
		for _, v := range st.Variants {
			variants = append(variants, flag.StrategyVariantInput{Key: v.Key, Value: v.Value})
		}
		out = append(out, flag.StrategyInput{
			Order:          st.Order,
			Name:           st.Name,
			Description:    st.Description,
			IsDefault:      st.IsDefault,
			ConditionJSON:  st.ConditionJSON,
			DefaultVariant: st.DefaultVariant,
			RolloutJSON:    st.RolloutJSON,
			Variants:       variants,
		})
	}
	return out
}
