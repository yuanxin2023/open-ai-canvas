package app

import (
	"archive/zip"
	"bytes"
	"context"
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

const testStaticSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 240 120"><path d="M0 0h240v120z" fill="#f00"/></svg>`

func TestA6APISVGInstallAndRestart(t *testing.T) {
	var data []byte
	var err error
	if name := os.Getenv("A6API_SVG_TEST_PACKAGE"); name != "" {
		data, err = os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var buffer bytes.Buffer
		archive := zip.NewWriter(&buffer)
		for _, name := range []string{"manifest.json", "README.md", "docs/interface.md"} {
			content, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "a6api-svg", filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			entry, err := archive.Create(name)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := entry.Write(content); err != nil {
				t.Fatal(err)
			}
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		data = buffer.Bytes()
	}
	dir := t.TempDir()
	runtime, err := newPluginRuntime(dir)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		installed, err := runtime.install(data, "a6api-svg-1.0.0.yingce-plugin")
		if err != nil || installed.Manifest.ID != "a6api-svg" {
			t.Fatalf("install=%+v err=%v", installed, err)
		}
	}
	runtime, err = newPluginRuntime(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a6api-image", "a6api-chat", "a6api-svg-image"} {
		if _, ok := runtime.registrySnapshot().Resolve(id); !ok {
			t.Fatalf("missing provider after restart: %s", id)
		}
	}
}

func TestA6APISVGMediaRecoveryPreservesOriginal(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, remote := range []bool{false, true} {
		t.Run(map[bool]string{false: "base64", true: "url"}[remote], func(t *testing.T) {
			s, db := newMediaRecoveryTestService(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", assets.SVGMIME)
				_, _ = io.WriteString(w, testStaticSVG)
			}))
			defer server.Close()
			config := providerConfig{BaseURL: server.URL}
			task := seedMediaTask(t, db, config)
			reference := protocol.MediaReference{MIMEType: assets.SVGMIME, DataURL: dataURL(assets.SVGMIME, []byte(testStaticSVG))}
			if remote {
				reference.DataURL, reference.URL = "", server.URL+"/result"
			}
			ctx := context.WithValue(withProviderAnalytics(context.Background(), s, *task), mediaExecutionTaskKey{}, *task)
			output, handled, err := recoverProtocolMedia(ctx, config, "image", []protocol.MediaReference{reference})
			if !handled || err != nil {
				t.Fatalf("handled=%v err=%v", handled, err)
			}
			item := output["images"].([]interface{})[0].(map[string]interface{})
			resource, body, err := s.OpenResource(task.UserID, item["resourceId"].(string))
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			got, err := io.ReadAll(body)
			if err != nil || string(got) != testStaticSVG || resource.Width != 240 || resource.Height != 120 || resource.MimeType != assets.SVGMIME || !strings.HasSuffix(resource.ObjectKey, ".svg") {
				t.Fatalf("original lost: resource=%+v body=%s err=%v", resource, got, err)
			}
			if _, _, err := s.OpenResource("other-user", resource.ID); err == nil {
				t.Fatal("SVG bypassed ownership")
			}
			if assets.InlineMediaType(resource.MimeType) {
				t.Fatal("SVG attachment boundary changed")
			}
		})
	}
}

func TestA6APISVGRejectsRasterAndActiveUpstream(t *testing.T) {
	t.Setenv("CANVAS_ALLOWED_PRIVATE_UPSTREAM_HOSTS", "127.0.0.1")
	for _, tc := range []struct{ name, mime, body string }{
		{"png", "image/png", string(mediaTestPNG(t))},
		{"spoofed-png", assets.SVGMIME, string(mediaTestPNG(t))},
		{"active-svg", assets.SVGMIME, `<svg xmlns="http://www.w3.org/2000/svg" width="10" height="10"><script>alert(1)</script></svg>`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, db := newMediaRecoveryTestService(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tc.mime)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			config := providerConfig{BaseURL: server.URL}
			task := seedMediaTask(t, db, config)
			ctx := context.WithValue(withProviderAnalytics(context.Background(), s, *task), mediaExecutionTaskKey{}, *task)
			reference := protocol.MediaReference{URL: server.URL, MIMEType: assets.SVGMIME}
			if _, _, err := recoverProtocolMedia(ctx, config, "image", []protocol.MediaReference{reference}); err == nil {
				t.Fatal("invalid SVG saved")
			}
			if _, _, err := protocolMediaBytesOnce(context.Background(), config, reference); err == nil {
				t.Fatal("invalid SVG returned through non-worker path")
			}
			var count int64
			if err := db.Model(&model.Resource{}).Count(&count).Error; err != nil || count != 0 {
				t.Fatalf("invalid resources=%d err=%v", count, err)
			}
		})
	}
}

func TestA6APISVGLegacyPersistence(t *testing.T) {
	s := newResourceTestService(t)
	result, err := s.persistGeneratedMediaResult("user", map[string]interface{}{"images": []any{map[string]interface{}{"dataUrl": dataURL(assets.SVGMIME, []byte(testStaticSVG))}}})
	if err != nil {
		t.Fatal(err)
	}
	image := result["images"].([]interface{})[0].(map[string]interface{})
	if image["mimeType"] != assets.SVGMIME || image["width"] != 240 || image["height"] != 120 {
		t.Fatalf("SVG metadata lost: %+v", image)
	}
}
