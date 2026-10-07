package auth

import (
	"errors"
	"strings"
	"testing"
	"time"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
)

func TestUpdateProfileSetsLoginUsernameAndKeepsEmailLogin(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	passwordHash, err := HashPassword("strong-password")
	if err != nil {
		t.Fatal(err)
	}
	user := model.User{
		ID:           "profile-user",
		Username:     "profile-user",
		Email:        "member@example.com",
		DisplayName:  "Before",
		Role:         model.UserRoleUser,
		Status:       model.UserStatusActive,
		PasswordHash: passwordHash,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	updated, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "  Maker1  "})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != "maker1" || updated.ProfileName != "maker1" || updated.DisplayName != "maker1" || updated.ID != user.ID || updated.Email != user.Email || updated.Role != user.Role {
		t.Fatalf("updated profile = %#v", updated)
	}

	var stored model.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Username != "maker1" || stored.ProfileName != "maker1" || stored.DisplayName != "maker1" || stored.Email != user.Email {
		t.Fatalf("stored profile = %#v", stored)
	}
	for _, account := range []string{"maker1", "MAKER1", user.Email} {
		if _, err := svc.Login(LoginRequest{Username: account, Password: "strong-password"}); err != nil {
			t.Fatalf("login with %q failed: %v", account, err)
		}
	}
	if _, err := svc.Login(LoginRequest{Username: user.ID, Password: "strong-password"}); err == nil {
		t.Fatal("generated internal username still allowed login after username change")
	}
}

func TestUpdateProfileValidatesLoginUsername(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	user := model.User{ID: "profile-user", Username: "profile-user", DisplayName: "Before", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	for _, input := range []string{"   ", "ab", "用户名实在太长了啊呢", strings.Repeat("a", 10), "name with spaces", "123456", "user_name", "😀用户", "admin"} {
		if _, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: input}); err == nil {
			t.Fatalf("invalid login username %q was accepted", input)
		}
	}
}

func TestUpdateProfileRejectsCaseInsensitiveDuplicateUsername(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	users := []model.User{
		{ID: "profile-user", Username: "profile-user", Role: model.UserRoleUser, Status: model.UserStatusActive},
		{ID: "existing-user", Username: "Creator", Role: model.UserRoleUser, Status: model.UserStatusActive},
	}
	if err := db.Create(&users).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.UpdateProfile(&users[0], UpdateProfileRequest{Username: "creator"}); err == nil {
		t.Fatal("case-insensitive duplicate username was accepted")
	}
}

func TestUpdateProfileBindsAndRemovesOwnedAvatar(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	user := model.User{ID: "profile-user", Username: "profile-user", DisplayName: "Before", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{ID: "avatar-resource", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: 1024}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	avatarID := resource.ID
	updated, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "maker", AvatarResourceID: &avatarID})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AvatarResourceID != resource.ID || updated.AvatarURL == "" {
		t.Fatalf("avatar was not exposed: %#v", updated)
	}

	empty := ""
	updated, err = svc.UpdateProfile(&user, UpdateProfileRequest{Username: "maker", AvatarResourceID: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AvatarResourceID != "" || updated.AvatarURL != "" {
		t.Fatalf("avatar was not removed: %#v", updated)
	}
}

func TestUpdateProfileRejectsInvalidAvatar(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	user := model.User{ID: "profile-user", Username: "profile-user", DisplayName: "Before", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	resources := []model.Resource{
		{ID: "other-avatar", UserID: "another-user", Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: 1024},
		{ID: "large-avatar", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: (2 << 20) + 1},
		{ID: "gif-avatar", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/gif", Size: 1024},
	}
	if err := db.Create(&resources).Error; err != nil {
		t.Fatal(err)
	}
	for _, resource := range resources {
		id := resource.ID
		if _, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "maker", AvatarResourceID: &id}); err == nil {
			t.Fatalf("invalid avatar %q was accepted", id)
		}
	}
}

func TestUpdateProfileAllowsAvatarOnlyForLegacyUsername(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	user := model.User{ID: "legacy-user", Username: "legacy_user_name", DisplayName: "Before", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	resource := model.Resource{ID: "legacy-avatar", UserID: user.ID, Kind: "image", Status: model.ResourceStatusReady, MimeType: "image/png", Size: 1024}
	if err := db.Create(&resource).Error; err != nil {
		t.Fatal(err)
	}
	avatarID := resource.ID
	updated, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: user.Username, AvatarResourceID: &avatarID})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != user.Username || updated.AvatarResourceID != avatarID {
		t.Fatalf("avatar-only update changed legacy identity: %#v", updated)
	}
}

