package handler

import "testing"

func TestLoginEnvironmentParsers(t *testing.T) {
	browser, version := browserFromClientHints(`"Not_A Brand";v="99", "Chromium";v="140", "Google Chrome";v="140"`)
	if browser != "Chrome" || version != "140" {
		t.Fatalf("client hints browser = %q %q", browser, version)
	}
	browser, version = browserFromUserAgent("Mozilla/5.0 (iPhone; CPU iPhone OS 18_6 like Mac OS X) AppleWebKit/605.1.15 Version/18.6 Mobile/15E148 Safari/604.1")
	if browser != "Safari" || version != "18.6" {
		t.Fatalf("user agent browser = %q %q", browser, version)
	}
	osName, osVersion := osFromUserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	if osName != "Windows" || osVersion != "10/11" {
		t.Fatalf("operating system = %q %q", osName, osVersion)
	}
	if got := deviceType("?0", "Mozilla/5.0 (iPad; CPU OS 18_6 like Mac OS X)"); got != "平板" {
		t.Fatalf("device type = %q", got)
	}
	if got := deviceType("?0", "Mozilla/5.0 (Linux; Android 15; Tablet)"); got != "平板" {
		t.Fatalf("Android tablet device type = %q", got)
	}
	if got := deviceType("", ""); got != "未知" {
		t.Fatalf("unknown device type = %q", got)
	}
}
