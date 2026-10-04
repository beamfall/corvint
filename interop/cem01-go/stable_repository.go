package main

// Canonical-repository-bounded Stable verification (S0E). The repository is
// admitted from bounded no-follow metadata reads, and every content read is a
// counted logical Git transaction run under an owned process group. Process
// ownership is platform specific (stable_process_*.go); this file holds the
// portable engine, the Git argv/env contract, the stdout parsers and the
// arbitration of transaction causes.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	stableCanonicalEnvelope = "canonical-repository-bounded/1"
	stableKeeperProtocol    = "__cem01-stable-keeper/1"
	stableCallLimit         = 1024
	stableBatchLimit        = 4 << 20
	stableProofLimit        = 128 << 20
	stableBlobLimit         = 64 << 20
	stableBlobAggregate     = 128 << 20
	stableDiffLimit         = 8 << 20
	stableCommitLimit       = 64 << 20
	stableFormatLimit       = 64
	stableHeaderLimit       = 512
	stableStderrLimit       = 64 << 10
	stableMaxDepth          = 128
	stableOpTimeout         = 5 * time.Minute
	stableRetireBound       = 10 * time.Second
	stablePostReapPollBound = 2 * time.Second
	stableHoldLine          = "cem01: process containment not proven; owned process group retained\n"
)

const (
	stableOpResolve byte = 1
	stableOpFormat  byte = 2
	stableOpBatch   byte = 3
	stableOpDiff    byte = 4
	stableOpBlob    byte = 5
)

var stableEmptyTree = map[int]string{
	40: "4b825dc642cb6eb9a060e54bf8d69288fbee4904",
	64: "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321",
}

var stableGitPins = []string{
	"core.fsmonitor=false", "core.untrackedCache=false", "core.attributesFile=/dev/null", "credential.helper=", "core.quotePath=false",
	"diff.suppressBlankEmpty=false", "diff.orderFile=/dev/null", "diff.noprefix=false", "diff.mnemonicPrefix=false", "diff.renames=false",
	"diff.algorithm=myers", "diff.wsErrorHighlight=none", "diff.srcPrefix=a/", "diff.dstPrefix=b/", "diff.external=", "diff.ignoreSubmodules=none",
	"advice.graftFileDeprecated=false",
}

var stableDiffArgs = []string{
	"--full-index", "--no-color", "--no-ext-diff", "--no-textconv", "--no-renames", "--no-indent-heuristic", "--diff-algorithm=myers",
	"--unified=3", "--inter-hunk-context=0", "--src-prefix=a/", "--dst-prefix=b/", "--ignore-submodules=none",
}

// stableGitArgv builds the only argv a keeper may execute for one protocol op.
func stableGitArgv(op byte, fields []string) ([]string, bool) {
	want := map[byte]int{stableOpResolve: 3, stableOpFormat: 2, stableOpBatch: 2, stableOpDiff: 5, stableOpBlob: 3}
	if want[op] == 0 || len(fields) != want[op] || !filepath.IsAbs(fields[0]) || !filepath.IsAbs(fields[1]) {
		return nil, false
	}
	for _, f := range fields[2:] {
		if !validOID(f) {
			return nil, false
		}
	}
	argv := []string{fields[0], "--no-optional-locks", "--git-dir=" + fields[1]}
	for _, p := range stableGitPins {
		argv = append(argv, "-c", p)
	}
	switch op {
	case stableOpResolve:
		argv = append(argv, "cat-file", "commit", fields[2])
	case stableOpFormat:
		argv = append(argv, "rev-parse", "--show-object-format")
	case stableOpBatch:
		argv = append(argv, "cat-file", "--batch")
	case stableOpDiff:
		argv = append(argv, "--attr-source="+fields[2], "diff")
		argv = append(argv, stableDiffArgs...)
		argv = append(argv, "--end-of-options", fields[3], fields[4], "--", ".", ":(exclude)"+sidecar03)
	case stableOpBlob:
		argv = append(argv, "cat-file", "blob", fields[2])
	}
	return argv, true
}

// stableGitEnv is the exact U06 environment; nothing else is inherited.
func stableGitEnv() []string {
	env := []string{}
	for _, k := range []string{"PATH", "SystemRoot", "TMPDIR", "TEMP", "TMP", "USERPROFILE"} {
		if v, ok := os.LookupEnv(k); ok {
			env = append(env, k+"="+v)
		}
	}
	return append(env, "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		"GIT_TERMINAL_PROMPT=0", "GIT_OPTIONAL_LOCKS=0", "GIT_NO_LAZY_FETCH=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_GRAFT_FILE=/dev/null",
		"GIT_ASKPASS=", "GIT_ATTR_NOSYSTEM=1", "GCM_INTERACTIVE=never")
}

// stableLedgerRow is one logical charge, recorded before admission of the call.
type stableLedgerRow struct {
	Ordinal            int            `json:"ordinal"`
	Operation          string         `json:"operation"`
	Phase              string         `json:"phase"`
	InputKey           map[string]any `json:"inputKey"`
	ReservationOutcome string         `json:"reservationOutcome,omitempty"`
}

