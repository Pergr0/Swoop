package pairing

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"strconv"
	"time"

	"swoop/core/invite"
)

const punchMagic = "SWOOPUNCH1"

type punchHello struct {
	Magic     string `json:"m"`
	SessionID string `json:"sid"`
	PeerID    string `json:"id"`
}

type punchAck struct {
	Magic     string `json:"m"`
	SessionID string `json:"sid"`
}

// RunPunchHost listens for importer UDP hellos until ctx is cancelled.
// Replies with ACK to punch NAT holes (no central server).
func RunPunchHost(ctx context.Context, conn *net.UDPConn, sessionID string, logf func(string, ...any)) {
	if conn == nil || sessionID == "" {
		return
	}
	buf := make([]byte, 512)
	for {
		if err := ctx.Err(); err != nil {
			return
		}
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			if logf != nil {
				logf("punch host read: %v", err)
			}
			continue
		}
		var hello punchHello
		if err := json.Unmarshal(buf[:n], &hello); err != nil || hello.Magic != punchMagic || hello.SessionID != sessionID {
			continue
		}
		if logf != nil {
			logf("punch host: hello from %s peer=%s", addr, hello.PeerID)
		}
		ack, _ := json.Marshal(punchAck{Magic: punchMagic, SessionID: sessionID})
		for i := 0; i < 5; i++ {
			_, _ = conn.WriteToUDP(ack, addr)
			time.Sleep(80 * time.Millisecond)
		}
	}
}

// ClientPunch sends UDP hellos to the inviter's punch port (public + LAN fallback).
func ClientPunch(ctx context.Context, parsed invite.Parsed, localPeerID string) error {
	if parsed.PunchPort <= 0 || parsed.SessionID == "" {
		return nil
	}
	targets := punchTargets(parsed)
	if len(targets) == 0 {
		return errors.New("no punch targets")
	}
	hello, err := json.Marshal(punchHello{
		Magic:     punchMagic,
		SessionID: parsed.SessionID,
		PeerID:    localPeerID,
	})
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return err
	}
	defer conn.Close()

	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(6 * time.Second)
	}

	ackCh := make(chan struct{}, 1)
	go readPunchAck(conn, parsed.SessionID, ackCh)

	for time.Now().Before(deadline) {
		for _, addr := range targets {
			_, _ = conn.WriteToUDP(hello, addr)
		}
		select {
		case <-ackCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return errors.New("punch timeout")
}

func punchTargets(parsed invite.Parsed) []*net.UDPAddr {
	var out []*net.UDPAddr
	add := func(host string, port int) {
		if host == "" || port <= 0 {
			return
		}
		if addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(host, itoa(port))); err == nil {
			out = append(out, addr)
		}
	}
	if parsed.HasReach() {
		add(parsed.ReachAddr, parsed.PunchPort)
	}
	add(parsed.Device.Address, parsed.PunchPort)
	return out
}

func readPunchAck(conn *net.UDPConn, sessionID string, ackCh chan<- struct{}) {
	buf := make([]byte, 256)
	for {
		_ = conn.SetReadDeadline(time.Now().Add(6 * time.Second))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		var ack punchAck
		if json.Unmarshal(buf[:n], &ack) == nil && ack.Magic == punchMagic && ack.SessionID == sessionID {
			select {
			case ackCh <- struct{}{}:
			default:
			}
			return
		}
	}
}

// SendPunchHello sends punch packets to a remote endpoint (host → joiner reverse punch).
func SendPunchHello(conn *net.UDPConn, sessionID, targetHost string, targetPort int) error {
	if conn == nil || sessionID == "" || targetHost == "" || targetPort <= 0 {
		return errors.New("invalid punch target")
	}
	addr, err := net.ResolveUDPAddr("udp4", net.JoinHostPort(targetHost, itoa(targetPort)))
	if err != nil {
		return err
	}
	hello, err := json.Marshal(punchHello{Magic: punchMagic, SessionID: sessionID, PeerID: "host"})
	if err != nil {
		return err
	}
	for i := 0; i < 5; i++ {
		if _, err := conn.WriteToUDP(hello, addr); err != nil {
			return err
		}
		time.Sleep(80 * time.Millisecond)
	}
	return nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
