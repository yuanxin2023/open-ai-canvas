package handler

import (
	"testing"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func TestCustomerServiceRoutesAreRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterCustomerServiceRoutes(router.Group("/api"), &service.Service{})
	wanted := map[string]bool{
		"GET /api/public/customer-service":              false,
		"GET /api/public/customer-service/button-image": false,
		"GET /api/admin/customer-service":               false,
		"PATCH /api/admin/customer-service":             false,
		"POST /api/admin/customer-service/button-image": false,
	}
	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		if _, exists := wanted[key]; exists {
			wanted[key] = true
		}
	}
	for route, found := range wanted {
		if !found {
			t.Errorf("route %s is not registered", route)
		}
	}
}
