package protocol

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestParsePluginPackageRejectsBareManifest(t *testing.T) {
	if _, err := ParsePluginPackage([]byte(`{"apiVersion":"open-ai-canvas.plugin/v1"}`)); err == nil {
		t.Fatal("bare JSON manifest was accepted as a plugin package")
	}
}

func TestParsePluginPackageAcceptsCompatibleBrandedV2(t *testing.T) {
	manifest := []byte(`{
		"apiVersion":"yingce.plugin/v2",
		"id":"compat-image",
		"name":"Compatible Image",
		"version":"1.0.0",
		"contributes":{"providers":[{"id":"compat-image","label":"Compatible Image","capabilities":["image"],"scopes":["canvas"],"create":{"method":"POST","path":"/v1/models/{{model}}"},"response":{"status":"succeeded","images":{"$ref":"response.url"}}}]}
	}`)
	pkg, err := ParsePluginPackage(zipPluginPackage(t, map[string][]byte{"manifest.json": manifest, "README.md": []byte("# Compatible Image"), "docs/interface.md": []byte("## 响应")}))
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.APIVersion != "open-ai-canvas.plugin/v2" || PluginPackageExtensionForAPIVersion(pkg.Manifest.APIVersion) != ".canvas-plugin" || PluginPackageExtensionForAPIVersion("yingce.plugin/v2") != ".yingce-plugin" {
		t.Fatalf("package = %#v", pkg.Manifest)
	}
	if !IsPluginPackageFileName("compat-image.yingce-plugin") || !IsPluginPackageFileName("lovwow-image.lovwow-plugin") || !IsPluginPackageFileName("neutral.canvas-plugin") || IsPluginPackageFileName("manifest.json") {
		t.Fatal("plugin package extension recognition is inconsistent")
	}
}

func TestCompatibleManifestEscapesModelPathSegments(t *testing.T) {
	request, err := buildManifestOperation(ManifestOperation{Method: "POST", Path: "/v1/models/{{model}}", ContentType: "application/json"}, ManifestAuth{}, GenerationRequest{Model: "vendor/model name+v1"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if request.Path != "/v1/models/vendor/model%20name+v1" {
		t.Fatalf("escaped path = %q", request.Path)
	}
}

func TestParsePluginPackageValidatesWebEntry(t *testing.T) {
	manifest := []byte(`{
        "apiVersion":"open-ai-canvas.plugin/v1",
        "id":"ui-extension",
        "name":"UI Extension",
        "version":"1.0.0",
        "entry":"web/entry.js",
        "surfaces":["fullscreen"],
        "runtime":{"web":"sandbox"},
        "contributes":{"commands":[{"id":"ui-extension/open","label":"Open UI"}]}
    }`)
	pkg, err := ParsePluginPackage(zipPluginPackage(t, map[string][]byte{"manifest.json": manifest, "web/entry.js": []byte("self.postMessage({type:'ready'})")}))
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.Entry != "web/entry.js" || len(pkg.Files["web/entry.js"]) == 0 {
		t.Fatalf("package = %#v", pkg)
	}

	missingEntry := zipPluginPackage(t, map[string][]byte{"manifest.json": manifest})
	if _, err := ParsePluginPackage(missingEntry); err == nil {
		t.Fatal("missing web entry was accepted")
	}

	forbidden := zipPluginPackage(t, map[string][]byte{"manifest.json": manifest, "web/entry.js": []byte("ready"), "server/entry.js": []byte("unsafe")})
	if _, err := ParsePluginPackage(forbidden); err == nil {
		t.Fatal("forbidden package path was accepted")
	}
}

func TestParsePluginPackageAcceptsTaggedRPCBackend(t *testing.T) {
	manifest := []byte(`{
        "apiVersion":"open-ai-canvas.plugin/v1",
        "id":"tagged-payment",
        "name":"Tagged Payment",
        "version":"1.0.0",
        "author":"Test",
        "enabled":true,
        "runtime":{"backend":"rpc","backendEntry":"backend/provider"},
        "contributes":{"paymentProviders":[{"id":"tagged-pay","label":"Tagged","icon":"brand:test","checkoutMode":"qr_code","expiryPolicy":{"defaultMinutes":30,"minMinutes":5,"maxMinutes":1440}}]}
    }`)
	pkg, err := ParsePluginPackage(zipPluginPackage(t, map[string][]byte{
		"manifest.json":                 manifest,
		"backend/provider-darwin-arm64": []byte("darwin-provider"),
	}))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := pkg.Files["backend/provider"]; ok {
		t.Fatal("canonical backend/provider should not be required when a tagged artifact exists")
	}

	if _, err := ParsePluginPackage(zipPluginPackage(t, map[string][]byte{"manifest.json": manifest})); err == nil {
		t.Fatal("rpc package without any backend artifact was accepted")
	}
}

func TestParsePluginPackageNormalizesEarlierProtocolNamespace(t *testing.T) {
	earlierNamespace := string([]byte{121, 105, 110, 103, 99, 101})
	manifest := []byte(fmt.Sprintf(`{
        "apiVersion":%q,
        "id":"compatible-extension",
        "name":"Compatible Extension",
        "version":"1.0.0",
        "contributes":{"commands":[{"id":"compatible-extension/open","label":"Open"}]}
    }`, earlierNamespace+".plugin/v1"))
	pkg, err := ParsePluginPackage(zipPluginPackage(t, map[string][]byte{"manifest.json": manifest}))
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Manifest.APIVersion != "open-ai-canvas.plugin/v1" || pkg.RPCVersion != earlierNamespace+".payment/v1" {
		t.Fatalf("normalized package = %#v", pkg)
	}
	var stored Manifest
	if err := json.Unmarshal(pkg.ManifestRaw, &stored); err != nil {
		t.Fatal(err)
	}
	if stored.APIVersion != "open-ai-canvas.plugin/v1" || strings.Contains(strings.ToLower(string(pkg.ManifestRaw)), earlierNamespace) {
		t.Fatalf("normalized manifest = %s", pkg.ManifestRaw)
	}
}

func TestParsePluginPackageRejectsUnrelatedProtocolNamespace(t *testing.T) {
	manifest := []byte(`{
        "apiVersion":"unrelated.plugin/v1",
        "id":"unrelated-extension",
        "name":"Unrelated Extension",
        "version":"1.0.0",
        "contributes":{"commands":[{"id":"unrelated-extension/open","label":"Open"}]}
    }`)
	if _, err := ParsePluginPackage(zipPluginPackage(t, map[string][]byte{"manifest.json": manifest})); err == nil {
		t.Fatal("unrelated protocol namespace was accepted")
	}
}

func zipPluginPackage(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, data := range files {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
