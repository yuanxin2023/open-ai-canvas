package handler

import (
	"io"
	"net/http"
	"time"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func RegisterCustomerServiceRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/public/customer-service", func(c *gin.Context) {
		setting, err := svc.CustomerService()
		if err != nil {
			failService(c, err)
			return
		}
		c.Header("Cache-Control", "no-store")
		ok(c, gin.H{"customerService": setting})
	})

	r.GET("/public/customer-service/button-image", func(c *gin.Context) {
		stream, err := svc.OpenCustomerServiceButtonImage(c.GetHeader("Range"))
		if err != nil {
			failService(c, err)
			return
		}
		defer stream.Body.Close()
		resource := stream.Resource
		mimeType := resource.MimeType
		if mimeType == "" {
			mimeType = "image/png"
		}
		if c.Query("v") != "" {
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Header("Cache-Control", "public, no-cache")
		}
		c.Header("Accept-Ranges", "bytes")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Content-Type-Options", "nosniff")
		if stream.ContentRange != "" {
			c.Header("Content-Range", stream.ContentRange)
		}
		if seeker, available := stream.Body.(io.ReadSeeker); available {
			c.Header("Content-Type", mimeType)
			http.ServeContent(c.Writer, c.Request, resource.ID, resource.UpdatedAt, seeker)
			return
		}
		c.DataFromReader(stream.StatusCode, stream.ContentLength, mimeType, stream.Body, nil)
	})

	r.GET("/admin/customer-service", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		setting, err := svc.AdminCustomerService(actor)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"setting": setting})
	})

	r.PATCH("/admin/customer-service", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		current, err := svc.AdminCustomerService(actor)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
		req := current.CustomerServiceSetting
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		setting, err := svc.UpdateCustomerService(actor, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"setting": setting})
	})

	r.POST("/admin/customer-service/button-image", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		policy, available := loadRuntimePolicy(c, svc)
		if !available || !enforceRateLimit(c, "admin-customer-service-upload:"+actor.ID, policy.Request.ResourceUploadPerMinute, time.Minute) {
			return
		}
		maxBytes := service.CustomerServiceButtonImageMaxBytes()
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes+(1<<20))
		file, err := c.FormFile("file")
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		resource, err := svc.UploadCustomerServiceButtonImage(actor, file)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"resource": resource})
	})
}
