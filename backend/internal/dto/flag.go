package dto

import (
	"encoding/json"
	"time"
)

type StrategyVariantInput struct {
	Key   string          `json:"key" binding:"required"`
	Value json.RawMessage `json:"value" binding:"required"`
}

type StrategyInput struct {
	Order          int                    `json:"order"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	IsDefault      bool                   `json:"isDefault"`
	ConditionJSON  json.RawMessage        `json:"condition"`
	DefaultVariant string                 `json:"defaultVariant" binding:"required"`
	RolloutJSON    json.RawMessage        `json:"rollout"`
	Variants       []StrategyVariantInput `json:"variants" binding:"required,min=1"`
}

type CreateFlagRequest struct {
	Key         string `json:"key" binding:"required"`
	Name        string `json:"name"`
	Description string `json:"description"`
	FlagType    string `json:"flagType" binding:"required"`
	Enabled     *bool  `json:"enabled"`
	// Strategies is optional at creation time: when omitted, the backend
	// derives a single starting catch-all strategy from FlagType.
	Strategies          []StrategyInput `json:"strategies"`
	PrerequisiteFlagKey string          `json:"prerequisiteFlagKey,omitempty"`
	PrerequisiteVariant string          `json:"prerequisiteVariant,omitempty"`
}

type UpdateFlagRequest struct {
	Name                *string         `json:"name"`
	Description         *string         `json:"description"`
	Tags                *[]string       `json:"tags"`
	Enabled             *bool           `json:"enabled"`
	Strategies          []StrategyInput `json:"strategies"`
	PrerequisiteFlagKey *string         `json:"prerequisiteFlagKey"`
	PrerequisiteVariant *string         `json:"prerequisiteVariant"`
}

type StrategyVariantDTO struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type StrategyDTO struct {
	Order          int                  `json:"order"`
	Name           string               `json:"name"`
	Description    string               `json:"description"`
	IsDefault      bool                 `json:"isDefault"`
	Condition      json.RawMessage      `json:"condition"`
	DefaultVariant string               `json:"defaultVariant"`
	Rollout        json.RawMessage      `json:"rollout,omitempty"`
	Variants       []StrategyVariantDTO `json:"variants"`
}

type FlagDTO struct {
	ID                  string        `json:"id"`
	Key                 string        `json:"key"`
	Name                string        `json:"name"`
	Description         string        `json:"description"`
	Tags                []string      `json:"tags"`
	FlagType            string        `json:"flagType"`
	Enabled             bool          `json:"enabled"`
	Archived            bool          `json:"archived"`
	Strategies          []StrategyDTO `json:"strategies"`
	PrerequisiteFlagKey string        `json:"prerequisiteFlagKey,omitempty"`
	PrerequisiteVariant string        `json:"prerequisiteVariant,omitempty"`
	CreatedAt           time.Time     `json:"createdAt"`
	CreatedBy           *FlagUserDTO  `json:"createdBy,omitempty"`
	Collaborators       []FlagUserDTO `json:"collaborators"`
}

type FlagUserDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
