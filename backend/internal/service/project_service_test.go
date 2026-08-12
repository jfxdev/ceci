package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
)

type fakeProjectRepository struct {
	projects map[uuid.UUID]*model.Project
	members  map[string]*model.ProjectMember // "projectID|userID" -> member
}

func newFakeProjectRepository() *fakeProjectRepository {
	return &fakeProjectRepository{
		projects: map[uuid.UUID]*model.Project{},
		members:  map[string]*model.ProjectMember{},
	}
}

func memberKey(projectID, userID uuid.UUID) string { return projectID.String() + "|" + userID.String() }

func (f *fakeProjectRepository) Create(ctx context.Context, p *model.Project) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	f.projects[p.ID] = p
	return nil
}

func (f *fakeProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	p, ok := f.projects[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return p, nil
}

func (f *fakeProjectRepository) Update(ctx context.Context, p *model.Project) error {
	f.projects[p.ID] = p
	return nil
}

func (f *fakeProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	delete(f.projects, id)
	return nil
}

func (f *fakeProjectRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error) {
	var out []model.Project
	for _, m := range f.members {
		if m.UserID == userID {
			out = append(out, *f.projects[m.ProjectID])
		}
	}
	return out, nil
}

func (f *fakeProjectRepository) AddMember(ctx context.Context, m *model.ProjectMember) error {
	f.members[memberKey(m.ProjectID, m.UserID)] = m
	return nil
}

func (f *fakeProjectRepository) UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role constants.ProjectRole) error {
	if m, ok := f.members[memberKey(projectID, userID)]; ok {
		m.Role = role
	}
	return nil
}

func (f *fakeProjectRepository) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	delete(f.members, memberKey(projectID, userID))
	return nil
}

func (f *fakeProjectRepository) FindMember(ctx context.Context, projectID, userID uuid.UUID) (*model.ProjectMember, error) {
	m, ok := f.members[memberKey(projectID, userID)]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return m, nil
}

func (f *fakeProjectRepository) ListMembers(ctx context.Context, projectID uuid.UUID) ([]repository.MemberWithUser, error) {
	var out []repository.MemberWithUser
	for _, m := range f.members {
		if m.ProjectID == projectID {
			out = append(out, repository.MemberWithUser{UserID: m.UserID, Role: m.Role})
		}
	}
	return out, nil
}

func TestProjectService_CreateAddsOwner(t *testing.T) {
	projRepo := newFakeProjectRepository()
	userRepo := newFakeUserRepository()
	svc := NewProjectService(projRepo, userRepo)
	ctx := context.Background()

	creator := uuid.New()
	p, err := svc.Create(ctx, creator, "Alpha", "alpha")
	require.NoError(t, err)

	role, err := svc.RoleOf(ctx, p.ID, creator)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleOwner, role)
}

func TestProjectService_RoleOf_NotMember(t *testing.T) {
	svc := NewProjectService(newFakeProjectRepository(), newFakeUserRepository())
	_, err := svc.RoleOf(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestProjectService_AddMember(t *testing.T) {
	projRepo := newFakeProjectRepository()
	userRepo := newFakeUserRepository()
	svc := NewProjectService(projRepo, userRepo)
	ctx := context.Background()

	target, err := svc.Create(ctx, uuid.New(), "Alpha", "alpha")
	require.NoError(t, err)

	member := &model.User{ID: uuid.New(), Email: "m@b.com"}
	require.NoError(t, userRepo.Create(ctx, member))

	require.NoError(t, svc.AddMember(ctx, target.ID, "m@b.com", constants.RoleEditor))

	err = svc.AddMember(ctx, target.ID, "m@b.com", constants.RoleEditor)
	assert.ErrorIs(t, err, ErrAlreadyMember)

	err = svc.AddMember(ctx, target.ID, "missing@b.com", constants.RoleEditor)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestProjectService_UpdateAndDelete(t *testing.T) {
	svc := NewProjectService(newFakeProjectRepository(), newFakeUserRepository())
	ctx := context.Background()

	p, err := svc.Create(ctx, uuid.New(), "Alpha", "alpha")
	require.NoError(t, err)

	updated, err := svc.Update(ctx, p.ID, "Beta")
	require.NoError(t, err)
	assert.Equal(t, "Beta", updated.Name)

	require.NoError(t, svc.Delete(ctx, p.ID))
	_, err = svc.Get(ctx, p.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestProjectService_RemoveAndUpdateMemberRole(t *testing.T) {
	projRepo := newFakeProjectRepository()
	userRepo := newFakeUserRepository()
	svc := NewProjectService(projRepo, userRepo)
	ctx := context.Background()

	creator := uuid.New()
	p, err := svc.Create(ctx, creator, "Alpha", "alpha")
	require.NoError(t, err)

	require.NoError(t, svc.UpdateMemberRole(ctx, p.ID, creator, constants.RoleAdmin))
	role, err := svc.RoleOf(ctx, p.ID, creator)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleAdmin, role)

	require.NoError(t, svc.RemoveMember(ctx, p.ID, creator))
	_, err = svc.RoleOf(ctx, p.ID, creator)
	assert.ErrorIs(t, err, ErrMemberNotFound)
}
