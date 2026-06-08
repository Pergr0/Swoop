package invite

import (
	"testing"
	"time"

	"swoop/core/identity"
	"swoop/core/protocol"
)

func TestCreateParseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	id, err := identity.LoadOrCreate(dir, "TestDevice")
	if err != nil {
		t.Fatal(err)
	}
	self := protocol.DeviceInfo{
		ID:          id.DeviceID,
		Name:        id.Name,
		Address:     "192.168.1.10",
		ControlPort: 53317,
		Fingerprint: id.Fingerprint,
		Platform:    protocol.PlatformLinux,
		Version:     protocol.Version,
	}
	bundle, err := Create(id, self, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Blob == "" || bundle.ShortCode == "" {
		t.Fatalf("empty bundle: %+v", bundle)
	}
	parsed, err := ParseAndVerify(bundle.Blob)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Device.ID != self.ID || parsed.Device.Fingerprint != self.Fingerprint {
		t.Fatalf("device mismatch: %+v", parsed.Device)
	}
	if parsed.ShortCode != bundle.ShortCode {
		t.Fatalf("short code mismatch")
	}
}

func TestPNGTextRoundTrip(t *testing.T) {
	dir := t.TempDir()
	id, err := identity.LoadOrCreate(dir, "PNGDevice")
	if err != nil {
		t.Fatal(err)
	}
	self := protocol.DeviceInfo{
		ID: id.DeviceID, Name: id.Name, Address: "10.0.0.2",
		ControlPort: 53317, Fingerprint: id.Fingerprint,
		Platform: protocol.PlatformMacOS, Version: protocol.Version,
	}
	bundle, err := Create(id, self, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	pngData, err := RenderPNG(bundle)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := DecodeFromPNG(pngData)
	if err != nil {
		t.Fatal(err)
	}
	if blob != bundle.Blob {
		t.Fatalf("blob mismatch after PNG decode")
	}
}

func TestReachRoundTrip(t *testing.T) {
	dir := t.TempDir()
	id, err := identity.LoadOrCreate(dir, "ReachDevice")
	if err != nil {
		t.Fatal(err)
	}
	self := protocol.DeviceInfo{
		ID: id.DeviceID, Name: id.Name, Address: "192.168.0.5",
		ControlPort: 53317, Fingerprint: id.Fingerprint,
		Platform: protocol.PlatformLinux, Version: protocol.Version,
	}
	reach := &Reach{Addr: "203.0.113.10", ControlPort: 53317, PunchPort: 54444}
	bundle, err := Create(id, self, time.Hour, reach)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseAndVerify(bundle.Blob)
	if err != nil {
		t.Fatal(err)
	}
	if !parsed.HasReach() || parsed.ReachAddr != reach.Addr || parsed.PunchPort != reach.PunchPort {
		t.Fatalf("reach mismatch: %+v", parsed)
	}
	dial := parsed.DialDevice()
	if dial.Address != reach.Addr {
		t.Fatalf("dial address %q", dial.Address)
	}
}

func TestBlobFromFile(t *testing.T) {
	b := Bundle{Blob: "abc123", ShortCode: "ABCD-EF12"}
	text := FileContent(b)
	got := BlobFromFile([]byte(text))
	if got != "abc123" {
		t.Fatalf("got %q", got)
	}
}
