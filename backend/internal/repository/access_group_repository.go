package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
)

type AccessGroupRepository interface {
	Create(ctx context.Context, group *model.AccessGroup) error
	List(ctx context.Context) ([]model.AccessGroup, error)
	FindByID(ctx context.Context, id uuid.UUID) (*model.AccessGroup, error)
	FindByName(ctx context.Context, name string) (*model.AccessGroup, error)
	Delete(ctx context.Context, id uuid.UUID) error
	AddMember(ctx context.Context, member *model.AccessGroupMember) error
	FindMember(ctx context.Context, groupID, userID uuid.UUID) (*model.AccessGroupMember, error)
	RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error
	ListMembers(ctx context.Context, groupID uuid.UUID) ([]MemberWithUser, error)
	AddOIDCMapping(ctx context.Context, mapping *model.OIDCAccessGroupMapping) error
	RemoveOIDCMapping(ctx context.Context, groupID uuid.UUID, externalGroup string) error
	ListOIDCMappings(ctx context.Context, groupID uuid.UUID) ([]model.OIDCAccessGroupMapping, error)
	ListMappedGroupIDs(ctx context.Context, externalGroups []string) ([]uuid.UUID, error)
	ListOIDCMemberGroupIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	RemoveOIDCMember(ctx context.Context, groupID, userID uuid.UUID) error
	GrantProject(ctx context.Context, grant *model.ProjectAccessGroup) error
	FindProjectGrant(ctx context.Context, projectID, groupID uuid.UUID) (*model.ProjectAccessGroup, error)
	UpdateProjectGrant(ctx context.Context, projectID, groupID uuid.UUID, role constants.ProjectRole) error
	RevokeProjectGrant(ctx context.Context, projectID, groupID uuid.UUID) error
	ListProjectGrants(ctx context.Context, projectID uuid.UUID) ([]ProjectGroupGrant, error)
	RoleForUserInProject(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error)
	ListProjectsForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error)
}

type ProjectGroupGrant struct {
	GroupID     uuid.UUID
	Name        string
	Description string
	Role        constants.ProjectRole
}

type postgresAccessGroupRepository struct{ db *gorm.DB }

func NewAccessGroupRepository(db *gorm.DB) AccessGroupRepository {
	return &postgresAccessGroupRepository{db: db}
}

func (r *postgresAccessGroupRepository) Create(ctx context.Context, group *model.AccessGroup) error {
	return r.db.WithContext(ctx).Create(group).Error
}

func (r *postgresAccessGroupRepository) List(ctx context.Context) ([]model.AccessGroup, error) {
	var groups []model.AccessGroup
	return groups, r.db.WithContext(ctx).Order("name ASC").Find(&groups).Error
}

func (r *postgresAccessGroupRepository) FindByID(ctx context.Context, id uuid.UUID) (*model.AccessGroup, error) {
	var group model.AccessGroup
	if err := r.db.WithContext(ctx).First(&group, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &group, nil
}

func (r *postgresAccessGroupRepository) FindByName(ctx context.Context, name string) (*model.AccessGroup, error) {
	var group model.AccessGroup
	if err := r.db.WithContext(ctx).Where("name = ?", name).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &group, nil
}

func (r *postgresAccessGroupRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("access_group_id = ?", id).Delete(&model.AccessGroupMember{}).Error; err != nil {
			return err
		}
		if err := tx.Where("access_group_id = ?", id).Delete(&model.ProjectAccessGroup{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.AccessGroup{}, "id = ?", id).Error
	})
}

func (r *postgresAccessGroupRepository) AddMember(ctx context.Context, member *model.AccessGroupMember) error {
	return r.db.WithContext(ctx).Create(member).Error
}

func (r *postgresAccessGroupRepository) FindMember(ctx context.Context, groupID, userID uuid.UUID) (*model.AccessGroupMember, error) {
	var member model.AccessGroupMember
	if err := r.db.WithContext(ctx).Where("access_group_id = ? AND user_id = ?", groupID, userID).First(&member).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &member, nil
}

func (r *postgresAccessGroupRepository) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("access_group_id = ? AND user_id = ?", groupID, userID).Delete(&model.AccessGroupMember{}).Error
}

func (r *postgresAccessGroupRepository) ListMembers(ctx context.Context, groupID uuid.UUID) ([]MemberWithUser, error) {
	var rows []MemberWithUser
	err := r.db.WithContext(ctx).Table("access_group_members").
		Select("access_group_members.user_id, users.email, users.name").
		Joins("JOIN users ON users.id = access_group_members.user_id").
		Where("access_group_members.access_group_id = ?", groupID).Order("users.email ASC").Scan(&rows).Error
	return rows, err
}