// stableHooks are test seams; production passes nil.
type stableHooks struct {
	fs         func(point, path string) error
	tx         func(t *stableTx, point string)
	event      func(name string)
	gitPath    func(t *stableTx) string
	opTimeout  func(t *stableTx) time.Duration
	limit      func(t *stableTx) int64
	aggregate  func(s *stableSession) int64
	ownerFail  func(t *stableTx, step string) bool
	setupFail  func(step string) bool
	closeFail  func() bool
	holdKeeper bool
	cause      func(t *stableTx) string
	ledger     func(row stableLedgerRow)
}

func (h *stableHooks) emit(name string) {
	if h != nil && h.event != nil {
		h.event(name)
	}
}
func (h *stableHooks) point(t *stableTx, p string) {
	if h != nil && h.tx != nil {
		h.tx(t, p)
	}
}

// stableTx is one logical Git transaction.
type stableTx struct {
	Ordinal   int
	Operation string
	Phase     string
	input     map[string]any
	op        byte
	fields    []string
	limit     int64
	// consume runs against the child's stdin/stdout; a non-nil error is a
	// complete parser/identity failure unless it is io.EOF/io.ErrUnexpectedEOF.
	consume func(w io.WriteCloser, r *bufio.Reader) error
	out     []byte
	held    bool
}

// stableFlags are the causes collected for arbitration after teardown.
type stableFlags struct {
	owner, deadline, parser, overflow, launch, nonzero, truncated bool
}

// stableCheckpointStage maps a transaction phase to its outer-cancellation stage.
func stableCheckpointStage(phase string) string {
	switch {
	case phase == "expected-base", phase == "base-sidecar", phase == "repeated-target-sidecar", phase == "target-sidecar":
		return "binding"
	case phase == "simulation", phase == "mechanical", strings.HasPrefix(phase, "evidence:"):
		return "verification"
	case phase == "drift-target", strings.HasPrefix(phase, "drift:"):
		return "drift"
	}
	return "repository"
}

type stableIdent struct {
	dev, ino uint64
	mode     uint32
}

type stableAdmission struct {
	root, admin, common string
	ids                 map[string]stableIdent
}

// stableHeld is a keeper parked by the hold seam until final owner close.
type stableHeld struct {
	proc  *os.Process
	pgid  int
	files []*os.File
}

type stableSession struct {
	ctx       context.Context
	hooks     *stableHooks
	exe       string
	git       string
	admin     string
	env       []string
	oidLen    int
	calls     int
	charged   map[string]bool
	bytes     int64
	hold      bool
	emergency time.Time
	held      []any
	holdLine  bool
}

func (s *stableSession) anchor() time.Time {
	if s.emergency.IsZero() {
		s.emergency = time.Now()
	}
	return s.emergency.Add(stableRetireBound)
}

// bound is min(10s, remaining) for ordinary teardown, or the single
// non-renewable emergency interval once outer cancellation was observed.
func (s *stableSession) bound() time.Time {
	if s.ctx.Err() != nil {
		return s.anchor()
	}
	d := stableRetireBound
	if dl, ok := s.ctx.Deadline(); ok && time.Until(dl) < d {
		d = time.Until(dl)
	}
	return time.Now().Add(d)
}

func (s *stableSession) arbitrate(t *stableTx, f *stableFlags) *cemError {
	if s.hooks != nil && s.hooks.cause != nil {
		switch s.hooks.cause(t) {
		case "nonzero":
			f.nonzero = true
		case "overflow":
			f.overflow = true
		}
	}
	canonical := t.Phase == "canonical"
	switch {
	case f.owner:
		return operational("unsupported-process-containment")
	case s.ctx.Err() != nil:
		return operational("verification-timeout")
	case f.deadline:
		return operational("git-timeout")
	case f.parser:
		return operational("repository-object-unavailable")
	case f.overflow:
		return operational("unsupported-resource-limit")
	case f.launch && canonical, f.nonzero && canonical:
		return operational("git-diff-failed")
	case f.launch:
		return operational("git-read-failed")
	case f.nonzero && t.Operation == "normal-blob":
		return operational("repository-object-unavailable")
	case f.nonzero:
		return operational("git-read-failed")
	case f.truncated:
		return operational("repository-object-unavailable")
	}
	return nil
}

func (s *stableSession) opTimeout(t *stableTx) time.Duration {
	if s.hooks != nil && s.hooks.opTimeout != nil {
		if d := s.hooks.opTimeout(t); d > 0 {
			return d
		}
	}
	return stableOpTimeout
}

