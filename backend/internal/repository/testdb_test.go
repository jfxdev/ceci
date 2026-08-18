package repository

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

// newTestDB spins up an in-memory sqlite database for repository tests.
// Model structs use postgres-specific defaults (gen_random_uuid()) that
// sqlite doesn't understand, so callers must set IDs explicitly before
// inserting — mirroring what postgres would otherwise generate for them.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// Named per-test so parallel/sequential tests don't share the same
	// process-wide in-memory database (which "file::memory:" without a
	// unique name would do under cache=shared, causing cross-test collisions).
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&model.User{},
		&model.AuthIdentity{},
		&model.RefreshToken{},
		&model.AccessGroup{},
		&model.AccessGroupMember{},
		&model.OIDCAccessGroupMapping{},
		&model.ProjectAccessGroup{},
		&model.InstanceSettings{},
		&model.Project{},
		&model.ProjectMember{},
		&model.Environment{},
		&model.EnvironmentTemplate{},
		&model.ContextField{},
		&model.ContextFieldValue{},
		&model.ProjectAPIKey{},
		&model.FeatureFlag{},
		&model.FlagEnvironmentConfig{},
		&model.FlagStrategy{},
		&model.FlagStrategyVariant{},
		&model.Parameter{},
		&model.ParameterVersion{},
	))
	return db
}
