package protocol

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewOptionalPackagesDoNotEnableBusinessProtocolsByDefault(t *testing.T) {
	for _, id := range []string{"cangyuan-midjourney", "cheaperinference-chat", "dashscope-minimax-video", "dola-pool", "doubao-streaming-tts", "kacang-midjourney", "km-kemei-seedream", "km-kemei-video", "maxforai-video", "qianwenai-maas-image", "volcengine-mediakit"} {
		t.Run(id, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("..", "..", "..", "plugin-packages", id+".canvas-plugin"))
			if err != nil {
				t.Fatal(err)
			}
			pkg, err := ParsePluginPackage(data)
			if err != nil {
				t.Fatal(err)
			}
			if pkg.Manifest.Metadata.ID != id || pkg.Manifest.Metadata.Enabled {
				t.Fatal("optional plugin identity or disabled default changed")
			}
			adapters, err := LoadInstalledProviders(pkg.ManifestRaw, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(adapters) == 0 {
				t.Fatal("optional package is not executable after explicit activation")
			}
			for _, adapter := range adapters {
				if adapter.Metadata().Enabled {
					t.Fatal("provider bypassed disabled plugin default")
				}
			}
		})
	}
}