// transact runs one charged logical transaction and returns the stage owning
// any failure (outer cancellation reports at the phase checkpoint).
func (s *stableSession) transact(t *stableTx) (string, *cemError) {
	stage := stableCheckpointStage(t.Phase)
	if s.ctx.Err() != nil {
		return stage, operational("verification-timeout")
	}
	s.calls++
	t.Ordinal = s.calls
	row := stableLedgerRow{Ordinal: t.Ordinal, Operation: t.Operation, Phase: t.Phase, InputKey: t.input}
	if s.calls > stableCallLimit {
		row.ReservationOutcome = "refused-before-spawn"
	}
	if s.hooks != nil && s.hooks.ledger != nil {
		s.hooks.ledger(row)
	}
	if s.calls > stableCallLimit {
		return "repository", operational("unsupported-resource-limit")
	}
	if s.hooks != nil && s.hooks.limit != nil {
		if l := s.hooks.limit(t); l > 0 {
			t.limit = l
		}
	}
	git := s.git
	if s.hooks != nil && s.hooks.gitPath != nil {
		if p := s.hooks.gitPath(t); p != "" {
			git = p
		}
	}
	t.fields = append([]string{git, s.admin}, t.fields...)
	if s.hooks != nil && s.hooks.holdKeeper && t.Ordinal == 1 {
		t.held = true
	}
	e := stableRun(s, t)
	if e != nil {
		if e.code == "verification-timeout" {
			return stage, e
		}
		return "repository", e
	}
	return "", nil
}

// --- object parsing ---------------------------------------------------------

type stableRec struct {
	name string
	en   *treeEntry
}

func stableParseTree(b []byte, oidLen int) ([]stableRec, bool) {
	raw := oidLen / 2
	out := []stableRec{}
	seen := map[string]bool{}
	for len(b) > 0 {
		sp := bytes.IndexByte(b, ' ')
		if sp <= 0 || sp > 7 {
			return nil, false
		}
		v, err := strconv.ParseUint(string(b[:sp]), 8, 32)
		if err != nil {
			return nil, false
		}
		rest := b[sp+1:]
		nul := bytes.IndexByte(rest, 0)
		if nul <= 0 || len(rest) < nul+1+raw {
			return nil, false
		}
		name := string(rest[:nul])
		if strings.Contains(name, "/") || seen[name] {
			return nil, false
		}
		seen[name] = true
		mode := fmt.Sprintf("%06o", v)
		typ := "blob"
		switch mode {
		case "040000":
			typ = "tree"
		case "160000":
			typ = "commit"
		}
		out = append(out, stableRec{name, &treeEntry{mode: mode, typ: typ, oid: hex.EncodeToString(rest[nul+1 : nul+1+raw])}})
		b = rest[nul+1+raw:]
	}
	return out, true
}

func stableCommitTree(b []byte, oidLen int) (string, bool) {
	if !bytes.HasPrefix(b, []byte("tree ")) || len(b) < 5+oidLen+1 || b[5+oidLen] != '\n' {
		return "", false
	}
	tree := string(b[5 : 5+oidLen])
	return tree, validOID(tree) && len(tree) == oidLen
}

var errStableParse = errors.New("parser")

// stableBatchGet requests one object on an interactive cat-file --batch.
func stableBatchGet(w io.Writer, r *bufio.Reader, oid, kind string, oidLen int, limit int64) ([]byte, error) {
	_, _ = io.WriteString(w, oid+"\n")
	line := make([]byte, 0, 128)
	for {
		c, err := r.ReadByte()
		if err != nil {
			return nil, io.ErrUnexpectedEOF
		}
		if c == '\n' {
			break
		}
		line = append(line, c)
		if len(line) >= stableHeaderLimit {
			return nil, errStableParse
		}
	}
	parts := strings.Split(string(line), " ")
	if len(parts) != 3 || parts[0] != oid || parts[1] != kind {
		return nil, errStableParse
	}
	n, err := strconv.ParseUint(parts[2], 10, 63)
	if err != nil || strconv.FormatUint(n, 10) != parts[2] || int64(n) > limit {
		return nil, errStableParse
	}
	body := make([]byte, n+1)
	if _, err := io.ReadFull(r, body); err != nil {
		return nil, io.ErrUnexpectedEOF
	}
	if body[n] != '\n' || candidateObjectHash(kind, body[:n], oidLen) != oid {
		return nil, errStableParse
	}
	return body[:n], nil
}

// --- logical operations -----------------------------------------------------

func (s *stableSession) resolve(phase, rev string) (string, string, *cemError) {
	t := &stableTx{Operation: "resolve-commit", Phase: phase, input: map[string]any{"revision": rev}, op: stableOpResolve, fields: []string{rev}, limit: stableCommitLimit}
	if stage, e := s.transact(t); e != nil {
		return "", stage, e
	}
	tree, ok := stableCommitTree(t.out, s.oidLen)
	if !ok || candidateObjectHash("commit", t.out, s.oidLen) != rev {
		return "", "repository", operational("repository-object-unavailable")
	}
	return tree, "", nil
}

