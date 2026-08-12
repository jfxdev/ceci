package service

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"ceci/backend/internal/constants"
	"ceci/backend/internal/model"
	"ceci/backend/internal/repository"
)

var (
	ErrForbidden       = errors.New("forbidden")
	ErrMemberNotFound  = errors.New("member not found")
	ErrUserNotFound    = errors.New("user not found")
	ErrAlreadyMember   = errors.New("user is already a member")
)

type ProjectService struct {
	projects repository.ProjectRepository
	users    repository.UserRepository
}

func NewProjectService(projects repository.ProjectRepository, users repository.UserRepository) *ProjectService {
	return &ProjectService{projects: projects, users: users}
}

// Create creates a project and adds the creator as owner.
func (s *ProjectService) Create(ctx context.Context, creatorID uuid.UUID, name, slug string) (*model.Project, error) {
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
	return s.projects.ListForUser(ctx, userID)
}

// RoleOf returns the caller's role in the project, or ErrMemberNotFound if not a member.
func (s *ProjectService) RoleOf(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	m, err := s.projects.FindMember(ctx, projectID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return "", ErrMemberNotFound
		}
		return "", err
	}
	return m.Role, nil
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
