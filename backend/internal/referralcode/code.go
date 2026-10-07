package referralcode

import (
	"crypto/rand"
	"strings"
)

const Length = 6
const Alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func New() (string, error) {
	bytes := make([]byte, Length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	for i := range bytes {
		bytes[i] = Alphabet[int(bytes[i])%len(Alphabet)]
	}
	return string(bytes), nil
}

func Valid(code string) bool {
	if len(code) != Length {
		return false
	}
	for i := range code {
		if strings.IndexByte(Alphabet, code[i]) < 0 {
			return false
		}
	}
	return true
}
