package app

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"infinite-canvas/backend/internal/protocol"
)

func TestA6APITransparentPNGUploadCoexistsWithBuiltinAndSurvivesRestart(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	var data []byte
	var err error
	if packagePath := os.Getenv("A6API_TEST_PACKAGE"); packagePath != "" {
		data, err = os.ReadFile(packagePath)
		if err != nil {
			t.Fatal(err)
		}
	} else {
		var buffer bytes.Buffer
		archive := zip.NewWriter(&buffer)
		for _, name := range []string{"manifest.json", "README.md", "docs/interface.md"} {
			content, readErr := os.ReadFile(filepath.Join(root, "plugin-packages", "a6api-transparent-png", filepath.FromSlash(name)))
			if readErr != nil {
				t.Fatal(readErr)
			}
			entry, createErr := archive.Create(name)
			if createErr != nil {
				t.Fatal(createErr)
			}
			if _, writeErr := entry.Write(content); writeErr != nil {
				t.Fatal(writeErr)
			}
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		data = buffer.Bytes()
	}
	dataDir := t.TempDir()
	center, err := newPluginRuntime(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(filepath.Join(root, "plugin-packages", "a6api.yingce-plugin"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := center.install(original, "a6api.yingce-plugin"); err == nil || !strings.Contains(err.Error(), "不能通过上传覆盖") {
		t.Fatalf("built-in overwrite rejection was not reproduced: %v", err)
	}
	for _, operation := range []string{"install", "update"} {
		t.Run(operation, func(t *testing.T) {
			installed, err := center.install(data, "a6api-transparent-png-1.1.1.yingce-plugin")
			if err != nil {
				t.Fatal(err)
			}
			if installed.Manifest.ID != "a6api-transparent-png" || installed.Source != "uploaded" || installed.Status != "enabled" {
				t.Fatalf("unexpected installed plugin: %#v", installed)
			}
		})
	}
	center, err = newPluginRuntime(dataDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a6api-chat", "a6api-image", "a6api-transparent-image"} {
		if _, ok := center.registrySnapshot().Resolve(id); !ok {
			t.Fatalf("provider %s missing after installation and restart", id)
		}
	}
	adapter, _ := center.registrySnapshot().Resolve("a6api-transparent-image")
	spec, err := adapter.BuildCreate(context.Background(), protocol.RequestContext{Request: protocol.GenerationRequest{
		Capability: protocol.CapabilityImage, Model: "gpt-image-2", Prompt: "isolated subject",
		ImageParameters: map[string]any{"transparentBackground": true, "outputFormat": "png", "count": 1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	body, ok := spec.Body.(map[string]any)
	if !ok || body["background"] != "transparent" || body["output_format"] != "png" {
		t.Fatalf("installed adapter lost transparent PNG mapping: %#v", spec.Body)
	}
}
