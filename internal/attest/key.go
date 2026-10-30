package attest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
)

// maxKeyFileSize bounds how much of a key file ReadPrivateKey will read.
const maxKeyFileSize = 16 * 1024

// ReadPrivateKey reads an Ed25519 private key from the PEM file at path. The
// file must contain a single PKCS#8 "PRIVATE KEY" block, the format
// `openssl genpkey -algorithm ed25519` produces. ReadPrivateKey never writes
// to path, and rejects any file larger than 16 KiB or any key type other
// than Ed25519.
func ReadPrivateKey(path string) (ed25519.PrivateKey, error) {
	der, err := readKeyBlock(path, "PRIVATE KEY")
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("attest: parse PKCS8 key: %w", err)
	}
	ed25519Key, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("attest: key is %T, want ed25519.PrivateKey", key)
	}
	return ed25519Key, nil
}

// ReadPublicKey reads an Ed25519 public key from the PEM file at path under the
// same bounds as ReadPrivateKey: a single PKIX "PUBLIC KEY" block, the format
// `openssl pkey -pubout` produces, in a file of at most 16 KiB.
func ReadPublicKey(path string) (ed25519.PublicKey, error) {
	der, err := readKeyBlock(path, "PUBLIC KEY")
	if err != nil {
		return nil, err
	}
	key, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("attest: parse PKIX key: %w", err)
	}
	ed25519Key, ok := key.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("attest: key is %T, want ed25519.PublicKey", key)
	}
	return ed25519Key, nil
}

// readKeyBlock returns the DER bytes of the single PEM block of blockType in
// the file at path, refusing a file over maxKeyFileSize or data before or
// after the block.
func readKeyBlock(path, blockType string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("attest: open key file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxKeyFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("attest: read key file: %w", err)
	}
	if len(data) > maxKeyFileSize {
		return nil, fmt.Errorf("attest: key file exceeds %d bytes", maxKeyFileSize)
	}

	// pem.Decode silently skips anything before the block it returns, a
	// malformed earlier block included, so exactly one BEGIN line must open
	// the file.
	if !bytes.HasPrefix(bytes.TrimLeft(data, " \t\r\n"), []byte("-----BEGIN ")) || bytes.Count(data, []byte("-----BEGIN ")) != 1 {
		return nil, errors.New("attest: key file carries data before the PEM block")
	}
	block, rest := pem.Decode(data)
	if block == nil {
		return nil, errors.New("attest: key file does not contain a PEM block")
	}
	if len(bytes.TrimSpace(rest)) != 0 {
		return nil, errors.New("attest: key file carries data after the PEM block")
	}
	if block.Type != blockType {
		return nil, fmt.Errorf("attest: key file has PEM block type %q, want %q", block.Type, blockType)
	}
	return block.Bytes, nil
}
