package handler

import (
	"net/http"
	"strconv"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func registerReferralRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/referrals/me", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		result, err := svc.ReferralDashboard(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.GET("/admin/referrals/policy", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		result, err := svc.AdminReferralPolicy(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.PUT("/admin/referrals/policy", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req service.ReferralPolicy
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.UpdateReferralPolicy(user, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.PUT("/admin/referrals/users/:id/rate", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req struct {
			RateBPS *int64 `json:"rateBps"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.AdminSetReferralRate(user, c.Param("id"), req.RateBPS)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.GET("/admin/referrals/users/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		result, err := svc.AdminReferralUser(user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.GET("/admin/referrals/rewards", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "30"))
		result, err := svc.AdminReferralRewards(user, c.Query("status"), page, pageSize)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	review := func(approve bool) gin.HandlerFunc {
		return func(c *gin.Context) {
			user, err := currentUser(c, svc)
			if err != nil {
				failService(c, err)
				return
			}
			var req struct {
				Note string `json:"note"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				fail(c, http.StatusBadRequest, err)
				return
			}
			result, err := svc.AdminReviewReferralReward(user, c.Param("id"), approve, req.Note)
			if err != nil {
				failService(c, err)
				return
			}
			ok(c, result)
		}
	}
	r.POST("/admin/referrals/rewards/:id/approve", review(true))
	r.POST("/admin/referrals/rewards/:id/reject", review(false))
}
