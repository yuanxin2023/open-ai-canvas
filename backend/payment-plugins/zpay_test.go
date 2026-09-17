package paymentplugins

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type zpayRoundTripFunc func(*http.Request) (*http.Response, error)

func (function zpayRoundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestSignZPayValuesCanonicalizesParameters(t *testing.T) {
	values := url.Values{"b": {"2"}, "a": {"1"}, "empty": {""}, "sign": {"ignored"}, "sign_type": {"MD5"}}
	digest := md5.Sum([]byte("a=1&b=2secret"))
	want := hex.EncodeToString(digest[:])
	if got := signZPayValues(values, "secret"); got != want {
		t.Fatalf("signZPayValues() = %q, want %q", got, want)
	}
}

func TestZPayCreateOrderUsesMultipartAndProviderType(t *testing.T) {
	for _, test := range []struct {
		paymentType string
		providerID  string
	}{
		{paymentType: "alipay", providerID: ProviderZPayAlipay},
		{paymentType: "wxpay", providerID: ProviderZPayWeChat},
	} {
		t.Run(test.paymentType, func(t *testing.T) {
			client := &http.Client{Transport: zpayRoundTripFunc(func(request *http.Request) (*http.Response, error) {
				if request.Method != http.MethodPost || request.URL.String() != "https://pay.example.com/mapi.php" {
					t.Fatalf("request = %s %s", request.Method, request.URL)
				}
				if err := request.ParseMultipartForm(1 << 20); err != nil {
					t.Fatal(err)
				}
				form := request.MultipartForm.Value
				for key, want := range map[string]string{
					"pid": "merchant-1", "cid": "channel-1,channel-2", "type": test.paymentType,
					"out_trade_no": "0123456789abcdef0123456789abcdef", "notify_url": "https://canvas.example.com/api/payments/notify/provider/config",
					"name": "积分充值", "money": "12.34", "clientip": "203.0.113.10", "sign_type": "MD5",
				} {
					if got := firstMultipartValue(form, key); got != want {
						t.Fatalf("form[%s] = %q, want %q", key, got, want)
					}
				}
				values := make(url.Values, len(form))
				for key, entries := range form {
					values[key] = append([]string(nil), entries...)
				}
				if got, want := values.Get("sign"), signZPayValues(values, "merchant-secret"); got != want {
					t.Fatalf("sign = %q, want %q", got, want)
				}
				return zpayJSONResponse(`{"code":1,"qrcode":"/pay/qr-code"}`), nil
			})}
			provider := NewZPayProvider(client, test.paymentType)
			if provider.Descriptor().ID != test.providerID {
				t.Fatalf("descriptor id = %q", provider.Descriptor().ID)
			}
			expiresAt := time.Now().Add(30 * time.Minute).Round(time.Second)
			checkout, err := provider.CreateOrder(context.Background(), zpayTestConfig(), CreateRequest{
				MerchantOrderNo: "0123456789abcdef0123456789abcdef", Description: "积分充值", AmountFen: 1234, Currency: "CNY",
				ExpiresAt: expiresAt, NotifyURL: "https://canvas.example.com/api/payments/notify/provider/config", ClientIP: "203.0.113.10",
			})
			if err != nil {
				t.Fatal(err)
			}
			if checkout.Mode != "qr_code" || checkout.Value != "https://pay.example.com/pay/qr-code" || !checkout.ExpiresAt.Equal(expiresAt) {
				t.Fatalf("checkout = %#v", checkout)
			}
		})
	}
}

func TestZPayCreateOrderCheckoutValuePriority(t *testing.T) {
	for _, test := range []struct {
		name     string
		response string
		want     string
		wantErr  bool
	}{
		{name: "qrcode", response: `{"code":"1","qrcode":"qr-value","payurl":"https://pay.example.com/primary"}`, want: "https://pay.example.com/qr-value"},
		{name: "payurl", response: `{"code":1,"payurl":"alipays://platformapi/startapp"}`, want: "alipays://platformapi/startapp"},
		{name: "payurl2", response: `{"code":1,"payurl2":"https://pay.example.com/h5"}`, want: "https://pay.example.com/h5"},
		{name: "image-only", response: `{"code":1,"img":"https://pay.example.com/code.jpg"}`, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := &http.Client{Transport: zpayRoundTripFunc(func(*http.Request) (*http.Response, error) {
				return zpayJSONResponse(test.response), nil
			})}
			provider := NewZPayProvider(client, "alipay")
			checkout, err := provider.CreateOrder(context.Background(), zpayTestConfig(), validZPayCreateRequest())
			if test.wantErr {
				if err == nil {
					t.Fatal("expected checkout value error")
				}
				return
			}
			if err != nil || checkout.Value != test.want {
				t.Fatalf("checkout = %#v, err = %v", checkout, err)
			}
		})
	}
}

