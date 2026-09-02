package db

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"leaflag/backend/internal/model"
)

func TestAutoMigrate(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, AutoMigrate(gdb))

	// Running twice should be idempotent.
	require.NoError(t, AutoMigrate(gdb))
}

func TestAutoMigrate_DeduplicatesLegacyStrategyPriorities(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.Exec(`CREATE TABLE flag_strategies (
		id text primary key, flag_id text not null, environment_id text not null,
		priority integer not null, name text, description text, is_default boolean not null default false,
		condition_json blob, default_variant text not null, rollout_json blob
	)`).Error)

	flagID, environmentID := uuid.New(), uuid.New()
	first, duplicate := uuid.New(), uuid.New()
	require.NoError(t, gdb.Exec("INSERT INTO flag_strategies (id, flag_id, environment_id, priority, default_variant) VALUES (?, ?, ?, ?, 'off'), (?, ?, ?, ?, 'off')",
		first, flagID, environmentID, 1, duplicate, flagID, environmentID, 1).Error)

	require.NoError(t, AutoMigrate(gdb))
	var strategies []model.FlagStrategy
	require.NoError(t, gdb.Where("flag_id = ? AND environment_id = ? AND priority = ?", flagID, environmentID, 1).Find(&strategies).Error)
	require.Len(t, strategies, 1)
	require.Contains(t, []uuid.UUID{first, duplicate}, strategies[0].ID)
}

func TestConnect_InvalidDSN(t *testing.T) {
	_, err := Connect("not a valid dsn")
	require.Error(t, err)
}
