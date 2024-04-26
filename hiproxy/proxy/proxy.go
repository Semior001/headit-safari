// Package proxy contains services and types for the proxy server.
package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/AdguardTeam/gomitmproxy/mitm"
)

// Rules is a set of rules for the specified base URL regex.
type Rules []Rule

// Rule specifies what to do with the request.
type Rule struct {
	// Host is a regex to match the request host.
	Host string `json:"host"`

	// AddHeaders specifies headers to add to the request.
	AddHeaders map[string]string `json:"add_headers"`
}

// LoadCert loads a certificate and key from the specified location.
// Returns os.ErrNotExist if certificate or key file does not exist.
func LoadCert(location string) (tls.Certificate, error) {
	fileExists := func(loc string) bool {
		_, err := os.Stat(loc)
		return err == nil
	}

	certLoc, keyLoc := filepath.Join(location, "cert.crt"), filepath.Join(location, "key.pem")

	if !fileExists(certLoc) || !fileExists(keyLoc) {
		if err := os.Remove(certLoc); err != nil && !os.IsNotExist(err) {
			return tls.Certificate{}, fmt.Errorf("remove old cert.crt: %w", err)
		}
		if err := os.Remove(keyLoc); err != nil && !os.IsNotExist(err) {
			return tls.Certificate{}, fmt.Errorf("remove old key.pem: %w", err)
		}
		if err := GenerateCert(location); err != nil {
			return tls.Certificate{}, fmt.Errorf("generate cert: %w", err)
		}
	}

	cert, err := tls.LoadX509KeyPair(certLoc, keyLoc)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("load x509 key pair: %w", err)
	}

	return cert, nil
}

// GenerateCert saves a self-signed certificate and key to the specified location.
func GenerateCert(location string) error {
	cert, key, err := mitm.NewAuthority("headit", "Semior001", time.Hour*24*365)
	if err != nil {
		return fmt.Errorf("make new authority: %w", err)
	}

	// save cert and key to the specified location
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	f, err := os.Create(path.Join(location, "cert.crt"))
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}
	defer f.Close()

	if _, err = f.Write(certPEM); err != nil {
		return fmt.Errorf("write cert file: %w", err)
	}

	f, err = os.Create(path.Join(location, "key.pem"))
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}
	defer f.Close()

	if _, err = f.Write(keyPEM); err != nil {
		return fmt.Errorf("write key file: %w", err)
	}

	return nil
}
