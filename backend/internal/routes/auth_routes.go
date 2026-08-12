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
	"ceci/backend/internal/service"
)

// authService is the subset of AuthService behavior routes depend on, kept as
// an interface so route handlers can be tested with a fake implementation.
type authService interface {
	Login(ctx context.Context, email, password string) (accessToken, refreshToken string, user *model.User, err error)
	Register(ctx context.Context, email, password, name string) (*model.User, error)
	Refresh(ctx context.Context, rawRefresh string) (accessToken, newRefreshToken string, err error)
	Logout(ctx context.Context, rawRefresh string) error
	Me(ctx context.Context, userID uuid.UUID) (*model.User, error)
	ParseAccessToken(tokenStr string) (uuid.UUID, error)
}

func RegisterAuthRoutes(rg *gin.RouterGroup, auth *service.AuthService) {
	registerAuthRoutes(rg, auth)
}

func registerAuthRoutes(rg *gin.RouterGroup, auth authService) {
	loginRateLimit := middleware.RateLimitPerIP(constants.LoginRateLimitPerMinute, constants.LoginRateLimitBurst)

	rg.POST("/auth/login", loginRateLimit, func(c *gin.Context) {
		var req dto.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		accessToken, refreshToken, user, err := auth.Login(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid credentials"})
			return
		}
		setRefreshCookie(c, refreshToken)
		c.JSON(http.StatusOK, dto.LoginResponse{
			AccessToken: accessToken,
			User:        dto.UserDTO{ID: user.ID.String(), Email: user.Email, Name: user.Name},
		})
	})

	registerRateLimit := middleware.RateLimitPerIP(constants.LoginRateLimitPerMinute, constants.LoginRateLimitBurst)

	rg.POST("/auth/register", registerRateLimit, func(c *gin.Context) {
		var req dto.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		if _, err := auth.Register(c.Request.Context(), req.Email, req.Password, req.Name); err != nil {
			if errors.Is(err, service.ErrEmailTaken) {
				c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "email already registered"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to register"})
			return
		}
		accessToken, refreshToken, user, err := auth.Login(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to register"})
			return
		}
		setRefreshCookie(c, refreshToken)
		c.JSON(http.StatusCreated, dto.LoginResponse{
			AccessToken: accessToken,
			User:        dto.UserDTO{ID: user.ID.String(), Email: user.Email, Name: user.Name},
		})
	})

	rg.POST("/auth/refresh", func(c *gin.Context) {
		rawRefresh, err := c.Cookie(constants.RefreshCookieName)
		if err != nil || rawRefresh == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing refresh token"})
			return
		}
		accessToken, newRefresh, err := auth.Refresh(c.Request.Context(), rawRefresh)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid refresh token"})
			return
		}
		setRefreshCookie(c, newRefresh)
		c.JSON(http.StatusOK, gin.H{"accessToken": accessToken})
	})

	rg.POST("/auth/logout", func(c *gin.Context) {
		rawRefresh, err := c.Cookie(constants.RefreshCookieName)
		if err == nil && rawRefresh != "" {
			_ = auth.Logout(c.Request.Context(), rawRefresh)
		}
		c.SetCookie(constants.RefreshCookieName, "", -1, "/", "", false, true)
		c.Status(http.StatusNoContent)
	})

	rg.GET("/me", middleware.RequireAuth(auth), func(c *gin.Context) {
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		user, err := auth.Me(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusOK, dto.UserDTO{ID: user.ID.String(), Email: user.Email, Name: user.Name})
	})
}

func setRefreshCookie(c *gin.Context, token string) {
	maxAge := int(constants.RefreshTokenTTL.Seconds())
	c.SetCookie(constants.RefreshCookieName, token, maxAge, "/", "", false, true)
}
