package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

type ControlPreviewQuery struct {
	Period     string `form:"period"`
	Page       int    `form:"page"`
	PageSize   int    `form:"pageSize"`
	Keyword    string `form:"keyword"`
	Model      string `form:"model"`
	Capability string `form:"capability"`
}

type ControlPreviewMeta struct {
	Period   string    `json:"period"`
	From     time.Time `json:"from"`
	To       time.Time `json:"to"`
	AsOf     time.Time `json:"asOf"`
	Timezone string    `json:"timezone"`
}

type ControlPreviewTrend struct {
	Time                 time.Time `json:"time"`
	Label                string    `json:"label"`
	InflowMicrocredits   int64     `json:"inflowMicrocredits"`
	ConsumedMicrocredits int64     `json:"consumedMicrocredits"`
}

type ControlPreviewOverview struct {
	ControlPreviewMeta
	KPI           repository.ControlPreviewTotals   `json:"kpi"`
	Ranking       []repository.ControlPreviewUser   `json:"ranking"`
	Models        []repository.ControlPreviewModel  `json:"models"`
	InflowSources []repository.ControlPreviewInflow `json:"inflowSources"`
	Trend         []ControlPreviewTrend             `json:"trend"`
}

type ControlPreviewUsers struct {
	ControlPreviewMeta
	Users    []repository.ControlPreviewUser `json:"users"`
	Total    int64                           `json:"total"`
	Page     int                             `json:"page"`
	PageSize int                             `json:"pageSize"`
}

type ControlPreviewUserDetail struct {
	ControlPreviewMeta
	User     repository.ControlPreviewUser     `json:"user"`
	Models   []repository.ControlPreviewModel  `json:"models"`
	Entries  []repository.ControlPreviewLedger `json:"entries"`
	Total    int64                             `json:"total"`
	Page     int                               `json:"page"`
	PageSize int                               `json:"pageSize"`
}

var controlPreviewLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

func controlPreviewRange(query ControlPreviewQuery, now time.Time) (ControlPreviewMeta, repository.ControlPreviewFilter, error) {
	period := strings.TrimSpace(query.Period)
	if period == "" {
		period = "day"
	}
	local := now.In(controlPreviewLocation)
	from := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, controlPreviewLocation)
	switch period {
	case "day":
	case "week":
		from = from.AddDate(0, 0, -(int(local.Weekday())+6)%7)
	case "month":
		from = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, controlPreviewLocation)
	default:
		return ControlPreviewMeta{}, repository.ControlPreviewFilter{}, BadAuthRequest("统计周期必须为 day、week 或 month")
	}
	if query.Page < 0 || query.PageSize < 0 || query.PageSize > 100 || query.Page > 1_000_000 {
		return ControlPreviewMeta{}, repository.ControlPreviewFilter{}, BadAuthRequest("分页参数无效，每页最多 100 条")
	}
	capability := strings.TrimSpace(query.Capability)
	if capability != "" && capability != "text" && capability != "image" && capability != "video" && capability != "audio" {
		return ControlPreviewMeta{}, repository.ControlPreviewFilter{}, BadAuthRequest("模型能力类型无效")
	}
	page, size := query.Page, query.PageSize
	if page == 0 {
		page = 1
	}
	if size == 0 {
		size = 20
	}
	meta := ControlPreviewMeta{Period: period, From: from.UTC(), To: now.UTC(), AsOf: now.UTC(), Timezone: "Asia/Shanghai"}
	filter := repository.ControlPreviewFilter{From: meta.From, To: meta.To, Page: page, PageSize: size,
		Model: strings.TrimSpace(query.Model), Capability: capability, Keyword: strings.TrimSpace(query.Keyword)}
	return meta, filter, nil
}

