// Package protocol defines the wire-level contract shared by every Swoop
// client (desktop now, mobile later). Keeping it dependency-free and stable
// is what lets any platform interoperate ("protocol-first").
package protocol

const (
	// Version is the wire protocol version. Bump on breaking changes.
	Version = 1

	// ServiceName is the mDNS/DNS-SD service type used for discovery.
	ServiceName = "_swoop._tcp"

	// DefaultControlPort is the default HTTPS control-plane port.
	DefaultControlPort = 53317

	// DefaultDataPort is the default raw-TCP data-plane port (file bytes).
	DefaultDataPort = 53319

	// MaxMessageBytes caps the size of a chat message body (UTF-8 text). Small
	// on purpose: the chat is for links/short notes, and a cap bounds spam/abuse.
	MaxMessageBytes = 8192

	// MaxPrepareUploadBodyBytes caps the JSON body of POST /api/v1/prepare-upload.
	MaxPrepareUploadBodyBytes = 4 << 20 // 4 MiB

	// Transfer limits for incoming offers (claimed sizes; enforced before accept).
	MaxTransferFiles       = 4096
	MaxTransferFileBytes   = 4 << 30  // 4 GiB per file
	MaxTransferTotalBytes  = 32 << 30 // 32 GiB per session

	// DiscoveryMulticastGroup / DiscoveryPort define the dependency-free UDP
	// multicast fallback used for discovery before mDNS is layered on top.
	DiscoveryMulticastGroup = "239.42.13.37"
	DiscoveryPort           = 53318
)

// Platform identifies the operating system of a device.
type Platform string

const (
	PlatformWindows Platform = "windows"
	PlatformMacOS   Platform = "darwin"
	PlatformLinux   Platform = "linux"
	PlatformAndroid Platform = "android"
	PlatformIOS     Platform = "ios"
	PlatformWeb     Platform = "web"
)

// Transfer modes select the data plane after prepare-upload accepts.
const (
	TransferTCPPush    = "tcp-push"
	TransferHTTPUpload = "http-upload"
	TransferHTTPPull   = "http-pull" // phase 2
)

// Capability strings advertised in DeviceInfo.Capabilities.
const (
	CapTCPPush    = "tcp-push"
	CapHTTPUpload = "http-upload"
	CapHTTPPull   = "http-pull" // phase 2
)

// DeviceInfo describes a peer on the local network. It is advertised over
// discovery and returned by the control-plane /info endpoint.
type DeviceInfo struct {
	ID   string   `json:"id"`
	Name string   `json:"name"`
	// Host is the machine hostname (not routable).
	Host string `json:"host"`
	// Address is the device's IP address on the LAN. For a discovered peer it
	// is the source address observed in its announcement (most reliable); for
	// the local device it is the primary outbound IP.
	Address     string   `json:"address"`
	Platform    Platform `json:"platform"`
	ControlPort int      `json:"controlPort"`
	Fingerprint string   `json:"fingerprint"`
	Version     int      `json:"version"`
	// Capabilities lists supported data-plane modes (e.g. tcp-push, http-upload).
	Capabilities []string `json:"capabilities,omitempty"`
	// Browser is set for platform web clients (parsed from User-Agent).
	Browser string `json:"browser,omitempty"`
}

// PresenceRequest is sent by a browser client to appear in the desktop grid.
type PresenceRequest struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PresenceResponse is returned after a successful presence heartbeat.
type PresenceResponse struct {
	Token string `json:"token"`
}

// FileMeta describes a single file queued for transfer.
type FileMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// RelPath is the destination path relative to the receive folder, using
	// forward slashes (e.g. "project/src/main.go"). Empty means Name only.
	RelPath string `json:"relPath,omitempty"`
	Size    int64  `json:"size"`
	MIME    string `json:"mime"`
}

// SendItem is one file the sender chose to transfer.
type SendItem struct {
	Path    string `json:"path"`
	RelPath string `json:"relPath"`
}

// PrepareUploadRequest is sent by a sender to ask a receiver to accept files.
type PrepareUploadRequest struct {
	Sender DeviceInfo `json:"sender"`
	Files  []FileMeta `json:"files"`
}

// PrepareUploadResponse is returned when a receiver accepts an upload. Mode
// selects the data plane: tcp-push uses DataPort; http-upload uses UploadPath.
type PrepareUploadResponse struct {
	SessionID  string `json:"sessionId"`
	Mode       string `json:"mode"`
	DataPort   int    `json:"dataPort,omitempty"`
	UploadPath string `json:"uploadPath,omitempty"`
	Token      string `json:"token"`
}

// PullOffer is shown to a browser client when the desktop has staged a send.
type PullOffer struct {
	SessionID string     `json:"sessionId"`
	Sender    DeviceInfo `json:"sender"`
	Files     []FileMeta `json:"files"`
	TotalSize int64      `json:"totalSize"`
	Count     int      `json:"count"`
}

// DownloadFile describes one HTTP pull target after the browser accepts.
type DownloadFile struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RelPath      string `json:"relPath,omitempty"`
	Size         int64  `json:"size"`
	DownloadPath string `json:"downloadPath"`
}

// PullAcceptResponse is returned when a browser accepts a desktop pull offer.
type PullAcceptResponse struct {
	SessionID string         `json:"sessionId"`
	Mode      string         `json:"mode"`
	Token     string         `json:"token"`
	Files     []DownloadFile `json:"files,omitempty"`
	// ArchivePath is set for multi-file / directory sends (single .zip download).
	ArchivePath string `json:"archivePath,omitempty"`
	ArchiveName string `json:"archiveName,omitempty"`
	ArchiveSize int64  `json:"archiveSize,omitempty"`
}

// ChatMessage is a short text message (e.g. a link or note) sent over the
// control plane. Text is opaque UTF-8 and is never interpreted as markup or a
// command by the receiver.
type ChatMessage struct {
	Sender DeviceInfo `json:"sender"`
	Text   string     `json:"text"`
	// Ts is the sender's send time (unix milliseconds). Both endpoints key a
	// message on this same value so read receipts can refer to it unambiguously.
	Ts int64 `json:"ts"`
}

// ReadReceipt tells the original sender that the reader has seen that sender's
// messages up to and including UpToTs (the sender's send timestamps).
type ReadReceipt struct {
	Reader DeviceInfo `json:"reader"`
	UpToTs int64      `json:"upToTs"`
}

// WebChatPollResponse is returned when a browser polls for desktop→browser chat.
type WebChatPollResponse struct {
	Messages []ChatMessage `json:"messages"`
	// ReadUpTo is the newest incoming message ts the desktop has read (read receipt).
	ReadUpTo int64 `json:"readUpTo"`
}
