package protocol

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func grsaiSVGAdapter(t *testing.T) Adapter {
	t.Helper()
	name := os.Getenv("GRSAI_SVG_TEST_PACKAGE")
	if name == "" {
		name = filepath.Join("..", "..", "..", "plugin-packages", "grsai-svg.yingce-plugin")
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := ParsePluginPackage(raw)
	if err != nil {
		t.Fatal(err)
	}
	adapters, err := LoadInstalledProviders(pkg.ManifestRaw, nil)
	if err != nil || len(adapters) != 1 {
		t.Fatalf("adapters=%v err=%v", adapters, err)
	}
	return adapters[0]
}

func TestGrsaiSVGRequest(t *testing.T) {
	a := grsaiSVGAdapter(t)
	if !a.Metadata().DetachedPolling || a.Metadata().ID != "grsai-svg-image" {
		t.Fatalf("wrong metadata: %+v", a.Metadata())
	}
	request := GenerationRequest{Capability: CapabilityImage, Model: "gpt-image-2.6", Prompt: "cat icon"}
	spec, err := a.BuildCreate(context.Background(), RequestContext{Request: request})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"model": "gpt-image-2.6", "prompt": "cat icon", "images": []any{}, "aspectRatio": "1024x1024", "quality": "auto", "outputFormat": "svg", "replyType": "async"}
	if !reflect.DeepEqual(manifestTestBody(t, spec), want) || spec.Path != "/v1/api/generate" || spec.Method != "POST" || !spec.OriginPath || spec.Auth.Type != "bearer" || spec.Auth.Field != "apiKey" {
		t.Fatalf("unexpected request: %+v", spec)
	}
	request.Images = []MediaReference{{URL: "https://example.com/ref.png", Order: 2}, {DataURL: "data:image/png;base64,cG5n", Order: 1}}
	request.ImageParameters = map[string]any{"size": "16:9", "count": 1, "quality": "auto", "outputFormat": "svg"}
	request.ProviderOptions = map[string]map[string]any{"grsai-svg-image": {"outputFormat": "png", "replyType": "stream", "model": "other"}}
	spec, err = a.BuildCreate(context.Background(), RequestContext{Request: request})
	if err != nil {
		t.Fatal(err)
	}
	body := manifestTestBody(t, spec)
	if body["aspectRatio"] != "16:9" || body["outputFormat"] != "svg" || body["replyType"] != "async" || body["model"] != "gpt-image-2.6" || !reflect.DeepEqual(body["images"], []any{"data:image/png;base64,cG5n", "https://example.com/ref.png"}) {
		t.Fatalf("reference/options mapping: %#v", body)
	}
	for _, id := range []string{"upstream-task-123", "task?with&query"} {
		spec, err = a.BuildPoll(context.Background(), PollContext{TaskID: id, Model: request.Model})
		if err != nil || spec.Path != "/v1/api/result" || spec.Method != "GET" || !spec.OriginPath || !reflect.DeepEqual(spec.Query["id"], []string{id}) || spec.Body != nil {
			t.Fatalf("poll=%+v err=%v", spec, err)
		}
	}
}

func TestGrsaiSVGRejectsUnsupportedRequests(t *testing.T) {
	a := grsaiSVGAdapter(t)
	for _, request := range []GenerationRequest{
		{Model: "gpt-image-2.6-flare"}, {Model: "gpt-image-2.6-sunburst"}, {Model: "gpt_image-2.6"},
		{ImageCount: 2}, {Output: OutputOptions{Count: 2}}, {Quality: "high"}, {AspectRatio: "2048x2048"},
		{ImageParameters: map[string]any{"outputFormat": "png"}},
		{ImageParameters: map[string]any{"count": 2}},
		{ImageParameters: map[string]any{"quality": "high"}},
		{ImageParameters: map[string]any{"size": "4096x4096"}},
		{Images: []MediaReference{{URL: "https://example.com/mask.png", Role: "mask"}}},
		{Images: []MediaReference{{ID: "missing-address"}}},
	} {
		request.Capability, request.Prompt = CapabilityImage, "cat"
		if request.Model == "" {
			request.Model = "gpt-image-2.6"
		}
		if _, err := a.BuildCreate(context.Background(), RequestContext{Request: request}); err == nil {
			t.Fatalf("unsupported request accepted: %+v", request)
		}
	}
}

func TestGrsaiSVGResponse(t *testing.T) {
	a := grsaiSVGAdapter(t)
	for _, tc := range []struct {
		body    string
		status  Status
		message string
	}{
		{`{"id":"task-1","status":"queued"}`, StatusPending, ""},
		{`{"id":"task-1","status":"running"}`, StatusProcessing, ""},
		{`{"id":"task-1","status":"succeeded","results":[{"url":"https://example.com/result.svg"}]}`, StatusSucceeded, ""},
		{`{"id":"task-1","status":"failed","error":"generate failed"}`, StatusFailed, "generate failed"},
		{`{"id":"task-1","status":"violation","error":"blocked"}`, StatusFailed, "blocked"},
		{`{"error":"invalid key"}`, StatusFailed, "invalid key"},
	} {
		created, err := a.ParseCreate(context.Background(), []byte(tc.body))
		if err != nil || created.Status != tc.status || created.Message != tc.message {
			t.Fatalf("create=%+v err=%v", created, err)
		}
		polled, err := a.ParsePoll(context.Background(), PollContext{TaskID: "task-1"}, []byte(tc.body))
		if err != nil || polled.TaskID != "task-1" || polled.Status != tc.status || polled.Message != tc.message {
			t.Fatalf("poll=%+v err=%v", polled, err)
		}
		if tc.status == StatusSucceeded {
			if polled.Result == nil || len(polled.Result.Images) != 1 || polled.Result.Images[0].MIMEType != "image/svg+xml" || polled.Result.Images[0].URL != "https://example.com/result.svg" {
				t.Fatalf("SVG result lost: %+v", polled)
			}
		}
	}
	if _, err := a.ParseCreate(context.Background(), []byte("not JSON")); err == nil {
		t.Fatal("invalid response accepted")
	}
}
