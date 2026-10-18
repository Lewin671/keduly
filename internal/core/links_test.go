package core

import "testing"

func TestDeviceLabel(t *testing.T) {
	for ua, want := range map[string]string{
		"Keduly/Android": "Keduly · Android",
		"Keduly/iPhone":  "Keduly · iPhone",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0 Safari/537.36":                       "Chrome · Mac",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.0 Mobile/15E148 Safari/604.1": "Safari · iPhone",
		"curl/8.0": "",
	} {
		if got := deviceLabel(ua); got != want {
			t.Errorf("deviceLabel(%q) = %q, want %q", ua, got, want)
		}
	}
}
