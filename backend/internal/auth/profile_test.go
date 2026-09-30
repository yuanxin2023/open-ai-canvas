package auth

import (
	"strings"
	"testing"

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

	updated, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "  new_creator  "})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Username != "new_creator" || updated.ProfileName != "new_creator" || updated.DisplayName != "new_creator" || updated.ID != user.ID || updated.Email != user.Email || updated.Role != user.Role {
		t.Fatalf("updated profile = %#v", updated)
	}

	var stored model.User
	if err := db.First(&stored, "id = ?", user.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Username != "new_creator" || stored.ProfileName != "new_creator" || stored.DisplayName != "new_creator" || stored.Email != user.Email {
		t.Fatalf("stored profile = %#v", stored)
	}
	for _, account := range []string{"new_creator", "NEW_CREATOR", user.Email} {
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

	for _, input := range []string{"   ", "ab", "中文用户名", strings.Repeat("a", 33), "name with spaces"} {
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
	updated, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "Creator", AvatarResourceID: &avatarID})
	if err != nil {
		t.Fatal(err)
	}
	if updated.AvatarResourceID != resource.ID || updated.AvatarURL == "" {
		t.Fatalf("avatar was not exposed: %#v", updated)
	}

	empty := ""
	updated, err = svc.UpdateProfile(&user, UpdateProfileRequest{Username: "Creator", AvatarResourceID: &empty})
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
		if _, err := svc.UpdateProfile(&user, UpdateProfileRequest{Username: "Creator", AvatarResourceID: &id}); err == nil {
			t.Fatalf("invalid avatar %q was accepted", id)
		}
	}
}
