package constants

import "time"

type ProjectRole string

const (
	RoleOwner  ProjectRole = "owner"
	RoleAdmin  ProjectRole = "admin"
	RoleEditor ProjectRole = "editor"
	RoleViewer ProjectRole = "viewer"
)

// roleRank enables comparisons like "does this role meet the minimum required".
var roleRank = map[ProjectRole]int{
	RoleViewer: 1,
	RoleEditor: 2,
	RoleAdmin:  3,
	RoleOwner:  4,
}

func (r ProjectRole) Meets(min ProjectRole) bool {
	return roleRank[r] >= roleRank[min]
}

func (r ProjectRole) Valid() bool {
	_, ok := roleRank[r]
	return ok
}

// OFREP evaluation reason codes.
const (
	ReasonStatic             = "STATIC"
	ReasonTargetingMatch     = "TARGETING_MATCH"
	ReasonDefault            = "DEFAULT"
	ReasonDisabled           = "DISABLED"
	ReasonError              = "ERROR"
	ReasonPrerequisiteFailed = "PREREQUISITE_FAILED"
	ReasonSplit              = "SPLIT"
)

// OFREP error codes.
const (
	ErrCodeFlagNotFound     = "FLAG_NOT_FOUND"
	ErrCodeParseError       = "PARSE_ERROR"
	ErrCodeGeneral          = "GENERAL"
	ErrCodeProviderNotReady = "PROVIDER_NOT_READY"
)

const (
	AccessTokenTTL  = 15 * time.Minute
	RefreshTokenTTL = 30 * 24 * time.Hour
)

const (
	LoginRateLimitPerMinute = 10
	LoginRateLimitBurst     = 5
)

const (
	Argon2Memory      uint32 = 64 * 1024
	Argon2Iterations  uint32 = 3
	Argon2Parallelism uint8  = 2
	Argon2SaltLength  uint32 = 16
	Argon2KeyLength   uint32 = 32
)

const (
	AuthHeaderName      = "Authorization"
	BearerPrefix        = "Bearer "
	RefreshCookieName   = "leaflag_refresh"
	ProjectAPIKeyPrefix = "leaflag_sk_"
)

const (
	FlagTypeBoolean = "boolean"
	FlagTypeString  = "string"
	FlagTypeNumber  = "number"
	FlagTypeObject  = "object"
)

const (
	DefaultEnvironmentKey  = "all"
	DefaultEnvironmentName = "All"
)
