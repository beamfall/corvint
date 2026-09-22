package attest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func writePKCS8PEM(t *testing.T, dir, name string, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	block := &pem.Block{Type: "PRIVATE KEY", Bytes: der}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	return path
}

func TestReadPrivateKeyRoundTrip(t *testing.T) {
	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	path := writePKCS8PEM(t, t.TempDir(), "key.pem", privateKey)

	got, err := ReadPrivateKey(path)
	if err != nil {
		t.Fatalf("ReadPrivateKey: %v", err)
	}
	message := []byte("sign me")
	signature := ed25519.Sign(got, message)
	if !ed25519.Verify(publicKey, message, signature) {
		t.Error("key read back from PEM does not sign correctly")
	}
	if !bytes.Equal(got, privateKey) {
		t.Error("key read back from PEM does not equal the original key bytes")
	}
}

func TestReadPrivateKeyRejectsRSAKey(t *testing.T) {
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa.GenerateKey: %v", err)
	}
	path := writePKCS8PEM(t, t.TempDir(), "rsa.pem", rsaKey)

	if _, err := ReadPrivateKey(path); err == nil {
		t.Fatal("ReadPrivateKey accepted an RSA key")
	}
}

func TestReadPrivateKeyRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.pem")
	oversized := bytes.Repeat([]byte("A"), maxKeyFileSize+1)
	if err := os.WriteFile(path, oversized, 0o600); err != nil {
		t.Fatalf("write oversized file: %v", err)
	}
	if _, err := ReadPrivateKey(path); err == nil {
		t.Fatal("ReadPrivateKey accepted a file over 16 KiB")
	}
}

// TestReadPrivateKeySizeBoundIsInclusive checks the FPK-V0-015 bound: a valid
// key padded with trailing whitespace to exactly 16 KiB is read, and one byte
// more is refused even though its first 16 KiB would parse.
func TestReadPrivateKeySizeBoundIsInclusive(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	dir := t.TempDir()
	for _, size := range []int{maxKeyFileSize, maxKeyFileSize + 1} {
		path := filepath.Join(dir, "padded.pem")
		padded := append(append([]byte{}, encoded...), bytes.Repeat([]byte("\n"), size-len(encoded))...)
		if err := os.WriteFile(path, padded, 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := ReadPrivateKey(path)
		if accepted, want := err == nil, size <= maxKeyFileSize; accepted != want {
			t.Errorf("%d-byte key file: accepted=%t, want %t (%v)", size, accepted, want, err)
		}
	}
}

func TestReadPrivateKeyRejectsMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.pem")
	if _, err := ReadPrivateKey(path); err == nil {
		t.Fatal("ReadPrivateKey accepted a missing file")
	}
}

func TestReadPrivateKeyRejectsDataAfterTheBlock(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	path := filepath.Join(t.TempDir(), "trailing.pem")
	if err := os.WriteFile(path, append(encoded, []byte("garbage\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivateKey(path); err == nil {
		t.Fatal("a key file with trailing data was accepted")
	}
}

// TestReadPrivateKeyRejectsWrongPEMBlockType checks that a PKCS8 body wrapped
// in a PEM block under another label (here what ReadPublicKey would write) is
// refused: pem.Decode does not itself check the label against the caller's
// expected block type.
func TestReadPrivateKeyRejectsWrongPEMBlockType(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	block := &pem.Block{Type: "PUBLIC KEY", Bytes: der}
	path := filepath.Join(t.TempDir(), "mislabeled.pem")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPrivateKey(path); err == nil {
		t.Fatal("ReadPrivateKey accepted a PKCS8 body under a \"PUBLIC KEY\" block label")
	}
}

// TestReadPublicKeyRejectsDataBeforeTheBlock checks that pem.Decode's silent
// skip of leading bytes, here a malformed second BEGIN line, is refused.
func TestReadPublicKeyRejectsDataBeforeTheBlock(t *testing.T) {
	publicKey, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		t.Fatal(err)
	}
	leading := []byte("-----BEGIN PRIVATE KEY-----\nnot base64\n")
	path := filepath.Join(t.TempDir(), "leading.pem")
	if err := os.WriteFile(path, append(leading, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der})...), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPublicKey(path); err == nil {
		t.Fatal("a key file with data before the block was accepted")
	}
}
