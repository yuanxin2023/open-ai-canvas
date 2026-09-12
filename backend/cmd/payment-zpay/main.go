package main

import (
	"os"

	paymentplugins "infinite-canvas/backend/payment-plugins"
)

func main() {
	client := paymentplugins.NewZPayHTTPClient()
	providers := map[string]paymentplugins.Provider{
		paymentplugins.ProviderZPayAlipay: paymentplugins.NewZPayProvider(client, "alipay"),
		paymentplugins.ProviderZPayWeChat: paymentplugins.NewZPayProvider(client, "wxpay"),
	}
	if err := paymentplugins.RunRPCProviders(providers, os.Stdin, os.Stdout); err != nil {
		os.Exit(1)
	}
}
