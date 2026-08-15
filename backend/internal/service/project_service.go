package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var (
	ErrForbidden      = errors.New("forbidden")
	ErrMemberNotFound = errors.New("member not found")
	ErrUserNotFound   = errors.New("user not found")
	ErrAlreadyMember  = errors.New("user is already a member")
)

type ProjectService struct {
	projects  repository.ProjectRepository
	users     repository.UserRepository
	envs      repository.EnvironmentRepository
	templates repository.EnvironmentTemplateRepository
	groups    repository.AccessGroupRepository
}

func NewProjectService(projects repository.ProjectRepository, users repository.UserRepository, envs repository.EnvironmentRepository, templates repository.EnvironmentTemplateRepository, groups ...repository.AccessGroupRepository) *ProjectService {
	var accessGroups repository.AccessGroupRepository
	if len(groups) > 0 {
		accessGroups = groups[0]
	}
	return &ProjectService{projects: projects, users: users, envs: envs, templates: templates, groups: accessGroups}
}

// Create creates a project, adds the creator as owner, and seeds the
// project's shared default environment — every project needs at
// least one environment to evaluate flags/parameters against.
func (s *ProjectService) Create(ctx context.Context, creatorID uuid.UUID, name, slug string, environmentTemplateKeys []string) (*model.Project, error) {
	p := &model.Project{Name: name, Slug: slug}
	if err := s.projects.Create(ctx, p); err != nil {
		return nil, err
	}
	if err := s.projects.AddMember(ctx, &model.ProjectMember{
		ProjectID: p.ID,
		UserID:    creatorID,
		Role:      constants.RoleOwner,
	}); err != nil {
		return nil, err
	}
	if err := s.envs.Create(ctx, &model.Environment{
		ProjectID: p.ID,
		Key:       constants.DefaultEnvironmentKey,
		Name:      constants.DefaultEnvironmentName,
	}); err != nil {
		return nil, err
	}
	templates, err := s.templates.List(ctx)
	if err != nil {
		return nil, err
	}
	selected := make(map[string]bool, len(environmentTemplateKeys))
	for _, key := range environmentTemplateKeys {
		selected[key] = true
	}
	for _, template := range templates {
		if !template.IsRequired && !selected[template.Key] {
			continue
		}
		if err := s.envs.Create(ctx, &model.Environment{
			ProjectID: p.ID,
			Key:       template.Key,
			Name:      template.Name,
			SortOrder: template.SortOrder + 1,
		}); err != nil {
			return nil, err
		}
	}
	return p, nil
}

func (s *ProjectService) Get(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	return s.projects.FindByID(ctx, id)
}

func (s *ProjectService) Update(ctx context.Context, id uuid.UUID, name string) (*model.Project, error) {
	p, err := s.projects.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Name = name
	if err := s.projects.Update(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

func (s *ProjectService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.projects.Delete(ctx, id)
}

func (s *ProjectService) ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error) {
	direct, err := s.projects.ListForUser(ctx, userID)
	if err != nil || s.groups == nil {
		return direct, err
	}
	viaGroups, err := s.groups.ListProjectsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	seen := make(map[uuid.UUID]bool, len(direct)+len(viaGroups))
	out := make([]model.Project, 0, len(direct)+len(viaGroups))
	for _, project := range append(direct, viaGroups...) {
		if !seen[project.ID] {
			seen[project.ID] = true
			out = append(out, project)
		}
	}
	return out, nil
}

// RoleOf returns the caller's role in the project, or ErrMemberNotFound if not a member.
func (s *ProjectService) RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	var best constants.ProjectRole
	m, directErr := s.projects.FindMember(ctx, projectID, userID)
	if directErr == nil {
		best = m.Role
	} else if !errors.Is(directErr, repository.ErrNotFound) {
		return "", directErr
	}
	if s.groups != nil {
		groupRole, groupErr := s.groups.RoleForUserInProject(ctx, projectID, userID)
		if groupErr == nil && (!best.Valid() || groupRole.Meets(best)) {
			best = groupRole
		} else if groupErr != nil && !errors.Is(groupErr, repository.ErrNotFound) {
			return "", groupErr
		}
	}
	if !best.Valid() {
		return "", ErrMemberNotFound
	}
	return best, nil
}

func (s *ProjectService) ListMembers(ctx context.Context, projectID uuid.UUID) ([]repository.MemberWithUser, error) {
	return s.projects.ListMembers(ctx, projectID)
}

func (s *ProjectService) AddMember(ctx context.Context, projectID uuid.UUID, email string, role constants.ProjectRole) error {
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if _, err := s.projects.FindMember(ctx, projectID, user.ID); err == nil {
		return ErrAlreadyMember
	}
	return s.projects.AddMember(ctx, &model.ProjectMember{ProjectID: projectID, UserID: user.ID, Role: role})
}

func (s *ProjectService) UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role constants.ProjectRole) error {
	return s.projects.UpdateMemberRole(ctx, projectID, userID, role)
}

func (s *ProjectService) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	return s.projects.RemoveMember(ctx, projectID, userID)
}