func (r *postgresAccessGroupRepository) AddOIDCMapping(ctx context.Context, mapping *model.OIDCAccessGroupMapping) error {
	return r.db.WithContext(ctx).Create(mapping).Error
}

func (r *postgresAccessGroupRepository) RemoveOIDCMapping(ctx context.Context, groupID uuid.UUID, externalGroup string) error {
	return r.db.WithContext(ctx).Where("access_group_id = ? AND external_group = ?", groupID, externalGroup).Delete(&model.OIDCAccessGroupMapping{}).Error
}

func (r *postgresAccessGroupRepository) ListOIDCMappings(ctx context.Context, groupID uuid.UUID) ([]model.OIDCAccessGroupMapping, error) {
	var mappings []model.OIDCAccessGroupMapping
	return mappings, r.db.WithContext(ctx).Where("access_group_id = ?", groupID).Order("external_group ASC").Find(&mappings).Error
}

func (r *postgresAccessGroupRepository) ListMappedGroupIDs(ctx context.Context, externalGroups []string) ([]uuid.UUID, error) {
	if len(externalGroups) == 0 {
		return nil, nil
	}
	var groupIDs []uuid.UUID
	err := r.db.WithContext(ctx).Model(&model.OIDCAccessGroupMapping{}).Where("external_group IN ?", externalGroups).Distinct().Pluck("access_group_id", &groupIDs).Error
	return groupIDs, err
}

func (r *postgresAccessGroupRepository) ListOIDCMemberGroupIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	var groupIDs []uuid.UUID
	err := r.db.WithContext(ctx).Model(&model.AccessGroupMember{}).Where("user_id = ? AND source = ?", userID, "oidc").Pluck("access_group_id", &groupIDs).Error
	return groupIDs, err
}

func (r *postgresAccessGroupRepository) RemoveOIDCMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("access_group_id = ? AND user_id = ? AND source = ?", groupID, userID, "oidc").Delete(&model.AccessGroupMember{}).Error
}

func (r *postgresAccessGroupRepository) GrantProject(ctx context.Context, grant *model.ProjectAccessGroup) error {
	return r.db.WithContext(ctx).Create(grant).Error
}

func (r *postgresAccessGroupRepository) FindProjectGrant(ctx context.Context, projectID, groupID uuid.UUID) (*model.ProjectAccessGroup, error) {
	var grant model.ProjectAccessGroup
	if err := r.db.WithContext(ctx).Where("project_id = ? AND access_group_id = ?", projectID, groupID).First(&grant).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &grant, nil
}

func (r *postgresAccessGroupRepository) UpdateProjectGrant(ctx context.Context, projectID, groupID uuid.UUID, role constants.ProjectRole) error {
	return r.db.WithContext(ctx).Model(&model.ProjectAccessGroup{}).Where("project_id = ? AND access_group_id = ?", projectID, groupID).Update("role", role).Error
}

func (r *postgresAccessGroupRepository) RevokeProjectGrant(ctx context.Context, projectID, groupID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("project_id = ? AND access_group_id = ?", projectID, groupID).Delete(&model.ProjectAccessGroup{}).Error
}

func (r *postgresAccessGroupRepository) ListProjectGrants(ctx context.Context, projectID uuid.UUID) ([]ProjectGroupGrant, error) {
	var rows []ProjectGroupGrant
	err := r.db.WithContext(ctx).Table("project_access_groups").
		Select("project_access_groups.access_group_id AS group_id, access_groups.name, access_groups.description, project_access_groups.role").
		Joins("JOIN access_groups ON access_groups.id = project_access_groups.access_group_id").
		Where("project_access_groups.project_id = ?", projectID).Order("access_groups.name ASC").Scan(&rows).Error
	return rows, err
}

func (r *postgresAccessGroupRepository) RoleForUserInProject(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	var grant model.ProjectAccessGroup
	err := r.db.WithContext(ctx).Table("project_access_groups").
		Joins("JOIN access_group_members ON access_group_members.access_group_id = project_access_groups.access_group_id").
		Where("project_access_groups.project_id = ? AND access_group_members.user_id = ?", projectID, userID).
		Order("CASE project_access_groups.role WHEN 'admin' THEN 3 WHEN 'editor' THEN 2 WHEN 'viewer' THEN 1 ELSE 0 END DESC").First(&grant).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrNotFound
		}
		return "", err
	}
	return grant.Role, nil
}

func (r *postgresAccessGroupRepository) ListProjectsForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error) {
	var projects []model.Project
	err := r.db.WithContext(ctx).Table("projects").
		Joins("JOIN project_access_groups ON project_access_groups.project_id = projects.id").
		Joins("JOIN access_group_members ON access_group_members.access_group_id = project_access_groups.access_group_id").
		Where("access_group_members.user_id = ?", userID).Distinct().Find(&projects).Error
	return projects, err
}
