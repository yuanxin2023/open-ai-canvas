package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func registerInspirationLibraryRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/inspiration-library", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		view, err := svc.InspirationLibrary(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, view)
	})
	r.POST("/inspiration-library/commands", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128<<20)
		decoder := json.NewDecoder(c.Request.Body)
		decoder.DisallowUnknownFields()
		var command service.InspirationLibraryCommand
		if err := decoder.Decode(&command); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			failService(c, service.BadAuthRequest("只允许一个 JSON 请求对象"))
			return
		}
		view, err := svc.ApplyInspirationLibrary(user, command)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, view)
	})
}
