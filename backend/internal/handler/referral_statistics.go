package handler

import (
	"net/http"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func registerReferralStatisticsRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/admin/referrals/statistics", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		result, err := svc.AdminReferralStatistics(c.Request.Context(), actor)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	for _, path := range []string{"/admin/referrals/statistics/users", "/admin/referrals/users/:id/invitees"} {
		r.GET(path, func(c *gin.Context) {
			actor, err := currentUser(c, svc)
			if err != nil {
				failService(c, err)
				return
			}
			var q service.ReferralStatisticsQuery
			if err := c.ShouldBindQuery(&q); err != nil {
				fail(c, http.StatusBadRequest, err)
				return
			}
			if c.Param("id") != "" {
				result, err := svc.AdminReferralInvitees(c.Request.Context(), actor, c.Param("id"), q)
				if err != nil {
					failService(c, err)
					return
				}
				ok(c, result)
				return
			}
			result, err := svc.AdminReferralStatisticsUsers(c.Request.Context(), actor, q)
			if err != nil {
				failService(c, err)
				return
			}
			ok(c, result)
		})
	}
}
