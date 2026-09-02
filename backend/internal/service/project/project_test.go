package project

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
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

type fakeEnvironmentRepository struct {
	envs map[uuid.UUID]*model.Environment
}

func newFakeEnvironmentRepository() *fakeEnvironmentRepository {
	return &fakeEnvironmentRepository{envs: map[uuid.UUID]*model.Environment{}}
}

func (f *fakeEnvironmentRepository) Create(ctx context.Context, env *model.Environment) error {
	if env.ID == uuid.Nil {
		env.ID = uuid.New()
	}
	f.envs[env.ID] = env
	return nil
}

func (f *fakeEnvironmentRepository) List(ctx context.Context, projectID uuid.UUID) ([]model.Environment, error) {
	var out []model.Environment
	for _, e := range f.envs {
		if e.ProjectID == projectID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (f *fakeEnvironmentRepository) FindByKey(ctx context.Context, projectID uuid.UUID, key string) (*model.Environment, error) {
	for _, e := range f.envs {
		if e.ProjectID == projectID && e.Key == key {
			return e, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeEnvironmentRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Environment, error) {
	e, ok := f.envs[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return e, nil
}

func (f *fakeEnvironmentRepository) Update(ctx context.Context, env *model.Environment) error {
	f.envs[env.ID] = env
	return nil
}

func (f *fakeEnvironmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	delete(f.envs, id)
	return nil
}

func (f *fakeEnvironmentRepository) Count(ctx context.Context, projectID uuid.UUID) (int64, error) {
	var count int64
	for _, e := range f.envs {
		if e.ProjectID == projectID {
			count++
		}
	}
	return count, nil
}

type fakeEnvironmentTemplateRepository struct {
	templates map[uuid.UUID]*model.EnvironmentTemplate
}

func newFakeEnvironmentTemplateRepository() *fakeEnvironmentTemplateRepository {
	return &fakeEnvironmentTemplateRepository{templates: map[uuid.UUID]*model.EnvironmentTemplate{}}
}

func (f *fakeEnvironmentTemplateRepository) Create(ctx context.Context, template *model.EnvironmentTemplate) error {
	if template.ID == uuid.Nil {
		template.ID = uuid.New()
	}
	f.templates[template.ID] = template
	return nil
}

func (f *fakeEnvironmentTemplateRepository) List(ctx context.Context) ([]model.EnvironmentTemplate, error) {
	out := make([]model.EnvironmentTemplate, 0, len(f.templates))
	for _, template := range f.templates {
		out = append(out, *template)
	}
	return out, nil
}

func (f *fakeEnvironmentTemplateRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.EnvironmentTemplate, error) {
	template, ok := f.templates[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return template, nil
}

func (f *fakeEnvironmentTemplateRepository) FindByKey(ctx context.Context, key string) (*model.EnvironmentTemplate, error) {
	for _, template := range f.templates {
		if template.Key == key {
			return template, nil
		}
	}
	return nil, repository.ErrNotFound
}

func (f *fakeEnvironmentTemplateRepository) Update(ctx context.Context, template *model.EnvironmentTemplate) error {
	f.templates[template.ID] = template
	return nil
}

func (f *fakeEnvironmentTemplateRepository) Delete(ctx context.Context, id uuid.UUID) error {
	delete(f.templates, id)
	return nil
}

// fakeUserRepository implements just enough of repository.UserRepository for
// Service.AddMember to look up members by email — the rest of the interface
// (auth identities, refresh tokens) is exercised by internal/service's own
// auth tests, not needed here.
type fakeUserRepository struct {
	usersByEmail map[string]*model.User
	usersByID    map[uuid.UUID]*model.User
}

func newFakeUserRepository() *fakeUserRepository {
	return &fakeUserRepository{usersByEmail: map[string]*model.User{}, usersByID: map[uuid.UUID]*model.User{}}
}

func (f *fakeUserRepository) Create(ctx context.Context, u *model.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	f.usersByEmail[u.Email] = u
	f.usersByID[u.ID] = u
	return nil
}

func (f *fakeUserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	u, ok := f.usersByEmail[email]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.User, error) {
	u, ok := f.usersByID[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserRepository) FindIdentity(ctx context.Context, provider, subject string) (*model.AuthIdentity, error) {
	return nil, repository.ErrNotFound
}

func (f *fakeUserRepository) CreateIdentity(ctx context.Context, identity *model.AuthIdentity) error {
	return nil
}

func (f *fakeUserRepository) SetAdmin(ctx context.Context, id uuid.UUID, isAdmin bool) error {
	return nil
}

func (f *fakeUserRepository) SetLocale(ctx context.Context, id uuid.UUID, locale string) error {
	if user, ok := f.usersByID[id]; ok {
		user.Locale = locale
	}
	return nil
}

func (f *fakeUserRepository) CreateRefreshToken(ctx context.Context, rt *model.RefreshToken) error {
	return nil
}

func (f *fakeUserRepository) FindRefreshToken(ctx context.Context, tokenHash string) (*model.RefreshToken, error) {
	return nil, repository.ErrNotFound
}

func (f *fakeUserRepository) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	return nil
}

func TestService_CreateAddsOwner(t *testing.T) {
	projRepo := newFakeProjectRepository()
	userRepo := newFakeUserRepository()
	svc := NewService(projRepo, userRepo, newFakeEnvironmentRepository(), newFakeEnvironmentTemplateRepository())
	ctx := context.Background()

	creator := uuid.New()
	p, err := svc.Create(ctx, creator, "Alpha", "alpha", nil)
	require.NoError(t, err)

	role, err := svc.RoleOf(ctx, p.ID, creator)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleOwner, role)
}

func TestService_CreateAppliesRequiredEnvironmentTemplates(t *testing.T) {
	projectRepo := newFakeProjectRepository()
	environmentRepo := newFakeEnvironmentRepository()
	templateRepo := newFakeEnvironmentTemplateRepository()
	require.NoError(t, templateRepo.Create(context.Background(), &model.EnvironmentTemplate{Key: "production", Name: "Production", IsRequired: true}))
	require.NoError(t, templateRepo.Create(context.Background(), &model.EnvironmentTemplate{Key: "staging", Name: "Staging"}))
	svc := NewService(projectRepo, newFakeUserRepository(), environmentRepo, templateRepo)

	project, err := svc.Create(context.Background(), uuid.New(), "Alpha", "alpha", []string{"staging"})
	require.NoError(t, err)
	environments, err := environmentRepo.List(context.Background(), project.ID)
	require.NoError(t, err)

	keys := map[string]bool{}
	for _, environment := range environments {
		keys[environment.Key] = true
	}
	assert.Equal(t, map[string]bool{"all": true, "production": true, "staging": true}, keys)
}

func TestService_RoleOf_NotMember(t *testing.T) {
	svc := NewService(newFakeProjectRepository(), newFakeUserRepository(), newFakeEnvironmentRepository(), newFakeEnvironmentTemplateRepository())
	_, err := svc.RoleOf(context.Background(), uuid.New(), uuid.New())
	assert.ErrorIs(t, err, ErrMemberNotFound)
}

func TestService_AddMember(t *testing.T) {
	projRepo := newFakeProjectRepository()
	userRepo := newFakeUserRepository()
	svc := NewService(projRepo, userRepo, newFakeEnvironmentRepository(), newFakeEnvironmentTemplateRepository())
	ctx := context.Background()

	target, err := svc.Create(ctx, uuid.New(), "Alpha", "alpha", nil)
	require.NoError(t, err)

	member := &model.User{ID: uuid.New(), Email: "m@b.com"}
	require.NoError(t, userRepo.Create(ctx, member))

	require.NoError(t, svc.AddMember(ctx, target.ID, "m@b.com", constants.RoleEditor))

	err = svc.AddMember(ctx, target.ID, "m@b.com", constants.RoleEditor)
	assert.ErrorIs(t, err, ErrAlreadyMember)

	err = svc.AddMember(ctx, target.ID, "missing@b.com", constants.RoleEditor)
	assert.ErrorIs(t, err, ErrUserNotFound)
}

func TestService_UpdateAndDelete(t *testing.T) {
	svc := NewService(newFakeProjectRepository(), newFakeUserRepository(), newFakeEnvironmentRepository(), newFakeEnvironmentTemplateRepository())
	ctx := context.Background()

	p, err := svc.Create(ctx, uuid.New(), "Alpha", "alpha", nil)
	require.NoError(t, err)

	updated, err := svc.Update(ctx, p.ID, "Beta")
	require.NoError(t, err)
	assert.Equal(t, "Beta", updated.Name)

	require.NoError(t, svc.Delete(ctx, p.ID))
	_, err = svc.Get(ctx, p.ID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
}

func TestService_RemoveAndUpdateMemberRole(t *testing.T) {
	projRepo := newFakeProjectRepository()
	userRepo := newFakeUserRepository()
	svc := NewService(projRepo, userRepo, newFakeEnvironmentRepository(), newFakeEnvironmentTemplateRepository())
	ctx := context.Background()

	creator := uuid.New()
	p, err := svc.Create(ctx, creator, "Alpha", "alpha", nil)
	require.NoError(t, err)

	require.NoError(t, svc.UpdateMemberRole(ctx, p.ID, creator, constants.RoleAdmin))
	role, err := svc.RoleOf(ctx, p.ID, creator)
	require.NoError(t, err)
	assert.Equal(t, constants.RoleAdmin, role)

	require.NoError(t, svc.RemoveMember(ctx, p.ID, creator))
	_, err = svc.RoleOf(ctx, p.ID, creator)
	assert.ErrorIs(t, err, ErrMemberNotFound)
}
