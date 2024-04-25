package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path"
)

func generateCert(loc string) error {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate rsa key: %w", err)
	}

	keyUsage := x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return fmt.Errorf("generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"HeadIt-Safari-Proxy"}},
		KeyUsage:              keyUsage,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	cert, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	certOut, err := os.Create(path.Join(loc, "cert.pem"))
	if err != nil {
		return fmt.Errorf("create cert file: %w", err)
	}

	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert}); err != nil {
		return fmt.Errorf("encode cert: %w", err)
	}

	if err := certOut.Close(); err != nil {
		return fmt.Errorf("close cert file: %w", err)
	}

	keyOut, err := os.Create(path.Join(loc, "key.pem"))
	if err != nil {
		return fmt.Errorf("create key file: %w", err)
	}

	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal private key: %w", err)
	}

	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("encode private key: %w", err)
	}

	if err := keyOut.Close(); err != nil {
		return fmt.Errorf("close key file: %w", err)
	}

	return nil
}
