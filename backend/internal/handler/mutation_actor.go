package handler

import (
	"github.com/gin-gonic/gin"
	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/service"
	"net/http"
)

// The expected account binds a pending browser action to its authenticated user;
// it never supplies identity or replaces the service's authorization checks.
func currentMutationActor(c *gin.Context, svc *service.Service) (*model.User, error) {
	user, err := currentUser(c, svc)
	if err != nil {
		return nil, err
	}
	if user == nil || user.ID == "" {
		return nil, kernel.NewAppError(http.StatusUnauthorized, "请先登录")
	}
	if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead && c.GetHeader("X-Canvas-Expected-User-ID") != user.ID {
		return nil, kernel.NewAppError(http.StatusConflict, "登录账号已变化，请重新打开操作页面")
	}
	return user, nil
}
