package db

import (
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAutoMigrate(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, AutoMigrate(gdb))

	// Running twice should be idempotent.
	require.NoError(t, AutoMigrate(gdb))
}

func TestConnect_InvalidDSN(t *testing.T) {
	_, err := Connect("not a valid dsn")
	require.Error(t, err)
}
