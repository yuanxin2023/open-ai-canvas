package repository

import "infinite-canvas/backend/internal/model"

// Retain current and historical drama media even after its generation task is archived.
func (r *Repository) dramaResourceReferences(userID string, resourceIDs []string) ([]ResourceDirectReference, error) {
	projects := r.db.Model(&model.DramaProduction{}).Select("project_id").Where("user_id = ?", userID)
	var references []ResourceDirectReference
	for _, source := range []struct {
		model        any
		column, kind string
	}{
		{&model.DramaCast{}, "image_resource_id", "短剧设定参考图"},
		{&model.DramaEpisode{}, "final_resource_id", "短剧成片"},
		{&model.DramaSegment{}, "video_resource_id", "短剧段视频"},
		{&model.DramaSegment{}, "tail_frame_resource_id", "短剧段尾帧"},
		{&model.DramaTake{}, "resource_id", "短剧历史产物"},
	} {
		var rows []struct{ ID, ResourceID string }
		if err := r.db.Model(source.model).Select("id, "+source.column+" AS resource_id").Where("project_id IN (?) AND "+source.column+" IN ?", projects, resourceIDs).Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, row := range rows {
			references = append(references, ResourceDirectReference{Kind: source.kind, ID: row.ID, ResourceID: row.ResourceID})
		}
	}
	return references, nil
}
