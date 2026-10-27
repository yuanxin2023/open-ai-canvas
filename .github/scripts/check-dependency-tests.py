"""Require named dependency-backed Go tests to run and pass (not merely avoid SKIP)."""

import json
import sys

REQUIRED = {
    ("database", "TestPostgresAssetIDMigration"),
    ("canvas", "TestRepairAssetBytesPostgres"),
    ("repository", "TestCanvasRevision/postgres"),
    ("platform", "TestRuntimeCoordinatorRedisLeasePreservesLongestTTL"),
    ("app", "TestClearAdminRuntimeCacheUsesRedisWhitelist"),
    ("database", "TestUpgradeOriginalSchema51Fixtures/sqlite_local"),
    ("database", "TestUpgradeOriginalSchema51Fixtures/sqlite_upstream"),
    ("database", "TestUpgradeOriginalSchema51Fixtures/postgres_local"),
    ("database", "TestUpgradeOriginalSchema51Fixtures/postgres_upstream"),
    ("skills", "TestBuiltinSyncRetainsDeployedOfficialImageIdentities/postgres"),
    ("skills", "TestBuiltinImageHistoryRejectsUnverifiedRecords/postgres"),
}


def missing_tests(events):
    ran, passed, failed = set(), set(), set()
    for event in events:
        key = (event.get("Package", "").rsplit("/", 1)[-1], event.get("Test"))
        action = event.get("Action")
        if action == "run":
            ran.add(key)
        elif action == "pass":
            passed.add(key)
        elif action in {"skip", "fail"}:
            failed.add(key)
    return REQUIRED - ((ran & passed) - failed)


if __name__ == "__main__":
    with open(sys.argv[1], encoding="utf-8") as stream:
        missing = missing_tests(json.loads(line) for line in stream if line.strip())
    if missing:
        sys.exit("Dependency tests missing, skipped or failed: " + repr(sorted(missing)))
    print(f"All {len(REQUIRED)} dependency-backed tests ran and passed.")
