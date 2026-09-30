package app

import (
	"context"
	"mime/multipart"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/skills"
)

const SkillPackageUploadMaxBytes = skills.SkillPackageUploadMaxBytes

type (
	SkillShowcaseMedia            = skills.SkillShowcaseMedia
	SkillEffectiveUser            = skills.SkillEffectiveUser
	SkillItem                     = skills.SkillItem
	SkillCategory                 = skills.SkillCategory
	SkillListRequest              = skills.SkillListRequest
	SkillList                     = skills.SkillList
	SkillMutationRequest          = skills.SkillMutationRequest
	SkillInstallRequest           = skills.SkillInstallRequest
	SkillGitHubInstallRequest     = skills.SkillGitHubInstallRequest
	SkillPackageFileItem          = skills.SkillPackageFileItem
	SkillPackageFileContent       = skills.SkillPackageFileContent
	SkillPackageBundleFile        = skills.SkillPackageBundleFile
	SkillPackageBundle            = skills.SkillPackageBundle
	SkillFileSearchResult         = skills.SkillFileSearchResult
	AdminSkillCategory            = skills.AdminSkillCategory
	AdminSkillItem                = skills.AdminSkillItem
	AdminSkillCatalog             = skills.AdminSkillCatalog
	AdminSkillAvailabilityRequest = skills.AdminSkillAvailabilityRequest
)

func (s *Service) skillDomain() *skills.Service {
	if s == nil {
		return skills.New(nil, "", nil)
	}
	if s.skills != nil {
		return s.skills
	}
	return skills.New(s.repo, s.dataDir, s.runWorkerLoop)
}

func (s *Service) Skills(userID string, req SkillListRequest) (*SkillList, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().Skills(userID, req)
}

func (s *Service) AddedSkills(userID string) ([]SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().AddedSkills(userID)
}

func (s *Service) SkillDetail(userID string, id string) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SkillDetail(userID, id)
}

func (s *Service) AdminSkills(actor *model.User) (*AdminSkillCatalog, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	globalAvailable, err := s.FeatureEnabled(FeatureSkillLibrary)
	if err != nil {
		return nil, err
	}
	return s.skillDomain().AdminSkills(globalAvailable)
}

func (s *Service) AdminInstallSkillUpload(actor *model.User, sourceType string, header *multipart.FileHeader, req SkillInstallRequest) (*AdminSkillCatalog, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	skill, err := s.skillDomain().InstallPlatformSkillUpload(actor.ID, sourceType, header, req)
	if err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "skill.install", "skill", skill.SkillID, "安装平台公共技能", map[string]any{
		"skillName": skill.SkillName, "sourceType": skill.SourceType, "tag": skill.Tag,
	}); err != nil {
		return nil, err
	}
	return s.AdminSkills(actor)
}

func (s *Service) AdminInstallGitHubSkill(actor *model.User, req SkillGitHubInstallRequest) (*AdminSkillCatalog, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	skill, err := s.skillDomain().InstallPlatformGitHubSkill(actor.ID, req)
	if err != nil {
		return nil, err
	}
	if err := s.appendAdminAudit(actor, "skill.install", "skill", skill.SkillID, "安装 GitHub 平台公共技能", map[string]any{
		"skillName": skill.SkillName, "sourceType": skill.SourceType, "tag": skill.Tag,
	}); err != nil {
		return nil, err
	}
	return s.AdminSkills(actor)
}

func (s *Service) UpdateAdminSkillAvailability(actor *model.User, req AdminSkillAvailabilityRequest) (*AdminSkillCatalog, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	audit, err := newAdminAuditEvent(actor, "skill.availability.batch_update", "skill", "batch", "批量更新平台公共技能可用状态", map[string]any{
		"skillIds": req.SkillIDs, "count": len(req.SkillIDs), "available": req.Available,
	})
	if err != nil {
		return nil, err
	}
	if err := s.skillDomain().SetPlatformSkillAvailability(req, actor.ID, audit); err != nil {
		return nil, err
	}
	return s.AdminSkills(actor)
}

func (s *Service) UpdateAdminSkillCategoryAvailability(actor *model.User, tag string, available bool) (*AdminSkillCatalog, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	audit, err := newAdminAuditEvent(actor, "skill.category_availability.update", "skill_category", tag, "更新平台技能分类可用状态", map[string]any{
		"tag": tag, "available": available,
	})
	if err != nil {
		return nil, err
	}
	if err := s.skillDomain().SetPlatformSkillCategoryAvailability(tag, available, actor.ID, audit); err != nil {
		return nil, err
	}
	return s.AdminSkills(actor)
}

func (s *Service) CreateSkill(userID string, req SkillMutationRequest) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().CreateSkill(userID, req)
}

func (s *Service) UpdateSkill(userID string, id string, req SkillMutationRequest) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().UpdateSkill(userID, id, req)
}

func (s *Service) DeleteSkill(userID string, id string) error {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return err
	}
	return s.skillDomain().DeleteSkill(userID, id)
}

func (s *Service) SetSkillAdded(userID string, id string, added bool) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SetSkillAdded(userID, id, added)
}

func (s *Service) SetSkillLiked(userID string, id string, liked bool) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SetSkillLiked(userID, id, liked)
}

func (s *Service) EnsureBuiltinSkills() error {
	return s.skillDomain().EnsureBuiltinSkills()
}

func (s *Service) EnsureSkillPackages() error {
	return s.skillDomain().EnsureSkillPackages()
}

func (s *Service) InstallSkillUpload(userID string, sourceType string, header *multipart.FileHeader, req SkillInstallRequest) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().InstallSkillUpload(userID, sourceType, header, req)
}

func (s *Service) InstallGitHubSkill(userID string, req SkillGitHubInstallRequest) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().InstallGitHubSkill(userID, req)
}

func (s *Service) SyncGitHubSkill(userID string, skillID string) (*SkillItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SyncGitHubSkill(userID, skillID)
}

func (s *Service) SkillPackageFiles(userID string, skillID string) ([]SkillPackageFileItem, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SkillPackageFiles(userID, skillID)
}

func (s *Service) SkillPackageFile(userID string, skillID string, filePath string) (*SkillPackageFileContent, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SkillPackageFile(userID, skillID, filePath)
}

func (s *Service) SkillPackageRawFile(userID string, skillID string, filePath string) ([]byte, string, string, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, "", "", err
	}
	return s.skillDomain().SkillPackageRawFile(userID, skillID, filePath)
}

func (s *Service) SkillPackageBundle(userID string, skillID string) (*SkillPackageBundle, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SkillPackageBundle(userID, skillID)
}

func (s *Service) SearchSkillPackage(userID string, skillID string, query string) ([]SkillFileSearchResult, error) {
	if err := s.RequireFeature(FeatureSkillLibrary); err != nil {
		return nil, err
	}
	return s.skillDomain().SearchSkillPackage(userID, skillID, query)
}

func (s *Service) startSkillSyncWorker(ctx context.Context) {
	s.skillDomain().StartSyncWorker(ctx)
}
