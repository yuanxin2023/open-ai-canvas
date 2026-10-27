package submission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

func ValidateKey(key string) error {
	if len(key) < 16 || len(key) > 96 {
		return errors.New("工作台请求标识长度无效")
	}
	for _, c := range key {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return errors.New("工作台请求标识格式无效")
		}
	}
	return nil
}

// Only the digest is persisted. Caller-specific secrets never become receipt data.
func Fingerprint(namespace string, input any) (string, error) {
	b, err := json.Marshal(input)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(namespace+"\x00"), b...))
	return hex.EncodeToString(sum[:]), nil
}
