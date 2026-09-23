// Package receiptbundle exports the receipts that one change already carries
// (its CEM, witness report, dogfood report and gate receipt) as exact byte
// copies into a new directory with a line-oriented manifest, so an auditor can
// recompute every digest offline with script/verify-receipt-bundle.sh
// (docs/specs/receipt-bundle-v0.md, RCB-V0).
//
// The export reads repository and Git-directory state and writes only the
// bundle directory the caller names outside the worktree. It never signs,
// publishes, or synthesizes a receipt: a receipt it cannot find or bind is
// listed as absent with a reason.
package receiptbundle

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/verify"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Frozen names and bounds (RCB-V0-002, RCB-V0-004).
const (
	Profile         = "corvint-receipt-bundle/0"
	ManifestName    = "manifest.json"
	MaxReceiptBytes = wire.MaxMapBytes

	CodeOutputRefused  = "bundle-output-refused"
	CodeMapUncommitted = "bundle-map-uncommitted"

	ReasonNotSupplied   = "not-supplied"
	ReasonNotFound      = "not-found"
	ReasonUnreadable    = "unreadable"
	ReasonOtherRevision = "binds-other-revision"

	dogfoodSource = ".corvint/dogfood-report.json"
	gateSource    = "$GIT_DIR/corvint/release-gate-receipt"
)

// axisValues are the string values that mark an axis a receipt did not run
// or did not produce (RCB-V0-003).
var axisValues = map[string]bool{"NOT_RUN": true, "NOT_PRODUCED": true, wire.DiscriminationNotRun: true}

// Options names the one change to export and where to put it.
type Options struct {
	MapPath      string // repository-relative CEM path
	ExpectedBase string // independent base the map must name (CEM-CB-010)
	Target       string // the commit whose canonical patch the map binds
	Output       string // absolute path of a directory that must not exist
	Witness      string // optional saved `corvint witness --json` report
}

// Axis is one NOT_RUN or NOT_PRODUCED value inside a receipt.
type Axis struct {
	Pointer string `json:"pointer"`
	Value   string `json:"value"`
}

type present struct {
	Kind   string `json:"kind"`
	State  string `json:"state"`
	File   string `json:"file"`
	Sha256 string `json:"sha256"`
	Source string `json:"source"`
	Base   string `json:"base,omitempty"`
	Target string `json:"target"`
	Axes   []Axis `json:"notRunOrNotProduced"`
	data   []byte
}

type absent struct {
	Kind   string `json:"kind"`
	State  string `json:"state"`
	Source string `json:"source"`
	Reason string `json:"reason"`
}

type exporter struct {
	repo   *gitauth.Repository
	base   string
	target string
	tree   string
}

// gateLine is the one canonical GOC-V0-010 receipt line, matched exactly as
// script/release-checklist reads it: no other bytes, one final LF.
var gateLine = regexp.MustCompile(`\Acorvint-gate-receipt/0 ([0-9a-f]{40}|[0-9a-f]{64}) ([0-9a-f]{40}|[0-9a-f]{64}) [0-9a-f]{64}\n\z`)

// Export writes the bundle and returns the command envelope.
func Export(ctx context.Context, root string, options Options) (map[string]any, error) {
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	parent, name, err := outputParent(repo, options.Output)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	cem, err := verifiedCEM(ctx, repo, options)
	if err != nil {
		return nil, err
	}
	tree, err := repo.CommitTree(ctx, cem.Target)
	if err != nil {
		return nil, err
	}
	e := &exporter{repo: repo, base: cem.Base, target: cem.Target, tree: tree}
	receipts := []any{cem, e.witness(options.Witness), e.dogfood(), e.gate()}
	manifest := e.manifest(receipts)
	if err := write(parent, name, receipts, manifest); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "mutates": false, "tool": "cem-export", "bundle": filepath.Join(parent.Name(), name),
		"manifestSha256": digest(manifest), "receipts": receipts,
	}, nil
}

