package paymentplugins

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

type rpcDispatchProvider struct {
	descriptor Descriptor
	called     *bool
}

func (provider rpcDispatchProvider) Descriptor() Descriptor { return provider.descriptor }
func (provider rpcDispatchProvider) ValidateConfig(Config) error {
	*provider.called = true
	return nil
}
func (rpcDispatchProvider) CreateOrder(context.Context, Config, CreateRequest) (Checkout, error) {
	return Checkout{}, nil
}
func (rpcDispatchProvider) QueryOrder(context.Context, Config, QueryRequest) (Result, error) {
	return Result{}, nil
}
func (rpcDispatchProvider) CloseOrder(context.Context, Config, CloseRequest) (Result, error) {
	return Result{}, nil
}
func (rpcDispatchProvider) VerifyNotification(context.Context, Config, http.Header, []byte) (Notification, error) {
	return Notification{}, nil
}
func (rpcDispatchProvider) DownloadTradeBill(context.Context, Config, time.Time) ([]BillRecord, error) {
	return nil, nil
}

func TestRunRPCProvidersDispatchesProviderID(t *testing.T) {
	firstCalled, secondCalled := false, false
	providers := map[string]Provider{
		"first":  rpcDispatchProvider{descriptor: Descriptor{ID: "first"}, called: &firstCalled},
		"second": rpcDispatchProvider{descriptor: Descriptor{ID: "second"}, called: &secondCalled},
	}
	var output bytes.Buffer
	input := strings.NewReader(`{"version":"yingce.payment/v1","providerId":"second","operation":"validate_config"}`)
	if err := RunRPCProviders(providers, input, &output); err != nil {
		t.Fatal(err)
	}
	if firstCalled || !secondCalled || !strings.Contains(output.String(), `"ok":true`) {
		t.Fatalf("dispatch = first:%v second:%v output:%s", firstCalled, secondCalled, output.String())
	}
}

func TestRunRPCProvidersRejectsUnknownProvider(t *testing.T) {
	var output bytes.Buffer
	input := strings.NewReader(`{"version":"yingce.payment/v1","providerId":"missing","operation":"validate_config"}`)
	if err := RunRPCProviders(map[string]Provider{}, input, &output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), `"code":"provider_unavailable"`) {
		t.Fatalf("output = %s", output.String())
	}
}
