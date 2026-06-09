package overlay

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/quic-go/quic-go"

	"swoop/core/transport"
)

const quicALPN = "swoop-overlay-v1"

func listenQUIC(udpConn *net.UDPConn, cert tls.Certificate) (*quic.Listener, error) {
	tr := &quic.Transport{Conn: udpConn}
	tlsConf := &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{quicALPN},
	}
	return tr.Listen(tlsConf, &quic.Config{
		MaxIdleTimeout:  5 * time.Minute,
		KeepAlivePeriod: 15 * time.Second,
	})
}

func dialQUIC(ctx context.Context, udpConn *net.UDPConn, addr string, fingerprint string) (quic.Connection, error) {
	tr := &quic.Transport{Conn: udpConn}
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{quicALPN},
		VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
			return transport.VerifyFingerprint(rawCerts, fingerprint)
		},
	}
	raddr, err := net.ResolveUDPAddr("udp4", addr)
	if err != nil {
		return nil, err
	}
	return tr.Dial(ctx, raddr, tlsConf, &quic.Config{
		MaxIdleTimeout:  5 * time.Minute,
		KeepAlivePeriod: 15 * time.Second,
	})
}

func yamuxOverQUIC(ctx context.Context, qconn quic.Connection, asServer bool) (*yamux.Session, error) {
	if asServer {
		stream, err := qconn.AcceptStream(ctx)
		if err != nil {
			return nil, err
		}
		return yamux.Server(stream, newYamuxConfig())
	}
	stream, err := qconn.OpenStreamSync(ctx)
	if err != nil {
		return nil, err
	}
	return yamux.Client(stream, newYamuxConfig())
}

func bindQUICUDP() (*net.UDPConn, int, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, 0, err
	}
	return conn, conn.LocalAddr().(*net.UDPAddr).Port, nil
}
