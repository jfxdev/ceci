package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
)

func TestProjectRepository_CreateFindUpdateDelete(t *testing.T) {
	repo := NewProjectRepository(newTestDB(t))
	ctx := context.Background()

	p := &model.Project{Name: "Alpha", Slug: "alpha"}
	require.NoError(t, repo.Create(ctx, p))
	require.NotEqual(t, uuid.Nil, p.ID)

	found, err := repo.FindByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Alpha", found.Name)

	found.Name = "Beta"
	require.NoError(t, repo.Update(ctx, found))
	updated, err := repo.FindByID(ctx, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Beta", updated.Name)

	require.NoError(t, repo.Delete(ctx, p.ID))
	_, err = repo.FindByID(ctx, p.ID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestProjectRepository_MembersAndListForUser(t *testing.T) {
	repo := NewProjectRepository(newTestDB(t))
	ctx := context.Background()

	p := &model.Project{Name: "Alpha", Slug: "alpha"}
	require.NoError(t, repo.Create(ctx, p))
	userID := uuid.New()

	_, err := repo.FindMember(ctx, p.ID, userID)
	assert.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, repo.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: userID, Role: constants.RoleOwner}))

	m, err := repo.FindMember(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleOwner, m.Role)

	projects, err := repo.ListForUser(ctx, userID)
	require.NoError(t, err)
	require.Len(t, projects, 1)
	assert.Equal(t, p.ID, projects[0].ID)

	require.NoError(t, repo.UpdateMemberRole(ctx, p.ID, userID, constants.RoleAdmin))
	m, err = repo.FindMember(ctx, p.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleAdmin, m.Role)

	require.NoError(t, repo.RemoveMember(ctx, p.ID, userID))
	_, err = repo.FindMember(ctx, p.ID, userID)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestProjectRepository_ListMembers(t *testing.T) {
	db := newTestDB(t)
	projectRepo := NewProjectRepository(db)
	userRepo := NewUserRepository(db)
	ctx := context.Background()

	p := &model.Project{Name: "Alpha", Slug: "alpha"}
	require.NoError(t, projectRepo.Create(ctx, p))

	u := &model.User{Email: "a@b.com", PasswordHash: "hash", Name: "Ana"}
	require.NoError(t, userRepo.Create(ctx, u))
	require.NoError(t, projectRepo.AddMember(ctx, &model.ProjectMember{ProjectID: p.ID, UserID: u.ID, Role: constants.RoleEditor}))

	members, err := projectRepo.ListMembers(ctx, p.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, "a@b.com", members[0].Email)
	assert.Equal(t, constants.RoleEditor, members[0].Role)
}