// lookup authenticates commit, root and intermediate trees in one batch.
func (s *stableSession) lookup(phase, rev, path string) (*treeEntry, string, *cemError) {
	var found *treeEntry
	t := &stableTx{Operation: "authenticated-path-lookup", Phase: phase, input: map[string]any{"revision": rev, "path": path}, op: stableOpBatch, limit: stableBatchLimit}
	t.consume = func(w io.WriteCloser, r *bufio.Reader) error {
		defer w.Close()
		c, err := stableBatchGet(w, r, rev, "commit", s.oidLen, stableBatchLimit)
		if err != nil {
			return err
		}
		tree, ok := stableCommitTree(c, s.oidLen)
		if !ok {
			return errStableParse
		}
		parts := strings.Split(path, "/")
		for i, name := range parts {
			b, err := stableBatchGet(w, r, tree, "tree", s.oidLen, stableBatchLimit)
			if err != nil {
				return err
			}
			recs, ok := stableParseTree(b, s.oidLen)
			if !ok {
				return errStableParse
			}
			var en *treeEntry
			for _, rec := range recs {
				if rec.name == name {
					en = rec.en
				}
			}
			if en == nil {
				return nil
			}
			if i == len(parts)-1 {
				found = en
				return nil
			}
			if en.typ != "tree" {
				return nil
			}
			tree = en.oid
		}
		return nil
	}
	if stage, e := s.transact(t); e != nil {
		return nil, stage, e
	}
	return found, "", nil
}

func (s *stableSession) blob(phase, oid string) ([]byte, string, *cemError) {
	t := &stableTx{Operation: "normal-blob", Phase: phase, input: map[string]any{"oid": oid}, op: stableOpBlob, fields: []string{oid}, limit: stableBlobLimit}
	if stage, e := s.transact(t); e != nil {
		return nil, stage, e
	}
	if candidateObjectHash("blob", t.out, s.oidLen) != oid {
		return nil, "repository", operational("repository-object-unavailable")
	}
	if e := s.charge(oid, len(t.out)); e != nil {
		return nil, "repository", e
	}
	return t.out, "", nil
}

func (s *stableSession) charge(oid string, n int) *cemError {
	if !s.charged[oid] {
		s.charged[oid] = true
		s.bytes += int64(n)
	}
	limit := int64(stableBlobAggregate)
	if s.hooks != nil && s.hooks.aggregate != nil {
		if l := s.hooks.aggregate(s); l > 0 {
			limit = l
		}
	}
	if s.bytes > limit {
		return operational("unsupported-resource-limit")
	}
	return nil
}

type stableChange struct {
	path string
	a, b *treeEntry
}

type stableCanonical struct {
	patch   []byte
	changed map[string]stableChange
	proof   map[string][]byte
}

