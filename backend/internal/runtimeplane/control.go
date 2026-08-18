package runtimeplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

// ControlSource builds snapshots only inside the Control Plane. It is never
// constructed by data-plane mode, keeping PostgreSQL out of the runtime path.
type ControlSource struct {
	db *gorm.DB
}

func NewControlSource(db *gorm.DB) *ControlSource {
	return &ControlSource{db: db}
}

func (s *ControlSource) Load(ctx context.Context, ifNoneMatch string) (*Snapshot, string, bool, error) {
	var keys []model.ProjectAPIKey
	if err := s.db.WithContext(ctx).
		Where("revoked_at IS NULL").
		Order("key_hash").
		Find(&keys).Error; err != nil {
		return nil, "", false, err
	}
	var flags []model.FeatureFlag
	if err := s.db.WithContext(ctx).
		Preload("Configs", func(tx *gorm.DB) *gorm.DB { return tx.Order("environment_id") }).
		Preload("Strategies", func(tx *gorm.DB) *gorm.DB { return tx.Order("environment_id, is_default, priority") }).
		Preload("Strategies.Variants").
		Order("project_id, key").
		Find(&flags).Error; err != nil {
		return nil, "", false, err
	}
	var parameters []model.Parameter
	if err := s.db.WithContext(ctx).Order("project_id, environment_id, key").Find(&parameters).Error; err != nil {
		return nil, "", false, err
	}

	snapshot := &Snapshot{
		APIKeys:    make([]APIKey, 0, len(keys)),
		Flags:      flags,
		Parameters: make([]RuntimeParameter, 0, len(parameters)),
	}
	for _, key := range keys {
		snapshot.APIKeys = append(snapshot.APIKeys, APIKey{KeyHash: key.KeyHash, ProjectID: key.ProjectID, EnvironmentID: key.EnvironmentID})
	}
	for _, parameter := range parameters {
		snapshot.Parameters = append(snapshot.Parameters, RuntimeParameter{
			ProjectID: parameter.ProjectID, EnvironmentID: parameter.EnvironmentID,
			Key: parameter.Key, Value: parameter.Value, Version: parameter.Version,
		})
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return nil, "", false, fmt.Errorf("encode runtime snapshot: %w", err)
	}
	sum := sha256.Sum256(payload)
	etag := `"` + hex.EncodeToString(sum[:]) + `"`
	if etag == ifNoneMatch {
		return nil, etag, true, nil
	}
	return snapshot, etag, false, nil
}
