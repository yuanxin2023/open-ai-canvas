package handler

import (
	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func registerAdminControlPreviewRoutes(r *gin.RouterGroup, svc *service.Service) {
	for _, path := range []string{"", "/users", "/users/:id", "/model-users"} {
		r.GET("/admin/analytics/control-preview"+path, func(c *gin.Context) {
			actor, err := currentUser(c, svc)
			if err != nil {
				failService(c, err)
				return
			}
			var query service.ControlPreviewQuery
			if err := c.ShouldBindQuery(&query); err != nil {
				failService(c, service.BadAuthRequest("统计查询参数无效"))
				return
			}
			var result any
			switch path {
			case "":
				result, err = svc.AdminControlPreview(c.Request.Context(), actor, query)
			case "/users/:id":
				result, err = svc.AdminControlPreviewUser(c.Request.Context(), actor, c.Param("id"), query)
			default:
				result, err = svc.AdminControlPreviewUsers(c.Request.Context(), actor, query, path == "/model-users")
			}
			if err != nil {
				failService(c, err)
				return
			}
			ok(c, result)
		})
	}
}
