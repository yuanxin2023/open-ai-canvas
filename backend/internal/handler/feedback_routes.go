package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/kernel"
	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func decodeFeedbackBody(c *gin.Context, target any) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return kernel.BadAuthRequest("反馈请求格式无效")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return kernel.BadAuthRequest("反馈请求只能包含一个对象")
	}
	return nil
}

func feedbackExpectedVersion(c *gin.Context) (int64, error) {
	value, err := strconv.ParseInt(strings.TrimSpace(c.GetHeader("X-Canvas-Feedback-Version")), 10, 64)
	if err != nil || value < 1 {
		return 0, kernel.BadAuthRequest("缺少有效反馈版本，请刷新列表")
	}
	return value, nil
}

func RegisterFeedbackRoutes(r *gin.RouterGroup, svc *service.Service) {
	// 用户提交反馈
	r.POST("/feedbacks", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}

		var req service.SubmitFeedbackRequest
		if err := decodeFeedbackBody(c, &req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}

		fb, err := svc.SubmitFeedback(user, req)
		if err != nil {
			failService(c, err)
			return
		}

		ok(c, fb)
	})

	// 用户查看自己的反馈列表（我的收件箱）
	r.GET("/feedbacks/my", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}

		list, err := svc.MyFeedbacks(user)
		if err != nil {
			failService(c, err)
			return
		}

		ok(c, list)
	})

	// 上传反馈附件（支持图片与视频）
	r.POST("/feedbacks/attachments", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.RequireFeature(service.FeatureUserFeedback); err != nil {
			failService(c, err)
			return
		}

		// 限制请求体上限 55MB（50MB 附件 + 表单头）
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 55*1024*1024)
		file, err := c.FormFile("file")
		if c.Request.MultipartForm != nil {
			defer c.Request.MultipartForm.RemoveAll()
		}
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}

		attachment, err := svc.UploadFeedbackAttachment(user, file)
		if err != nil {
			failService(c, err)
			return
		}

		ok(c, attachment)
	})

	r.GET("/feedbacks/:id/attachments/:resourceID", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		delivery, err := svc.OpenFeedbackAttachment(user, c.Param("id"), c.Param("resourceID"), c.GetHeader("Range"))
		if err != nil {
			failService(c, err)
			return
		}
		serveResourceDelivery(c, delivery, "private, no-store", "inline")
	})

	// 管理员获取全量反馈列表
	r.GET("/admin/feedbacks", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}

		page, limit, err := parsePaginationQuery(c, 20)
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}

		filter := service.FeedbackFilter{
			Type:    c.Query("type"),
			Status:  c.Query("status"),
			Keyword: c.Query("keyword"),
			Page:    page,
			Limit:   limit,
		}

		if arch := c.Query("archived"); arch == "true" || arch == "1" {
			v := true
			filter.Archived = &v
		} else if arch == "false" || arch == "0" {
			v := false
			filter.Archived = &v
		}

		result, err := svc.AdminListFeedbacks(user, filter)
		if err != nil {
			failService(c, err)
			return
		}

		ok(c, result)
	})

	// 管理员更新处理状态及回复
	r.PATCH("/admin/feedbacks/:id", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}

		var req service.AdminUpdateFeedbackRequest
		if err := decodeFeedbackBody(c, &req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}

		req.ExpectedVersion, err = feedbackExpectedVersion(c)
		if err != nil {
			failService(c, err)
			return
		}
		fb, err := svc.AdminUpdateFeedback(user, c.Param("id"), req)
		if err != nil {
			failService(c, err)
			return
		}

		ok(c, fb)
	})

	// 管理员归档反馈
	r.POST("/admin/feedbacks/:id/archive", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}

		version, err := feedbackExpectedVersion(c)
		if err != nil {
			failService(c, err)
			return
		}
		fb, err := svc.AdminArchiveFeedback(user, c.Param("id"), true, version)
		if err != nil {
			failService(c, err)
			return
		}

		ok(c, fb)
	})

	// 管理员取消归档
	r.POST("/admin/feedbacks/:id/unarchive", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}

		version, err := feedbackExpectedVersion(c)
		if err != nil {
			failService(c, err)
			return
		}
		fb, err := svc.AdminArchiveFeedback(user, c.Param("id"), false, version)
		if err != nil {
			failService(c, err)
			return
		}

		ok(c, fb)
	})

	// 管理员删除反馈
	r.DELETE("/admin/feedbacks/:id", func(c *gin.Context) {
		user, err := currentMutationActor(c, svc)
		if err != nil {
			failService(c, err)
			return
		}

		version, err := feedbackExpectedVersion(c)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.AdminDeleteFeedback(user, c.Param("id"), version); err != nil {
			failService(c, err)
			return
		}

		ok(c, gin.H{"ok": true})
	})
}
