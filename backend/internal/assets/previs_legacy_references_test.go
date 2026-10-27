package assets

import (
	"os"
	"reflect"
	"testing"

	"infinite-canvas/backend/internal/canvas/previslegacy"
)

func TestLegacyPrevisConversionKeepsResourceDeletionProtection(t *testing.T) {
	raw, err := os.ReadFile("../canvas/previslegacy/testdata/director.json")
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := previslegacy.Normalize(raw)
	if err != nil {
		t.Fatal(err)
	}
	before, after := map[string]struct{}{}, map[string]struct{}{}
	if err := CollectOwnedDocumentReferences(string(raw), before); err != nil {
		t.Fatal(err)
	}
	if err := CollectOwnedDocumentReferences(string(normalized), after); err != nil {
		t.Fatal(err)
	}
	if len(before) != 2 || !reflect.DeepEqual(before, after) {
		t.Fatalf("resource protection changed: before=%v after=%v", before, after)
	}
}
