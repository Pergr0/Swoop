package rendezvous

// Enabled reports whether a rendezvous server address is configured.
func Enabled() bool {
	return DefaultServerHost != "" && DefaultServerHost != "YOUR_VPS_HOST"
}
