package worksource

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/cem/wire"
	"github.com/Beamfall/corvint/internal/workqueue"
)

const maxEntries = 100000
const maxBytes = 256 << 20
const maxFileBytes = 32 << 20

// ErrWorktreeNotClean and ErrIndexDiffers let callers name the two ordinary
// uncommitted-state refusals to an operator (WQO-V0-051).
var (
	ErrWorktreeNotClean = errors.New("worktree is not clean")
	ErrIndexDiffers     = errors.New("index differs from pinned tree")
)

// Entry contains the verified raw worktree bytes of a pinned tree blob.
type Entry struct {
	Path, Mode, BlobOID string
	Raw                 []byte
}
type object struct {
	kind string
	raw  []byte
}

// Source owns only its private Git execution environment; Close must be called.
// Identity and Entries describe immutable content, not a write-prevention claim.
type Source struct {
	Identity                         workqueue.RepositorySource
	Root, GitDir, CommonDir, GitPath string
	GitEnvironment                   []string
	Entries                          []Entry
	temporary                        string
	objects                          map[string]object
}

func Acquire(ctx context.Context, root string) (*Source, error) {
	return AcquireWithScratch(ctx, root, "")
}

// AcquireWithScratch nests all acquisition storage beneath an explicitly owned
// private parent. It is an internal producer/observer seam, not an environment
// override: ordinary Acquire never uses ambient TMPDIR.
func AcquireWithScratch(ctx context.Context, root, scratch string) (*Source, error) {
	if err := ValidateScratchLocation(ctx, root, scratch); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	source, err := newSourceWithScratch(scratch)
	if err != nil {
		return nil, err
	}
	if err = source.acquire(ctx, root); err != nil {
		source.Close()
		return nil, err
	}
	return source, nil
}

func (source *Source) acquire(ctx context.Context, root string) error {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	source.Root, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		return err
	}
	layout, err := source.layout(ctx)
	if err != nil {
		return err
	}
	if layout[0] != source.Root || layout[3] != "false" || layout[4] != "false" {
		return errors.New("unsupported repository layout or shallow history")
	}
	source.GitDir, source.CommonDir = layout[1], layout[2]
	if err := source.unsupportedState(ctx); err != nil {
		return err
	}
	identity, err := source.Git(ctx, 4096, "rev-parse", "--show-object-format", "HEAD^{commit}", "HEAD^{tree}")
	if err != nil {
		return err
	}
	fields := strings.Split(strings.TrimSuffix(string(identity), "\n"), "\n")
	if len(fields) != 3 || (fields[0] != "sha1" && fields[0] != "sha256") || !validOID(fields[1], fields[0]) || !validOID(fields[2], fields[0]) {
		return errors.New("invalid source identity")
	}
	source.Identity = workqueue.RepositorySource{ObjectFormat: fields[0], Commit: fields[1], Tree: fields[2]}
	status, index, err := source.indexState(ctx)
	if err != nil {
		return err
	}
	tree, err := source.Git(ctx, 32<<20, "ls-tree", "-r", "-z", "--full-tree", source.Identity.Tree)
	if err != nil {
		return err
	}
	source.Entries, err = parseTree(tree, source.Identity.ObjectFormat)
	if err != nil {
		return err
	}
	if !bytes.Equal(index, canonicalIndex(source.Entries)) {
		return ErrIndexDiffers
	}
	if err := source.readEntries(ctx, source.Root, true); err != nil {
		return err
	}
	if err := source.cacheObjects(ctx); err != nil {
		return err
	}
	source.Identity.StatusSHA256 = workqueue.SHA256Hex(status)
	source.Identity.MaterializationSHA256 = materializationDigest(source.Entries)
	workqueue.RefreshRepositorySource(&source.Identity)
	closing, err := source.Git(ctx, 4096, "rev-parse", "--show-object-format", "HEAD^{commit}", "HEAD^{tree}")
	if err != nil || !bytes.Equal(identity, closing) {
		return errors.New("source identity drift")
	}
	closingLayout, err := source.layout(ctx)
	if err != nil || strings.Join(layout, "\x00") != strings.Join(closingLayout, "\x00") {
		return errors.New("repository layout drift")
	}
	_, closingIndex, err := source.indexState(ctx)
	if err != nil || !bytes.Equal(index, closingIndex) {
		return errors.New("source index drift")
	}
	if err := source.unsupportedState(ctx); err != nil {
		return err
	}
	return source.VerifyMaterialization(ctx, source.Root)
}

