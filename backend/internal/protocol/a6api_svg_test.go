package protocol

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func a6apiSVGAdapter(t *testing.T) Adapter {
	t.Helper()
	var raw []byte
	var err error
	if name := os.Getenv("A6API_SVG_TEST_PACKAGE"); name != "" {
		data, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		pkg, parseErr := ParsePluginPackage(data)
		if parseErr != nil {
			t.Fatal(parseErr)
		}
		raw = pkg.ManifestRaw
	} else {
		raw, err = os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "a6api-svg", "manifest.json"))
		if err != nil {
			t.Fatal(err)
		}
	}
	adapters, err := LoadInstalledProviders(raw, nil)
	if err != nil || len(adapters) != 1 {
		t.Fatalf("adapters=%v err=%v", adapters, err)
	}
	return adapters[0]
}

func TestA6APISVGRequestAndResponse(t *testing.T) {
	a := a6apiSVGAdapter(t)
	spec, err := a.BuildCreate(context.Background(), RequestContext{Request: GenerationRequest{
		Capability: CapabilityImage, Model: "gpt-image-2.6", Prompt: "cat icon",
		ImageParameters: map[string]any{"size": "1024x1024", "quality": "high", "count": 1, "outputFormat": "svg"},
		ProviderOptions: map[string]map[string]any{"a6api-svg-image": {"output_format": "png", "stream": true}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	body := manifestTestBody(t, spec)
	if spec.Path != "/images/generations" || body["output_format"] != "svg" || body["n"] != float64(1) || body["quality"] != "high" || body["size"] != "1024x1024" {
		t.Fatalf("wrong SVG request: %#v", spec)
	}
	for _, field := range []string{"stream", "background", "response_format", "output_compression", "partial_images"} {
		if _, ok := body[field]; ok {
			t.Fatalf("unsupported field sent: %s", field)
		}
	}
	encoded := base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><path d="M0 0h16v16z"/></svg>`))
	for _, item := range []map[string]string{{"b64_json": encoded}, {"url": "https://example.com/result.svg"}} {
		payload, _ := json.Marshal(map[string]any{"data": []any{item}})
		result, err := a.ParseCreate(context.Background(), payload)
		if err != nil || result.Result == nil || len(result.Result.Images) != 1 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		media := result.Result.Images[0]
		if media.MIMEType != "image/svg+xml" {
			t.Fatalf("wrong MIME: %+v", media)
		}
		if item["b64_json"] != "" && media.DataURL != "data:image/svg+xml;base64,"+encoded {
			t.Fatalf("SVG bytes changed: %+v", media)
		}
		if item["url"] != "" && media.URL != item["url"] {
			t.Fatalf("URL changed: %+v", media)
		}
	}
}

func TestA6APISVGRejectsUnsupportedRequests(t *testing.T) {
	a := a6apiSVGAdapter(t)
	for _, request := range []GenerationRequest{
		{Images: []MediaReference{{URL: "https://example.com/reference.png"}}},
		{ImageCount: 2},
		{ImageParameters: map[string]any{"count": 2}},
		{ImageParameters: map[string]any{"outputFormat": "png"}},
	} {
		request.Capability = CapabilityImage
		request.Model, request.Prompt = "gpt-image-2.6", "cat"
		if _, err := a.BuildCreate(context.Background(), RequestContext{Request: request}); err == nil {
			t.Fatalf("invalid request accepted: %+v", request)
		}
	}
	result, err := a.ParseCreate(context.Background(), []byte(`{"error":{"code":"unsupported_format","message":"SVG not supported"}}`))
	if err == nil && result.Status != StatusFailed {
		t.Fatalf("upstream error hidden: %+v", result)
	}
}