// outputParent refuses a relative, existing, or in-repository output
// (RCB-V0-005) and returns its opened parent and its name, so every later
// write goes through the directory that was checked.
func outputParent(repo *gitauth.Repository, output string) (*os.Root, string, error) {
	if !filepath.IsAbs(output) {
		return nil, "", refused("--output must be an absolute path")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return nil, "", refused("--output must name a path that does not exist")
	}
	parentPath, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return nil, "", refused("--output parent directory does not resolve")
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, "", refused("--output parent directory cannot be opened")
	}
	if err := outsideRepository(repo, parent, parentPath); err != nil {
		parent.Close()
		return nil, "", err
	}
	return parent, filepath.Base(output), nil
}

func refused(message string) error {
	return cemcode.New(CodeOutputRefused, "%s", message)
}

// outsideRepository compares the opened parent and each of its ancestors by
// file identity, never by path text, with every protected directory: case
// variants and volume aliases name the same file.
func outsideRepository(repo *gitauth.Repository, parent *os.Root, parentPath string) error {
	opened, err := parent.Stat(".")
	if err != nil {
		return refused("--output parent directory cannot be examined")
	}
	chain, err := ancestors(parentPath)
	if err != nil || !os.SameFile(opened, chain[0]) {
		return refused("--output parent directory cannot be examined")
	}
	protected := protectedDirs(repo)
	for _, dir := range chain {
		if slices.ContainsFunc(protected, func(owned os.FileInfo) bool { return os.SameFile(dir, owned) }) {
			return refused("--output must lie outside every worktree and Git directory")
		}
	}
	return nil
}

// ancestors stats dir and every directory above it.
func ancestors(dir string) ([]os.FileInfo, error) {
	chain := []os.FileInfo{}
	for {
		info, err := os.Stat(dir)
		if err != nil {
			return nil, err
		}
		chain = append(chain, info)
		up := filepath.Dir(dir)
		if up == dir {
			return chain, nil
		}
		dir = up
	}
}

// protectedDirs are this worktree, its Git directories, the primary worktree,
// and every linked worktree the common directory records.
func protectedDirs(repo *gitauth.Repository) []os.FileInfo {
	paths := append([]string{repo.Root, repo.GitDir, repo.CommonDir}, primaryWorktree(repo.CommonDir)...)
	paths = append(paths, linkedWorktrees(repo.CommonDir)...)
	infos := []os.FileInfo{}
	for _, path := range paths {
		if info, err := os.Stat(path); err == nil {
			infos = append(infos, info)
		}
	}
	return infos
}

// primaryWorktree is the common directory's parent when that parent's .git
// is the common directory itself; a bare common directory has none.
func primaryWorktree(common string) []string {
	parent := filepath.Dir(common)
	marker, markerErr := os.Stat(filepath.Join(parent, ".git"))
	own, ownErr := os.Stat(common)
	if markerErr != nil || ownErr != nil || !os.SameFile(marker, own) {
		return nil
	}
	return []string{parent}
}

// linkedWorktrees reads each $COMMON/worktrees/*/gitdir back-pointer, which
// names a linked worktree's .git file, absolute or relative to itself.
func linkedWorktrees(common string) []string {
	admin := filepath.Join(common, "worktrees")
	entries, _ := os.ReadDir(admin)
	dirs := []string{}
	for _, entry := range entries {
		data, err := readRegular(filepath.Join(admin, entry.Name()), "gitdir")
		if err != nil {
			continue
		}
		marker := strings.TrimSuffix(string(data), "\n")
		if !filepath.IsAbs(marker) {
			marker = filepath.Join(admin, entry.Name(), marker)
		}
		dirs = append(dirs, filepath.Dir(marker))
	}
	return dirs
}

