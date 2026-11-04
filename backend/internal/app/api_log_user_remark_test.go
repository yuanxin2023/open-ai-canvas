package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAPICallLogUserRemarkVisibilitySearchAndRefresh(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&model.User{}, &model.ModelChannel{}, &model.ApiCallLog{}, &model.BillingOrder{}); err != nil {
		t.Fatal(err)
	}
	user := model.User{ID: "creator", Username: "creator", AdminRemark: "企业客户 YC-2042"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	order := model.BillingOrder{ID: "order", UserID: user.ID, IdempotencyKey: "remark-test"}
	if err := db.Create(&order).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	logs := []model.ApiCallLog{
		{ID: "request-1", UserID: user.ID, RequestKind: "create", Capability: "text", Billable: true, BillingOrderID: order.ID, CreatedAt: now},
		{ID: "request-2", UserID: user.ID, RequestKind: "create", Capability: "text", CreatedAt: now.Add(-time.Second)},
		{ID: "request-missing-user", UserID: "missing", RequestKind: "create", Capability: "text", CreatedAt: now.Add(-2 * time.Second)},
	}
	if err := db.Create(&logs).Error; err != nil {
		t.Fatal(err)
	}
	svc := &Service{repo: repository.New(db)}
	actor := &model.User{Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionAPILogs}}
	query := APICallLogQuery{Keyword: "yc-2042", Limit: 1}
	page, err := svc.AdminAPICallLogs(actor, query)
	if err != nil || page.Total != 2 || len(page.Logs) != 1 || page.Logs[0].UserRemark != user.AdminRemark {
		t.Fatalf("remark search and count: page=%+v err=%v", page, err)
	}
	query.Page = 2
	page, err = svc.AdminAPICallLogs(actor, query)
	if err != nil || len(page.Logs) != 1 || page.Logs[0].ID != "request-2" {
		t.Fatalf("remark search pagination: page=%+v err=%v", page, err)
	}
	data, err := svc.AdminAPICallLogsCSV(actor, query)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf")))).ReadAll()
	if err != nil || len(rows) != 3 {
		t.Fatalf("export must use the same remark filter without pagination: rows=%v err=%v", rows, err)
	}
	for _, denied := range []*model.User{
		nil,
		{Role: model.UserRoleUser},
		{Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionUsers}},
		{Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{model.AdminPermissionCredits}},
	} {
		if _, err := svc.AdminAPICallLogs(denied, query); err == nil {
			t.Fatal("actor without request-log permission could search internal remarks")
		}
		if _, err := svc.AdminAPICallLog(denied, "request-1"); err == nil {
			t.Fatal("actor without request-log permission could read internal remarks")
		}
		if _, err := svc.AdminAPICallLogsCSV(denied, query); err == nil {
			t.Fatal("actor without request-log permission could export remark search results")
		}
	}
	for _, permission := range []model.AdminPermission{model.AdminPermissionAPILogs, model.AdminPermissionCredits, model.AdminPermissionPaymentOrders} {
		viewer := &model.User{Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{permission}}
		log, err := svc.AdminAPICallLogByBillingOrder(viewer, order.ID)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(log)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := strings.Contains(string(encoded), "userRemark"), permission == model.AdminPermissionAPILogs; got != want {
			t.Fatalf("remark visibility for %s = %v, want %v", permission, got, want)
		}
	}
	for _, remark := range []string{"已更名的新备注", ""} {
		if err := db.Model(&model.User{}).Where("id = ?", user.ID).Update("admin_remark", remark).Error; err != nil {
			t.Fatal(err)
		}
		log, err := svc.AdminAPICallLog(actor, "request-1")
		if err != nil || log.UserRemark != remark {
			t.Fatalf("detail must reflect current remark %q: log=%+v err=%v", remark, log, err)
		}
		page, err := svc.AdminAPICallLogs(actor, APICallLogQuery{Keyword: "yc-2042"})
		if err != nil || page.Total != 0 {
			t.Fatalf("old remark must no longer match: page=%+v err=%v", page, err)
		}
		if remark != "" {
			page, err = svc.AdminAPICallLogs(actor, APICallLogQuery{Keyword: remark})
			if err != nil || page.Total != 2 || page.Logs[0].UserRemark != remark {
				t.Fatalf("updated remark must match historical requests: page=%+v err=%v", page, err)
			}
		}
	}
	log, err := svc.AdminAPICallLog(actor, "request-missing-user")
	if err != nil || log.UserRemark != "" {
		t.Fatalf("missing user should have no remark: log=%+v err=%v", log, err)
	}
}
