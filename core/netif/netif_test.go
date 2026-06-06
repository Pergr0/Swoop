package netif

import "testing"

func TestKindOfName(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"Wi-Fi", "wifi"},
		{"Беспроводная сеть 3", "wifi"},
		{"Ethernet 5", "ethernet"},
		{"wlp3s0", "wifi"},
		{"eth0", "ethernet"},
		{"InvisibleMan-XRay", "tunnel"},
		{"Подключение по локальной сети", "ethernet"},
		{"something-random", "other"},
	}
	for _, tc := range tests {
		if got := kindOfName(tc.name); got != tc.want {
			t.Errorf("kindOfName(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
