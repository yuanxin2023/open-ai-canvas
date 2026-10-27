package submission

import "testing"

func TestFingerprintPreservesRequestMeaning(t *testing.T) {
	a, err := Fingerprint("image", map[string]any{"count": 1, "audio": false, "options": map[string]any{"b": 2, "a": 1}})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Fingerprint("image", map[string]any{"options": map[string]any{"a": 1, "b": 2}, "audio": false, "count": 1})
	if a != b || len(a) != 64 {
		t.Fatal("map order changed the digest")
	}
	for _, value := range []any{map[string]any{"count": 2}, map[string]any{"count": 1}, map[string]any{"count": 1, "audio": true}} {
		c, _ := Fingerprint("image", value)
		if c == a {
			t.Fatal("different request reused the digest")
		}
	}
	c, _ := Fingerprint("video", map[string]any{"count": 1, "audio": false, "options": map[string]any{"a": 1, "b": 2}})
	if c == a {
		t.Fatal("namespace was not bound")
	}
}

func TestKeyBounds(t *testing.T) {
	for _, key := range []string{"", "short", " leading-request-key", "unsafe/request-key", "请求标识-request-key"} {
		if ValidateKey(key) == nil {
			t.Fatalf("accepted %q", key)
		}
	}
	if err := ValidateKey("12345678-1234-1234-1234-123456789012"); err != nil {
		t.Fatal(err)
	}
}