// canonical runs the canonical phase in ledger order: format, diff, root
// pair, changed levels breadth first, proof blobs.
func (s *stableSession) canonical(base, target, baseTree, targetTree string) (*stableCanonical, *cemError) {
	format := &stableTx{Operation: "format-query", Phase: "canonical", input: map[string]any{}, op: stableOpFormat, limit: stableFormatLimit}
	if _, e := s.transact(format); e != nil {
		return nil, e
	}
	name := strings.TrimSuffix(string(format.out), "\n")
	if want := map[int]string{40: "sha1", 64: "sha256"}[s.oidLen]; name != want {
		return nil, operational("repository-object-unavailable")
	}
	diff := &stableTx{Operation: "canonical-diff-child", Phase: "canonical", input: map[string]any{"base": base, "target": target}, op: stableOpDiff, fields: []string{stableEmptyTree[s.oidLen], base, target}, limit: stableDiffLimit}
	if _, e := s.transact(diff); e != nil {
		return nil, e
	}
	out := &stableCanonical{patch: diff.out, changed: map[string]stableChange{}, proof: map[string][]byte{}}
	var roots [2][]stableRec
	pair := &stableTx{Operation: "canonical-root-pair", Phase: "canonical", input: map[string]any{"revisions": []any{base, target}}, op: stableOpBatch, limit: stableBatchLimit}
	pair.consume = func(w io.WriteCloser, r *bufio.Reader) error {
		defer w.Close()
		var trees [2]string
		for i, rev := range []string{base, target} {
			c, err := stableBatchGet(w, r, rev, "commit", s.oidLen, stableBatchLimit)
			if err != nil {
				return err
			}
			tree, ok := stableCommitTree(c, s.oidLen)
			if !ok || tree != []string{baseTree, targetTree}[i] {
				return errStableParse
			}
			trees[i] = tree
		}
		for i, tree := range trees {
			b, err := stableBatchGet(w, r, tree, "tree", s.oidLen, stableBatchLimit)
			if err != nil {
				return err
			}
			recs, ok := stableParseTree(b, s.oidLen)
			if !ok {
				return errStableParse
			}
			roots[i] = recs
		}
		return nil
	}
	if _, e := s.transact(pair); e != nil {
		return nil, e
	}
	type dirPair struct {
		path string
		a, b []stableRec
	}
	type pending struct {
		path   string
		ea, eb *treeEntry
	}
	level := []dirPair{{"", roots[0], roots[1]}}
	proofOIDs := []string{}
	seenProof := map[string]bool{}
	for depth := 1; ; depth++ {
		next := []pending{}
		for _, d := range level {
			am, bm := map[string]*treeEntry{}, map[string]*treeEntry{}
			names := map[string]bool{}
			for _, rec := range d.a {
				am[rec.name] = rec.en
				names[rec.name] = true
			}
			for _, rec := range d.b {
				bm[rec.name] = rec.en
				names[rec.name] = true
			}
			sorted := make([]string, 0, len(names))
			for n := range names {
				sorted = append(sorted, n)
			}
			sort.Strings(sorted)
			for _, n := range sorted {
				p := n
				if d.path != "" {
					p = d.path + "/" + n
				}
				a, b := am[n], bm[n]
				at, bt := a != nil && a.typ == "tree", b != nil && b.typ == "tree"
				if at || bt {
					if !(at && bt && a.oid == b.oid) {
						pa, pb := a, b
						if !at {
							pa = nil
						}
						if !bt {
							pb = nil
						}
						next = append(next, pending{p, pa, pb})
					}
					continue
				}
				if p == sidecar03 {
					continue
				}
				if (a == nil) != (b == nil) || a != nil && b != nil && (a.oid != b.oid || a.mode != b.mode) {
					out.changed[p] = stableChange{p, a, b}
					for _, en := range []*treeEntry{a, b} {
						if en != nil && en.typ == "blob" && !seenProof[en.oid] {
							seenProof[en.oid] = true
							proofOIDs = append(proofOIDs, en.oid)
						}
					}
				}
			}
		}
		if len(next) == 0 {
			break
		}
		if depth > stableMaxDepth {
			return nil, operational("unsupported-resource-limit")
		}
		paths, oids := []any{}, []any{}
		for _, p := range next {
			paths = append(paths, p.path)
			for _, en := range []*treeEntry{p.ea, p.eb} {
				if en != nil {
					oids = append(oids, en.oid)
				}
			}
		}
		results := make([]dirPair, len(next))
		lt := &stableTx{Operation: "canonical-changed-level", Phase: "canonical", input: map[string]any{"depth": depth, "paths": paths, "treeOids": oids}, op: stableOpBatch, limit: stableBatchLimit}
		lt.consume = func(w io.WriteCloser, r *bufio.Reader) error {
			defer w.Close()
			for i, p := range next {
				results[i].path = p.path
				for j, en := range []*treeEntry{p.ea, p.eb} {
					if en == nil {
						continue
					}
					b, err := stableBatchGet(w, r, en.oid, "tree", s.oidLen, stableBatchLimit)
					if err != nil {
						return err
					}
					recs, ok := stableParseTree(b, s.oidLen)
					if !ok {
						return errStableParse
					}
					if j == 0 {
						results[i].a = recs
					} else {
						results[i].b = recs
					}
				}
			}
			return nil
		}
		if _, e := s.transact(lt); e != nil {
			return nil, e
		}
		level = results
	}
	if len(proofOIDs) > 0 {
		oids := []any{}
		for _, o := range proofOIDs {
			oids = append(oids, o)
		}
		pt := &stableTx{Operation: "canonical-proof-blobs", Phase: "canonical", input: map[string]any{"oids": oids}, op: stableOpBatch, limit: stableProofLimit}
		pt.consume = func(w io.WriteCloser, r *bufio.Reader) error {
			defer w.Close()
			for _, o := range proofOIDs {
				b, err := stableBatchGet(w, r, o, "blob", s.oidLen, stableProofLimit)
				if err != nil {
					return err
				}
				out.proof[o] = b
			}
			return nil
		}
		if _, e := s.transact(pt); e != nil {
			return nil, e
		}
	}
	return out, nil
}

// --- engine -----------------------------------------------------------------

