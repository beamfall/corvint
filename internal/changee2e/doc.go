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
	CompilerVersion    = "corvint-test-e2e-compile/1"
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

// mapUnavailableHint names, in an e2e-map-unavailable refusal, the command that checks the
// manifest and the manifest's schema and spec (CCE-V0-119). The refusal code is unchanged.
const mapUnavailableHint = "the e2e commands build the map from the committed application-map-manifest/0 at --manifest " +
	"(default appmap.json; schema AMAP-V0-001, docs/specs/application-map-v0.md), not a built application-map/0: the e2e commands build the map at each revision; check it with " +
	"`corvint flows appmap build --manifest FILE --revision REV`; guide docs/E2E-ADOPTION.md"

// refuseMapUnavailable refuses e2e-map-unavailable for a manifest that does not build at a
// revision, with the side ("" or "base: ") and the hint naming the producing command.
func refuseMapUnavailable(side string, err error) error {
	return refuse("e2e-map-unavailable", "%s%v; %s", side, err, mapUnavailableHint)
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

// DocumentByteCap is the CCE-V0-014 default byte cap of every emitted document.
const DocumentByteCap = 1 << 20

// docByteCap is the cap in force; tests lower it.
var docByteCap = DocumentByteCap

// capList encodes a document and, when it exceeds the byte cap, keeps the longest prefix of
// *items that fits, recording the dropped count in *truncated (CCE-V0-014, CCE-V0-040). Every
// verdict, state and count is computed before this call, so truncation never changes them. A
// document that does not fit even with no items refuses `e2e-document-over-cap`.
func capList[T any](items *[]T, truncated *int, encode func() ([]byte, error)) ([]byte, error) {
	*truncated = 0
	data, err := encode()
	if err != nil || len(data) <= docByteCap {
		return data, err
	}
	all := *items
	fits := func(k int) ([]byte, bool, error) {
		*items, *truncated = all[:k], len(all)-k
		b, err := encode()
		return b, err == nil && len(b) <= docByteCap, err
	}
	// The largest k in [0, len(all)) whose encoding fits; encodings grow with k.
	lo, hi := 0, len(all)-1
	best := -1
	for lo <= hi {
		mid := lo + (hi-lo)/2
		_, ok, err := fits(mid)
		if err != nil {
			return nil, err
		}
		if ok {
			best, lo = mid, mid+1
		} else {
			hi = mid - 1
		}
	}
	if best < 0 {
		*items, *truncated = all, 0
		return nil, refuse("e2e-document-over-cap", "document exceeds %d bytes with no entries", docByteCap)
	}
	data, _, err = fits(best)
	return data, err
}
