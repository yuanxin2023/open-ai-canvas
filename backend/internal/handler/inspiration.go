package handler

import (
	"net/http"
	"strconv"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterInspirationRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/inspirations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		rows, err := svc.Inspirations(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"inspirations": rows})
	})
	r.GET("/inspirations/:id/cover", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		stream, err := svc.OpenInspirationCover(user, c.Param("id"), c.GetHeader("Range"))
		if err != nil {
			failService(c, err)
			return
		}
		writeInspirationCover(c, stream)
	})
	r.GET("/admin/inspiration-covers/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		stream, err := svc.OpenInspirationCoverDraft(user, c.Param("id"), c.GetHeader("Range"))
		if err != nil {
			failService(c, err)
			return
		}
		writeInspirationCover(c, stream)
	})
	r.GET("/admin/inspirations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		page, limit, err := parsePaginationQuery(c, 20)
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.AdminInspirationPage(user, service.AdminListQuery{Keyword: c.Query("keyword"), Status: c.Query("status"), Type: c.Query("mode"), Page: page, Limit: limit})
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.POST("/admin/inspirations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req service.InspirationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		row, err := svc.CreateInspiration(user, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"inspiration": row})
	})
	r.PUT("/admin/inspirations/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req service.InspirationRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		row, err := svc.UpdateInspiration(user, c.Param("id"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"inspiration": row})
	})
	r.PATCH("/admin/inspirations/:id/status", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req struct {
			Status model.InspirationStatus `json:"status"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		row, err := svc.SetInspirationStatus(user, c.Param("id"), req.Status)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"inspiration": row})
	})
	r.DELETE("/admin/inspirations/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.DeleteInspirations(user, []string{c.Param("id")}); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
	r.POST("/admin/inspirations/batch-delete", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req struct {
			IDs []string `json:"ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		if err := svc.DeleteInspirations(user, req.IDs); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
	r.GET("/admin/inspirations/order", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		items, err := svc.InspirationOrder(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"items": items})
	})
	r.PUT("/admin/inspirations/order", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var req service.InspirationOrderRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		if err := svc.SaveInspirationOrder(user, req); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"saved": true})
	})
	r.POST("/admin/inspiration-covers", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		policy, available := loadRuntimePolicy(c, svc)
		if !available || !enforceRateLimit(c, "admin-inspiration-cover-upload:"+user.ID, policy.Request.ResourceUploadPerMinute, time.Minute) {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.InspirationCoverMaxBytes+(1<<20))
		file, err := c.FormFile("file")
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		width, _ := strconv.Atoi(c.PostForm("width"))
		height, _ := strconv.Atoi(c.PostForm("height"))
		resource, err := svc.UploadInspirationCover(user, file, width, height)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"resource": resource})
	})
	r.DELETE("/admin/inspiration-covers/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.DiscardInspirationCover(user, c.Param("id")); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
}

func writeInspirationCover(c *gin.Context, stream *service.ResourceStream) {
	defer stream.Body.Close()
	mimeType := stream.Resource.MimeType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.Header("Referrer-Policy", "no-referrer")
	c.Header("Accept-Ranges", stream.AcceptRanges)
	c.Header("X-Content-Type-Options", "nosniff")
	if stream.ContentRange != "" {
		c.Header("Content-Range", stream.ContentRange)
	}
	c.DataFromReader(stream.StatusCode, stream.ContentLength, mimeType, stream.Body, nil)
}
