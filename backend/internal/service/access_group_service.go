package service

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"leaflag/backend/internal/constants"
	"leaflag/backend/internal/model"
	"leaflag/backend/internal/repository"
)

var (
	ErrAccessGroupNotFound = errors.New("access group not found")
	ErrAccessGroupTaken    = errors.New("access group name already exists")
	ErrAccessGroupMember   = errors.New("user is already a group member")
	ErrGroupGrantExists    = errors.New("access group is already linked to this project")
	ErrGroupOwnerRole      = errors.New("access groups cannot be project owners")
	ErrOIDCMappingExists   = errors.New("oidc group is already mapped")
)

type AccessGroupService struct {
	groups repository.AccessGroupRepository
	users  repository.UserRepository
}

func NewAccessGroupService(groups repository.AccessGroupRepository, users repository.UserRepository) *AccessGroupService {
	return &AccessGroupService{groups: groups, users: users}
}

func (s *AccessGroupService) List(ctx context.Context) ([]model.AccessGroup, error) {
	return s.groups.List(ctx)
}

func (s *AccessGroupService) Create(ctx context.Context, name, description string) (*model.AccessGroup, error) {
	group := &model.AccessGroup{Name: strings.TrimSpace(name), Description: strings.TrimSpace(description)}
	if _, err := s.groups.FindByName(ctx, group.Name); err == nil {
		return nil, ErrAccessGroupTaken
	} else if !errors.Is(err, repository.ErrNotFound) {
		return nil, err
	}
	if err := s.groups.Create(ctx, group); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrAccessGroupTaken
		}
		return nil, err
	}
	return group, nil
}

func (s *AccessGroupService) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := s.groups.FindByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrAccessGroupNotFound
		}
		return err
	}
	return s.groups.Delete(ctx, id)
}

func (s *AccessGroupService) ListMembers(ctx context.Context, groupID uuid.UUID) ([]repository.MemberWithUser, error) {
	if _, err := s.groups.FindByID(ctx, groupID); err != nil {
		return nil, err
	}
	return s.groups.ListMembers(ctx, groupID)
}

func (s *AccessGroupService) AddMember(ctx context.Context, groupID uuid.UUID, email string) error {
	if _, err := s.groups.FindByID(ctx, groupID); err != nil {
		return err
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrUserNotFound
		}
		return err
	}
	if _, err := s.groups.FindMember(ctx, groupID, user.ID); err == nil {
		return ErrAccessGroupMember
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if err := s.groups.AddMember(ctx, &model.AccessGroupMember{AccessGroupID: groupID, UserID: user.ID, Source: "local"}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrAccessGroupMember
		}
		return err
	}
	return nil
}

func (s *AccessGroupService) ListOIDCMappings(ctx context.Context, groupID uuid.UUID) ([]model.OIDCAccessGroupMapping, error) {
	if _, err := s.groups.FindByID(ctx, groupID); err != nil {
		return nil, err
	}
	return s.groups.ListOIDCMappings(ctx, groupID)
}

func (s *AccessGroupService) AddOIDCMapping(ctx context.Context, groupID uuid.UUID, externalGroup string) error {
	if _, err := s.groups.FindByID(ctx, groupID); err != nil {
		return err
	}
	mapping := &model.OIDCAccessGroupMapping{AccessGroupID: groupID, ExternalGroup: strings.TrimSpace(externalGroup)}
	if err := s.groups.AddOIDCMapping(ctx, mapping); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrOIDCMappingExists
		}
		return err
	}
	return nil
}

func (s *AccessGroupService) RemoveOIDCMapping(ctx context.Context, groupID uuid.UUID, externalGroup string) error {
	return s.groups.RemoveOIDCMapping(ctx, groupID, externalGroup)
}

// SyncOIDCGroups applies the current groups claim to mapped groups. Only
// memberships previously granted by OIDC are removed; local membership stays.
func (s *AccessGroupService) SyncOIDCGroups(ctx context.Context, userID uuid.UUID, externalGroups []string) error {
	desired, err := s.groups.ListMappedGroupIDs(ctx, externalGroups)
	if err != nil {
		return err
	}
	existing, err := s.groups.ListOIDCMemberGroupIDs(ctx, userID)
	if err != nil {
		return err
	}
	desiredSet := make(map[uuid.UUID]bool, len(desired))
	existingSet := make(map[uuid.UUID]bool, len(existing))
	for _, id := range desired {
		desiredSet[id] = true
	}
	for _, id := range existing {
		existingSet[id] = true
	}
	for id := range desiredSet {
		if existingSet[id] {
			continue
		}
		if _, err := s.groups.FindMember(ctx, id, userID); err == nil {
			continue
		} else if !errors.Is(err, repository.ErrNotFound) {
			return err
		}
		if err := s.groups.AddMember(ctx, &model.AccessGroupMember{AccessGroupID: id, UserID: userID, Source: "oidc"}); err != nil {
			return err
		}
	}
	for id := range existingSet {
		if !desiredSet[id] {
			if err := s.groups.RemoveOIDCMember(ctx, id, userID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *AccessGroupService) RemoveMember(ctx context.Context, groupID, userID uuid.UUID) error {
	return s.groups.RemoveMember(ctx, groupID, userID)
}

func (s *AccessGroupService) ListProjectGrants(ctx context.Context, projectID uuid.UUID) ([]repository.ProjectGroupGrant, error) {
	return s.groups.ListProjectGrants(ctx, projectID)
}

func (s *AccessGroupService) GrantProject(ctx context.Context, projectID, groupID uuid.UUID, role constants.ProjectRole) error {
	if role == constants.RoleOwner {
		return ErrGroupOwnerRole
	}
	if _, err := s.groups.FindByID(ctx, groupID); err != nil {
		return err
	}
	if _, err := s.groups.FindProjectGrant(ctx, projectID, groupID); err == nil {
		return ErrGroupGrantExists
	} else if !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	if err := s.groups.GrantProject(ctx, &model.ProjectAccessGroup{ProjectID: projectID, AccessGroupID: groupID, Role: role}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return ErrGroupGrantExists
		}
		return err
	}
	return nil
}

func (s *AccessGroupService) UpdateProjectGrant(ctx context.Context, projectID, groupID uuid.UUID, role constants.ProjectRole) error {
	if role == constants.RoleOwner {
		return ErrGroupOwnerRole
	}
	return s.groups.UpdateProjectGrant(ctx, projectID, groupID, role)
}

func (s *AccessGroupService) RevokeProjectGrant(ctx context.Context, projectID, groupID uuid.UUID) error {
	return s.groups.RevokeProjectGrant(ctx, projectID, groupID)
}

func (s *AccessGroupService) RoleForUserInProject(ctx context.Context, projectID, userID uuid.UUID) (constants.ProjectRole, error) {
	return s.groups.RoleForUserInProject(ctx, projectID, userID)
}

func (s *AccessGroupService) ListProjectsForUser(ctx context.Context, userID uuid.UUID) ([]model.Project, error) {
	return s.groups.ListProjectsForUser(ctx, userID)
}
