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
	"leaflag/backend/internal/service/auth"
)

// authService is the subset of AuthService behavior routes depend on, kept as
// an interface so route handlers can be tested with a fake implementation.
type authService interface {
	Login(ctx context.Context, email, password string) (accessToken, refreshToken string, user *model.User, err error)
	Register(ctx context.Context, email, password, name string) (*model.User, error)
	Refresh(ctx context.Context, rawRefresh string) (accessToken, newRefreshToken string, err error)
	Logout(ctx context.Context, rawRefresh string) error
	Me(ctx context.Context, userID uuid.UUID) (*model.User, error)
	UpdateLocale(ctx context.Context, userID uuid.UUID, locale string) (*model.User, error)
	ParseAccessToken(tokenStr string) (uuid.UUID, error)
}

func RegisterAuthRoutes(rg *gin.RouterGroup, authSvc *auth.Service) {
	registerAuthRoutes(rg, authSvc)
}

func registerAuthRoutes(rg *gin.RouterGroup, authSvc authService) {
	loginRateLimit := middleware.RateLimitPerIP(constants.LoginRateLimitPerMinute, constants.LoginRateLimitBurst)

	rg.POST("/auth/login", loginRateLimit, func(c *gin.Context) {
		var req dto.LoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		accessToken, refreshToken, user, err := authSvc.Login(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "invalid credentials"})
			return
		}
		setRefreshCookie(c, refreshToken)
		c.JSON(http.StatusOK, dto.LoginResponse{
			AccessToken: accessToken,
			User:        toUserDTO(user),
		})
	})

	registerRateLimit := middleware.RateLimitPerIP(constants.LoginRateLimitPerMinute, constants.LoginRateLimitBurst)

	rg.POST("/auth/register", registerRateLimit, func(c *gin.Context) {
		var req dto.RegisterRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: err.Error()})
			return
		}
		if _, err := authSvc.Register(c.Request.Context(), req.Email, req.Password, req.Name); err != nil {
			if errors.Is(err, auth.ErrEmailTaken) {
				c.JSON(http.StatusConflict, dto.ErrorResponse{Error: "email already registered"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to register"})
			return
		}
		accessToken, refreshToken, user, err := authSvc.Login(c.Request.Context(), req.Email, req.Password)
		if err != nil {
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to register"})
			return
		}
		setRefreshCookie(c, refreshToken)
		c.JSON(http.StatusCreated, dto.LoginResponse{
			AccessToken: accessToken,
			User:        toUserDTO(user),
		})
	})

	rg.POST("/auth/refresh", func(c *gin.Context) {
		rawRefresh, err := c.Cookie(constants.RefreshCookieName)
		if err != nil || rawRefresh == "" {
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{Error: "missing refresh token"})
			return
		}
		accessToken, newRefresh, err := authSvc.Refresh(c.Request.Context(), rawRefresh)
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
			_ = authSvc.Logout(c.Request.Context(), rawRefresh)
		}
		c.SetCookie(constants.RefreshCookieName, "", -1, "/", "", false, true)
		c.Status(http.StatusNoContent)
	})

	rg.GET("/me", middleware.RequireAuth(authSvc), func(c *gin.Context) {
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		user, err := authSvc.Me(c.Request.Context(), userID)
		if err != nil {
			c.JSON(http.StatusNotFound, dto.ErrorResponse{Error: "user not found"})
			return
		}
		c.JSON(http.StatusOK, toUserDTO(user))
	})

	rg.PUT("/me/preferences", middleware.RequireAuth(authSvc), func(c *gin.Context) {
		var req dto.UpdateUserPreferencesRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid preferences", Code: "validation.invalid_preferences"})
			return
		}
		userID := c.MustGet(middleware.ContextUserIDKey).(uuid.UUID)
		user, err := authSvc.UpdateLocale(c.Request.Context(), userID, req.Locale)
		if err != nil {
			if errors.Is(err, auth.ErrInvalidLocale) {
				c.JSON(http.StatusBadRequest, dto.ErrorResponse{Error: "invalid locale", Code: "validation.invalid_locale"})
				return
			}
			c.JSON(http.StatusInternalServerError, dto.ErrorResponse{Error: "failed to update preferences", Code: "preferences.update_failed"})
			return
		}
		c.JSON(http.StatusOK, toUserDTO(user))
	})
}

func toUserDTO(user *model.User) dto.UserDTO {
	locale := user.Locale
	if locale == "" {
		locale = "en"
	}
	return dto.UserDTO{ID: user.ID.String(), Email: user.Email, Name: user.Name, IsAdmin: user.IsAdmin, Locale: locale}
}

func setRefreshCookie(c *gin.Context, token string) {
	maxAge := int(constants.RefreshTokenTTL.Seconds())
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     constants.RefreshCookieName,
		Value:    token,
		MaxAge:   maxAge,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   requestIsSecure(c),
	})
}
