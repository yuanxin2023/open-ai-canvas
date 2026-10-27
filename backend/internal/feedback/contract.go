package feedback

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

func ValidRequestID(value string) bool {
	if len(value) < 16 || len(value) > 96 {
		return false
	}
	for _, char := range value {
		if !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' || char >= '0' && char <= '9' || char == '-' || char == '_') {
			return false
		}
	}
	return true
}

func SubmissionID(userID, requestID string) string {
	return "fb_" + uuid.NewSHA1(uuid.NameSpaceOID, []byte("canvas/feedback/v1/"+userID+"\x00"+requestID)).String()
}

func SafePathname(value string) string {
	path, _, _ := strings.Cut(value, "?")
	path, _, _ = strings.Cut(path, "#")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if path == "/" {
		return path
	}
	switch parts[0] {
	case "canvas", "projects":
		if len(parts) > 1 {
			return "/" + parts[0] + "/:id"
		}
		return "/" + parts[0]
	case "settings", "create", "prompts", "tasks", "assets", "plugins", "skills", "toolbox":
		return "/" + parts[0]
	case "admin":
		return "/admin/:section"
	default:
		return "/:page"
	}
}

type Environment struct {
	UserAgent string `json:"userAgent,omitempty"`
	Platform  string `json:"platform,omitempty"`
	Language  string `json:"language,omitempty"`
	Screen    string `json:"screen,omitempty"`
	Viewport  string `json:"viewport,omitempty"`
	Pathname  string `json:"pathname,omitempty"`
}

func NormalizeEnvironment(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "{}", nil
	}
	if len(raw) > 4096 || !strings.HasPrefix(strings.TrimSpace(raw), "{") {
		return "", errors.New("诊断信息格式无效")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var env Environment
	if err := decoder.Decode(&env); err != nil {
		return "", errors.New("诊断信息包含不支持的字段")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", errors.New("诊断信息只能包含一个对象")
	}
	if utf8.RuneCountInString(env.UserAgent) > 512 || utf8.RuneCountInString(env.Platform) > 128 || utf8.RuneCountInString(env.Language) > 64 || len(env.Screen) > 32 || len(env.Viewport) > 32 {
		return "", errors.New("诊断信息超出长度限制")
	}
	if env.Pathname != "" {
		env.Pathname = SafePathname(env.Pathname)
	}
	encoded, err := json.Marshal(env)
	return string(encoded), err
}
