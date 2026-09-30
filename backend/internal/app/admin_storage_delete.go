package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

const maxAdminResourceDeleteCount = 100
const reasonInspirationCoverConfirmationRequired ErrorReason = "inspiration_cover_confirmation_required"

type AdminResourceDeleteRequest struct {
	ResourceIDs              []string `json:"resourceIds"`
	ConfirmInspirationCovers bool     `json:"confirmInspirationCovers"`
}

type AdminResourceReferenceView struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
}

type AdminResourceDeleteBlocked struct {
	ID         string                       `json:"id"`
	Reason     string                       `json:"reason"`
	References []AdminResourceReferenceView `json:"references"`
}

type AdminResourceDeleteWarning struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type AdminResourceDeleteConfirmation struct {
	ID         string                       `json:"id"`
	References []AdminResourceReferenceView `json:"references"`
}

type AdminResourceDeletePreview struct {
	InspirationCovers []AdminResourceDeleteConfirmation `json:"inspirationCovers"`
}

type AdminResourceDeleteResult struct {
	Deleted  []string                     `json:"deleted"`
	Blocked  []AdminResourceDeleteBlocked `json:"blocked"`
	Warnings []AdminResourceDeleteWarning `json:"warnings"`
}

func (s *Service) PreviewAdminResourceDelete(actor *model.User, req AdminResourceDeleteRequest) (*AdminResourceDeletePreview, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	resourceIDs, err := normalizeAdminResourceDeleteIDs(req.ResourceIDs)
	if err != nil {
		return nil, err
	}
	references, err := s.repo.InspirationResourceReferences(resourceIDs)
	if err != nil {
		return nil, err
	}
	return &AdminResourceDeletePreview{InspirationCovers: adminResourceDeleteConfirmations(resourceIDs, references)}, nil
}