// verifiedCEM reads the required CEM and admits it only as `cem verify` would
// with the caller's independent base and target (CEM-CB-010), and only when
// the target commits these exact bytes at the sidecar path (RCB-V0-004).
func verifiedCEM(ctx context.Context, repo *gitauth.Repository, options Options) (*present, error) {
	data, err := readRegular(repo.Root, options.MapPath)
	if err != nil {
		return nil, cemcode.New(cemcode.MapUnavailable, "map %s is not a readable regular file", options.MapPath)
	}
	document, err := wire.ParseMap(data)
	if err != nil {
		return nil, err
	}
	outcome, _, err := verify.Canonical(ctx, repo, document, verify.CanonicalOptions{
		ExpectedBase: options.ExpectedBase, Target: options.Target, RawMapBytes: data,
	})
	if err := verdict(outcome, err); err != nil {
		return nil, err
	}
	target, err := repo.Resolve(ctx, options.Target)
	if err != nil {
		return nil, err
	}
	if err := committedAt(ctx, repo, target); err != nil {
		return nil, err
	}
	receipt, err := newPresent("cem", "receipts/cem.json", options.MapPath, data)
	if err != nil {
		return nil, err
	}
	receipt.Base, receipt.Target = document.BaseRevision, target
	return receipt, nil
}

// verdict admits exactly what `cem verify` reports valid: success, or evidence
// drift that still carries its outcome.
func verdict(outcome *verify.Outcome, err error) error {
	if cemcode.CodeOf(err) == cemcode.EvidenceDrift && outcome != nil {
		return nil
	}
	return err
}

// committedAt requires the target to hold the sidecar; canonical verification
// already refused one whose bytes differ from the map (CEM-CB-009).
func committedAt(ctx context.Context, repo *gitauth.Repository, target string) error {
	_, exists, err := repo.LookupTreeEntry(ctx, target, wire.ExcludedCEMPath)
	if err != nil {
		return err
	}
	if !exists {
		return cemcode.New(CodeMapUncommitted, "the target does not commit the map at %s", wire.ExcludedCEMPath)
	}
	return nil
}

func (e *exporter) witness(path string) any {
	const kind, source = "witness", "--witness"
	if path == "" {
		return absent{kind, "absent", source, ReasonNotSupplied}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return absent{kind, "absent", source, ReasonNotFound}
	}
	var report struct {
		Profile string `json:"profile"`
		Range   struct {
			Base string `json:"base"`
			Head string `json:"head"`
		} `json:"range"`
	}
	data, reason := readJSON(filepath.Dir(absolute), filepath.Base(absolute), &report)
	if reason == "" && report.Profile != "corvint-witness/0" {
		reason = ReasonUnreadable
	}
	if reason == "" && !(report.Range.Base == e.base && report.Range.Head == e.target) {
		reason = ReasonOtherRevision
	}
	return e.finish(kind, "receipts/witness.json", source, e.base, data, reason)
}

func (e *exporter) dogfood() any {
	const kind = "dogfood"
	var report struct {
		Profile string `json:"profile"`
		Base    string `json:"base"`
		Target  string `json:"target"`
	}
	data, reason := readJSON(e.repo.Root, dogfoodSource, &report)
	if reason == "" && report.Profile != "corvint-dogfood-change/0" {
		reason = ReasonUnreadable
	}
	if reason == "" && !(report.Base == e.base && report.Target == e.target) {
		reason = ReasonOtherRevision
	}
	return e.finish(kind, "receipts/dogfood-report.json", dogfoodSource, e.base, data, reason)
}

// gate reads the GOC-V0-010 full-gate receipt, which binds a commit and its
// tree but no base, so its entry carries no base.
func (e *exporter) gate() any {
	const kind = "gate-receipt"
	data, err := readRegular(filepath.Join(e.repo.GitDir, "corvint"), "release-gate-receipt")
	reason := readReason(err)
	if reason == "" && !gateLine.Match(data) {
		reason = ReasonUnreadable
	}
	fields := strings.Fields(string(data))
	if reason == "" && !(fields[1] == e.target && fields[2] == e.tree) {
		reason = ReasonOtherRevision
	}
	return e.finish(kind, "receipts/gate-receipt.txt", gateSource, "", data, reason)
}

func (e *exporter) finish(kind, file, source, base string, data []byte, reason string) any {
	if reason != "" {
		return absent{kind, "absent", source, reason}
	}
	receipt, err := newPresent(kind, file, source, data)
	if err != nil {
		return absent{kind, "absent", source, ReasonUnreadable}
	}
	receipt.Base, receipt.Target = base, e.target
	return receipt
}

