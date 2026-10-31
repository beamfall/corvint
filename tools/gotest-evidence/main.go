// Command gotest-evidence derives test evidence records from a `go test -json`
// stream (docs/specs/go-test-evidence-v0.md). The Go test suite stays the
// source of truth: the tool never runs tests, it records what one stream
// observed under a stable per-test identity, so a flaky test gains a durable
// cross-run history.
//
// Usage:
//
//	go test -json ./... | gotest-evidence record -ledger FILE -tree SHA [-toolchain V] [-platform OS/ARCH] [-command CMD]
//	gotest-evidence summary -ledger FILE
//
// record appends one record per terminal test or package outcome. A record's
// identity is its package, its test and subtest path, its ordinal among that
// test's outcomes in the stream, the stream's own digest and the declared
// provenance, so recording the same stream twice adds nothing and leaves the
// ledger byte-identical, and a renamed test is a new identity. Provenance the
// caller does not declare is NOT_OBSERVED. summary prints per-identity outcome
// counts and marks a test flaky only when it both passed and failed on the
// same tree, toolchain and platform. Nothing reads the ledger for ranking,
// authority or learning.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	schema      = "corvint-gotest-evidence/0"
	notObserved = "NOT_OBSERVED"
	// maxStream bounds the bytes of one input stream, maxLine one event and
	// maxRecords the records of one stream and of the whole ledger.
	maxStream  = 256 << 20
	maxLine    = 4 << 20
	maxRecords = 200000
	maxField   = 4096
)

var (
	treeSHA  = regexp.MustCompile(`^([0-9a-f]{40}|[0-9a-f]{64})$`)
	platform = regexp.MustCompile(`^[a-z0-9]+/[a-z0-9]+$`)
	// actions are every Action `go test -json` (test2json) emits.
	actions = map[string]bool{"start": true, "run": true, "pause": true, "cont": true, "pass": true, "bench": true, "fail": true, "output": true, "skip": true, "build-output": true, "build-fail": true}
)

// Record is one observed terminal outcome of a test, subtest or package.
type Record struct {
	Schema    string   `json:"schema"`
	ID        string   `json:"id"`
	Run       string   `json:"run"`
	Tree      string   `json:"tree"`
	Toolchain string   `json:"toolchain"`
	Platform  string   `json:"platform"`
	Command   string   `json:"command"`
	Package   string   `json:"package"`
	Test      string   `json:"test"`
	Path      []string `json:"path"`
	Ordinal   int      `json:"ordinal"`
	Outcome   string   `json:"outcome"`
	Elapsed   *float64 `json:"elapsed"`
	// Sha256 covers every other field, so an edited line is refused.
	Sha256 string `json:"sha256"`
}

// Provenance is what the caller declares about the run that produced a stream.
type Provenance struct {
	Tree, Toolchain, Platform, Command string
}

