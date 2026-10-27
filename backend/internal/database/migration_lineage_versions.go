package database

// Only the original divergent segment is remapped. Appending migrations must
// never change the identities already recorded by either supported lineage.
func upstreamFirstMigrationVersion(version int64) int64 {
	switch {
	case version >= 24 && version <= 27:
		return version + 9
	case version >= 28 && version <= 36:
		return version - 4
	default:
		return version
	}
}
