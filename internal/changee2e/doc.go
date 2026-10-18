// Package changee2e is the experimental CCE-V0 change-compiled E2E slice behind the
// `corvint-test e2e` companion group: a revision-pinned block catalog, diff obligations,
// deterministic scenario compilation from admitted block text, and an opt-in live proof.
// Core `corvint` does not import it (CCE-V0-015).
package changee2e

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
)

// Schemas, compiler identity and the declared-blocks path (CCE-V0-001, -003, -007).
const (
	CatalogSchema      = "e2e-block-catalog/0"
	DeclarationsSchema = "e2e-block-declarations/0"
	ObligationsSchema  = "e2e-obligations/0"
	CompileSchema      = "e2e-compile/0"
	ProveSchema        = "e2e-prove/0"
	CompilerVersion    = "corvint-test-e2e-compile/0"
	DeclarationsPath   = ".corvint/e2e-blocks.json"
)

// Limits from Trust boundary, limits and failure modes; each refuses with a named code.
const (
	MaxObligations = 64
	MaxScenarios   = 16
	MaxRepeats     = 100
	DefaultRepeats = 5
	maxGitBytes    = 32 << 20
)

// Refusal is a named, fail-closed refusal (for example `e2e-out-not-allowed`).
type Refusal struct {
	Code   string
	Detail string
}

func (r *Refusal) Error() string {
	if r.Detail == "" {
		return r.Code
	}
	return r.Code + ": " + r.Detail
}

func refuse(code, format string, args ...any) error {
	return &Refusal{Code: code, Detail: fmt.Sprintf(format, args...)}
}

// encode renders v as indented JSON without HTML escaping, ending in a newline.
func encode(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// seal sets a document's digest to SHA-256 over its encoding with the digest left empty
// (the ANL-V0-005 convention, CCE-V0-007) and returns the final bytes.
func seal(doc any, digest *string) ([]byte, error) {
	*digest = ""
	data, err := encode(doc)
	if err != nil {
		return nil, err
	}
	*digest = "sha256:" + sha256Hex(data)
	return encode(doc)
}

// CheckOut resolves an --out path and refuses it unless it lies outside the repository root
// (CCE-V0-001); `.git/` and `.corvint/` are inside that root.
func CheckOut(root, out string) (string, error) {
	if strings.TrimSpace(out) == "" {
		return "", refuse("e2e-out-not-allowed", "empty --out path")
	}
	absRoot, err := resolveExisting(root)
	if err != nil {
		return "", err
	}
	absOut, err := resolveExisting(out)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(absRoot, absOut); err != nil || rel == "." || (!strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != "..") {
		return "", refuse("e2e-out-not-allowed", "%s is inside the repository root", out)
	}
	return absOut, nil
}

// resolveExisting makes p absolute and resolves symlinks through its deepest existing ancestor.
func resolveExisting(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	rest := []string{}
	cur := abs
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			return filepath.Join(append([]string{resolved}, rest...)...), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return abs, nil
		}
		rest = append([]string{filepath.Base(cur)}, rest...)
		cur = parent
	}
}

// git runs one bounded git read in root with the isolated environment BBF uses.
func git(ctx context.Context, root string, args ...string) ([]byte, error) {
	return gitrun.Run(ctx, gitrun.NewBudget(1, 60*time.Second), gitrun.Options{
		Dir: root, Env: []string{"HOME=" + os.TempDir(), "PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C", "TZ=UTC",
			"GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_TERMINAL_PROMPT=0"},
		StdoutLimit: maxGitBytes,
	}, args...)
}

// resolveRev returns the full commit OID for rev.
func resolveRev(ctx context.Context, root, rev string) (string, error) {
	if strings.TrimSpace(rev) == "" || strings.HasPrefix(rev, "-") {
		return "", refuse("e2e-revision-invalid", "revision %q", rev)
	}
	out, err := git(ctx, root, "rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return "", refuse("e2e-revision-invalid", "revision %q does not resolve", rev)
	}
	return strings.TrimSpace(string(out)), nil
}

// blobAt reads path at rev; ok is false when the path is absent there.
func blobAt(ctx context.Context, root, rev, path string) (data []byte, oid string, ok bool, err error) {
	out, err := git(ctx, root, "ls-tree", "-z", "--full-tree", rev, "--", path)
	if err != nil {
		return nil, "", false, err
	}
	fields := strings.Fields(strings.TrimRight(string(out), "\x00"))
	if len(fields) < 3 || fields[1] != "blob" {
		return nil, "", false, nil
	}
	data, err = git(ctx, root, "cat-file", "blob", fields[2])
	return data, fields[2], err == nil, err
}

var errNoRoot = errors.New("e2e-root-required")
