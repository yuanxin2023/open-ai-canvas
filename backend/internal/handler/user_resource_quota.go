package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/service"
)

func registerUserResourceQuotaRoutes(r *gin.RouterGroup, svc *service.Service) {
	routes := []struct {
		path  string
		read  func(*model.User, string) (*service.UserResourceQuotaView, error)
		patch func(*model.User, string, service.UserResourceQuotaRequest) (*service.UserResourceQuotaView, error)
	}{
		{"/admin/users/:id/resource-quota", svc.AdminUserResourceQuota, svc.PatchAdminUserResourceQuota},
		{"/admin/administrators/:id/resource-quota", svc.AdminAdministratorResourceQuota, svc.PatchAdminAdministratorResourceQuota},
	}
	for _, route := range routes {
		r.GET(route.path, func(c *gin.Context) {
			actor, err := currentUser(c, svc)
			if err != nil {
				failService(c, err)
				return
			}
			view, err := route.read(actor, c.Param("id"))
			if err != nil {
				failService(c, err)
				return
			}
			ok(c, view)
		})
		r.PATCH(route.path, func(c *gin.Context) {
			actor, err := currentUser(c, svc)
			if err != nil {
				failService(c, err)
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
			var request service.UserResourceQuotaRequest
			decoder := json.NewDecoder(c.Request.Body)
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&request); err != nil {
				fail(c, http.StatusBadRequest, err)
				return
			}
			if err := decoder.Decode(new(any)); err != io.EOF {
				fail(c, http.StatusBadRequest, service.BadAuthRequest("请求只能包含一个 JSON 对象"))
				return
			}
			view, err := route.patch(actor, c.Param("id"), request)
			if err != nil {
				failService(c, err)
				return
			}
			ok(c, view)
		})
	}
}
