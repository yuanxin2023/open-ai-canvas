package referralcode

import "testing"

func TestNewProducesSixCharacterCodes(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := New()
		if err != nil {
			t.Fatal(err)
		}
		if !Valid(code) {
			t.Fatalf("generated invalid code %q", code)
		}
	}
}

func TestValidRejectsOldAndAmbiguousCodes(t *testing.T) {
	for _, code := range []string{"", "ABC123456789", "ABC1234", "ABC12I", "ABC12O", "ABC120", "ABC121", "abc234"} {
		if Valid(code) {
			t.Fatalf("accepted invalid code %q", code)
		}
	}
	if !Valid("ABC234") {
		t.Fatal("rejected six-character code")
	}
}