func runStableCanonical(ctx context.Context, repo string, raw []byte, base, target, artifacts string, hooks *stableHooks) (stableResult, int) {
	r := newStableResult(base, target)
	r.RepositoryEnvelope = stableCanonicalEnvelope
	r.MapSHA256 = string03(shaHex(raw))
	m, e := decodeStable(raw)
	if e != nil {
		return failStable(&r, "wire", e, false)
	}
	r.Axes["coverageValidation"] = "ABSENT"
	r.Axes["discriminationValidation"] = "ABSENT"
	for _, h := range m.Hunks {
		if h.Coverage != nil {
			r.Axes["coverageValidation"] = "WIRE_VALIDATED_ONLY"
		}
		if h.Discriminates != nil {
			r.Axes["discriminationValidation"] = "WIRE_VALIDATED_ONLY"
		}
	}
	if e := validateStableReferences(&m); e != nil {
		return failStable(&r, "references", e, false)
	}
	r.Hunks = m.Hunks
	r.References = m.stableReferences
	if base == "" {
		return failStable(&r, "authority-arguments", operational("expected-base-required"), false)
	}
	if target == "" {
		return failStable(&r, "authority-arguments", operational("target-required"), false)
	}
	if artifacts == "" {
		return failStable(&r, "authority-arguments", operational("artifacts-required"), false)
	}
	if !filepath.IsAbs(artifacts) {
		return failStable(&r, "authority-arguments", operational("invalid-arguments"), false)
	}
	if !validOID(base) || !validOID(target) || len(base) != len(target) {
		return failStable(&r, "authority-arguments", operational("invalid-arguments"), false)
	}
	for _, h := range m.Hunks {
		if structuralReason03(h.Reason) {
			r.Runtime.StructuralQualification = "EXPERIMENTAL_TUPLE_PENDING_FULL_CORPUS"
			if runtime.Version() != "go1.27.1" {
				r.Runtime.StructuralQualification = "UNSUPPORTED"
				r.Axes["structuralProof"] = "UNSUPPORTED_RUNTIME"
				return failStable(&r, "runtime", operational("unsupported-structural-runtime"), false)
			}
		}
	}
	if !stableOwnerSupported() {
		return failStable(&r, "runtime", operational("unsupported-process-containment"), false)
	}
	adm, e := stableAdmit(repo, hooks)
	if e != nil {
		return failStable(&r, "repository", e, false)
	}
	s := &stableSession{ctx: ctx, hooks: hooks, admin: adm.admin, env: stableGitEnv(), oidLen: len(base), charged: map[string]bool{}}
	closed := false
	fail := func(stage string, e *cemError, verification bool) (stableResult, int) {
		if !closed {
			closed = true
			stableClose(s)
		}
		if s.hold {
			return failStable(&r, "repository", operational("unsupported-process-containment"), false)
		}
		return failStable(&r, stage, e, verification)
	}
	exe, err := os.Executable()
	if err != nil || !filepath.IsAbs(exe) {
		return fail("repository", operational("unsupported-process-containment"), false)
	}
	s.exe = exe
	git, err := exec.LookPath("git")
	if err != nil || !filepath.IsAbs(git) {
		return fail("repository", operational("git-read-failed"), false)
	}
	s.git = git

	baseTree, stage, e := s.resolve("expected-base", base)
	if e != nil {
		return fail(stage, e, false)
	}
	r.ExpectedBase = string03(base)
	r.Assurance = string03("structural-only")
	if m.BaseRevision != base {
		return fail("binding", invalid("base-revision-mismatch"), true)
	}
	targetTree, stage, e := s.resolve("target", target)
	if e != nil {
		return fail(stage, e, false)
	}
	r.TargetRevision = string03(target)
	en, stage, e := s.lookup("base-sidecar", base, sidecar03)
	if e != nil {
		return fail(stage, e, false)
	}
	if en != nil && (en.typ != "blob" || en.mode != "100644") {
		return fail("binding", invalid("excluded-path-not-file"), true)
	}
	initial, stage, e := s.lookup("initial-target-sidecar", target, sidecar03)
	if e != nil {
		return fail(stage, e, false)
	}
	r.Sidecar = "ABSENT"
	if initial != nil {
		if initial.typ != "blob" || initial.mode != "100644" {
			r.Sidecar = "UNSUPPORTED_KIND"
			return fail("binding", invalid("excluded-artifact-mismatch"), true)
		}
		r.Sidecar = "MISMATCH"
		again, stage, e := s.lookup("repeated-target-sidecar", target, sidecar03)
		if e != nil {
			return fail(stage, e, false)
		}
		if again == nil || *again != *initial {
			return fail("repository", operational("repository-object-unavailable"), false)
		}
		b, stage, e := s.blob("target-sidecar", initial.oid)
		if e != nil {
			return fail(stage, e, false)
		}
		if !bytes.Equal(b, raw) {
			return fail("binding", invalid("excluded-artifact-mismatch"), true)
		}
		r.Sidecar = "EXACT"
	}
	canon, e := s.canonical(base, target, baseTree, targetTree)
	if e != nil {
		if e.code == "verification-timeout" {
			return fail("repository", e, false)
		}
		return fail("repository", e, false)
	}
	r.PatchSHA256 = string03(shaHex(canon.patch))
	r.Assurance = string03("canonical")
	if *r.PatchSHA256 != m.PatchSHA256 {
		return fail("verification", invalid("patch-digest-mismatch"), true)
	}
	parsed, e := parsePatch(canon.patch)
	if e != nil {
		return fail("verification", e, true)
	}
	ph := map[string]*patchHunk{}
	groups := map[string]*filePatch{}
	for _, f := range parsed.files {
		for _, h := range f.hunks {
			ph[h.ID] = h
			groups[h.ID] = f
		}
	}
	if len(m.Hunks) != len(ph) {
		return fail("verification", invalid("invalid-field"), true)
	}
	seen := map[string]bool{}
	for _, h := range m.Hunks {
		p := ph[h.ID]
		if p == nil || seen[h.ID] {
			return fail("verification", invalid("invalid-field"), true)
		}
		seen[h.ID] = true
		path := p.OldPath
		if p.NewPath != nil {
			path = p.NewPath
		}
		if path == nil || h.Path != *path || h.OldRange != p.OldRange || h.NewRange != p.NewRange {
			return fail("verification", invalid("invalid-field"), true)
		}
	}
	mapped := map[string]bool{}
	for _, f := range parsed.files {
		path := ""
		if f.oldPath != nil {
			path = *f.oldPath
		}
		if f.newPath != nil {
			if path != "" && path != *f.newPath {
				return fail("verification", invalid("invalid-field"), true)
			}
			path = *f.newPath
		}
		c, ok := canon.changed[path]
		a, b := c.a, c.b
		if !ok || mapped[path] || (f.oldPath == nil) != (a == nil) || (f.newPath == nil) != (b == nil) {
			return fail("verification", invalid("invalid-field"), true)
		}
		mapped[path] = true
		var old, new []byte
		if a != nil {
			if !regularFile(a) {
				return fail("repository", operational("unsupported-tree-mode"), false)
			}
			got, stage, e := s.lookup("simulation", base, path)
			if e != nil {
				return fail(stage, e, false)
			}
			if got == nil || got.oid != a.oid {
				return fail("repository", operational("repository-object-unavailable"), false)
			}
			old, stage, e = s.blob("simulation", a.oid)
			if e != nil {
				return fail(stage, e, false)
			}
		}
		if b != nil {
			if !regularFile(b) {
				return fail("repository", operational("unsupported-tree-mode"), false)
			}
			data, ok := canon.proof[b.oid]
			if !ok {
				return fail("repository", operational("repository-object-unavailable"), false)
			}
			new = data
		}
		if f.oldPath == nil && b != nil && f.mode != b.mode || f.newPath == nil && a != nil && f.mode != a.mode {
			return fail("verification", invalid("diff-metadata-mismatch"), true)
		}
		got, e := simulate(ctx, old, f.hunks)
		if e != nil {
			return fail("verification", e, true)
		}
		if !bytes.Equal(got, new) {
			return fail("verification", invalid("invalid-field"), true)
		}
	}
	if len(mapped) != len(canon.changed) {
		return fail("repository", operational("unsupported-patch-inventory"), false)
	}
	evidenceIDs := map[string]bool{}
	for i := range m.Evidence {
		item := &m.Evidence[i]
		if evidenceIDs[item.ID] {
			return fail("verification", invalid("invalid-field"), true)
		}
		evidenceIDs[item.ID] = true
		phase := "evidence:" + item.ID
		en, stage, e := s.lookup(phase, base, item.Path)
		if e != nil {
			return fail(stage, e, false)
		}
		if en == nil || !regularFile(en) || en.oid != item.BlobOID {
			return fail("verification", invalid("evidence-unavailable"), true)
		}
		data, stage, e := s.blob(phase, en.oid)
		if e != nil {
			return fail(stage, e, false)
		}
		if item.Span.End > uint64(len(data)) {
			return fail("verification", invalid("evidence-unavailable"), true)
		}
		item.data = append([]byte(nil), data[item.Span.Start:item.Span.End]...)
		if shaHex(item.data) != item.SpanSHA256 || "evidence:sha256:"+shaHex(canonicalEvidence(*item)) != item.ID {
			return fail("verification", invalid("fabricated-evidence-id"), true)
		}
	}
	cited := map[string]bool{}
	for _, h := range m.Hunks {
		pairs := map[string]bool{}
		for _, b := range h.Basis {
			key := b.EvidenceID + "\x00" + b.Relation
			if !evidenceIDs[b.EvidenceID] || pairs[key] {
				return fail("verification", invalid("invalid-field"), true)
			}
			pairs[key] = true
			cited[b.EvidenceID] = true
		}
	}
	for id := range evidenceIDs {
		if !cited[id] {
			return fail("verification", invalid("invalid-field"), true)
		}
	}
	mechanical := false
	for _, h := range m.Hunks {
		if h.Disposition != "mechanical" {
			continue
		}
		mechanical = true
		p := ph[h.ID]
		proven := false
		if structuralReason03(h.Reason) {
			f := groups[h.ID]
			if f.oldPath != nil {
				en, stage, e := s.lookup("mechanical", base, *f.oldPath)
				if e != nil {
					return fail(stage, e, false)
				}
				if en != nil && regularFile(en) {
					data, stage, e := s.blob("mechanical", en.oid)
					if e != nil {
						return fail(stage, e, false)
					}
					proven = structuralProof03(ctx, h.Path, data, f, p, h.Reason)
				}
			}
		} else {
			proven = proveMechanical(p, h.Reason)
		}
		if ctx.Err() != nil {
			return fail("verification", operational("verification-timeout"), false)
		}
		if !proven {
			return fail("verification", invalid("unproven-mechanical"), true)
		}
	}
	r.Axes["structuralProof"] = "NOT_REQUIRED"
	if mechanical {
		r.Axes["structuralProof"] = "PROVEN_DECLARED_FILE_PREDICATES"
	}
	publish := func() {
		sort.Slice(r.Drift, func(i, j int) bool { return r.Drift[i].EvidenceID < r.Drift[j].EvidenceID })
	}
	if _, stage, e := s.resolve("drift-target", target); e != nil {
		return fail(stage, e, false)
	}
	driftEvidence := append([]evidence(nil), m.Evidence...)
	sort.Slice(driftEvidence, func(i, j int) bool { return driftEvidence[i].ID < driftEvidence[j].ID })
	unsafe := false
	for _, ev := range driftEvidence {
		phase := "drift:" + ev.ID
		d := drift03{EvidenceID: ev.ID, Path: ev.Path, BaseBlobOID: ev.BlobOID, Status: "deleted"}
		en, stage, e := s.lookup(phase, target, ev.Path)
		if e != nil {
			publish()
			return fail(stage, e, false)
		}
		if en != nil && en.typ == "blob" {
			if en.oid == ev.BlobOID {
				d.Status = "stable"
				d.TargetBlobOID = string03(en.oid)
				d.TargetSpan = &nullableSpan{ev.Span.Start, ev.Span.End}
			} else if regularFile(en) {
				data, stage, e := s.blob(phase, en.oid)
				if e != nil {
					publish()
					return fail(stage, e, false)
				}
				d.TargetBlobOID = string03(en.oid)
				first, matches, err := firstTwoMatches(ctx, data, ev.data)
				if err != nil {
					publish()
					return fail("repository", operational("verification-timeout"), false)
				}
				switch matches {
				case 0:
					d.Status = "stale"
				case 1:
					d.Status = "relocated"
					d.TargetSpan = &nullableSpan{uint64(first), uint64(first + len(ev.data))}
				default:
					d.Status = "ambiguous"
				}
			}
		}
		if d.Status != "stable" && d.Status != "relocated" {
			unsafe = true
		}
		r.Drift = append(r.Drift, d)
	}
	publish()
	if unsafe {
		return fail("drift", invalid("evidence-drift"), true)
	}
	r.Axes["changeIntegrity"] = "VERIFIED"
	hooks.emit("before-artifacts")
	if ctx.Err() != nil {
		return fail("artifacts", operational("verification-timeout"), false)
	}
	checks, e := checkStableArtifacts(ctx, artifacts, m.Artifacts)
	if e != nil {
		return fail("artifacts", e, true)
	}
	r.ArtifactChecks = checks
	r.Axes["referenceIntegrity"] = "REFERENCE_INTEGRITY_ONLY"
	if len(m.Artifacts) == 0 {
		r.Axes["referenceIntegrity"] = "EMPTY_REFERENCE_SET"
	}
	hooks.emit("after-artifacts")
	if e := stableRevalidate(adm); e != nil {
		return fail("repository", e, false)
	}
	closed = true
	hooks.emit("close-start")
	stableClose(s)
	if s.hold {
		return failStable(&r, "repository", operational("unsupported-process-containment"), false)
	}
	hooks.emit("before-final-checkpoint")
	if ctx.Err() != nil {
		return failStable(&r, "verification", operational("verification-timeout"), false)
	}
	yes := true
	r.Accept = &yes
	r.Outcome = "ACCEPT"
	r.Stage = "complete"
	return r, 0
}