func (source *Source) layout(ctx context.Context) ([]string, error) {
	// Discovery must not pin --git-dir: repeat it independently to catch rebinding.
	discovery := *source
	discovery.GitDir = ""
	raw, err := discovery.Git(ctx, 32<<10, "rev-parse", "--path-format=absolute", "--show-toplevel", "--absolute-git-dir", "--git-common-dir", "--is-bare-repository", "--is-shallow-repository")
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
	if len(fields) != 5 {
		return nil, errors.New("invalid repository layout")
	}
	for i := 0; i < 3; i++ {
		fields[i], err = filepath.EvalSymlinks(fields[i])
		if err != nil {
			return nil, err
		}
	}
	return fields, nil
}

func (source *Source) unsupportedState(ctx context.Context) error {
	raw, err := source.Git(ctx, 1<<20, "config", "--local", "--no-includes", "--null", "--list")
	if err != nil {
		return err
	}
	for _, record := range bytes.Split(raw, []byte{0}) {
		key := strings.ToLower(strings.SplitN(string(record), "\n", 2)[0])
		switch key {
		case "core.sparsecheckout", "core.sparsecheckoutcone", "core.splitindex", "extensions.worktreeconfig":
			value := strings.SplitN(string(record), "\n", 2)
			if len(value) != 2 || (value[1] != "false" && value[1] != "0") {
				return fmt.Errorf("unsupported configuration %s", key)
			}
		case "core.worktree", "core.alternaterefscommand":
			return fmt.Errorf("unsupported configuration %s", key)
		}
		if strings.HasPrefix(key, "filter.") {
			return errors.New("unsupported executable Git filter configuration")
		}
		if key == "include.path" || strings.HasPrefix(key, "includeif.") {
			return errors.New("unsupported included Git configuration")
		}
	}
	for _, root := range []string{source.GitDir, source.CommonDir} {
		for _, name := range []string{"info/grafts", "info/sparse-checkout", "objects/info/alternates", "objects/info/http-alternates"} {
			if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(name))); !os.IsNotExist(err) {
				return fmt.Errorf("unsupported Git state %s", name)
			}
		}
		matches, err := filepath.Glob(filepath.Join(root, "sharedindex.*"))
		if err != nil || len(matches) != 0 {
			return errors.New("unsupported split index")
		}
	}
	replacements, err := source.Git(ctx, 1<<20, "for-each-ref", "--format=%(refname)", "refs/replace/")
	if err != nil {
		return err
	}
	if len(replacements) != 0 {
		return errors.New("unsupported replacement objects")
	}
	return nil
}

