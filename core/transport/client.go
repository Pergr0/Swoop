package transport

import (
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"net/http"
	"time"
)

// NewPinnedClient builds an HTTPS client that trusts a peer purely by its
// certificate fingerprint (TOFU), matching the rest of the control plane. The
// self-signed cert is otherwise not validated.
func NewPinnedClient(fingerprint string, timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true, // trust is pinned by fingerprint below
				VerifyPeerCertificate: func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
					return VerifyFingerprint(rawCerts, fingerprint)
				},
			},
		},
	}
}

// VerifyFingerprint checks that the peer's leaf certificate matches the
// expected "sha256:<hex>" fingerprint. Empty expected is rejected (fail closed).
func VerifyFingerprint(rawCerts [][]byte, expected string) error {
	if len(rawCerts) == 0 {
		return errors.New("no peer certificate")
	}
	if expected == "" {
		return errors.New("missing peer fingerprint")
	}
	sum := sha256.Sum256(rawCerts[0])
	if "sha256:"+hex.EncodeToString(sum[:]) != expected {
		return errors.New("fingerprint mismatch")
	}
	return nil
}
