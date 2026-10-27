package app

import (
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/skills"
)

type SkillCuration = skills.SkillCuration
type SkillCurationUpdate = skills.SkillCurationUpdate

func (s *Service) SkillCuration(actor *model.User, admin bool) (*SkillCuration, error) {
	if admin {
		if err := s.RequireAdminPermission(actor, model.AdminPermissionSkills); err != nil {
			return nil, err
		}
	} else if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SkillCuration(actor, admin)
}
func (s *Service) UpdateSkillCuration(actor *model.User, req SkillCurationUpdate, requestID string) (*SkillCuration, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionSkills); err != nil {
		return nil, err
	}
	return s.skillDomain().UpdateSkillCuration(actor, req, requestID)
}
