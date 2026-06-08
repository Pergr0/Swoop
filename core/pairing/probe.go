package pairing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"time"

	"swoop/core/protocol"
	"swoop/core/transport"
)

// ProbeInfo dials the peer control plane and returns live DeviceInfo from GET /api/v1/info.
// The invite fingerprint must match the TLS certificate (TOFU).
func ProbeInfo(ctx context.Context, device protocol.DeviceInfo) (protocol.DeviceInfo, error) {
	if device.Fingerprint == "" {
		return protocol.DeviceInfo{}, errors.New("missing fingerprint")
	}
	if device.Address == "" || device.ControlPort == 0 {
		return protocol.DeviceInfo{}, errors.New("missing address")
	}
	client := transport.NewPinnedClient(device.Fingerprint, 8*time.Second)
	url := "https://" + net.JoinHostPort(device.Address, strconv.Itoa(device.ControlPort)) + "/api/v1/info"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return protocol.DeviceInfo{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return protocol.DeviceInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return protocol.DeviceInfo{}, fmt.Errorf("info HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return protocol.DeviceInfo{}, err
	}
	var info protocol.DeviceInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return protocol.DeviceInfo{}, err
	}
	if info.ID != device.ID {
		return protocol.DeviceInfo{}, fmt.Errorf("device id mismatch")
	}
	if info.Fingerprint != device.Fingerprint {
		return protocol.DeviceInfo{}, fmt.Errorf("fingerprint mismatch")
	}
	return info, nil
}
