package rendezvous

// HostRegisterRequest is sent when creating an internet invite (host side).
type HostRegisterRequest struct {
	SessionID   string `json:"sessionId"`
	PeerID      string `json:"peerId"`
	DeviceName  string `json:"deviceName"`
	LanAddr     string `json:"lanAddr"`
	ControlPort int    `json:"controlPort"`
	PunchPort   int    `json:"punchPort"`
	ReachAddr   string `json:"reachAddr,omitempty"`
	ReachPort   int    `json:"reachPort,omitempty"`
}

// JoinRequest is sent when importing an invite (joiner side).
type JoinRequest struct {
	SessionID    string   `json:"sessionId"`
	PeerID       string   `json:"peerId"`
	PunchPort    int      `json:"punchPort"`
	LanAddr      string   `json:"lanAddr,omitempty"`
	DeviceName   string   `json:"deviceName,omitempty"`
	Fingerprint  string   `json:"fingerprint,omitempty"`
	ControlPort  int      `json:"controlPort,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// HostInfo is returned to the joiner so it can reach the host (P2P endpoints only).
type HostInfo struct {
	PeerID          string `json:"peerId"`
	DeviceName      string `json:"deviceName"`
	LanAddr         string `json:"lanAddr"`
	ReachAddr       string `json:"reachAddr,omitempty"`
	ReflexiveAddr   string `json:"reflexiveAddr,omitempty"`
	ControlPort     int    `json:"controlPort"`
	PunchPort       int    `json:"punchPort"`
	JoinerReflexive string `json:"joinerReflexive,omitempty"`
}

// TouchRequest extends rendezvous room lifetime after pairing (activity keepalive).
type TouchRequest struct {
	SessionID string `json:"sessionId"`
}

// JoinerInfo is returned to the host when a joiner appears (for reverse punch).
type JoinerInfo struct {
	PeerID        string   `json:"peerId"`
	DeviceName    string   `json:"deviceName,omitempty"`
	LanAddr       string   `json:"lanAddr,omitempty"`
	Fingerprint   string   `json:"fingerprint,omitempty"`
	ControlPort   int      `json:"controlPort,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
	ReflexiveAddr string   `json:"reflexiveAddr"`
	PunchPort     int      `json:"punchPort"`
}
