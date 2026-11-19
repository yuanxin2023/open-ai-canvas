package assets

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAdminMediaCleanup(t *testing.T) {
	deleted := map[string]struct{}{"gone": {}}
	cases := []struct {
		name, raw string
		nodes     int
		had, has  bool
	}{
		{"last media with text", `{"nodes":[{"id":"i","type":"image","metadata":{"storageKey":"resource:gone","content":"/api/resources/gone/file"}},{"id":"t","type":"text","metadata":{"content":"保留的提示词"}}],"connections":[]}`, 1, true, false},
		{"text URL is not media", `{"nodes":[{"id":"t","type":"text","metadata":{"content":"https://example.com/article"}}]}`, 0, false, false},
		{"surviving external media", `{"nodes":[{"id":"i","type":"image","metadata":{"storageKey":"resource:gone"}},{"id":"v","type":"video","metadata":{"content":"https://example.com/a.mp4"}}]}`, 1, true, true},
		{"cover does not count", `{"coverUrl":"resource:gone","nodes":[{"id":"t","type":"text"}]}`, 0, false, false},
		{"chat does not count", `{"nodes":[{"id":"i","type":"image","metadata":{"storageKey":"resource:gone"}}],"chatSessions":[{"url":"https://example.com/old.png"}]}`, 1, true, false},
		{"timeline counts", `{"nodes":[{"id":"i","type":"image","metadata":{"storageKey":"resource:gone"}}],"timeline":{"clips":[{"id":"c","resourceId":"keep"}]}}`, 1, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cleaned, err := CleanMediaDocument(tc.raw, deleted, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(cleaned.RemovedNodeIDs) != tc.nodes || cleaned.HadMedia != tc.had || cleaned.HasMedia != tc.has {
				t.Fatalf("result = %#v", cleaned)
			}
		})
	}
}

func TestAdminMediaCleanupPromotesVersionAndPrunesReferences(t *testing.T) {
	raw := `{"nodes":[{"id":"i","type":"image","metadata":{"storageKey":"resource:gone","assetId":"old-asset","mediaVersions":[{"id":"v1","number":1,"media":{"storageKey":"resource:keep","content":"/api/resources/keep/file"}},{"id":"v2","number":2,"media":{"storageKey":"resource:gone"}}]}},{"id":"d","type":"audio","metadata":{"storageKey":"resource:gone"}},{"id":"group","type":"frame","metadata":{"nodeIds":["i","d"]}}],"connections":[{"id":"e","fromNodeId":"i","toNodeId":"d"}],"timeline":{"clips":[{"id":"c","resourceId":"gone"}]}}`
	cleaned, err := CleanMediaDocument(raw, map[string]struct{}{"gone": {}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(cleaned.RemovedNodeIDs) != 1 || cleaned.RemovedNodeIDs[0] != "d" || !cleaned.HasMedia {
		t.Fatalf("cleanup=%#v", cleaned)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(cleaned.Payload), &doc); err != nil {
		t.Fatal(err)
	}
	nodes := doc["nodes"].([]any)
	metadata := nodes[0].(map[string]any)["metadata"].(map[string]any)
	if metadata["storageKey"] != "resource:keep" || metadata["currentMediaVersionId"] != "v1" || len(metadata["mediaVersions"].([]any)) != 1 || metadata["assetId"] != nil {
		t.Fatalf("metadata=%#v", metadata)
	}
	if len(doc["connections"].([]any)) != 0 || strings.Contains(cleaned.Payload, "resource:gone") {
		t.Fatalf("stale document: %s", cleaned.Payload)
	}
}
