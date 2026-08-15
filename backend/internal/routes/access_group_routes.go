package routes

import (
	"context"
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"leaflag/backend/internal/dto"
	"leaflag/backend/internal/middleware"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
	"leaflag/backend/internal/service"
)

type accessGroupService interface {
	List(ctx context.Context) ([]model.AccessGroup, error)
	Create(ctx context.Context, name, description string) (*model.AccessGroup, error)
	Delete(ctx context.Context, id uuid.UUID) error
	ListMembers(ctx context.Context, groupID uuid.UUID) ([]repository.MemberWithUser, error)
	AddMember(ctx context.Context, groupID uuid.UUID, email string) error
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	ListOIDCMappings(ctx context.Context, groupID uuid.UUID) ([]model.OIDCAccessGroupMapping, error)
	AddOIDCMapping(ctx context.Context, groupID uuid.UUID, externalGroup string) error
	RemoveOIDCMapping(ctx context.Context, groupID uuid.UUID, externalGroup string) error
}

func RegisterAccessGroupRoutes(rg *gin.RouterGroup, auth middleware.TokenParser, admins adminAuthorizer, groups accessGroupService) {
	group := rg.Group("/admin/access-groups")
	group.Use(middleware.RequireAuth(auth))
	group.GET("", func(c *gin.Context) {
		items, err := groups.List(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to list access groups"})
			return
		}
		out := make([]dto.AccessGroupDTO, 0, len(items))
		for _, item := range items {
			out = append(out, toAccessGroupDTO(&item))
		}
		c.JSON(http.StatusOK, out)
	})
	admin := group.Group("")
	admin.Use(requireAdmin(admins))
	admin.POST("", func(c *gin.Context) {
		var req dto.CreateAccessGroupRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		item, err := groups.Create(c.Request.Context(), req.Name, req.Description)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to create access group"})
			return
		}
		c.JSON(http.StatusCreated, toAccessGroupDTO(item))
	})
	admin.DELETE("/:groupID", func(c *gin.Context) {
		groupID, ok := parseAccessGroupID(c)
		if !ok {
			return
		}
		if err := groups.Delete(c.Request.Context(), groupID); err != nil {
			if errors.Is(err, service.ErrAccessGroupNotFound) {
				c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "access group not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to delete access group"})
			return
		}
		c.Status(http.StatusNoContent)
	})
	admin.GET("/:groupID/members", func(c *gin.Context) {
		groupID, ok := parseAccessGroupID(c)
		if !ok {
			return
		}
		members, err := groups.ListMembers(c.Request.Context(), groupID)
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "access group not found"})
			return
		}
		out := make([]dto.MemberDTO, 0, len(members))
		for _, member := range members {
			out = append(out, dto.MemberDTO{UserID: member.UserID.String(), Email: member.Email, Name: member.Name})
		}
		c.JSON(http.StatusOK, out)
	})
	admin.POST("/:groupID/members", func(c *gin.Context) {
		groupID, ok := parseAccessGroupID(c)
		if !ok {
			return
		}
		var req dto.AddAccessGroupMemberRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		err := groups.AddMember(c.Request.Context(), groupID, req.Email)
		switch {
		case err == nil:
			c.Status(http.StatusCreated)
		case errors.Is(err, service.ErrUserNotFound):
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "user not found"})
		case errors.Is(err, service.ErrAccessGroupMember):
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "user is already a group member"})
		default:
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to add group member"})
		}
	})
	admin.DELETE("/:groupID/members/:userID", func(c *gin.Context) {
		groupID, ok := parseAccessGroupID(c)
		if !ok {
			return
		}
		userID, err := uuid.Parse(c.Param("userID"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid user id"})
			return
		}
		if err := groups.RemoveMember(c.Request.Context(), groupID, userID); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to remove group member"})
			return
		}
		c.Status(http.StatusNoContent)
	})
	admin.GET("/:groupID/oidc-mappings", func(c *gin.Context) {
		groupID, ok := parseAccessGroupID(c)
		if !ok {
			return
		}
		mappings, err := groups.ListOIDCMappings(c.Request.Context(), groupID)
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "access group not found"})
			return
		}
		out := make([]dto.OIDCAccessGroupMappingDTO, 0, len(mappings))
		for _, mapping := range mappings {
			out = append(out, dto.OIDCAccessGroupMappingDTO{ExternalGroup: mapping.ExternalGroup})
		}
		c.JSON(http.StatusOK, out)
	})
	admin.POST("/:groupID/oidc-mappings", func(c *gin.Context) {
		groupID, ok := parseAccessGroupID(c)
		if !ok {
			return
		}
		var req dto.AddOIDCAccessGroupMappingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		err := groups.AddOIDCMapping(c.Request.Context(), groupID, req.ExternalGroup)
		switch {
		case err == nil:
			c.Status(http.StatusCreated)
		case errors.Is(err, service.ErrOIDCMappingExists):
			c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "oidc group is already mapped"})
		default:
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to add oidc group mapping"})
		}
	})
	admin.DELETE("/:groupID/oidc-mappings/:externalGroup", func(c *gin.Context) {
		groupID, ok := parseAccessGroupID(c)
		if !ok {
			return
		}
		externalGroup, err := url.PathUnescape(c.Param("externalGroup"))
		if err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid oidc group"})
			return
		}
		if err := groups.RemoveOIDCMapping(c.Request.Context(), groupID, externalGroup); err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to remove oidc group mapping"})
			return
		}
		c.Status(http.StatusNoContent)
	})
}

func parseAccessGroupID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("groupID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid access group id"})
		return uuid.Nil, false
	}
	return id, true
}

func toAccessGroupDTO(group *model.AccessGroup) dto.AccessGroupDTO {
	return dto.AccessGroupDTO{ID: group.ID.String(), Name: group.Name, Description: group.Description}
}
