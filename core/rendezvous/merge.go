package rendezvous

import "swoop/core/invite"

// ApplyHostInfo fills reach/reflexive endpoints on a parsed invite for P2P dial.
// Existing UPnP reach from the signed invite blob is kept when the rendezvous
// host record has no fresher reach (avoids clobbering with HTTP reflexive only).
func ApplyHostInfo(parsed invite.Parsed, host HostInfo) invite.Parsed {
	out := parsed
	if host.ReachAddr != "" && host.ControlPort > 0 {
		out.ReachAddr = host.ReachAddr
		out.ReachPort = host.ControlPort
	} else if !out.HasReach() && host.ReflexiveAddr != "" && host.ControlPort > 0 {
		out.ReachAddr = host.ReflexiveAddr
		out.ReachPort = host.ControlPort
	}
	if host.PunchPort > 0 {
		out.PunchPort = host.PunchPort
	}
	return out
}