func (s *Service) DeleteAdminResources(actor *model.User, req AdminResourceDeleteRequest) (*AdminResourceDeleteResult, error) {
	if err := s.RequireAdmin(actor); err != nil {
		return nil, err
	}
	resourceIDs, err := normalizeAdminResourceDeleteIDs(req.ResourceIDs)
	if err != nil {
		return nil, err
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	inspirationReferences, err := s.repo.InspirationResourceReferences(resourceIDs)
	if err != nil {
		return nil, err
	}
	if len(inspirationReferences) > 0 && !req.ConfirmInspirationCovers {
		confirmationErr := NewAppError(http.StatusConflict, "所选资源包含首页灵感提示词展示图，请确认后再删除")
		confirmationErr.Reason = reasonInspirationCoverConfirmationRequired
		return nil, confirmationErr
	}
	inspirationCoverIDs := make(map[string]struct{}, len(inspirationReferences))
	for _, reference := range inspirationReferences {
		inspirationCoverIDs[reference.ResourceID] = struct{}{}
	}

	resources, err := s.repo.AdminResourcesByIDs(resourceIDs)
	if err != nil {
		return nil, err
	}
	resourcesByID := make(map[string]model.Resource, len(resources))
	resourcesByUser := make(map[string][]model.Resource)
	for _, resource := range resources {
		resourcesByID[resource.ID] = resource
		resourcesByUser[resource.UserID] = append(resourcesByUser[resource.UserID], resource)
	}
	blockedByID := make(map[string]AdminResourceDeleteBlocked)
	for _, resourceID := range resourceIDs {
		if _, exists := resourcesByID[resourceID]; !exists {
			blockedByID[resourceID] = AdminResourceDeleteBlocked{ID: resourceID, Reason: "资源不存在", References: []AdminResourceReferenceView{}}
		}
	}
	for userID, userResources := range resourcesByUser {
		userResourceIDs := make([]string, 0, len(userResources))
		for _, resource := range userResources {
			userResourceIDs = append(userResourceIDs, resource.ID)
		}
		snapshot, snapshotErr := s.repo.ResourceReferenceSnapshot(userID, "", userResourceIDs)
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		for resourceID, references := range adminResourceReferences(snapshot, userResources) {
			blockedByID[resourceID] = AdminResourceDeleteBlocked{ID: resourceID, Reason: "资源正在被保护功能使用", References: references}
		}
	}
	avatarReferences, err := s.repo.UserAvatarResourceReferences(resourceIDs)
	if err != nil {
		return nil, err
	}
	for _, reference := range avatarReferences {
		blocked := blockedByID[reference.ResourceID]
		blocked.ID = reference.ResourceID
		blocked.Reason = "资源正在被用户头像使用"
		blocked.References = appendUniqueAdminResourceReference(blocked.References, AdminResourceReferenceView{Kind: reference.Kind, ID: reference.ID, Title: reference.Title})
		blockedByID[reference.ResourceID] = blocked
	}
	for resourceID, references := range s.appearanceResourceReferences(resourceIDs) {
		for _, reference := range references {
			blocked := blockedByID[resourceID]
			blocked.ID = resourceID
			blocked.Reason = "资源正在被平台外观使用"
			blocked.References = appendUniqueAdminResourceReference(blocked.References, reference)
			blockedByID[resourceID] = blocked
		}
	}
	for resourceID, references := range s.customerServiceResourceReferences(resourceIDs) {
		for _, reference := range references {
			blocked := blockedByID[resourceID]
			blocked.ID = resourceID
			blocked.Reason = "资源正在被客服配置使用"
			blocked.References = appendUniqueAdminResourceReference(blocked.References, reference)
			blockedByID[resourceID] = blocked
		}
	}

	deletable := make([]model.Resource, 0, len(resources))
	deletableIDs := make([]string, 0, len(resources))
	for _, resourceID := range resourceIDs {
		resource, exists := resourcesByID[resourceID]
		if !exists {
			continue
		}
		if _, blocked := blockedByID[resourceID]; blocked {
			continue
		}
		deletable = append(deletable, resource)
		deletableIDs = append(deletableIDs, resource.ID)
	}
	deletableIDsSet := make(map[string]struct{}, len(deletable))
	deletableUserIDs := make([]string, 0, len(resourcesByUser))
	seenDeletableUsers := make(map[string]struct{}, len(resourcesByUser))
	for _, resource := range deletable {
		deletableIDsSet[resource.ID] = struct{}{}
		if _, exists := seenDeletableUsers[resource.UserID]; !exists {
			seenDeletableUsers[resource.UserID] = struct{}{}
			deletableUserIDs = append(deletableUserIDs, resource.UserID)
		}
	}
	tools, err := s.repo.AdminToolsByOwners(deletableUserIDs)
	if err != nil {
		return nil, err
	}
	toolCleanups, err := adminToolResourceCleanups(tools, deletableIDsSet)
	if err != nil {
		return nil, err
	}

	physicalObjects := map[string]*model.Resource{}
	checkedPhysical := map[string]bool{}
	warningsByID := make(map[string]AdminResourceDeleteWarning)
	physicalSkipReasons := make(map[string]string)
	for index := range deletable {
		resource := &deletable[index]
		if strings.TrimSpace(resource.ObjectKey) == "" {
			physicalSkipReasons[resource.ID] = "empty_object_key"
			continue
		}
		if !supportedResourceDeleteProvider(resource.Provider) {
			physicalSkipReasons[resource.ID] = "unsupported_provider"
			warningsByID[resource.ID] = AdminResourceDeleteWarning{ID: resource.ID, Reason: "资源记录已删除，但存储类型不受支持，实际文件可能需要手动清理"}
			continue
		}
		identity := resourceStorageIdentity(resource)
		if checkedPhysical[identity] {
			physicalSkipReasons[resource.ID] = "batched_physical_object"
			continue
		}
		checkedPhysical[identity] = true
		sharedCount, countErr := s.repo.ResourceStorageReferenceCount(resource, deletableIDs)
		if countErr != nil {
			return nil, countErr
		}
		if sharedCount == 0 {
			physicalObjects[identity] = resource
		} else {
			physicalSkipReasons[resource.ID] = "shared_physical_object"
		}
	}
	deletionJobs := resourceDeletionJobsForResources(physicalObjects)
	queuedResourceIDs := make(map[string]bool, len(deletionJobs))
	for _, job := range deletionJobs {
		queuedResourceIDs[job.ResourceID] = true
	}
	audits := make([]model.AdminAuditEvent, 0, len(deletable))
	for _, resource := range deletable {
		metadata := map[string]any{
			"userId": resource.UserID, "kind": resource.Kind, "provider": normalizedResourceProvider(resource.Provider),
			"objectKey": resource.ObjectKey, "deleteMode": "admin_force", "dependencyCleanup": "structured_references",
			"physicalDeleteQueued": queuedResourceIDs[resource.ID],
		}
		if reason := physicalSkipReasons[resource.ID]; reason != "" {
			metadata["physicalDeleteSkippedReason"] = reason
		}
		if _, isInspirationCover := inspirationCoverIDs[resource.ID]; isInspirationCover {
			metadata["inspirationCoverConfirmed"] = true
		}
		event, eventErr := newAdminAuditEvent(actor, "resource.delete", "resource", resource.ID, "管理员强制删除存储资源", metadata)
		if eventErr != nil {
			return nil, eventErr
		}
		audits = append(audits, *event)
	}
	if err := s.repo.DeleteAdminResources(deletable, toolCleanups, deletionJobs, audits, req.ConfirmInspirationCovers); err != nil {
		if errors.Is(err, repository.ErrAdminResourceDeleteChanged) || errors.Is(err, repository.ErrAdminResourceProtected) {
			return nil, BadAuthRequest("资源状态或保护引用已变化，请刷新后重试")
		}
		return nil, err
	}
	if len(deletionJobs) > 0 {
		s.runWorkerTask(func() { s.drainResourceDeletionJobs(len(deletionJobs)) })
	}

	result := &AdminResourceDeleteResult{Deleted: []string{}, Blocked: []AdminResourceDeleteBlocked{}, Warnings: []AdminResourceDeleteWarning{}}
	deletedSet := make(map[string]struct{}, len(deletable))
	for _, resource := range deletable {
		deletedSet[resource.ID] = struct{}{}
	}
	for _, resourceID := range resourceIDs {
		if _, deleted := deletedSet[resourceID]; deleted {
			result.Deleted = append(result.Deleted, resourceID)
			if warning, exists := warningsByID[resourceID]; exists {
				result.Warnings = append(result.Warnings, warning)
			}
		} else if blocked, exists := blockedByID[resourceID]; exists {
			result.Blocked = append(result.Blocked, blocked)
		}
	}
	return result, nil
}

func adminResourceDeleteConfirmations(resourceIDs []string, references []repository.ResourceDirectReference) []AdminResourceDeleteConfirmation {
	byResourceID := make(map[string][]AdminResourceReferenceView)
	for _, reference := range references {
		byResourceID[reference.ResourceID] = appendUniqueAdminResourceReference(byResourceID[reference.ResourceID], AdminResourceReferenceView{
			Kind: reference.Kind, ID: reference.ID, Title: reference.Title,
		})
	}
	result := make([]AdminResourceDeleteConfirmation, 0, len(byResourceID))
	for _, resourceID := range resourceIDs {
		references := byResourceID[resourceID]
		if len(references) == 0 {
			continue
		}
		sort.Slice(references, func(i, j int) bool { return references[i].ID < references[j].ID })
		result = append(result, AdminResourceDeleteConfirmation{ID: resourceID, References: references})
	}
	return result
}

func normalizeAdminResourceDeleteIDs(values []string) ([]string, error) {
	if len(values) == 0 {
		return nil, BadAuthRequest("请选择要删除的资源")
	}
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 80 {
			return nil, BadAuthRequest("资源 ID 无效")
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	if len(result) == 0 || len(result) > maxAdminResourceDeleteCount {
		return nil, BadAuthRequest("单次最多删除 100 个资源")
	}
	return result, nil
}

func adminResourceReferences(snapshot repository.ResourceReferenceSnapshot, resources []model.Resource) map[string][]AdminResourceReferenceView {
	result := make(map[string][]AdminResourceReferenceView)
	seen := make(map[string]map[string]struct{})
	for _, reference := range snapshot.Direct {
		switch reference.Kind {
		case "个人头像":
			appendAdminResourceReference(result, seen, reference.ResourceID, AdminResourceReferenceView{Kind: reference.Kind, ID: reference.ID, Title: reference.Title})
		case "Agent 待执行引用":
			appendAdminResourceReference(result, seen, reference.ResourceID, AdminResourceReferenceView{Kind: "活动 Agent", ID: reference.ID, Title: reference.Title})
		}
	}
	for _, resource := range resources {
		candidate := map[string]struct{}{resource.ID: {}}
		for _, document := range snapshot.Documents {
			kind, active := adminActiveResourceDocumentKind(document)
			if active && (documentReferencesResources(document.PrimaryJSON, candidate) || documentReferencesResources(document.SecondaryJSON, candidate)) {
				appendAdminResourceReference(result, seen, resource.ID, AdminResourceReferenceView{Kind: kind, ID: document.ID, Title: document.Title})
			}
		}
	}
	for resourceID := range result {
		sort.Slice(result[resourceID], func(i, j int) bool {
			left, right := result[resourceID][i], result[resourceID][j]
			if left.Kind != right.Kind {
				return left.Kind < right.Kind
			}
			return left.ID < right.ID
		})
	}
	return result
}

func adminActiveResourceDocumentKind(document repository.ResourceReferenceDocument) (string, bool) {
	switch document.TaskStatus {
	case model.TaskStatusQueued, model.TaskStatusRunning, model.TaskStatusTextReplay:
		return "活动任务", true
	}
	if document.Kind == "创作会话" || document.Kind == "创作执行项" {
		switch document.ExecutionStatus {
		case "running", "waiting_answer", "waiting_proposal", "waiting_canvas", "waiting_payment", "waiting_task":
			return "活动创作", true
		}
	}
	return "", false
}

func adminToolResourceCleanups(tools []model.Tool, resourceIDs map[string]struct{}) ([]repository.AdminToolResourceCleanup, error) {
	cleanups := make([]repository.AdminToolResourceCleanup, 0)
	for _, tool := range tools {
		nextCover, nextMediaURL := tool.Cover, tool.MediaURL
		if documentReferencesResources(tool.Cover, resourceIDs) {
			nextCover = ""
		}
		if documentReferencesResources(tool.MediaURL, resourceIDs) {
			nextMediaURL = ""
		}
		nextExtraInfoJSON := tool.ExtraInfoJSON
		if strings.TrimSpace(tool.ExtraInfoJSON) != "" {
			var references []string
			if err := json.Unmarshal([]byte(tool.ExtraInfoJSON), &references); err != nil {
				return nil, err
			}
			filtered := references[:0]
			for _, reference := range references {
				if !documentReferencesResources(reference, resourceIDs) {
					filtered = append(filtered, reference)
				}
			}
			if len(filtered) != len(references) {
				encoded, err := json.Marshal(filtered)
				if err != nil {
					return nil, err
				}
				nextExtraInfoJSON = string(encoded)
			}
		}
		if nextCover == tool.Cover && nextMediaURL == tool.MediaURL && nextExtraInfoJSON == tool.ExtraInfoJSON {
			continue
		}
		cleanups = append(cleanups, repository.AdminToolResourceCleanup{
			ID: tool.ID, ExpectedCover: tool.Cover, ExpectedMediaURL: tool.MediaURL, ExpectedExtraInfoJSON: tool.ExtraInfoJSON,
			Cover: nextCover, MediaURL: nextMediaURL, ExtraInfoJSON: nextExtraInfoJSON,
		})
	}
	sort.Slice(cleanups, func(i, j int) bool { return cleanups[i].ID < cleanups[j].ID })
	return cleanups, nil
}

func appendAdminResourceReference(result map[string][]AdminResourceReferenceView, seen map[string]map[string]struct{}, resourceID string, reference AdminResourceReferenceView) {
	if resourceID == "" {
		return
	}
	if seen[resourceID] == nil {
		seen[resourceID] = map[string]struct{}{}
	}
	key := reference.Kind + "\x00" + reference.ID
	if _, exists := seen[resourceID][key]; exists {
		return
	}
	seen[resourceID][key] = struct{}{}
	result[resourceID] = append(result[resourceID], reference)
}

func appendUniqueAdminResourceReference(references []AdminResourceReferenceView, reference AdminResourceReferenceView) []AdminResourceReferenceView {
	for _, existing := range references {
		if existing.Kind == reference.Kind && existing.ID == reference.ID {
			return references
		}
	}
	return append(references, reference)
}

func supportedResourceDeleteProvider(provider string) bool {
	provider = normalizedResourceProvider(provider)
	return oneOf(provider, "local", aliyunOSSProvider, tencentCOSProvider, qiniuKodoProvider, s3Provider)
}

func resourceDeletionJobsForResources(physicalObjects map[string]*model.Resource) []model.ResourceDeletionJob {
	resourcesByUser := make(map[string]map[string]*model.Resource)
	for identity, resource := range physicalObjects {
		if resourcesByUser[resource.UserID] == nil {
			resourcesByUser[resource.UserID] = map[string]*model.Resource{}
		}
		resourcesByUser[resource.UserID][identity] = resource
	}
	userIDs := make([]string, 0, len(resourcesByUser))
	for userID := range resourcesByUser {
		userIDs = append(userIDs, userID)
	}
	sort.Strings(userIDs)
	jobs := []model.ResourceDeletionJob{}
	for _, userID := range userIDs {
		jobs = append(jobs, resourceDeletionJobs(userID, resourcesByUser[userID])...)
	}
	return jobs
}
