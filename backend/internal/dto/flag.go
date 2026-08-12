package dto

import "encoding/json"

type FlagVariantInput struct {
	Key   string          `json:"key" binding:"required"`
	Value json.RawMessage `json:"value" binding:"required"`
}

type FlagRuleInput struct {
	Priority      int             `json:"priority"`
	Description   string          `json:"description"`
	ConditionJSON json.RawMessage `json:"condition" binding:"required"`
	VariantKey    string          `json:"variantKey" binding:"required"`
	RolloutJSON   json.RawMessage `json:"rollout"`
}

type CreateFlagRequest struct {
	Key            string             `json:"key" binding:"required"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	FlagType       string             `json:"flagType" binding:"required"`
	DefaultVariant string             `json:"defaultVariant" binding:"required"`
	Variants       []FlagVariantInput `json:"variants" binding:"required,min=1"`
	Rules          []FlagRuleInput    `json:"rules"`
}

type UpdateFlagRequest struct {
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	Enabled        *bool              `json:"enabled"`
	DefaultVariant string             `json:"defaultVariant"`
	Variants       []FlagVariantInput `json:"variants"`
	Rules          []FlagRuleInput    `json:"rules"`
}

type FlagVariantDTO struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type FlagRuleDTO struct {
	Priority    int             `json:"priority"`
	Description string          `json:"description"`
	Condition   json.RawMessage `json:"condition"`
	VariantKey  string          `json:"variantKey"`
	Rollout     json.RawMessage `json:"rollout,omitempty"`
}

type FlagDTO struct {
	ID             string           `json:"id"`
	Key            string           `json:"key"`
	Name           string           `json:"name"`
	Description    string           `json:"description"`
	FlagType       string           `json:"flagType"`
	Enabled        bool             `json:"enabled"`
	DefaultVariant string           `json:"defaultVariant"`
	Variants       []FlagVariantDTO `json:"variants"`
	Rules          []FlagRuleDTO    `json:"rules"`
}
