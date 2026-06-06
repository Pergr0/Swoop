package webpresence

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net"
)

func newPresenceSecret() []byte {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return b
}

func presenceToken(secret []byte, clientID, remoteIP string) string {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(clientID))
	_, _ = mac.Write([]byte{0})
	_, _ = mac.Write([]byte(remoteIP))
	return hex.EncodeToString(mac.Sum(nil))
}

func verifyPresenceToken(secret []byte, clientID, remoteIP, token string) bool {
	if token == "" || clientID == "" || remoteIP == "" {
		return false
	}
	want := presenceToken(secret, clientID, remoteIP)
	return hmac.Equal([]byte(token), []byte(want))
}

func remoteHost(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}
	return host
}
