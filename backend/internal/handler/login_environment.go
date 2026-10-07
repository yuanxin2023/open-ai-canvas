package handler

import (
	"regexp"
	"strings"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

var clientBrandPattern = regexp.MustCompile(`"([^"]+)"\s*;\s*v="([^"]+)"`)

func loginEnvironment(c *gin.Context) service.LoginEnvironment {
	userAgent := strings.TrimSpace(c.GetHeader("User-Agent"))
	browser, browserVersion := browserFromClientHints(c.GetHeader("Sec-CH-UA"))
	if browser == "" {
		browser, browserVersion = browserFromUserAgent(userAgent)
	}
	osName := trimClientHint(c.GetHeader("Sec-CH-UA-Platform"))
	osVersion := trimClientHint(c.GetHeader("Sec-CH-UA-Platform-Version"))
	if osName == "" {
		osName, osVersion = osFromUserAgent(userAgent)
	}
	return service.LoginEnvironment{
		IPAddress:      c.ClientIP(),
		UserAgent:      userAgent,
		DeviceType:     deviceType(c.GetHeader("Sec-CH-UA-Mobile"), userAgent),
		Browser:        browser,
		BrowserVersion: browserVersion,
		OS:             osName,
		OSVersion:      osVersion,
	}
}

func browserFromClientHints(value string) (string, string) {
	brands := clientBrandPattern.FindAllStringSubmatch(value, -1)
	preferred := []struct {
		brand string
		name  string
	}{{"Microsoft Edge", "Edge"}, {"Google Chrome", "Chrome"}, {"Opera", "Opera"}, {"Chromium", "Chromium"}}
	for _, candidate := range preferred {
		for _, match := range brands {
			if len(match) == 3 && strings.EqualFold(match[1], candidate.brand) {
				return candidate.name, match[2]
			}
		}
	}
	return "", ""
}

func browserFromUserAgent(value string) (string, string) {
	checks := []struct {
		token string
		name  string
	}{{"Edg/", "Edge"}, {"OPR/", "Opera"}, {"CriOS/", "Chrome"}, {"Chrome/", "Chrome"}, {"FxiOS/", "Firefox"}, {"Firefox/", "Firefox"}, {"Version/", "Safari"}}
	for _, check := range checks {
		if check.name == "Safari" && !strings.Contains(value, "Safari/") {
			continue
		}
		if version := tokenVersion(value, check.token); version != "" {
			return check.name, version
		}
	}
	return "未知", ""
}

func osFromUserAgent(value string) (string, string) {
	switch {
	case strings.Contains(value, "Windows NT "):
		version := tokenVersion(value, "Windows NT ")
		windowsNames := map[string]string{"10.0": "10/11", "6.3": "8.1", "6.2": "8", "6.1": "7"}
		if mapped := windowsNames[version]; mapped != "" {
			version = mapped
		}
		return "Windows", version
	case strings.Contains(value, "Android "):
		return "Android", tokenVersion(value, "Android ")
	case strings.Contains(value, "iPhone OS "):
		return "iOS", strings.ReplaceAll(tokenVersion(value, "iPhone OS "), "_", ".")
	case strings.Contains(value, "CPU OS "):
		return "iPadOS", strings.ReplaceAll(tokenVersion(value, "CPU OS "), "_", ".")
	case strings.Contains(value, "Mac OS X "):
		return "macOS", strings.ReplaceAll(tokenVersion(value, "Mac OS X "), "_", ".")
	case strings.Contains(value, "Linux"):
		return "Linux", ""
	default:
		return "未知", ""
	}
}

func deviceType(mobileHint string, userAgent string) string {
	mobileHint = strings.TrimSpace(mobileHint)
	if strings.EqualFold(mobileHint, "?1") {
		return "手机"
	}
	lower := strings.ToLower(userAgent)
	switch {
	case strings.Contains(lower, "ipad") || strings.Contains(lower, "tablet") || (strings.Contains(lower, "android") && !strings.Contains(lower, "mobile")) || (strings.Contains(lower, "macintosh") && strings.Contains(lower, "mobile")):
		return "平板"
	case strings.Contains(lower, "mobile") || strings.Contains(lower, "iphone") || strings.Contains(lower, "android"):
		return "手机"
	case lower == "" && mobileHint == "":
		return "未知"
	default:
		return "电脑"
	}
}

func tokenVersion(value string, token string) string {
	start := strings.Index(value, token)
	if start < 0 {
		return ""
	}
	value = value[start+len(token):]
	if end := strings.IndexAny(value, " ;()\t"); end >= 0 {
		value = value[:end]
	}
	return strings.TrimSpace(value)
}

func trimClientHint(value string) string {
	return strings.Trim(strings.TrimSpace(value), `"`)
}
