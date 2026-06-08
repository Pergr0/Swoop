package pairing

import (
	"fmt"
	"net"
)

// ListenPunchUDP binds an ephemeral UDP port for NAT punch coordination.
func ListenPunchUDP() (*net.UDPConn, int, error) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return nil, 0, err
	}
	port := conn.LocalAddr().(*net.UDPAddr).Port
	if port <= 0 {
		conn.Close()
		return nil, 0, fmt.Errorf("invalid punch port")
	}
	return conn, port, nil
}
