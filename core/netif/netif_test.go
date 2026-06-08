package netif

import "testing"

func TestHardwarePortKindFromLabel(t *testing.T) {
	tests := []struct {
		port string
		want string
	}{
		{"Wi-Fi", "wifi"},
		{"AirPort", "wifi"},
		{"Ethernet", "ethernet"},
		{"Thunderbolt Ethernet", "ethernet"},
		{"InvisibleMan-XRay VPN", "tunnel"},
		{"VMware Virtual Ethernet", "virtual"},
		{"Something Else", ""},
	}
	for _, tc := range tests {
		if got := hardwarePortKindFromLabel(tc.port); got != tc.want {
			t.Errorf("hardwarePortKindFromLabel(%q) = %q, want %q", tc.port, got, tc.want)
		}
	}
}

func TestSortAndRecommend(t *testing.T) {
	list := []NetInterface{
		{Name: "utun0", Kind: "tunnel"},
		{Name: "en5", Kind: "ethernet"},
		{Name: "en0", Kind: "wifi"},
		{Name: "bridge0", Kind: "virtual"},
		{Name: "z0", Kind: "other"},
	}
	sortInterfaces(list)
	if list[0].Kind != "wifi" || list[1].Kind != "ethernet" || list[2].Kind != "tunnel" {
		t.Fatalf("sort order: %+v", list)
	}
	markRecommended(list)
	if !list[0].Recommended || list[0].Kind != "wifi" {
		t.Fatalf("expected wifi recommended, got %+v", list[0])
	}
}

func TestParseNetshWLANInterfaces(t *testing.T) {
	sample := `
There is 1 interface on the system:

    Name                   : Wi-Fi
    Description            : Intel(R) Wi-Fi 6 AX201 160MHz
    GUID                   : x
    Physical address       : aa:bb:cc:dd:ee:ff
    Interface type         : Primary
    State                  : connected
    SSID                   : HomeNetwork
    BSSID                  : 00:11:22:33:44:55
`
	sampleRU := `
    Имя                    : Беспроводная сеть
    SSID                   : ДомашняяСеть
`
	got := parseNetshWLANInterfaces(sample)
	if got["Wi-Fi"] != "HomeNetwork" {
		t.Fatalf("english parse: got %v", got)
	}
	gotRU := parseNetshWLANInterfaces(sampleRU)
	if gotRU["Беспроводная сеть"] != "ДомашняяСеть" {
		t.Fatalf("russian parse: got %v", gotRU)
	}
}

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
