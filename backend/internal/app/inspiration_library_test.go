package app

import (
	"testing"

	"infinite-canvas/backend/internal/model"
)

func TestClosedInspirationLibraryRejectsReadAndWriteBeforeDomainQueries(t *testing.T) {
	svc, _ := newFeatureAvailabilityTestService(t)
	actor := &model.User{ID: "owner", Role: model.UserRoleUser}
	if _, err := svc.InspirationLibrary(actor); appStatusForInspiration(err) != 403 {
		t.Fatalf("closed read = %v", err)
	}
	if _, err := svc.ApplyInspirationLibrary(actor, InspirationLibraryCommand{}); appStatusForInspiration(err) != 403 {
		t.Fatalf("closed write = %v", err)
	}
}

func TestInspirationLibraryHasNoAnonymousOrEmptyUserFallback(t *testing.T) {
	svc := &Service{}
	for _, actor := range []*model.User{nil, {Role: model.UserRoleUser}} {
		if _, err := svc.InspirationLibrary(actor); appStatusForInspiration(err) != 401 {
			t.Fatalf("anonymous read = %v", err)
		}
		if _, err := svc.ApplyInspirationLibrary(actor, InspirationLibraryCommand{}); appStatusForInspiration(err) != 401 {
			t.Fatalf("anonymous write = %v", err)
		}
	}
}

func appStatusForInspiration(err error) int {
	if app, ok := err.(*AppError); ok {
		return app.Status
	}
	return 0
}
