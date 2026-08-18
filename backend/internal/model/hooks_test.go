package model

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBeforeCreateHooks_GenerateIDWhenNil(t *testing.T) {
	u := &User{}
	require := assert.New(t)
	require.NoError(u.BeforeCreate(nil))
	require.NotEqual(uuid.Nil, u.ID)

	p := &Project{}
	require.NoError(p.BeforeCreate(nil))
	require.NotEqual(uuid.Nil, p.ID)

	f := &FeatureFlag{}
	require.NoError(f.BeforeCreate(nil))
	require.NotEqual(uuid.Nil, f.ID)
}

func TestBeforeCreateHooks_PreserveExistingID(t *testing.T) {
	existing := uuid.New()
	u := &User{ID: existing}
	assert.NoError(t, u.BeforeCreate(nil))
	assert.Equal(t, existing, u.ID, "should not overwrite an already-set ID")
}
