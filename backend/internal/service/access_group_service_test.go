package service

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

func newAccessGroupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Project{}, &model.ProjectMember{}, &model.AccessGroup{}, &model.AccessGroupMember{}, &model.OIDCAccessGroupMapping{}, &model.ProjectAccessGroup{}))
	return db
}

func newAccessGroupServiceForTest(t *testing.T) (*AccessGroupService, repository.AccessGroupRepository, repository.UserRepository, *gorm.DB) {
	t.Helper()
	db := newAccessGroupTestDB(t)
	groups := repository.NewAccessGroupRepository(db)
	users := repository.NewUserRepository(db)
	return NewAccessGroupService(groups, users), groups, users, db
}

func TestAccessGroupService_CreateTrimsAndRejectsDuplicate(t *testing.T) {
	svc, _, _, _ := newAccessGroupServiceForTest(t)
	ctx := context.Background()

	group, err := svc.Create(ctx, "  Payments editors  ", "  Payment team  ")
	require.NoError(t, err)
	assert.Equal(t, "Payments editors", group.Name)
	assert.Equal(t, "Payment team", group.Description)

	_, err = svc.Create(ctx, "Payments editors", "Different description")
	assert.ErrorIs(t, err, ErrAccessGroupTaken)
}

func TestAccessGroupService_AddMemberValidatesUserAndDuplicate(t *testing.T) {
	svc, groups, users, _ := newAccessGroupServiceForTest(t)
	ctx := context.Background()
	group, err := svc.Create(ctx, "Payments viewers", "")
	require.NoError(t, err)

	err = svc.AddMember(ctx, group.ID, "missing@example.com")
	assert.ErrorIs(t, err, ErrUserNotFound)
	user := &model.User{Email: "ana@example.com", PasswordHash: "hash", Name: "Ana"}
	require.NoError(t, users.Create(ctx, user))
	require.NoError(t, svc.AddMember(ctx, group.ID, user.Email))
	err = svc.AddMember(ctx, group.ID, user.Email)
	assert.ErrorIs(t, err, ErrAccessGroupMember)

	members, err := svc.ListMembers(ctx, group.ID)
	require.NoError(t, err)
	require.Len(t, members, 1)
	assert.Equal(t, user.ID, members[0].UserID)
	require.NoError(t, svc.RemoveMember(ctx, group.ID, user.ID))
	_, err = groups.FindMember(ctx, group.ID, user.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestAccessGroupService_ProjectGrantsExcludeOwnerAndPreventDuplicates(t *testing.T) {
	svc, groups, _, db := newAccessGroupServiceForTest(t)
	ctx := context.Background()
	group, err := svc.Create(ctx, "Payments engineers", "")
	require.NoError(t, err)
	project := &model.Project{Name: "Payments", Slug: "payments"}
	require.NoError(t, db.Create(project).Error)

	err = svc.GrantProject(ctx, project.ID, group.ID, constants.RoleOwner)
	assert.ErrorIs(t, err, ErrGroupOwnerRole)
	require.NoError(t, svc.GrantProject(ctx, project.ID, group.ID, constants.RoleViewer))
	err = svc.GrantProject(ctx, project.ID, group.ID, constants.RoleViewer)
	assert.ErrorIs(t, err, ErrGroupGrantExists)
	require.NoError(t, svc.UpdateProjectGrant(ctx, project.ID, group.ID, constants.RoleAdmin))
	grants, err := svc.ListProjectGrants(ctx, project.ID)
	require.NoError(t, err)
	require.Len(t, grants, 1)
	assert.Equal(t, constants.RoleAdmin, grants[0].Role)
	require.NoError(t, svc.RevokeProjectGrant(ctx, project.ID, group.ID))
	_, err = groups.FindProjectGrant(ctx, project.ID, group.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestProjectService_UsesHighestDirectOrGroupRole(t *testing.T) {
	db := newAccessGroupTestDB(t)
	groups := repository.NewAccessGroupRepository(db)
	users := repository.NewUserRepository(db)
	projects := repository.NewProjectRepository(db)
	projectService := NewProjectService(projects, users, nil, nil, groups)
	ctx := context.Background()
	project := &model.Project{Name: "Payments", Slug: "payments"}
	user := &model.User{Email: "ana@example.com", PasswordHash: "hash"}
	group := &model.AccessGroup{Name: "Payments editors"}
	require.NoError(t, db.Create(project).Error)
	require.NoError(t, users.Create(ctx, user))
	require.NoError(t, groups.Create(ctx, group))
	require.NoError(t, projects.AddMember(ctx, &model.ProjectMember{ProjectID: project.ID, UserID: user.ID, Role: constants.RoleViewer}))
	require.NoError(t, groups.AddMember(ctx, &model.AccessGroupMember{AccessGroupID: group.ID, UserID: user.ID}))
	require.NoError(t, groups.GrantProject(ctx, &model.ProjectAccessGroup{ProjectID: project.ID, AccessGroupID: group.ID, Role: constants.RoleEditor}))

	role, err := projectService.RoleOf(ctx, project.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleEditor, role)
	require.NoError(t, projects.UpdateMemberRole(ctx, project.ID, user.ID, constants.RoleOwner))
	role, err = projectService.RoleOf(ctx, project.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleOwner, role)
	listed, err := projectService.ListForUser(ctx, user.ID)
	require.NoError(t, err)
	assert.Len(t, listed, 1, "direct and group membership must not duplicate the project")
	assert.Equal(t, project.ID, listed[0].ID)
}

func TestAccessGroupService_DeleteMissingGroup(t *testing.T) {
	svc, _, _, _ := newAccessGroupServiceForTest(t)
	assert.ErrorIs(t, svc.Delete(context.Background(), uuid.New()), ErrAccessGroupNotFound)
}

func TestAccessGroupService_SyncOIDCGroupsKeepsLocalMembership(t *testing.T) {
	svc, groups, users, _ := newAccessGroupServiceForTest(t)
	ctx := context.Background()
	group, err := svc.Create(ctx, "Payments editors", "")
	require.NoError(t, err)
	user := &model.User{Email: "ana@example.com", PasswordHash: "hash"}
	require.NoError(t, users.Create(ctx, user))
	require.NoError(t, svc.AddOIDCMapping(ctx, group.ID, "entra-group-guid"))
	require.NoError(t, svc.SyncOIDCGroups(ctx, user.ID, []string{"entra-group-guid"}))
	membership, err := groups.FindMember(ctx, group.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "oidc", membership.Source)
	require.NoError(t, svc.SyncOIDCGroups(ctx, user.ID, nil))
	_, err = groups.FindMember(ctx, group.ID, user.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)

	require.NoError(t, svc.AddMember(ctx, group.ID, user.Email))
	require.NoError(t, svc.SyncOIDCGroups(ctx, user.ID, []string{"entra-group-guid"}))
	membership, err = groups.FindMember(ctx, group.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "local", membership.Source)
	require.NoError(t, svc.SyncOIDCGroups(ctx, user.ID, nil))
	_, err = groups.FindMember(ctx, group.ID, user.ID)
	assert.NoError(t, err, "removing the external group must not remove local access")
}