func (source *Source) indexState(ctx context.Context) ([]byte, []byte, error) {
	status, err := source.Git(ctx, 16<<20, "status", "--porcelain=v2", "-z", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil {
		return nil, nil, err
	}
	if len(status) != 0 {
		return nil, nil, ErrWorktreeNotClean
	}
	flags, err := source.Git(ctx, 32<<20, "ls-files", "-v", "-z")
	if err != nil {
		return nil, nil, err
	}
	if len(flags) > 0 && flags[len(flags)-1] != 0 {
		return nil, nil, errors.New("incomplete index flag output")
	}
	for _, record := range bytes.Split(flags, []byte{0}) {
		if len(record) != 0 && (len(record) < 3 || record[0] != 'H' || record[1] != ' ') {
			return nil, nil, errors.New("unsupported index flags")
		}
	}
	split, err := source.Git(ctx, 8192, "rev-parse", "--shared-index-path")
	if err != nil {
		return nil, nil, err
	}
	if len(bytes.TrimSpace(split)) != 0 {
		return nil, nil, errors.New("unsupported split index")
	}
	index, err := source.Git(ctx, 32<<20, "ls-files", "--stage", "-z")
	return status, index, err
}

func parseTree(raw []byte, format string) ([]Entry, error) {
	if len(raw) > 0 && raw[len(raw)-1] != 0 {
		return nil, errors.New("incomplete tree output")
	}
	entries := make([]Entry, 0)
	folded := make(map[string]string)
	for _, record := range bytes.Split(raw, []byte{0}) {
		if len(record) == 0 {
			continue
		}
		header, path, ok := bytes.Cut(record, []byte{'\t'})
		fields := strings.Split(string(header), " ")
		if !ok || len(fields) != 3 || fields[1] != "blob" || !validOID(fields[2], format) || !validPath(string(path)) {
			return nil, errors.New("unsupported tree entry")
		}
		if fields[0] != "100644" && fields[0] != "100755" && fields[0] != "120000" {
			return nil, errors.New("unsupported tree mode")
		}
		name := string(path)
		// Reject collisions at directories too: Foo/a and foo/b are ambiguous.
		components := strings.Split(name, "/")
		for i := range components {
			prefix := strings.Join(components[:i+1], "/")
			key := strings.ToLower(prefix)
			if previous, ok := folded[key]; ok && previous != prefix {
				return nil, errors.New("case ambiguous tree")
			}
			folded[key] = prefix
		}
		entries = append(entries, Entry{Path: name, Mode: fields[0], BlobOID: fields[2]})
		if len(entries) > maxEntries {
			return nil, errors.New("tree entry limit")
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	for i := 1; i < len(entries); i++ {
		if entries[i-1].Path == entries[i].Path {
			return nil, errors.New("duplicate tree path")
		}
	}
	return entries, nil
}

func validPath(path string) bool {
	if len(path) == 0 || len(path) > 4096 || !utf8.ValidString(path) || strings.Contains(path, "\\") || strings.HasPrefix(path, "/") {
		return false
	}
	for _, component := range strings.Split(path, "/") {
		if component == "" || component == "." || component == ".." || strings.EqualFold(component, ".git") {
			return false
		}
	}
	for _, r := range path {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}
func validOID(oid, format string) bool {
	size := 40
	if format == "sha256" {
		size = 64
	}
	if len(oid) != size || strings.ToLower(oid) != oid {
		return false
	}
	_, err := hex.DecodeString(oid)
	return err == nil
}
func canonicalIndex(entries []Entry) []byte {
	var raw bytes.Buffer
	for _, entry := range entries {
		fmt.Fprintf(&raw, "%s %s 0\t%s%c", entry.Mode, entry.BlobOID, entry.Path, 0)
	}
	return raw.Bytes()
}
func objectID(format, kind string, raw []byte) string {
	var digest hash.Hash = sha1.New()
	if format == "sha256" {
		digest = sha256.New()
	}
	fmt.Fprintf(digest, "%s %d%c", kind, len(raw), 0)
	digest.Write(raw)
	return hex.EncodeToString(digest.Sum(nil))
}
func materializationDigest(entries []Entry) string {
	rows := make([]wire.Value, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, wire.Value{Kind: wire.KindObject, Obj: &wire.Object{Keys: []string{"blobOid", "mode", "path", "rawSha256"}, Values: map[string]wire.Value{
			"blobOid": {Kind: wire.KindString, Str: entry.BlobOID}, "mode": {Kind: wire.KindString, Str: entry.Mode}, "path": {Kind: wire.KindString, Str: entry.Path}, "rawSha256": {Kind: wire.KindString, Str: workqueue.SHA256Hex(entry.Raw)},
		}}})
	}
	raw := wire.CanonicalValue(wire.Value{Kind: wire.KindArray, Arr: rows})
	return workqueue.SHA256Hex(append([]byte("verification-materialization\x00verification-materialization/0\x00"), raw...))
}
