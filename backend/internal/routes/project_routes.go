package routes

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/dto"
	"ceci/backend/internal/middleware"
	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
	"ceci/backend/internal/service"
)

// projectService is the subset of ProjectService behavior routes depend on.
type projectService interface {
	Create(ctx context.Context, creatorID uuid.UUID, name, slug string) (*model.Project, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Project, error)
	Update(ctx context.Context, id uuid.UUID, name string) (*model.Project, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error)
	RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error)
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]repository.MemberWithUser, error)
	AddMember(ctx context.Context, projectID uuid.UUID, email string, role constants.ProjectRole) error
	UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role constants.ProjectRole) error
	RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error
}

func RegisterProjectRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, projects *service.ProjectService) {
	registerProjectRoutes(rg, auth, projects)
}

func registerProjectRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, projects projectService) {
	authed := rg.Group("/projects")
	authed.Use(middleware.RequireAuth(auth))

	authed.GET("", func(c *gin.Context) {
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		list, err := projects.ListForUser(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list projects"})
			return
		}
		out := make([]dto.ProjectDTO, 0, len(list))
		for _, p := range list {
			out = append(out, dto.ProjectDTO{ID: p.ID.String(), Name: p.Name, Slug: p.Slug})
		}
		c.JSON(http.StatusOK, out)
	})

	authed.POST("", func(c *gin.Context) {
		var req dto.CreateProjectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		p, err := projects.Create(c.Request.Context(), userID, req.Name, req.Slug)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create project"})
			return
		}
		c.JSON(http.StatusCreated, dto.ProjectDTO{ID: p.ID.String(), Name: p.Name, Slug: p.Slug, Role: string(constants.RoleOwner)})
	})

	resolver := roleResolverAdapter{svc: projects}
	scoped := authed.Group("/:projectID")

	scoped.GET("", middleware.RequireProjectRole(resolver, constants.RoleViewer), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		p, err := projects.Get(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "project not found"})
			return
		}
		role := c.MustGet(middleware.ContextProjectRoleKey).(constants.ProjectRole)
		c.JSON(http.StatusOK, dto.ProjectDTO{ID: p.ID.String(), Name: p.Name, Slug: p.Slug, Role: string(role)})
	})

	scoped.PATCH("", middleware.RequireProjectRole(resolver, constants.RoleOwner), func(c *gin.Context) {
		var req dto.UpdateProjectRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		p, err := projects.Update(c.Request.Context(), projectID, req.Name)
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "project not found"})
			return
		}
		c.JSON(http.StatusOK, dto.ProjectDTO{ID: p.ID.String(), Name: p.Name, Slug: p.Slug})
	})

	scoped.DELETE("", middleware.RequireProjectRole(resolver, constants.RoleOwner), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := projects.Delete(c.Request.Context(), projectID); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete project"})
			return
		}
		c.Status(http.StatusNoContent)
	})

	scoped.GET("/members", middleware.RequireProjectRole(resolver, constants.RoleViewer), func(c *gin.Context) {
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		members, err := projects.ListMembers(c.Request.Context(), projectID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list members"})
			return
		}
		out := make([]dto.MemberDTO, 0, len(members))
		for _, m := range members {
			out = append(out, dto.MemberDTO{UserID: m.UserID.String(), Email: m.Email, Name: m.Name, Role: string(m.Role)})
		}
		c.JSON(http.StatusOK, out)
	})

	scoped.POST("/members", middleware.RequireProjectRole(resolver, constants.RoleAdmin), func(c *gin.Context) {
		var req dto.AddMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		if !req.Role.Valid() {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid role"})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		err := projects.AddMember(c.Request.Context(), projectID, req.Email, req.Role)
		switch {
		case err == nil:
			c.Status(http.StatusCreated)
		case errors.Is(err, service.ErrUserNotFound):
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "user not found"})
		case errors.Is(err, service.ErrAlreadyMember):
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "user is already a member"})
		default:
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to add member"})
		}
	})

	scoped.PATCH("/members/:userID", middleware.RequireProjectRole(resolver, constants.RoleAdmin), func(c *gin.Context) {
		memberID, err := uuid.Parse(c.Param("userID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid user id"})
			return
		}
		var req dto.UpdateMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		if !req.Role.Valid() {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid role"})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := projects.UpdateMemberRole(c.Request.Context(), projectID, memberID, req.Role); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update member"})
			return
		}
		c.Status(http.StatusOK)
	})

	scoped.DELETE("/members/:userID", middleware.RequireProjectRole(resolver, constants.RoleAdmin), func(c *gin.Context) {
		memberID, err := uuid.Parse(c.Param("userID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid user id"})
			return
		}
		projectID := c.MustGet(middleware.ContextProjectIDKey).(uuid.UUID)
		if err := projects.RemoveMember(c.Request.Context(), projectID, memberID); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to remove member"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}

// roleResolverAdapter adapts projectService.RoleOf to middleware.ProjectRoleResolver.
type roleResolverAdapter struct {
	svc projectService
}

func (a roleResolverAdapter) RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	return a.svc.RoleOf(ctx, projectID, userID)
}
