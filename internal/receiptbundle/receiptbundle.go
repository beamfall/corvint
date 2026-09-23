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
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/cemcode"
	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// Frozen names and bounds (RCB-V0-002, RCB-V0-004).
const (
	Profile         = "corvint-receipt-bundle/0"
	ManifestName    = "manifest.json"
	MaxReceiptBytes = wire.MaxMapBytes

	CodeOutputRefused = "bundle-output-refused"

	ReasonNotSupplied   = "not-supplied"
	ReasonNotFound      = "not-found"
	ReasonUnreadable    = "unreadable"
	ReasonOtherRevision = "binds-other-revision"

	dogfoodSource = ".corvint/dogfood-report.json"
	gateSource    = "$GIT_DIR/corvint/release-gate-receipt"
	gatePrefix    = "corvint-gate-receipt/0"
)

// axisValues are the string values that mark an axis a receipt did not run
// or did not produce (RCB-V0-003).
var axisValues = map[string]bool{"NOT_RUN": true, "NOT_PRODUCED": true, wire.DiscriminationNotRun: true}

// Options names the one change to export and where to put it.
type Options struct {
	MapPath string // repository-relative CEM path
	Target  string // the revision the CEM is bound to
	Output  string // absolute path of a directory that must not exist
	Witness string // optional saved `corvint witness --json` report
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
}

// Export writes the bundle and returns the command envelope.
func Export(ctx context.Context, root string, options Options) (map[string]any, error) {
	repo, err := gitauth.Open(root, gitrun.NewDefaultBudget())
	if err != nil {
		return nil, err
	}
	output, err := outputPath(repo, options.Output)
	if err != nil {
		return nil, err
	}
	cem, base, err := readCEM(repo, options.MapPath)
	if err != nil {
		return nil, err
	}
	resolvedBase, err := repo.Resolve(ctx, base)
	if err != nil {
		return nil, err
	}
	target, err := repo.Resolve(ctx, options.Target)
	if err != nil {
		return nil, err
	}
	e := &exporter{repo: repo, base: resolvedBase, target: target}
	cem.Base, cem.Target = resolvedBase, target
	receipts := []any{cem, e.witness(ctx, options.Witness), e.dogfood(ctx), e.gate(ctx)}
	manifest := e.manifest(receipts)
	if err := write(output, receipts, manifest); err != nil {
		return nil, err
	}
	return map[string]any{
		"ok": true, "mutates": false, "tool": "cem-export", "bundle": output,
		"manifestSha256": digest(manifest), "receipts": receipts,
	}, nil
}

// outputPath refuses a relative, existing, or in-repository output
// (RCB-V0-005) and returns it with its parent's symlinks resolved.
func outputPath(repo *gitauth.Repository, output string) (string, error) {
	if !filepath.IsAbs(output) {
		return "", cemcode.New(CodeOutputRefused, "--output must be an absolute path")
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		return "", cemcode.New(CodeOutputRefused, "--output must name a path that does not exist")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(output))
	if err != nil {
		return "", cemcode.New(CodeOutputRefused, "--output parent directory does not resolve")
	}
	resolved := filepath.Join(parent, filepath.Base(output))
	for _, owned := range []string{repo.Root, repo.GitDir, repo.CommonDir} {
		if within(resolved, owned) {
			return "", cemcode.New(CodeOutputRefused, "--output must lie outside the worktree and Git directories")
		}
	}
	return resolved, nil
}

func within(path, dir string) bool {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	relative, err := filepath.Rel(dir, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// readCEM reads the required CEM receipt; a map that is absent or invalid is
// an error, never an absent receipt (RCB-V0-004).
func readCEM(repo *gitauth.Repository, mapPath string) (*present, string, error) {
	data, err := readRegular(repo.Root, mapPath)
	if err != nil {
		return nil, "", cemcode.New(cemcode.MapUnavailable, "map %s is not a readable regular file", mapPath)
	}
	parsed, err := wire.ParseMap(data)
	if err != nil {
		return nil, "", err
	}
	receipt, err := newPresent("cem", "receipts/cem.json", mapPath, data)
	return receipt, parsed.BaseRevision, err
}

func (e *exporter) witness(ctx context.Context, path string) any {
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
	if reason == "" && !(e.binds(ctx, report.Range.Base, e.base) && e.binds(ctx, report.Range.Head, e.target)) {
		reason = ReasonOtherRevision
	}
	return e.finish(kind, "receipts/witness.json", source, e.base, data, reason)
}

func (e *exporter) dogfood(ctx context.Context) any {
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
	if reason == "" && !(e.binds(ctx, report.Base, e.base) && e.binds(ctx, report.Target, e.target)) {
		reason = ReasonOtherRevision
	}
	return e.finish(kind, "receipts/dogfood-report.json", dogfoodSource, e.base, data, reason)
}

// gate reads the GOC-V0-010 full-gate receipt, which binds a commit but no
// base, so its entry carries no base.
func (e *exporter) gate(ctx context.Context) any {
	const kind = "gate-receipt"
	data, err := readRegular(filepath.Join(e.repo.GitDir, "corvint"), "release-gate-receipt")
	reason := readReason(err)
	fields := strings.Fields(string(data))
	if reason == "" && (len(fields) != 4 || fields[0] != gatePrefix) {
		reason = ReasonUnreadable
	}
	if reason == "" && !e.binds(ctx, fields[1], e.target) {
		reason = ReasonOtherRevision
	}
	return e.finish(kind, "receipts/gate-receipt.txt", gateSource, "", data, reason)
}

// binds reports whether revision names the commit want.
func (e *exporter) binds(ctx context.Context, revision, want string) bool {
	resolved, err := e.repo.Resolve(ctx, revision)
	return err == nil && resolved == want
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

// write creates the bundle directory and every file exclusively; a failed
// write removes only the directory it created (RCB-V0-005).
func write(output string, receipts []any, manifest []byte) (err error) {
	if err := os.Mkdir(output, 0o700); err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle directory cannot be created")
	}
	defer func() {
		if err != nil {
			os.RemoveAll(output)
		}
	}()
	if err := os.Mkdir(filepath.Join(output, "receipts"), 0o700); err != nil {
		return cemcode.New(cemcode.PublishFailed, "bundle receipts directory cannot be created")
	}
	for _, receipt := range receipts {
		if held, ok := receipt.(*present); ok {
			if err := writeFile(filepath.Join(output, held.File), held.data); err != nil {
				return err
			}
		}
	}
	return writeFile(filepath.Join(output, ManifestName), manifest)
}

func writeFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
