package delta

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const MaxProviders = 32
const MaxProviderBytes = 1 << 20
const MaxProviderTotalBytes = 16 << 20
const MaxPreviousBytes = 16 << 20

var errInput = errors.New("delta-input-unavailable")

func capture(root, name string, limit int) ([]byte, error) {
	if name == "" || strings.ContainsAny(name, "\x00\r\n") || strings.Contains(name, "://") || strings.HasPrefix(name, "command:") || strings.HasPrefix(name, "mcp:") {
		return nil, errInput
	}
	if !filepath.IsAbs(name) {
		name = filepath.Join(root, name)
	}
	f, err := openRegular(name)
	if err != nil {
		return nil, errInput
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil || !before.Mode().IsRegular() || before.Size() > int64(limit) {
		return nil, errInput
	}
	raw, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(raw) > limit {
		return nil, errInput
	}
	after, err := f.Stat()
	if err != nil || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return nil, errInput
	}
	return raw, nil
}
func digest(domain string, raw []byte) string {
	sum := sha256.Sum256(append([]byte("corvint-delta/0:"+domain+"\x00"), raw...))
	return hex.EncodeToString(sum[:])
}

// Referencing os.File here keeps the platform implementations a single explicit boundary.
var _ *os.File
