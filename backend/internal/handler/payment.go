package handler

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

const paymentNotificationMaxBytes = 1 << 20

var paymentOrderIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)

func RegisterPaymentRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/payments/providers", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		providers, err := svc.PaymentProviders(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"providers": providers})
	})
	r.GET("/payments/products", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		catalog, err := svc.TopupProducts(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})
	r.GET("/payments/promotion/image", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		stream, err := svc.OpenPaymentPromotionImage(user, c.GetHeader("Range"))
		if err != nil {
			failService(c, err)
			return
		}
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
	})
	r.POST("/payments/orders", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if !enforceRateLimit(c, "payment-order:"+user.ID, 20, time.Hour) {
			return
		}
		var request service.CreatePaymentOrderRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		request.ClientIP = c.ClientIP()
		order, err := svc.CreatePaymentOrder(c.Request.Context(), user, request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})
	r.GET("/payments/orders", func(c *gin.Context) {
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
		result, err := svc.PaymentOrderPage(user, c.Query("status"), page, limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	r.GET("/payments/orders/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		order, err := svc.PaymentOrder(user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})
	r.POST("/payments/orders/:id/query", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if !enforceRateLimit(c, "payment-query:"+user.ID+":"+c.Param("id"), 30, time.Minute) {
			return
		}
		order, err := svc.QueryPaymentOrder(c.Request.Context(), user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})
	r.POST("/payments/orders/:id/close", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if !enforceRateLimit(c, "payment-close:"+user.ID+":"+c.Param("id"), 10, time.Minute) {
			return
		}
		order, err := svc.ClosePaymentOrder(c.Request.Context(), user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})
	r.GET("/payments/orders/:id/checkout", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		target, err := svc.PaymentCheckout(user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		c.Redirect(http.StatusFound, target)
	})
	r.POST("/payments/orders/:id/checkout/refresh", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if !enforceRateLimit(c, "payment-checkout-refresh:"+user.ID+":"+c.Param("id"), 5, time.Hour) {
			return
		}
		order, err := svc.RefreshPaymentCheckout(c.Request.Context(), user, c.Param("id"), c.ClientIP())
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})

	// Provider callbacks are intentionally unauthenticated at the application
	// layer. Authenticity is established by the pinned provider config and raw
	// request signature before any durable event is accepted.
	paymentNotificationHandler := func(c *gin.Context) {
		rawBody, err := paymentNotificationPayload(c)
		if err != nil {
			status := http.StatusBadRequest
			if errors.Is(err, errPaymentNotificationTooLarge) {
				status = http.StatusRequestEntityTooLarge
			}
			writePaymentNotificationFailure(c, svc, c.Param("providerId"), status)
			return
		}
		if len(rawBody) == 0 && c.Request.URL.RawQuery != "" {
			rawBody = []byte(c.Request.URL.RawQuery)
		}
		err = svc.AcceptPaymentNotification(c.Request.Context(), c.Param("providerId"), c.Param("configId"), c.Request.Header.Clone(), rawBody)
		if err != nil {
			status := http.StatusInternalServerError
			var appErr *service.AppError
			if errors.As(err, &appErr) && appErr.Status < http.StatusInternalServerError {
				status = http.StatusBadRequest
			}
			writePaymentNotificationFailure(c, svc, c.Param("providerId"), status)
			return
		}
		status, contentType, body := svc.PaymentNotificationResponse(c.Param("providerId"), true)
		if body != "" {
			c.Data(status, contentType, []byte(body))
			return
		}
		c.Status(status)
	}
	r.GET("/payments/notify/:providerId/:configId", paymentNotificationHandler)
	r.POST("/payments/notify/:providerId/:configId", paymentNotificationHandler)
	r.GET("/payments/return/:providerId", func(c *gin.Context) {
		orderID := strings.ToLower(strings.TrimSpace(c.Query("orderId")))
		if !paymentOrderIDPattern.MatchString(orderID) {
			c.Redirect(http.StatusFound, "/wallet?payment=invalid")
			return
		}
		c.Redirect(http.StatusFound, "/wallet?paymentOrder="+url.QueryEscape(orderID))
	})

	admin := r.Group("/admin/payments")
	registerPaymentExportRoutes(admin, svc)
	admin.GET("/providers", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		providers, err := svc.AdminPaymentProviders(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"providers": providers})
	})
	admin.PUT("/providers/:id/config", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request service.UpdatePaymentProviderConfigRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		provider, err := svc.UpdatePaymentProviderConfig(user, c.Param("id"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"provider": provider})
	})
	admin.GET("/products", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		products, err := svc.AdminTopupProducts(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"products": products})
	})
	admin.POST("/products", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request service.TopupProductRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		product, err := svc.CreateTopupProduct(user, request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"product": product})
	})
	admin.PUT("/products/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request service.TopupProductRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		product, err := svc.UpdateTopupProduct(user, c.Param("id"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"product": product})
	})
	admin.GET("/promotion", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		promotion, err := svc.AdminPaymentPromotion(user)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"promotion": promotion})
	})
	admin.GET("/promotion/image", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		stream, err := svc.OpenAdminPaymentPromotionImage(user, c.GetHeader("Range"))
		if err != nil {
			failService(c, err)
			return
		}
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
	})
	admin.PUT("/promotion", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request service.PaymentPromotionSetting
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		promotion, err := svc.UpdatePaymentPromotion(user, request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"promotion": promotion})
	})
	admin.POST("/promotion/image", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		policy, available := loadRuntimePolicy(c, svc)
		if !available || !enforceRateLimit(c, "admin-payment-promotion-upload:"+user.ID, policy.Request.ResourceUploadPerMinute, time.Minute) {
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, service.PaymentPromotionImageMaxBytes+(1<<20))
		file, err := c.FormFile("file")
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		resource, err := svc.UploadPaymentPromotionImage(user, file)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"resource": resource})
	})
	admin.DELETE("/promotion/image/:id", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		if err := svc.DiscardPaymentPromotionImage(user, c.Param("id")); err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"ok": true})
	})
	admin.GET("/orders", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		page, limit, err := parsePaginationQuery(c, 30)
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.AdminPaymentOrderPage(user, paymentOrderQuery(c), page, limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	admin.POST("/orders/:id/query", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		order, err := svc.AdminQueryPaymentOrder(c.Request.Context(), user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})
	admin.POST("/orders/:id/close", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		order, err := svc.AdminClosePaymentOrder(c.Request.Context(), user, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"order": order})
	})
	admin.POST("/reconciliations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		var request service.RunPaymentReconciliationRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		run, err := svc.RunPaymentReconciliation(c.Request.Context(), user, request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})
	admin.GET("/reconciliations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		page, limit, err := parsePaginationQuery(c, 30)
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.AdminPaymentReconciliationPage(user, paymentReconciliationQuery(c), page, limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	admin.GET("/reconciliations/:id/items", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		page, limit, err := parsePaginationQuery(c, 50)
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		result, err := svc.AdminPaymentReconciliationItems(user, c.Param("id"), c.Query("result"), page, limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
}

var errPaymentNotificationTooLarge = errors.New("payment notification payload is too large")

func paymentNotificationPayload(c *gin.Context) ([]byte, error) {
	if c.Request.Method == http.MethodGet {
		payload := []byte(c.Request.URL.RawQuery)
		if len(payload) > paymentNotificationMaxBytes {
			return nil, errPaymentNotificationTooLarge
		}
		return payload, nil
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, paymentNotificationMaxBytes)
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

func writePaymentNotificationFailure(c *gin.Context, svc *service.Service, providerID string, status int) {
	responseStatus, contentType, body := svc.PaymentNotificationFailureResponse(providerID, status)
	if body != "" {
		c.Data(responseStatus, contentType, []byte(body))
		return
	}
	c.JSON(responseStatus, gin.H{"code": "FAIL", "message": http.StatusText(responseStatus)})
}
