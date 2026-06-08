// Package invite implements the signed SwoopInvite blob used to pair devices
// over the internet without a central relay (out-of-band transfer via QR/PNG/file).
package invite

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"swoop/core/identity"
	"swoop/core/protocol"
)

const (
	formatVersion = 1
	defaultTTL    = 15 * time.Minute
)

// Bundle is returned to UIs after generating an invite.
type Bundle struct {
	Blob       string `json:"blob"`
	ShortCode  string `json:"shortCode"`
	ExpiresAt  int64  `json:"expiresAt"`
	DeviceName string `json:"deviceName"`
}

// Parsed is a verified invite ready for pairing (connection step is separate).
type Parsed struct {
	Device    protocol.DeviceInfo `json:"device"`
	ShortCode string              `json:"shortCode"`
	ExpiresAt int64               `json:"expiresAt"`
	SessionID string              `json:"sessionId"`
}

type payload struct {
	V   int    `json:"v"`
	Exp int64  `json:"exp"`
	Sid string `json:"sid"`
	ID  string `json:"id"`
	N   string `json:"n"`
	A   string `json:"a"`
	P   int    `json:"p"`
	F   string `json:"f"`
	Pl  string `json:"pl"`
	C   string `json:"c"` // base64url DER of signer cert (for signature verify)
}

type document struct {
	P payload `json:"p"`
	S string  `json:"s"`
}

var (
	ErrExpired = errors.New("invite expired")
	ErrInvalid = errors.New("invalid invite")
	ErrBadSig  = errors.New("invite signature invalid")
)

// Create builds a signed invite for the given device identity and advertised self info.
func Create(id *identity.Identity, self protocol.DeviceInfo, ttl time.Duration) (Bundle, error) {
	if id == nil || len(id.Certificate.Certificate) == 0 {
		return Bundle{}, ErrInvalid
	}
	if ttl <= 0 {
		ttl = defaultTTL
	}
	sidRaw := make([]byte, 16)
	if _, err := rand.Read(sidRaw); err != nil {
		return Bundle{}, err
	}
	sid := hex.EncodeToString(sidRaw)
	exp := time.Now().Add(ttl).Unix()

	p := payload{
		V:   formatVersion,
		Exp: exp,
		Sid: sid,
		ID:  self.ID,
		N:   self.Name,
		A:   self.Address,
		P:   self.ControlPort,
		F:   self.Fingerprint,
		Pl:  string(self.Platform),
		C:   base64.RawURLEncoding.EncodeToString(id.Certificate.Certificate[0]),
	}
	sig, err := signPayload(id, p)
	if err != nil {
		return Bundle{}, err
	}
	doc := document{P: p, S: sig}
	raw, err := json.Marshal(doc)
	if err != nil {
		return Bundle{}, err
	}
	blob := base64.RawURLEncoding.EncodeToString(raw)
	return Bundle{
		Blob:       blob,
		ShortCode:  shortCodeFromSession(sid),
		ExpiresAt:  exp,
		DeviceName: self.Name,
	}, nil
}

// ParseAndVerify decodes a base64url invite blob and verifies signature and expiry.
func ParseAndVerify(blob string) (Parsed, error) {
	blob = strings.TrimSpace(blob)
	if blob == "" {
		return Parsed{}, ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(blob)
	if err != nil {
		return Parsed{}, ErrInvalid
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return Parsed{}, ErrInvalid
	}
	if doc.P.V != formatVersion || doc.P.Sid == "" || doc.P.F == "" || doc.P.C == "" {
		return Parsed{}, ErrInvalid
	}
	if time.Now().Unix() > doc.P.Exp {
		return Parsed{}, ErrExpired
	}
	msg, err := json.Marshal(doc.P)
	if err != nil {
		return Parsed{}, ErrInvalid
	}
	sig, err := base64.RawURLEncoding.DecodeString(doc.S)
	if err != nil {
		return Parsed{}, ErrInvalid
	}
	if err := verifyPayload(msg, sig, doc.P); err != nil {
		return Parsed{}, err
	}
	return Parsed{
		Device: protocol.DeviceInfo{
			ID:          doc.P.ID,
			Name:        doc.P.N,
			Address:     doc.P.A,
			ControlPort: doc.P.P,
			Fingerprint: doc.P.F,
			Platform:    protocol.Platform(doc.P.Pl),
			Version:     protocol.Version,
		},
		ShortCode: shortCodeFromSession(doc.P.Sid),
		ExpiresAt: doc.P.Exp,
		SessionID: doc.P.Sid,
	}, nil
}

// FileContent returns the canonical .swoopinvite file body.
func FileContent(bundle Bundle) string {
	var b strings.Builder
	b.WriteString("# SwoopInvite v1\n")
	b.WriteString(bundle.Blob)
	b.WriteByte('\n')
	b.WriteString(bundle.ShortCode)
	b.WriteByte('\n')
	return b.String()
}

// BlobFromFile extracts the invite blob from a .swoopinvite file or raw text.
func BlobFromFile(data []byte) string {
	text := strings.TrimSpace(string(data))
	if strings.HasPrefix(text, "# SwoopInvite") {
		lines := strings.Split(text, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if strings.Contains(line, "-") && len(line) <= 12 && !strings.Contains(line, "_") {
				continue // short code line
			}
			return line
		}
	}
	// Raw blob or JSON line
	if idx := strings.Index(text, "\n"); idx > 0 {
		return strings.TrimSpace(text[:idx])
	}
	return text
}

func signPayload(id *identity.Identity, p payload) (string, error) {
	msg, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	key, ok := id.Certificate.PrivateKey.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("identity key is not ECDSA")
	}
	sum := sha256.Sum256(msg)
	sig, err := ecdsa.SignASN1(rand.Reader, key, sum[:])
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(sig), nil
}

func verifyPayload(msg, sig []byte, p payload) error {
	certDER, err := base64.RawURLEncoding.DecodeString(p.C)
	if err != nil {
		return ErrInvalid
	}
	sum := sha256.Sum256(certDER)
	wantFP := "sha256:" + hex.EncodeToString(sum[:])
	if p.F != wantFP {
		return ErrBadSig
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		return ErrInvalid
	}
	pub, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return ErrBadSig
	}
	hash := sha256.Sum256(msg)
	if !ecdsa.VerifyASN1(pub, hash[:], sig) {
		return ErrBadSig
	}
	return nil
}

func shortCodeFromSession(sid string) string {
	sum := sha256.Sum256([]byte(sid))
	h := strings.ToUpper(hex.EncodeToString(sum[:4]))
	if len(h) < 8 {
		return h
	}
	return h[:4] + "-" + h[4:8]
}
