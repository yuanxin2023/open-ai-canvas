package feedback

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSubmissionKeysAreScopedAndValidated(t *testing.T) {
	key := "request-0000000001"
	if !ValidRequestID(key) {
		t.Fatal("valid request rejected")
	}
	for _, value := range []string{"", "short", strings.Repeat("x", 97), "request key-0000001", "请求-0000000000001"} {
		if ValidRequestID(value) {
			t.Fatalf("accepted %q", value)
		}
	}
	if SubmissionID("a", key) != SubmissionID("a", key) {
		t.Fatal("replay changed identity")
	}
	if SubmissionID("a", key) == SubmissionID("b", key) || SubmissionID("a", key) == SubmissionID("a", key+"2") {
		t.Fatal("account/request scopes overlap")
	}
}

func TestEnvironmentRejectsUnapprovedFieldsAndMalformedObjects(t *testing.T) {
	for _, raw := range []string{`{"currentUrl":"https://host/?token=secret"}`, `{"cookie":"secret"}`, `{"apiKey":"secret"}`, `[]`, `null`, `{}`, `{} {}`} {
		_, err := NormalizeEnvironment(raw)
		if raw == `{}` {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	if _, err := NormalizeEnvironment(`{"userAgent":"` + strings.Repeat("x", 513) + `"}`); err == nil {
		t.Fatal("oversized user agent accepted")
	}
}

func TestEnvironmentNormalizesDynamicRouteAndOmitsSecrets(t *testing.T) {
	got, err := NormalizeEnvironment(`{"userAgent":"Chrome","platform":"Windows","language":"zh-CN","screen":"1920x1080","viewport":"1280x720","pathname":"/canvas/private-id?token=secret#private"}`)
	if err != nil {
		t.Fatal(err)
	}
	var value Environment
	if err := json.Unmarshal([]byte(got), &value); err != nil {
		t.Fatal(err)
	}
	if value.Pathname != "/canvas/:id" || strings.Contains(got, "secret") || strings.Contains(got, "private") {
		t.Fatalf("unsafe environment %s", got)
	}
	for path, want := range map[string]string{"/": "/", "/projects/p/private": "/projects/:id", "/settings?email=private": "/settings", "/admin/users/secret": "/admin/:section", "/unknown/private": "/:page"} {
		if got := SafePathname(path); got != want {
			t.Fatalf("%q -> %q, want %q", path, got, want)
		}
	}
	if got, err := NormalizeEnvironment(""); err != nil || got != "{}" {
		t.Fatalf("empty opt-out = %s, %v", got, err)
	}
}
