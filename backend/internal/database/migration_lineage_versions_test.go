package database

import (
	"fmt"
	"testing"
)

func TestUpstreamFirstMigrationVersionPreservesHistoricalAssignments(t *testing.T) {
	assignments := map[int64]int64{
		24: 33, 25: 34, 26: 35, 27: 36,
		28: 24, 29: 25, 30: 26, 31: 27, 32: 28,
		33: 29, 34: 30, 35: 31, 36: 32,
	}
	for local, expected := range assignments {
		t.Run(fmt.Sprintf("local_%d", local), func(t *testing.T) {
			if got := upstreamFirstMigrationVersion(local); got != expected {
				t.Fatalf("version %d mapped to %d, want historical version %d", local, got, expected)
			}
		})
	}
}

func TestUpstreamFirstMigrationVersionKeepsSharedAndAppendedVersions(t *testing.T) {
	for _, version := range []int64{1, 6, 7, 23, 37, 44, 51, 52, 62, 78, 79, 100} {
		t.Run(fmt.Sprintf("version_%d", version), func(t *testing.T) {
			if got := upstreamFirstMigrationVersion(version); got != version {
				t.Fatalf("shared or appended version %d was renumbered to %d", version, got)
			}
		})
	}
}

func TestUpstreamFirstMigrationVersionsRemainUniqueAfterAppending(t *testing.T) {
	for _, last := range []int64{51, 52, 78, 79, 100} {
		t.Run(fmt.Sprintf("through_%d", last), func(t *testing.T) {
			seen := make(map[int64]int64, last)
			for local := int64(1); local <= last; local++ {
				mapped := upstreamFirstMigrationVersion(local)
				if mapped < 1 || mapped > last {
					t.Fatalf("version %d maps outside 1..%d: %d", local, last, mapped)
				}
				if previous, exists := seen[mapped]; exists {
					t.Fatalf("versions %d and %d both map to %d", previous, local, mapped)
				}
				seen[mapped] = local
			}
		})
	}
}