func executeStableCanonicalCLI(ctx context.Context, argv []string) (stableResult, int) {
	flags := map[string]string{}
	allowed := map[string]bool{"--repository": true, "--map": true, "--expected-base": true, "--target": true, "--artifacts": true}
	bad := len(argv)%2 != 0
	if !bad {
		for i := 0; i < len(argv); i += 2 {
			if !allowed[argv[i]] || flags[argv[i]] != "" || argv[i+1] == "" {
				bad = true
				break
			}
			flags[argv[i]] = argv[i+1]
		}
	}
	r := newStableResult(flags["--expected-base"], flags["--target"])
	r.RepositoryEnvelope = stableCanonicalEnvelope
	if bad || flags["--repository"] == "" || flags["--map"] == "" || !filepath.IsAbs(flags["--repository"]) || !filepath.IsAbs(flags["--map"]) {
		return failStable(&r, "arguments", operational("invalid-arguments"), false)
	}
	raw, e := readBounded(ctx, flags["--map"], maxJSONBytes)
	if e != nil {
		return failStable(&r, "input", operational("map-unavailable"), false)
	}
	return runStableCanonical(ctx, flags["--repository"], raw, flags["--expected-base"], flags["--target"], flags["--artifacts"], nil)
}

func runStableCanonicalCLI(argv []string) {
	ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
	r, status := executeStableCanonicalCLI(ctx, argv)
	cancel()
	_ = json.NewEncoder(os.Stdout).Encode(r)
	os.Exit(status)
}
