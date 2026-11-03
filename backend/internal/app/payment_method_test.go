package app

import (
	"testing"

	"infinite-canvas/backend/internal/payment"
)

func TestCustomerPaymentMethod(t *testing.T) {
	for _, tc := range []struct{ provider, payType, want string }{
		{PaymentProviderEpay, "", "alipay"},
		{PaymentProviderEpay, "wxpay", "wechat"},
		{PaymentProviderEpay, "alipay", "alipay"},
		{PaymentProviderEpay, "qqpay", "online"},
		{PaymentProviderEpay, "usdt", "online"},
		{PaymentProviderZhiFuFM, "wxpay", "wechat"},
		{PaymentProviderWeChat, "", "wechat"},
		{PaymentProviderZPayAlipay, "", "alipay"},
		{"unknown", "wxpay", "online"},
	} {
		if got := customerPaymentMethod(tc.provider, payment.Config{"payType": tc.payType}); got != tc.want {
			t.Errorf("%s/%s: got %s, want %s", tc.provider, tc.payType, got, tc.want)
		}
	}
}
