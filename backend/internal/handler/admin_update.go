package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func decodeUpdateBody(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return service.BadAuthRequest("只允许一个 JSON 请求对象")
	}
	return nil
}

func RegisterAdminUpdateRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/admin/system-update", func(c *gin.Context) {
		actor, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		status, err := svc.AdminUpdateStatus(c.Request.Context(), actor)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, status)
	})
	r.POST("/admin/system-update/check", func(c *gin.Context) {
		actor, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		status, err := svc.AdminCheckUpdate(c.Request.Context(), actor)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, status)
	})
	r.POST("/admin/system-update/start", func(c *gin.Context) {
		actor, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request struct {
			TargetVersion string `json:"targetVersion" binding:"required"`
		}
		if err := decodeUpdateBody(c, &request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		status, err := svc.AdminStartUpdate(c.Request.Context(), actor, request.TargetVersion)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, status)
	})
	r.POST("/admin/system-update/rollback", func(c *gin.Context) {
		actor, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request struct {
			Reason string `json:"reason" binding:"required"`
		}
		if err := decodeUpdateBody(c, &request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		status, err := svc.AdminRollbackUpdate(c.Request.Context(), actor, request.Reason)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, status)
	})
}