func TestZPayQueryOrderUsesFormAndSupportsNestedResponse(t *testing.T) {
	client := &http.Client{Transport: zpayRoundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/api.php" {
			t.Fatalf("request = %s %s", request.Method, request.URL.RequestURI())
		}
		if err := request.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if request.Form.Get("act") != "order" || request.Form.Get("pid") != "merchant-1" || request.Form.Get("key") != "merchant-secret" || request.Form.Get("out_trade_no") != "order-1" {
			t.Fatalf("form = %#v", request.Form)
		}
		return zpayJSONResponse(`{"code":1,"data":{"status":1,"trade_no":"trade-1","money":"12.34","endtime":"2026-09-12 01:02:03"}}`), nil
	})}
	result, err := NewZPayProvider(client, "alipay").QueryOrder(context.Background(), zpayTestConfig(), QueryRequest{MerchantOrderNo: "order-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Paid || result.AmountFen != 1234 || result.ProviderTradeNo != "trade-1" || result.ProviderStatus != "TRADE_SUCCESS" || result.PaidAt.IsZero() {
		t.Fatalf("result = %#v", result)
	}
}

func TestZPayQueryOrderRejectsMismatchedReturnedIdentity(t *testing.T) {
	client := &http.Client{Transport: zpayRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return zpayJSONResponse(`{"code":1,"pid":"another-merchant","status":0,"type":"alipay","out_trade_no":"order-1"}`), nil
	})}
	_, err := NewZPayProvider(client, "alipay").QueryOrder(context.Background(), zpayTestConfig(), QueryRequest{MerchantOrderNo: "order-1"})
	if err == nil || !strings.Contains(err.Error(), "商户不匹配") {
		t.Fatalf("error = %v", err)
	}
}

func TestZPayVerifyNotificationChecksSignatureAndIdentity(t *testing.T) {
	values := url.Values{
		"pid": {"merchant-1"}, "type": {"wxpay"}, "out_trade_no": {"order-1"}, "trade_no": {"trade-1"},
		"trade_status": {"TRADE_SUCCESS"}, "money": {"12.34"}, "name": {"积分充值"}, "sign_type": {"MD5"},
	}
	values.Set("sign", signZPayValues(values, "merchant-secret"))
	provider := NewZPayProvider(nil, "wxpay")
	notification, err := provider.VerifyNotification(context.Background(), zpayTestConfig(), nil, []byte(values.Encode()))
	if err != nil {
		t.Fatal(err)
	}
	if notification.EventID != "trade-1:TRADE_SUCCESS" || !notification.Paid || notification.AmountFen != 1234 || notification.MerchantOrderNo != "order-1" {
		t.Fatalf("notification = %#v", notification)
	}
	values.Set("money", "99.00")
	if _, err := provider.VerifyNotification(context.Background(), zpayTestConfig(), nil, []byte(values.Encode())); err == nil {
		t.Fatal("tampered notification was accepted")
	}
}

func TestValidateZPayAPIBaseURLRejectsUnsafeTargets(t *testing.T) {
	for _, value := range []string{
		"http://pay.example.com", "https://127.0.0.1", "https://10.0.0.1", "https://[::1]",
		"https://pay.example.com/path", "https://user:pass@pay.example.com", "https://pay.example.com?token=x", "https://192.0.2.1",
	} {
		if err := validateZPayAPIBaseURL(value); err == nil {
			t.Errorf("validateZPayAPIBaseURL(%q) succeeded", value)
		}
	}
	if err := validateZPayAPIBaseURL("https://pay.example.com:8443/"); err != nil {
		t.Fatalf("public HTTPS root rejected: %v", err)
	}
}

func zpayTestConfig() Config {
	return Config{
		"apiBaseUrl": "https://pay.example.com", "pid": "merchant-1", "merchantKey": "merchant-secret", "cid": "channel-1,channel-2",
	}
}

func validZPayCreateRequest() CreateRequest {
	return CreateRequest{
		MerchantOrderNo: "0123456789abcdef0123456789abcdef", Description: "积分充值", AmountFen: 100, Currency: "CNY",
		ExpiresAt: time.Now().Add(30 * time.Minute), NotifyURL: "https://canvas.example.com/api/payments/notify/provider/config", ClientIP: "203.0.113.10",
	}
}

func firstMultipartValue(values map[string][]string, key string) string {
	if len(values[key]) == 0 {
		return ""
	}
	return values[key][0]
}

func zpayJSONResponse(body string) *http.Response {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
}
