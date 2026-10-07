package handler

import (
	"net/http"
	"path/filepath"
	"strings"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

type adminSkillAvailabilityBody struct {
	SkillIDs  []string `json:"skillIds"`
	Available *bool    `json:"available"`
}

type adminSkillCategoryAvailabilityBody struct {
	Available *bool `json:"available"`
}

func RegisterAdminSkillRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/admin/skills", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		catalog, err := svc.AdminSkills(actor)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})

	r.POST("/admin/skills/install", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.SkillPackageUploadMaxBytes)
		file, err := c.FormFile("file")
		if err != nil {
			failService(c, service.BadAuthRequest("请选择 Markdown 或 ZIP 技能文件"))
			return
		}
		sourceType := strings.ToLower(strings.TrimSpace(c.PostForm("sourceType")))
		if sourceType == "" {
			switch strings.ToLower(filepath.Ext(file.Filename)) {
			case ".md", ".markdown":
				sourceType = "markdown"
			case ".zip":
				sourceType = "zip"
			}
		}
		catalog, err := svc.AdminInstallSkillUpload(actor, sourceType, file, service.SkillInstallRequest{
			Name: c.PostForm("name"), Description: c.PostForm("description"), Tag: c.PostForm("tag"),
		})
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})

	r.POST("/admin/skills/install/github", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 32<<10)
		var req service.SkillGitHubInstallRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			failService(c, service.BadAuthRequest("GitHub 技能数据格式无效"))
			return
		}
		catalog, err := svc.AdminInstallGitHubSkill(actor, req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})

	r.PUT("/admin/skills/availability", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var body adminSkillAvailabilityBody
		if err := c.ShouldBindJSON(&body); err != nil || body.Available == nil {
			failService(c, service.BadAuthRequest("技能可用状态请求格式无效"))
			return
		}
		catalog, err := svc.UpdateAdminSkillAvailability(actor, service.AdminSkillAvailabilityRequest{SkillIDs: body.SkillIDs, Available: *body.Available})
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})

	r.PUT("/admin/skills/categories/:tag/availability", func(c *gin.Context) {
		actor, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 8<<10)
		var body adminSkillCategoryAvailabilityBody
		if err := c.ShouldBindJSON(&body); err != nil || body.Available == nil {
			failService(c, service.BadAuthRequest("分类可用状态请求格式无效"))
			return
		}
		catalog, err := svc.UpdateAdminSkillCategoryAvailability(actor, c.Param("tag"), *body.Available)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})
}
