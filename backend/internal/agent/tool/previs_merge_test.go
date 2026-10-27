package tool

import "testing"

func TestPrevisToolsKeepOfficialPermissionAndBillingBoundary(t *testing.T) {
	for _, name := range []string{PrevisSceneRead, PrevisPreview, PrevisSceneCreate, PrevisApplyPatch} {
		p := Of(name)
		if p.Allowed(Scope{}) || p.Media() {
			t.Fatalf("%s lost its canvas or non-media boundary", name)
		}
		read := name == PrevisSceneRead
		if p.Allowed(Scope{Canvas: true, ReadOnly: true}) != read {
			t.Fatalf("read-only access changed for %s", name)
		}
		if !p.Allowed(Scope{Canvas: true}) {
			t.Fatalf("authorized canvas mode cannot use %s", name)
		}
		write := name == PrevisSceneCreate || name == PrevisApplyPatch
		if p.ApprovalRequired("request_approval") != write || p.ApprovalRequired("auto") {
			t.Fatalf("official approval mode changed for %s", name)
		}
	}
	if !Of(GenerateMedia).ApprovalRequired("auto") {
		t.Fatal("3D trust mode bypassed paid generation approval")
	}
}