type event struct {
	Action  string   `json:"Action"`
	Package string   `json:"Package"`
	Test    string   `json:"Test"`
	Elapsed *float64 `json:"Elapsed"`
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprint(stdout, usage)
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	ledger := fs.String("ledger", "", "ledger file (JSON lines), required")
	var p Provenance
	if args[0] == "record" {
		fs.StringVar(&p.Tree, "tree", "", "full Git commit or tree SHA the tests ran on, required")
		fs.StringVar(&p.Toolchain, "toolchain", notObserved, "Go toolchain that ran the tests, such as go1.27.3")
		fs.StringVar(&p.Platform, "platform", notObserved, "GOOS/GOARCH the tests ran on")
		fs.StringVar(&p.Command, "command", notObserved, "the go test command line, as declared by the caller")
	}
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *ledger == "" || fs.NArg() != 0 {
		fmt.Fprint(stderr, "gotest-evidence: -ledger is required and no positional arguments are accepted\n")
		return 2
	}
	switch args[0] {
	case "record":
		added, err := record(*ledger, stdin, p)
		if err != nil {
			fmt.Fprintf(stderr, "gotest-evidence: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "gotest-evidence: %d new records\n", added)
		return 0
	case "summary":
		records, err := readLedger(*ledger)
		if err != nil {
			fmt.Fprintf(stderr, "gotest-evidence: %v\n", err)
			return 1
		}
		io.WriteString(stdout, summary(records))
		return 0
	}
	fmt.Fprintf(stderr, "gotest-evidence: unknown command %q\n", args[0])
	return 2
}

const usage = `usage:
  go test -json ./... | gotest-evidence record -ledger FILE -tree SHA [-toolchain V] [-platform OS/ARCH] [-command CMD]
  gotest-evidence summary -ledger FILE

record derives one evidence record per terminal test, subtest or package
outcome of the go test -json stream on stdin and appends the new ones to
FILE. -tree is the full SHA of the commit the tests ran on. Undeclared
toolchain, platform and command are NOT_OBSERVED. Recording the same stream
again adds nothing. summary prints per-test outcome counts across runs.

example:
  go test -json -count=3 ./internal/x | gotest-evidence record -ledger ~/.cache/corvint/gotest.jsonl \
    -tree "$(git rev-parse HEAD)" -toolchain "$(go env GOVERSION)" -platform linux/amd64 -command "go test -json -count=3 ./internal/x"
`

// Derive turns one go test -json stream into records. It refuses a stream
// that is not wholly test2json events, so a partial or foreign log is never
// recorded as evidence.
func Derive(stream []byte, p Provenance) ([]Record, error) {
	if !treeSHA.MatchString(p.Tree) {
		return nil, errors.New("-tree must be a full lowercase Git SHA")
	}
	if p.Platform != notObserved && !platform.MatchString(p.Platform) {
		return nil, errors.New("-platform must be GOOS/GOARCH")
	}
	for _, v := range []string{p.Toolchain, p.Command} {
		if v == "" || len(v) > maxField || strings.ContainsAny(v, "\x00\n") {
			return nil, errors.New("-toolchain and -command must be one non-empty line")
		}
	}
	if len(stream) > maxStream {
		return nil, errors.New("stream exceeds its byte bound")
	}
	sum := sha256.Sum256(stream)
	runID := hex.EncodeToString(sum[:])
	type key struct{ pkg, test string }
	terminal := map[key]int{}
	open := map[key]bool{}
	var order []key
	var records []Record
	add := func(k key, outcome string, elapsed *float64) error {
		if len(records) >= maxRecords {
			return errors.New("stream exceeds its record bound")
		}
		terminal[k]++
		r := Record{Schema: schema, Run: runID, Tree: p.Tree, Toolchain: p.Toolchain, Platform: p.Platform, Command: p.Command, Package: k.pkg, Test: k.test, Path: []string{}, Ordinal: terminal[k], Outcome: outcome, Elapsed: elapsed}
		if k.test != "" {
			r.Path = strings.Split(k.test, "/")
		}
		r.ID = recordID(r)
		r.Sha256 = recordSum(r)
		records = append(records, r)
		return nil
	}
	events := 0
	sc := bufio.NewScanner(bytes.NewReader(stream))
	sc.Buffer(make([]byte, 64<<10), maxLine)
	for sc.Scan() {
		line := sc.Bytes()
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var e event
		dec := json.NewDecoder(bytes.NewReader(line))
		if err := dec.Decode(&e); err != nil || dec.More() {
			return nil, fmt.Errorf("line %d is not a go test -json event", events+1)
		}
		events++
		if !actions[e.Action] {
			return nil, fmt.Errorf("line %d has unknown action %q", events, e.Action)
		}
		if e.Action == "build-output" || e.Action == "build-fail" {
			continue
		}
		if e.Package == "" || len(e.Package) > maxField || len(e.Test) > maxField {
			return nil, fmt.Errorf("line %d names no bounded package", events)
		}
		k := key{e.Package, e.Test}
		switch e.Action {
		case "run":
			if !open[k] {
				open[k] = true
				order = append(order, k)
			}
		case "pass", "fail", "skip":
			open[k] = false
			if err := add(k, e.Action, e.Elapsed); err != nil {
				return nil, err
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("stream: %w", err)
	}
	if events == 0 {
		return nil, errors.New("stream holds no go test -json events")
	}
	// A test that started and never ended (a panic, a timeout, a killed
	// process) is recorded as NOT_OBSERVED, never as a pass.
	for _, k := range order {
		if open[k] {
			open[k] = false
			if err := add(k, notObserved, nil); err != nil {
				return nil, err
			}
		}
	}
	return records, nil
}

func recordID(r Record) string {
	h := sha256.New()
	for _, f := range []string{schema, r.Run, r.Tree, r.Toolchain, r.Platform, r.Command, r.Package, r.Test, fmt.Sprint(r.Ordinal)} {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func recordSum(r Record) string {
	r.Sha256 = ""
	b, _ := json.Marshal(r)
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func record(ledger string, stdin io.Reader, p Provenance) (int, error) {
	stream, err := io.ReadAll(io.LimitReader(stdin, maxStream+1))
	if err != nil {
		return 0, err
	}
	derived, err := Derive(stream, p)
	if err != nil {
		return 0, err
	}
	existing, err := readLedger(ledger)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return 0, err
	}
	seen := map[string]bool{}
	for _, r := range existing {
		seen[r.ID] = true
	}
	var fresh []Record
	for _, r := range derived {
		if !seen[r.ID] {
			seen[r.ID] = true
			fresh = append(fresh, r)
		}
	}
	if len(fresh) == 0 {
		return 0, nil
	}
	if len(existing)+len(fresh) > maxRecords {
		return 0, errors.New("ledger would exceed its record bound")
	}
	var out bytes.Buffer
	for _, r := range append(existing, fresh...) {
		b, _ := json.Marshal(r)
		out.Write(b)
		out.WriteByte('\n')
	}
	return len(fresh), replace(ledger, out.Bytes())
}

// replace writes the whole ledger through a private temporary file and a
// rename, so a reader never sees a torn ledger.
func replace(ledger string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(ledger), "."+filepath.Base(ledger)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err = tmp.Write(data); err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp.Name(), ledger)
}

// readLedger refuses a ledger with any record it cannot verify, so a
// hand-edited or foreign line is never counted.
func readLedger(ledger string) ([]Record, error) {
	data, err := os.ReadFile(ledger)
	if err != nil {
		return nil, err
	}
	var records []Record
	for n, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		if len(data) == 0 {
			break
		}
		var r Record
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&r); err != nil || r.Schema != schema || r.ID != recordID(r) || r.Sha256 != recordSum(r) {
			return nil, fmt.Errorf("ledger line %d is not a verified %s record", n+1, schema)
		}
		if len(records) >= maxRecords {
			return nil, errors.New("ledger exceeds its record bound")
		}
		records = append(records, r)
	}
	return records, nil
}

// summary prints one tab-separated row per package and test: outcome counts,
// distinct runs, and whether the test is flaky, that is, passed and failed on
// the same tree, toolchain and platform.
func summary(records []Record) string {
	type row struct {
		counts map[string]int
		runs   map[string]bool
		passed map[string]bool
		failed map[string]bool
	}
	rows := map[[2]string]*row{}
	for _, r := range records {
		k := [2]string{r.Package, r.Test}
		x := rows[k]
		if x == nil {
			x = &row{map[string]int{}, map[string]bool{}, map[string]bool{}, map[string]bool{}}
			rows[k] = x
		}
		x.counts[r.Outcome]++
		x.runs[r.Run] = true
		env := r.Tree + "\x00" + r.Toolchain + "\x00" + r.Platform
		if r.Outcome == "pass" {
			x.passed[env] = true
		}
		if r.Outcome == "fail" {
			x.failed[env] = true
		}
	}
	keys := make([][2]string, 0, len(rows))
	for k := range rows {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	var b strings.Builder
	b.WriteString("package\ttest\tpass\tfail\tskip\tnot_observed\truns\tflaky\n")
	for _, k := range keys {
		x := rows[k]
		flaky := false
		for env := range x.passed {
			flaky = flaky || x.failed[env]
		}
		fmt.Fprintf(&b, "%s\t%s\t%d\t%d\t%d\t%d\t%d\t%t\n", k[0], k[1], x.counts["pass"], x.counts["fail"], x.counts["skip"], x.counts[notObserved], len(x.runs), flaky)
	}
	return b.String()
}
