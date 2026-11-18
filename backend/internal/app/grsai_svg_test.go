package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/assets"
	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/protocol"
)

const grsaiTestSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 120"><path d="M0 0h240v120z" fill="#f00"/></svg>`

func TestGeneratedSVGInlinePreservesBytes(t *testing.T) {
	inline := dataURL(assets.SVGMIME, []byte(grsaiTestSVG))
	reference := protocol.MediaReference{DataURL: inline, MIMEType: assets.SVGMIME}
	data, mimeType, err := protocolMediaBytesOnce(context.Background(), providerConfig{}, reference)
	if err != nil || mimeType != assets.SVGMIME || string(data) != grsaiTestSVG {
		t.Fatalf("SVG download changed: mime=%s err=%v", mimeType, err)
	}
	s := newResourceTestService(t)
	result, err := s.persistGeneratedMediaResult("user", map[string]interface{}{"images": []any{map[string]interface{}{"dataUrl": inline}}})
	if err != nil {
		t.Fatal(err)
	}
	item := result["images"].([]interface{})[0].(map[string]interface{})
	if item["mimeType"] != assets.SVGMIME || item["width"] != 240 || item["height"] != 120 {
		t.Fatalf("SVG metadata lost: %+v", item)
	}
	reference.DataURL = dataURL("image/png", mediaTestPNG(t))
	if _, _, err := protocolMediaBytesOnce(context.Background(), providerConfig{}, reference); err == nil {
		t.Fatal("raster response accepted as SVG")
	}
}

func TestGrsaiSVGPackageInstallAndMedia(t *testing.T) {
	name := os.Getenv("GRSAI_SVG_TEST_PACKAGE")
	if name == "" {
		name = filepath.Join("..", "..", "..", "plugin-packages", "grsai-svg.yingce-plugin")
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	// 模拟尚未内置此插件的已部署宿主，验证真实上传流程。
	officialDir := t.TempDir()
	for _, id := range []string{"a6api", "openai-images"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", id+".yingce-plugin"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(officialDir, id+".yingce-plugin"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CANVAS_OFFICIAL_PLUGIN_DIR", officialDir)
	dir := t.TempDir()
	runtime, err := newPluginRuntime(dir)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		installed, err := runtime.install(raw, "grsai-svg-1.0.0.yingce-plugin")
		if err != nil || installed.Manifest.ID != "grsai-svg" || installed.Source != "uploaded" || installed.Status != "enabled" {
			t.Fatalf("install=%+v err=%v", installed, err)
		}
	}
	runtime, err = newPluginRuntime(dir)
	if err != nil {
		t.Fatal(err)
	}
	a, ok := runtime.registrySnapshot().Resolve("grsai-svg-image")
	if !ok {
		t.Fatal("GrsAI SVG provider missing after restart")
	}
	if _, ok := runtime.registrySnapshot().Resolve("a6api-image"); !ok {
		t.Fatal("existing provider missing after install")
	}
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, tc := range []struct {
		name, mime, body string
		valid            bool
	}{
		{"svg", assets.SVGMIME, grsaiTestSVG, true},
		{"png", "image/png", string(mediaTestPNG(t)), false},
		{"spoofed-png", assets.SVGMIME, string(mediaTestPNG(t)), false},
		{"active-svg", assets.SVGMIME, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><script>alert(1)</script></svg>`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, db := newMediaRecoveryTestService(t)
			creates, polls, downloads := 0, 0, 0
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v1/api/generate":
					creates++
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
					}
					if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-grsai-key" || body["outputFormat"] != "svg" || body["replyType"] != "async" || body["model"] != "gpt-image-2.6" {
						t.Errorf("unexpected create: %s %#v", r.Method, body)
					}
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, `{"id":"grsai-task-1","status":"queued"}`)
				case "/v1/api/result":
					polls++
					if r.Method != "GET" || r.URL.Query().Get("id") != "grsai-task-1" || r.Header.Get("Authorization") != "Bearer test-grsai-key" {
						t.Errorf("unexpected poll: %s %s", r.Method, r.URL)
					}
					w.Header().Set("Content-Type", "application/json")
					json.NewEncoder(w).Encode(map[string]any{"id": "grsai-task-1", "status": "succeeded", "results": []any{map[string]any{"url": server.URL + "/image.svg"}}})
				case "/image.svg":
					downloads++
					w.Header().Set("Content-Type", tc.mime)
					io.WriteString(w, tc.body)
				default:
					t.Errorf("unexpected URL: %s", r.URL)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			config := providerConfig{BaseURL: server.URL + "/v1", APIKey: "test-grsai-key", Model: "gpt-image-2.6", InterfaceType: "grsai-svg-image"}
			request := protocol.GenerationRequest{Capability: protocol.CapabilityImage, Model: config.Model, Prompt: "cat icon"}
			spec, err := a.BuildCreate(context.Background(), protocol.RequestContext{Request: request})
			if err != nil {
				t.Fatal(err)
			}
			body, err := executeProtocolRequest(context.Background(), config, spec)
			if err != nil {
				t.Fatal(err)
			}
			created, err := a.ParseCreate(context.Background(), body)
			if err != nil || created.TaskID != "grsai-task-1" || created.Status != protocol.StatusPending {
				t.Fatalf("create=%+v err=%v", created, err)
			}
			poll := protocol.PollContext{TaskID: created.TaskID, Request: request, Model: config.Model}
			spec, err = a.BuildPoll(context.Background(), poll)
			if err != nil {
				t.Fatal(err)
			}
			body, err = executeProtocolRequest(context.Background(), config, spec)
			if err != nil {
				t.Fatal(err)
			}
			result, err := a.ParsePoll(context.Background(), poll, body)
			if err != nil || result.Result == nil || len(result.Result.Images) != 1 {
				t.Fatalf("poll=%+v err=%v", result, err)
			}
			task := seedMediaTask(t, db, config)
			ctx := context.WithValue(withProviderAnalytics(context.Background(), s, *task), mediaExecutionTaskKey{}, *task)
			output, handled, err := recoverProtocolMedia(ctx, config, "image", result.Result.Images)
			if !handled {
				t.Fatal("media recovery not handled")
			}
			if !tc.valid {
				if err == nil {
					t.Fatal("invalid SVG accepted")
				}
				var count int64
				if err := db.Model(&model.Resource{}).Count(&count).Error; err != nil || count != 0 {
					t.Fatalf("invalid resource saved: count=%d err=%v", count, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				item := output["images"].([]interface{})[0].(map[string]interface{})
				resource, stream, err := s.OpenResource(task.UserID, item["resourceId"].(string))
				if err != nil {
					t.Fatal(err)
				}
				defer stream.Close()
				data, err := io.ReadAll(stream)
				if err != nil || string(data) != tc.body || resource.MimeType != assets.SVGMIME || resource.Width != 240 || resource.Height != 120 || !strings.HasSuffix(resource.ObjectKey, ".svg") {
					t.Fatalf("SVG changed: resource=%+v err=%v", resource, err)
				}
				if _, _, err := s.OpenResource("another-user", resource.ID); err == nil {
					t.Fatal("resource ownership bypassed")
				}
			}
			if creates != 1 || polls != 1 || downloads != 1 {
				t.Fatalf("unexpected repeated request: creates=%d polls=%d downloads=%d", creates, polls, downloads)
			}
		})
	}
}