func newPresent(kind, file, source string, data []byte) (*present, error) {
	axes, err := notRunAxes(data)
	if err != nil {
		return nil, err
	}
	return &present{
		Kind: kind, State: "present", File: file, Sha256: digest(data),
		Source: source, Axes: axes, data: data,
	}, nil
}

// notRunAxes lists every NOT_RUN or NOT_PRODUCED string a JSON receipt holds,
// by RFC 6901 pointer in sorted-key order. A non-JSON receipt has none.
func notRunAxes(data []byte) ([]Axis, error) {
	axes := []Axis{}
	if !json.Valid(data) {
		return axes, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var document any
	if err := decoder.Decode(&document); err != nil {
		return nil, err
	}
	collect(document, "", &axes)
	return axes, nil
}

func collect(value any, pointer string, axes *[]Axis) {
	switch typed := value.(type) {
	case string:
		if axisValues[typed] {
			*axes = append(*axes, Axis{Pointer: pointer, Value: typed})
		}
	case []any:
		for index, item := range typed {
			collect(item, pointer+"/"+strconv.Itoa(index), axes)
		}
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			collect(typed[key], pointer+"/"+escapePointer(key), axes)
		}
	}
}

func escapePointer(key string) string {
	return strings.ReplaceAll(strings.ReplaceAll(key, "~", "~0"), "/", "~1")
}

// readJSON reads one bounded regular file and decodes it into report.
func readJSON(dir, name string, report any) ([]byte, string) {
	data, err := readRegular(dir, name)
	if reason := readReason(err); reason != "" {
		return nil, reason
	}
	if json.Unmarshal(data, report) != nil {
		return nil, ReasonUnreadable
	}
	return data, ""
}

func readReason(err error) string {
	switch {
	case err == nil:
		return ""
	case errors.Is(err, os.ErrNotExist):
		return ReasonNotFound
	default:
		return ReasonUnreadable
	}
}

// readRegular reads name under dir without following a final symlink and
// refuses anything but a regular file of at most MaxReceiptBytes.
func readRegular(dir, name string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer root.Close()
	info, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxReceiptBytes+1))
	if err == nil && len(data) > MaxReceiptBytes {
		err = errors.New("receipt exceeds the byte bound")
	}
	return data, err
}

// manifest renders the line-oriented manifest (RCB-V0-002): a header line,
// one receipt per line, and a closing line, so a POSIX shell can read it.
func (e *exporter) manifest(receipts []any) []byte {
	header, _ := json.Marshal(struct {
		Profile string `json:"profile"`
		Base    string `json:"base"`
		Target  string `json:"target"`
	}{Profile, e.base, e.target})
	lines := make([]string, 0, len(receipts))
	for _, receipt := range receipts {
		line, _ := json.Marshal(receipt)
		lines = append(lines, string(line))
	}
	opening := strings.TrimSuffix(string(header), "}") + `,"receipts":[`
	return []byte(opening + "\n" + strings.Join(lines, ",\n") + "\n]}\n")
}

// write creates the bundle directory and every file exclusively through the
// opened parent; a failed write removes only the directory it created
// (RCB-V0-005).
func write(parent *os.Root, name string, receipts []any, manifest []byte) (err error) {
	if err := parent.Mkdir(name, 0o700); err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle directory cannot be created")
	}
	defer func() {
		if err != nil {
			parent.RemoveAll(name)
		}
	}()
	if err := parent.Mkdir(filepath.Join(name, "receipts"), 0o700); err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle receipts directory cannot be created")
	}
	for _, receipt := range receipts {
		if held, ok := receipt.(*present); ok {
			if err := writeFile(parent, filepath.Join(name, filepath.FromSlash(held.File)), held.data); err != nil {
				return err
			}
		}
	}
	return writeFile(parent, filepath.Join(name, ManifestName), manifest)
}

func writeFile(parent *os.Root, path string, data []byte) error {
	file, err := parent.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle file cannot be created")
	}
	_, err = file.Write(data)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle file cannot be written")
	}
	return nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
