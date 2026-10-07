package handler

import (
	"testing"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func TestInspirationRoutesRegisterWithoutWildcardConflicts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterInspirationRoutes(router.Group("/api"), &service.Service{})
	want := map[string]bool{
		"GET /api/inspirations":                     false,
		"PUT /api/admin/inspirations/order":         false,
		"POST /api/admin/inspirations/batch-delete": false,
		"PATCH /api/admin/inspirations/:id/status":  false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := want[key]; exists {
			want[key] = true
		}
	}
	for route, found := range want {
		if !found {
			t.Fatalf("route %s was not registered", route)
		}
	}
}
