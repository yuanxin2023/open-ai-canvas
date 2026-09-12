package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestPaymentNotificationPayloadPreservesGETRawQuery(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/notify?name=%E7%A7%AF%E5%88%86&sign=abc%2B123", nil)
	payload, err := paymentNotificationPayload(context)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(payload), "name=%E7%A7%AF%E5%88%86&sign=abc%2B123"; got != want {
		t.Fatalf("payload = %q, want %q", got, want)
	}
}

func TestPaymentNotificationPayloadReadsPOSTBodyAndLimitsSize(t *testing.T) {
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodPost, "/notify", strings.NewReader("trade_no=1"))
	payload, err := paymentNotificationPayload(context)
	if err != nil || string(payload) != "trade_no=1" {
		t.Fatalf("payload = %q, err = %v", payload, err)
	}

	context, _ = gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(http.MethodGet, "/notify?"+strings.Repeat("a", paymentNotificationMaxBytes+1), nil)
	if _, err := paymentNotificationPayload(context); err == nil {
		t.Fatal("oversized GET notification was accepted")
	}
}
