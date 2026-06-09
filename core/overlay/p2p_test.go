package overlay

import (
	"errors"
	"testing"
)

func TestClassifyUpgradeFail(t *testing.T) {
	offer := upgradeOffer{LanAddr: "192.168.1.10"}
	if got := classifyUpgradeFail(errUpgradePunch, offer); got != P2PNoteNoUpnp {
		t.Fatalf("punch+private lan: got %q want %q", got, P2PNoteNoUpnp)
	}
	offer.ReachAddr = "203.0.113.1"
	if got := classifyUpgradeFail(errUpgradePunch, offer); got != P2PNotePunch {
		t.Fatalf("punch+reach: got %q", got)
	}
	if got := classifyUpgradeFail(errUpgradeQuicDial, offer); got != P2PNoteQuic {
		t.Fatalf("quic: got %q", got)
	}
	if got := classifyUpgradeFail(errors.New("other"), offer); got != P2PNoteQuic {
		t.Fatalf("default: got %q", got)
	}
}

func TestIsPrivateIPv4(t *testing.T) {
	if !isPrivateIPv4("10.0.0.1") {
		t.Fatal("10/8 private")
	}
	if isPrivateIPv4("8.8.8.8") {
		t.Fatal("public")
	}
}