func TestUpdateProfileUsernameChangeQuota(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	user := model.User{ID: "quota-user", Username: "system1", DisplayName: "system1", ProfileName: "system1", Role: model.UserRoleUser, Status: model.UserStatusActive}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	names := []string{"第一名", "second", "third1", "fourth", "fifth1"}
	for index, name := range names {
		updated, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: name})
		if index < 4 {
			if err != nil {
				t.Fatalf("change %d failed: %v", index+1, err)
			}
			user.Username = updated.Username
			user.UsernameCustomizedAt = updated.UsernameCustomizedAt
			if index == 0 {
				replacement := model.User{ID: "replacement-user", Username: "system1", DisplayName: "system1", ProfileName: "system1", Role: model.UserRoleUser, Status: model.UserStatusActive}
				if createErr := db.Create(&replacement).Error; createErr != nil {
					t.Fatalf("released old username was not reusable: %v", createErr)
				}
				if _, invalidErr := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "bad_name"}); invalidErr == nil {
					t.Fatal("invalid failed change was accepted")
				}
				if _, duplicateErr := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "system1"}); duplicateErr == nil {
					t.Fatal("duplicate failed change was accepted")
				}
			}
			continue
		}
		var appErr *kernel.AppError
		if !errors.As(err, &appErr) || appErr.Status != 429 || appErr.Reason != kernel.ReasonUsernameChangeLimit || appErr.RetryAfterSeconds <= 0 {
			t.Fatalf("fifth change error = %#v, want username change limit", err)
		}
	}

	var changes []model.UserUsernameChange
	if err := db.Where("user_id = ?", user.ID).Order("created_at asc").Find(&changes).Error; err != nil {
		t.Fatal(err)
	}
	if len(changes) != 4 || changes[0].CountsTowardLimit || !changes[1].CountsTowardLimit || !changes[2].CountsTowardLimit || !changes[3].CountsTowardLimit {
		t.Fatalf("username change history = %#v", changes)
	}
}

func TestUpdateProfileExpiredQuotaAndAdminUnlimited(t *testing.T) {
	svc, db := newPasswordResetTestService(t)
	now := time.Now()
	customizedAt := now.Add(-60 * 24 * time.Hour)
	user := model.User{ID: "expired-user", Username: "start1", DisplayName: "start1", ProfileName: "start1", UsernameCustomizedAt: &customizedAt, Role: model.UserRoleUser, Status: model.UserStatusActive}
	admin := model.User{ID: "admin-user", Username: "boss01", DisplayName: "boss01", ProfileName: "boss01", UsernameCustomizedAt: &customizedAt, Role: model.UserRoleAdmin, Status: model.UserStatusActive}
	if err := db.Create(&[]model.User{user, admin}).Error; err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 3; index++ {
		createdAt := now.Add(-31*24*time.Hour - time.Duration(index)*time.Hour)
		if err := db.Create(&model.UserUsernameChange{ID: kernel.NewID(), UserID: user.ID, OldUsername: "oldone", NewUsername: "oldtwo", CountsTowardLimit: true, CreatedAt: createdAt}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "fresh1"}); err != nil {
		t.Fatalf("expired quota was not restored: %v", err)
	}
	for _, name := range []string{"boss02", "boss03", "boss04", "boss05"} {
		updated, err := svc.UpdateProfile(&admin, UpdateProfileRequest{Username: name})
		if err != nil {
			t.Fatalf("admin change to %q failed: %v", name, err)
		}
		admin.Username = updated.Username
		admin.UsernameCustomizedAt = updated.UsernameCustomizedAt
	}
}
