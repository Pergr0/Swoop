// Package identity manages a device's stable identity and TLS material.
// On first run it generates a keypair and a self-signed certificate; the
// certificate fingerprint is what peers use to establish trust (TOFU).
package identity

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	crand "crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

const (
	certFile = "device-cert.pem"
	keyFile  = "device-key.pem"
	idFile   = "device-id"
)

// Identity holds the stable device identity and its TLS material.
type Identity struct {
	DeviceID    string
	Name        string
	Fingerprint string
	Certificate tls.Certificate
}

// LoadOrCreate loads identity material from dir, generating it on first run.
func LoadOrCreate(dir, name string) (*Identity, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	certPath := filepath.Join(dir, certFile)
	keyPath := filepath.Join(dir, keyFile)
	idPath := filepath.Join(dir, idFile)

	if _, err := os.Stat(certPath); err != nil {
		if err := generate(certPath, keyPath, idPath); err != nil {
			return nil, err
		}
	}

	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	idBytes, err := os.ReadFile(idPath)
	if err != nil {
		return nil, err
	}

	sum := sha256.Sum256(cert.Certificate[0])
	return &Identity{
		DeviceID:    string(idBytes),
		Name:        name,
		Fingerprint: "sha256:" + hex.EncodeToString(sum[:]),
		Certificate: cert,
	}, nil
}

func generate(certPath, keyPath, idPath string) error {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), crand.Reader)
	if err != nil {
		return err
	}

	idRaw := make([]byte, 16)
	if _, err := crand.Read(idRaw); err != nil {
		return err
	}
	deviceID := hex.EncodeToString(idRaw)

	serial, err := crand.Int(crand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	tmpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "swoop-" + deviceID},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}
	der, err := x509.CreateCertificate(crand.Reader, &tmpl, &tmpl, &priv.PublicKey, priv)
	if err != nil {
		return err
	}

	certOut, err := os.OpenFile(certPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: der}); err != nil {
		return err
	}

	keyBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer keyOut.Close()
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes}); err != nil {
		return err
	}

	return os.WriteFile(idPath, []byte(deviceID), 0o600)
}
