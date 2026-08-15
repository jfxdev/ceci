package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
)

func TestAccessGroupRepository_GrantsProjectAccess(t *testing.T) {
	db := newTestDB(t)
	groups := NewAccessGroupRepository(db)
	ctx := context.Background()

	project := &model.Project{Name: "Payments", Slug: "payments"}
	require.NoError(t, db.Create(project).Error)
	user := &model.User{Email: "ana@example.com", PasswordHash: "hash", Name: "Ana"}
	require.NoError(t, db.Create(user).Error)
	group := &model.AccessGroup{Name: "Payments editors", Description: "Payment service team"}
	require.NoError(t, groups.Create(ctx, group))
	require.NotEqual(t, uuid.Nil, group.ID)
	require.NoError(t, groups.AddMember(ctx, &model.AccessGroupMember{AccessGroupID: group.ID, UserID: user.ID}))
	require.NoError(t, groups.GrantProject(ctx, &model.ProjectAccessGroup{ProjectID: project.ID, AccessGroupID: group.ID, Role: constants.RoleEditor}))

	role, err := groups.RoleForUserInProject(ctx, project.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleEditor, role)
	projects, err := groups.ListProjectsForUser(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, project.ID, projects[0].ID)
	grants, err := groups.ListProjectGrants(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, group.Name, grants[0].Name)

	require.NoError(t, groups.RevokeProjectGrant(ctx, project.ID, group.ID))
	_, err = groups.RoleForUserInProject(ctx, project.ID, user.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}