func (s *Service) AdminControlPreview(ctx context.Context, actor *model.User, query ControlPreviewQuery) (*ControlPreviewOverview, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionControlPreview); err != nil {
		return nil, err
	}
	meta, filter, err := controlPreviewRange(query, time.Now())
	if err != nil {
		return nil, err
	}
	// Overview never inherits a drill-down filter, keeping card and ranking denominators identical.
	filter.Model, filter.Capability, filter.Keyword = "", "", ""
	s.initReadCaches()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	key := meta.Period + ":" + meta.From.Format(time.RFC3339)
	return s.controlPreviewReadCache.Get(ctx, key, func(ctx context.Context) (*ControlPreviewOverview, int, error) {
		snapshot, err := s.repo.WithContext(ctx).ControlPreviewSnapshot(filter, meta.Period == "day")
		if err != nil {
			return nil, 0, err
		}
		result := &ControlPreviewOverview{ControlPreviewMeta: meta, KPI: snapshot.Totals, Ranking: snapshot.Ranking,
			Models: snapshot.Models, InflowSources: snapshot.InflowSources, Trend: controlPreviewTrend(meta, snapshot.Buckets)}
		body, err := json.Marshal(result)
		return result, len(body), err
	})
}

func controlPreviewTrend(meta ControlPreviewMeta, buckets []repository.ControlPreviewBucket) []ControlPreviewTrend {
	byBucket := make(map[string]repository.ControlPreviewBucket, len(buckets))
	for _, bucket := range buckets {
		byBucket[bucket.Bucket] = bucket
	}
	result := []ControlPreviewTrend{}
	for at := meta.From.In(controlPreviewLocation); at.Before(meta.To); {
		key, label := at.Format("2006-01-02"), at.Format("01-02")
		if meta.Period == "day" {
			key, label = at.Format("2006-01-02 15:00"), at.Format("15:00")
		}
		bucket := byBucket[key]
		result = append(result, ControlPreviewTrend{Time: at.UTC(), Label: label, InflowMicrocredits: bucket.InflowMicrocredits, ConsumedMicrocredits: bucket.ConsumedMicrocredits})
		if meta.Period == "day" {
			at = at.Add(time.Hour)
		} else {
			at = at.AddDate(0, 0, 1)
		}
	}
	return result
}

func (s *Service) AdminControlPreviewUsers(ctx context.Context, actor *model.User, query ControlPreviewQuery, modelOnly bool) (*ControlPreviewUsers, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionControlPreview); err != nil {
		return nil, err
	}
	meta, filter, err := controlPreviewRange(query, time.Now())
	if err != nil {
		return nil, err
	}
	if modelOnly && (filter.Model == "" || filter.Capability == "") {
		return nil, BadAuthRequest("请选择模型和能力类型")
	}
	if !modelOnly {
		filter.Model, filter.Capability = "", ""
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	rows, total, err := s.repo.WithContext(ctx).ControlPreviewUsers(filter, modelOnly)
	if err != nil {
		return nil, err
	}
	return &ControlPreviewUsers{ControlPreviewMeta: meta, Users: rows, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}

func (s *Service) AdminControlPreviewUser(ctx context.Context, actor *model.User, userID string, query ControlPreviewQuery) (*ControlPreviewUserDetail, error) {
	if err := s.RequireAdminPermission(actor, model.AdminPermissionControlPreview); err != nil {
		return nil, err
	}
	meta, filter, err := controlPreviewRange(query, time.Now())
	if err != nil {
		return nil, err
	}
	filter.UserID = strings.TrimSpace(userID)
	filter.Model, filter.Capability = "", ""
	if filter.UserID == "" {
		return nil, BadAuthRequest("用户 ID 不能为空")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reader := s.repo.WithContext(ctx)
	user, err := reader.ControlPreviewUser(filter)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, NotFound("普通用户不存在或已注销")
	}
	if err != nil {
		return nil, err
	}
	models, err := reader.ControlPreviewModels(filter)
	if err != nil {
		return nil, err
	}
	entries, total, err := reader.ControlPreviewLedger(filter)
	if err != nil {
		return nil, err
	}
	return &ControlPreviewUserDetail{ControlPreviewMeta: meta, User: *user, Models: models, Entries: entries, Total: total, Page: filter.Page, PageSize: filter.PageSize}, nil
}
