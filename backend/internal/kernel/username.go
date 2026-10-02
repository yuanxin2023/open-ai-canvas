package kernel

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

var reservedUsernames = map[string]struct{}{
	"admin": {}, "root": {}, "system": {}, "openai": {},
	"官方": {}, "官方账号": {}, "系统": {}, "管理员": {}, "客服": {}, "平台": {},
}

func NormalizeLoginUsername(value string) string {
	return strings.ToLower(strings.TrimSpace(norm.NFKC.String(value)))
}

func ValidateLoginUsername(value string) error {
	value = NormalizeLoginUsername(value)
	runes := []rune(value)
	hasHan := false
	hasLetter := false
	for _, char := range runes {
		switch {
		case unicode.Is(unicode.Han, char):
			hasHan = true
		case char >= 'a' && char <= 'z':
			hasLetter = true
		case char >= '0' && char <= '9':
		default:
			return errors.New("用户名只支持中文、英文字母和数字")
		}
	}
	if !hasHan && !hasLetter {
		return errors.New("用户名至少需要包含中文或英文字母")
	}
	if len(runes) < 3 || len(runes) > 9 {
		return errors.New("用户名需为 3-9 位")
	}
	if _, reserved := reservedUsernames[value]; reserved {
		return errors.New("该用户名为系统保留名称")
	}
	return nil
}

func DefaultLoginUsernameCandidate(email string, userID string, attempt int) string {
	local, _, _ := strings.Cut(strings.TrimSpace(email), "@")
	baseRunes := make([]rune, 0, 6)
	hasLetter := false
	for _, char := range NormalizeLoginUsername(local) {
		if char >= 'a' && char <= 'z' {
			hasLetter = true
			baseRunes = append(baseRunes, char)
		} else if char >= '0' && char <= '9' {
			baseRunes = append(baseRunes, char)
		}
	}
	if attempt == 0 && hasLetter && len(baseRunes) >= 3 {
		candidate := string(baseRunes[:min(len(baseRunes), 6)])
		if ValidateLoginUsername(candidate) == nil {
			return candidate
		}
	}
	stem := baseRunes
	if !hasLetter {
		stem = []rune("usr")
	}
	if len(stem) > 3 {
		stem = stem[:3]
	}
	digest := sha256.Sum256([]byte(userID + ":" + fmt.Sprint(attempt)))
	alphabet := "0123456789abcdefghijklmnopqrstuvwxyz"
	suffix := []byte{alphabet[int(digest[0])%len(alphabet)], alphabet[int(digest[1])%len(alphabet)], alphabet[int(digest[2])%len(alphabet)]}
	return string(stem) + string(suffix)
}
