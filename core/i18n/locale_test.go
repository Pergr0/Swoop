package i18n

import "testing"

func TestIsRussianTag(t *testing.T) {
	cases := []struct {
		tag string
		ru  bool
	}{
		{"ru", true},
		{"ru-RU", true},
		{"ru_RU.UTF-8", true},
		{"en", false},
		{"en-US", false},
		{"de-DE", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isRussianTag(c.tag); got != c.ru {
			t.Fatalf("isRussianTag(%q) = %v, want %v", c.tag, got, c.ru)
		}
	}
}

func TestPick(t *testing.T) {
	if Pick("да", "no") == "" {
		t.Fatal("Pick returned empty")
	}
}
