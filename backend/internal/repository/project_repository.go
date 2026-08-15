package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
)

type ProjectRepository interface {
	Create(ctx context.Context, p *model.Project) error
	FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error)
	Update(ctx context.Context, p *model.Project) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error)

	AddMember(ctx context.Context, m *model.ProjectMember) error
	UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role constants.ProjectRole) error
	RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error
	FindMember(ctx context.Context, projectID, userID uuid.UUID) (*model.ProjectMember, error)
	ListMembers(ctx context.Context, projectID uuid.UUID) ([]MemberWithUser, error)
}

// MemberWithUser joins a project membership row with the user's profile
// fields needed by the API response.
type MemberWithUser struct {
	UserID uuid.UUID
	Email  string
	Name   string
	Role   constants.ProjectRole
}

type postgresProjectRepository struct {
	db *gorm.DB
}

func NewProjectRepository(db *gorm.DB) ProjectRepository {
	return &postgresProjectRepository{db: db}
}

func (r *postgresProjectRepository) Create(ctx context.Context, p *model.Project) error {
	return r.db.WithContext(ctx).Create(p).Error
}

func (r *postgresProjectRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	var p model.Project
	if err := r.db.WithContext(ctx).First(&p, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *postgresProjectRepository) Update(ctx context.Context, p *model.Project) error {
	return r.db.WithContext(ctx).Save(p).Error
}

func (r *postgresProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Delete(&model.Project{}, "id = ?", id).Error
}

func (r *postgresProjectRepository) ListForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error) {
	var projects []model.Project
	err := r.db.WithContext(ctx).
		Joins("JOIN project_members ON project_members.project_id = projects.id").
		Where("project_members.user_id = ?", userID).
		Find(&projects).Error
	return projects, err
}

func (r *postgresProjectRepository) AddMember(ctx context.Context, m *model.ProjectMember) error {
	return r.db.WithContext(ctx).Create(m).Error
}

func (r *postgresProjectRepository) UpdateMemberRole(ctx context.Context, projectID, userID uuid.UUID, role constants.ProjectRole) error {
	return r.db.WithContext(ctx).Model(&model.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Update("role", role).Error
}

func (r *postgresProjectRepository) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		Delete(&model.ProjectMember{}).Error
}

func (r *postgresProjectRepository) FindMember(ctx context.Context, projectID, userID uuid.UUID) (*model.ProjectMember, error) {
	var m model.ProjectMember
	err := r.db.WithContext(ctx).
		Where("project_id = ? AND user_id = ?", projectID, userID).
		First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

func (r *postgresProjectRepository) ListMembers(ctx context.Context, projectID uuid.UUID) ([]MemberWithUser, error) {
	var rows []MemberWithUser
	err := r.db.WithContext(ctx).
		Table("project_members").
		Select("project_members.user_id, users.email, users.name, project_members.role").
		Joins("JOIN users ON users.id = project_members.user_id").
		Where("project_members.project_id = ?", projectID).
		Scan(&rows).Error
	return rows, err
}
