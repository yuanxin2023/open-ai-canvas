package app

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

// 分组权限同时限制目录、报价和实际账单，放行后积分价格保持不变。
func TestUserGroupChannelVisibilityPreservesCreditPricing(t *testing.T) {
	s, _, _, _ := creationTestService(t)
	admin := &model.User{ID: "group-admin", Role: model.UserRoleAdmin}
	quote := func() (int64, error) {
		result, err := s.QuoteChannelModel("user", ChannelModelQuoteRequest{ChannelID: "channel", ModelKey: "text-test", Intent: ModelRequestIntent{Capability: "text", Operation: "text_generation"}})
		if err != nil {
			return 0, err
		}
		return result.AmountMicrocredits, nil
	}
	catalogUnitPrice := func() (int64, bool) {
		user, err := s.repo.User("user")
		if err != nil {
			t.Fatal(err)
		}
		catalog, err := s.ModelCatalogForUser(user, nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, channel := range catalog.Channels {
			for _, item := range channel.Models {
				if channel.ID == "channel" && item.ModelKey == "text-test" && len(item.PriceTiers) > 0 {
					return item.PriceTiers[0].UnitPriceMicrocredits, true
				}
			}
		}
		return 0, false
	}

	baselineQuote, err := quote()
	if err != nil || baselineQuote <= 0 {
		t.Fatalf("default group quote = %d, %v", baselineQuote, err)
	}

	vip, err := s.CreateUserGroup(admin, UserGroupRequest{Code: "vip", Name: "VIP", BadgeLabel: "VIP", BadgeStyle: "vip"})
	if err != nil {
		t.Fatal(err)
	}
	group := "vip"
	if _, err := s.UpdateUser(admin, "user", UpdateUserRequest{UserGroup: &group}); err != nil {
		t.Fatal(err)
	}
	if _, visible := catalogUnitPrice(); visible {
		t.Fatal("unbound channel must be hidden from the vip catalog")
	}
	if _, err := quote(); err == nil {
		t.Fatal("quote on an unbound channel must fail closed")
	}
	if _, err := s.newBillingOrder("user", "task", "denied-group-test", "channel", "text-test", "text", "test", 1, tokenBillingEstimate{}); err == nil {
		t.Fatal("billing on an unbound channel must fail closed")
	}

	update := UserGroupRequest{Name: "VIP", BadgeLabel: "VIP", BadgeStyle: "vip", ChannelIDs: []string{"channel"}}
	if _, err := s.UpdateUserGroup(admin, vip.ID, update); err != nil {
		t.Fatal(err)
	}
	if amount, err := quote(); err != nil || amount != baselineQuote {
		t.Fatalf("vip quote = %d, %v; want unchanged price %d", amount, err, baselineQuote)
	}
	if price, visible := catalogUnitPrice(); !visible || price != 100 {
		t.Fatalf("vip catalog price = %d (visible %v); want 100", price, visible)
	}

	order, err := s.newBillingOrder("user", "task", "group-test", "channel", "text-test", "text", "test", 1, tokenBillingEstimate{})
	if err != nil {
		t.Fatal(err)
	}
	if order.UserGroup != "vip" || order.UnitPriceMicrocredits != 100 || order.MultiplierBasisPoints != 10_000 || order.AmountMicrocredits != baselineQuote {
		t.Fatalf("billing snapshot = %+v", order)
	}

	if err := s.DeleteUserGroup(admin, vip.ID); err == nil {
		t.Fatal("group with users must not be deletable")
	}
}
