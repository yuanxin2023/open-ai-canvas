package app

import (
	"errors"
	"testing"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func autoRefundPolicyTestService(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+newID()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	connection, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	if err := db.AutoMigrate(&model.SystemSetting{}, &model.AdminAuditEvent{}); err != nil {
		t.Fatal(err)
	}
	return &Service{repo: repository.New(db)}, db
}

func TestBillingAutoRefundRequiresExplicitCreditPermission(t *testing.T) {
	for _, test := range []struct {
		name       string
		permission model.AdminPermission
		allowed    bool
	}{
		{"credit-operator", model.AdminPermissionCredits, true},
		{"payment-operator", model.AdminPermissionPaymentOrders, false},
		{"user-operator", model.AdminPermissionUsers, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			svc, db := autoRefundPolicyTestService(t)
			actor := &model.User{ID: test.name, Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelScoped, AdminPermissions: []model.AdminPermission{test.permission}}
			setting, err := svc.UpdateBillingAutoRefundSetting(actor, BillingAutoRefundSetting{Enabled: true})
			if (err == nil) != test.allowed {
				t.Fatalf("allowed=%t, error=%v", test.allowed, err)
			}
			persisted, readErr := svc.billingAutoRefundSetting()
			if readErr != nil || persisted.Enabled != test.allowed || (test.allowed && !setting.Enabled) {
				t.Fatalf("setting=%+v error=%v", persisted, readErr)
			}
			var audits int64
			if err := db.Model(&model.AdminAuditEvent{}).Where("action = ?", "billing_auto_refund.update").Count(&audits).Error; err != nil {
				t.Fatal(err)
			}
			if (audits == 1) != test.allowed {
				t.Fatalf("unexpected audit count=%d", audits)
			}
		})
	}
}

func TestBillingAutoRefundAuditFailureDoesNotEnablePolicy(t *testing.T) {
	svc, db := autoRefundPolicyTestService(t)
	if err := db.Callback().Create().Before("gorm:create").Register("test:reject-refund-audit", func(tx *gorm.DB) {
		if tx.Statement.Table == "admin_audit_events" {
			tx.AddError(errors.New("injected audit failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test:reject-refund-audit") })
	actor := &model.User{ID: "root", Role: model.UserRoleAdmin, AdminLevel: model.AdminLevelFull}
	if _, err := svc.UpdateBillingAutoRefundSetting(actor, BillingAutoRefundSetting{Enabled: true}); err == nil {
		t.Fatal("audit failure ignored")
	}
	setting, err := svc.billingAutoRefundSetting()
	if err != nil || setting.Enabled {
		t.Fatalf("failed request enabled refunds: %+v, %v", setting, err)
	}
}

func TestBillingAutoRefundDisabledDoesNotReadBillingQueue(t *testing.T) {
	svc, db := autoRefundPolicyTestService(t)
	queueReads := 0
	if err := db.Callback().Query().Before("gorm:query").Register("test:count-refund-queue", func(tx *gorm.DB) {
		if tx.Statement.Table == "billing_orders" {
			queueReads++
		}
	}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Callback().Query().Remove("test:count-refund-queue") })
	if refunded := svc.autoRefundReviewBilling(t.Context()); refunded != 0 || queueReads != 0 {
		t.Fatalf("disabled policy read queue %d times and refunded %d", queueReads, refunded)
	}
}
