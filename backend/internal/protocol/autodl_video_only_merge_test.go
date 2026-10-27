package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAutoDLCanonicalPackageStopsLegacyAudioWithoutChangingVideoIdentity(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", "autodl-comfyui.canvas-plugin"))
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := ParsePluginPackage(data)
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.Metadata.ID != "autodl-comfyui" || pkg.Manifest.Metadata.Version != "2.3.0" {
		t.Fatal("AutoDL identity or reviewed version changed")
	}
	adapters, err := LoadInstalledProviders(pkg.ManifestRaw, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := NewRegistry(adapters...)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := registry.Resolve("autodl-comfyui"); !ok {
		t.Fatal("existing video provider missing")
	}
	if _, ok := registry.Resolve("autodl-comfyui-audio"); ok {
		t.Fatal("retired audio provider is still executable")
	}
	for _, workflow := range pkg.Manifest.Contributes.Workflows {
		if workflow.ID == "indextts2-v1" {
			t.Fatal("retired IndexTTS2 workflow is still distributed")
		}
	}
}
