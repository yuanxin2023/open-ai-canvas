package kernel

import "testing"

func TestNormalizeLoginUsername(t *testing.T) {
	if got := NormalizeLoginUsername("　ＡＢＣ１２　"); got != "abc12" {
		t.Fatalf("NormalizeLoginUsername() = %q, want abc12", got)
	}
}

func TestValidateLoginUsername(t *testing.T) {
	tests := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "two Chinese", value: "小序", valid: true},
		{name: "six Chinese", value: "一二三四五六", valid: true},
		{name: "ascii letters", value: "abc", valid: true},
		{name: "ascii mixed digits", value: "a12", valid: true},
		{name: "mixed Chinese", value: "小a", valid: true},
		{name: "case normalized", value: "AbC12", valid: true},
		{name: "full width normalized", value: "ＡＢＣ", valid: true},
		{name: "one Chinese", value: "小", valid: false},
		{name: "short ascii", value: "ab", valid: false},
		{name: "too long", value: "abcdefg", valid: false},
		{name: "pure digits", value: "123456", valid: false},
		{name: "underscore", value: "abc_1", valid: false},
		{name: "hyphen", value: "abc-1", valid: false},
		{name: "punctuation", value: "abc!", valid: false},
		{name: "emoji", value: "小序😀", valid: false},
		{name: "reserved ascii", value: "ADMIN", valid: false},
		{name: "reserved Chinese", value: "管理员", valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateLoginUsername(test.value)
			if (err == nil) != test.valid {
				t.Fatalf("ValidateLoginUsername(%q) error = %v, valid = %v", test.value, err, test.valid)
			}
		})
	}
}

func TestDefaultLoginUsernameCandidate(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		attempt int
		want    string
		prefix  string
	}{
		{name: "normal", email: "maker@example.com", want: "maker"},
		{name: "long", email: "creatorlong@example.com", want: "creato"},
		{name: "dot and plus", email: "a.b+c@example.com", want: "abc"},
		{name: "pure digits", email: "123456@example.com", attempt: 1, prefix: "usr"},
		{name: "reserved", email: "admin@example.com", attempt: 1, prefix: "adm"},
		{name: "conflict retry", email: "maker@example.com", attempt: 1, prefix: "mak"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := DefaultLoginUsernameCandidate(test.email, "user-id", test.attempt)
			if test.want != "" && got != test.want {
				t.Fatalf("candidate = %q, want %q", got, test.want)
			}
			if test.prefix != "" && (len(got) != 6 || got[:3] != test.prefix) {
				t.Fatalf("candidate = %q, want six characters with prefix %q", got, test.prefix)
			}
			if err := ValidateLoginUsername(got); err != nil {
				t.Fatalf("generated candidate %q is invalid: %v", got, err)
			}
		})
	}
}
