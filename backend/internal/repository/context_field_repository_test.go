package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/model"
)

func TestContextFieldRepository_ListValuesAndDelete(t *testing.T) {
	db := newTestDB(t)
	repo := NewContextFieldRepository(db)
	ctx := context.Background()
	projectID := uuid.New()
	field := &model.ContextField{ID: uuid.New(), ProjectID: projectID, Key: "region"}
	require.NoError(t, repo.Create(ctx, field))
	require.NoError(t, repo.ReplaceValues(ctx, field.ID, []model.ContextFieldValue{
		{ID: uuid.New(), Value: "br", SortOrder: 1},
		{ID: uuid.New(), Value: "us", SortOrder: 0},
	}))

	fields, err := repo.List(ctx, projectID)
	require.NoError(t, err)
	require.Len(t, fields, 1)
	assert.Equal(t, []string{"us", "br"}, []string{fields[0].Values[0].Value, fields[0].Values[1].Value})

	require.NoError(t, repo.Delete(ctx, field.ID))
	_, err = repo.FindByKey(ctx, projectID, "region")
	assert.ErrorIs(t, err, ErrNotFound)
}
