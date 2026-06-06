package webpresence

import "testing"

func TestParseBrowser(t *testing.T) {
	tests := []struct {
		ua   string
		want string
	}{
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "Safari"},
		{"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/120.0.0.0 Mobile Safari/537.36", "Chrome"},
		{"", "Браузер"},
	}
	for _, tc := range tests {
		if got := ParseBrowser(tc.ua); got != tc.want {
			t.Fatalf("ua=%q got %q want %q", tc.ua, got, tc.want)
		}
	}
}
