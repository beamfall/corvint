package analyzercap

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// ceilingWarnMargin is the width of the warning band below each candidate's
// ceiling. It is derived from the ceiling amendments this test has actually
// taken (git history of the `amended` table): the observed growth steps are
// 12, 104, 321, 899, 1_924, 2_421, 2_433, 2_563, 6_003 and 12_730 bytes —
// median ~2.2KiB, and 8 of 10 at or below 4KiB. A 4KiB band therefore means a
// typical growth step lands a candidate IN the band rather than through the
// ceiling, giving one commit of prior signal; only the two outlier steps could
// clear the band in a single move. It is also ~6% of the base ceiling, small
// enough that it can never be read as slack. The band only warns — a candidate
// inside it (four sit at exactly zero headroom today) still passes.
const ceilingWarnMargin = 4_096

// ceilingVerdict classifies one candidate's measurement against its ceiling.
// Only verdictOver fails the test: the band is a signal, never a ratchet.
type ceilingVerdict int

const (
	verdictOK ceilingVerdict = iota
	verdictWarn
	verdictOver
)

// classifyCeiling preserves exactly what this test has always guaranteed: an
// upper bound. A candidate that shrinks is not an error (the ratchet has never
// pinned an exact count), a candidate over its ceiling always is.
func classifyCeiling(total, ceiling, margin int) ceilingVerdict {
	switch {
	case total > ceiling:
		return verdictOver
	case ceiling-total <= margin:
		return verdictWarn
	default:
		return verdictOK
	}
}

func (v ceilingVerdict) label() string {
	switch v {
	case verdictOver:
		return "OVER"
	case verdictWarn:
		return "WARN"
	default:
		return "ok"
	}
}

// closure is the measured dependency closure of one candidate executable.
type closure struct {
	total    int
	files    int
	packages []packageBytes
}

type packageBytes struct {
	importPath string
	bytes      int
	files      int
}

// candidateMeasurement pairs one candidate's measured closure with the ceiling
// it was judged against.
type candidateMeasurement struct {
	name    string
	ceiling int
	verdict ceilingVerdict
	closure closure
}

func (m candidateMeasurement) headroom() int { return m.ceiling - m.closure.total }

// writeCeilingWarnings emits one line per candidate inside the warning band and
// nothing at all for any other verdict, so a run with every candidate clear of
// its band stays byte-for-byte silent.
//
// Warnings go to the test binary's stderr rather than t.Logf because t.Logf on
// a PASSING test is only rendered under -v: the near-miss this band exists to
// catch — a candidate a few bytes under its ceiling on an otherwise green run —
// would print nothing exactly when it matters. Note the reach of this is set by
// how `go test` is invoked, not by the writer: run in local-directory mode
// (`cd internal/analyzercap && go test`) the lines stream to the terminal on a
// green run, while in package-list mode (`go test ./...`) the go tool buffers
// per-package output and discards it unless the package fails.
func writeCeilingWarnings(w io.Writer, measurements []candidateMeasurement) {
	for _, m := range measurements {
		if m.verdict != verdictWarn {
			continue
		}
		fmt.Fprintf(w, "analyzercap: ACP-009 WARNING: candidate %s has only %d bytes of headroom (%d/%d), "+
			"inside the %d-byte warning band. Its closure is %s. Adding production source reachable from "+
			"this executable will fail TestCandidateSourceCeilingPerExecutable.\n",
			m.name, m.headroom(), m.closure.total, m.ceiling, ceilingWarnMargin, describeClosure(m.closure))
	}
}

// TestCandidateSourceCeilingPerExecutable enforces ACP-009's per-candidate
// source ceiling as a deterministic per-executable measurement: the byte total
// of every non-test Go production file in every first-party package of the
// candidate CLI's dependency closure. Amended ceilings mirror the
// per-candidate table in docs/specs/analyzer-candidate-profiles.md.
//
// The measurement is reported for every candidate on every run, pass or fail,
// so growth is visible before it trips rather than only once it has.
func TestCandidateSourceCeilingPerExecutable(t *testing.T) {
	const baseCeiling = 65_536
	amended := map[string]int{
		"corvint-analyzer-js":          92_045,
		"corvint-analyzer-python":      90_197,
		"corvint-analyzer-c-jni":       76_380,
		"corvint-analyzer-java":        76_382,
		"corvint-analyzer-objective-c": 76_395,
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		t.Fatal(err)
	}
	measurements := make([]candidateMeasurement, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || !strings.HasPrefix(name, "corvint-analyzer-") {
			continue
		}
		measured := reachableProductionSource(t, root, "./cmd/"+name)
		ceiling := baseCeiling
		if amendedCeiling, ok := amended[name]; ok {
			ceiling = amendedCeiling
		}
		measurements = append(measurements, candidateMeasurement{
			name:    name,
			ceiling: ceiling,
			verdict: classifyCeiling(measured.total, ceiling, ceilingWarnMargin),
			closure: measured,
		})
	}
	if len(measurements) == 0 {
		t.Fatal("no candidate CLIs enumerated under cmd/")
	}

	// Full table stays on t.Logf: it is diagnostic detail for a -v or failing
	// run, and promoting every row to stderr on every run would be noise.
	t.Logf("ACP-009 reachable production source per candidate executable (bytes); warning band = %d", ceilingWarnMargin)
	t.Logf("  %-30s %9s %9s %10s  %s", "candidate", "bytes", "ceiling", "headroom", "status")
	for _, m := range measurements {
		t.Logf("  %-30s %9d %9d %+10d  %s",
			m.name, m.closure.total, m.ceiling, m.headroom(), m.verdict.label())
	}

	writeCeilingWarnings(os.Stderr, measurements)

	for _, m := range measurements {
		if m.verdict != verdictOver {
			continue
		}
		t.Errorf("candidate %s reachable production source=%d exceeds ceiling %d by %d bytes.\n"+
			"    ACP-009 reachable production source per candidate executable (bytes):\n%s\n"+
			"    closure: %s\n"+
			"    largest packages: %s\n"+
			"    ACP-009 is a ratchet against candidate bloat: the fix is to shrink or re-scope this "+
			"closure. Raising the ceiling in the amended table (and in "+
			"docs/specs/analyzer-candidate-profiles.md) is an operator adjudication, not a routine "+
			"test repair — do not amend it to make this run green.",
			m.name, m.closure.total, m.ceiling, -m.headroom(), formatCeilingTable(measurements),
			describeClosure(m.closure), describeLargestPackages(m.closure, 5))
	}
}

func formatCeilingTable(measurements []candidateMeasurement) string {
	var b strings.Builder
	fmt.Fprintf(&b, "    %-30s %9s %9s %10s  %s", "candidate", "bytes", "ceiling", "headroom", "status")
	for _, m := range measurements {
		fmt.Fprintf(&b, "\n    %-30s %9d %9d %+10d  %s",
			m.name, m.closure.total, m.ceiling, m.headroom(), m.verdict.label())
	}
	return b.String()
}

func describeClosure(c closure) string {
	return fmt.Sprintf("%d first-party packages, %d production files, %d bytes",
		len(c.packages), c.files, c.total)
}

func describeLargestPackages(c closure, top int) string {
	ranked := append([]packageBytes(nil), c.packages...)
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].bytes != ranked[j].bytes {
			return ranked[i].bytes > ranked[j].bytes
		}
		return ranked[i].importPath < ranked[j].importPath
	})
	if len(ranked) > top {
		ranked = ranked[:top]
	}
	described := make([]string, 0, len(ranked))
	for _, pkg := range ranked {
		described = append(described, fmt.Sprintf("%s=%d", pkg.importPath, pkg.bytes))
	}
	return strings.Join(described, ", ")
}

func reachableProductionSource(t *testing.T, root, target string) closure {
	t.Helper()
	deps := goListLines(t, root, "list", "-deps", target)
	firstParty := make([]string, 0, len(deps))
	for _, dep := range deps {
		if strings.HasPrefix(dep, "github.com/Beamfall/corvint") {
			firstParty = append(firstParty, dep)
		}
	}
	if len(firstParty) == 0 {
		t.Fatalf("candidate %s reaches no first-party packages", target)
	}
	records := goListLines(t, root, append([]string{"list", "-f", `{{.ImportPath}}|{{.Dir}}|{{join .GoFiles " "}}`}, firstParty...)...)
	measured := closure{packages: make([]packageBytes, 0, len(records))}
	for _, record := range records {
		parts := strings.SplitN(record, "|", 3)
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" {
			t.Fatalf("malformed source record=%q", record)
		}
		pkg := packageBytes{importPath: parts[0]}
		for _, file := range strings.Fields(parts[2]) {
			body, err := os.ReadFile(filepath.Join(parts[1], file))
			if err != nil {
				t.Fatal(err)
			}
			pkg.bytes += len(body)
			pkg.files++
		}
		measured.total += pkg.bytes
		measured.files += pkg.files
		measured.packages = append(measured.packages, pkg)
	}
	return measured
}

// TestClassifyCeiling is the negative control for the warning band: it pins
// that the band never converts an over-ceiling candidate into a warning, and
// that a candidate at exactly zero headroom still passes.
func TestClassifyCeiling(t *testing.T) {
	const margin = 4_096
	cases := []struct {
		name    string
		total   int
		ceiling int
		want    ceilingVerdict
	}{
		{name: "one byte over still fails", total: 65_537, ceiling: 65_536, want: verdictOver},
		{name: "far over still fails", total: 93_612, ceiling: 90_197, want: verdictOver},
		{name: "exactly at ceiling warns, does not fail", total: 76_351, ceiling: 76_351, want: verdictWarn},
		{name: "inside band warns", total: 64_508, ceiling: 65_536, want: verdictWarn},
		{name: "at band edge warns", total: 65_536 - margin, ceiling: 65_536, want: verdictWarn},
		{name: "one byte outside band is quiet", total: 65_536 - margin - 1, ceiling: 65_536, want: verdictOK},
		{name: "well under is quiet", total: 1_024, ceiling: 65_536, want: verdictOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyCeiling(tc.total, tc.ceiling, margin); got != tc.want {
				t.Fatalf("classifyCeiling(%d, %d, %d)=%s, want %s",
					tc.total, tc.ceiling, margin, got.label(), tc.want.label())
			}
		})
	}
}

// TestWriteCeilingWarnings pins the emission contract independently of the
// filesystem: in-band candidates each produce exactly one named line, and a set
// with nothing in the band produces no output whatsoever.
func TestWriteCeilingWarnings(t *testing.T) {
	measure := func(name string, total, ceiling int) candidateMeasurement {
		return candidateMeasurement{
			name:    name,
			ceiling: ceiling,
			verdict: classifyCeiling(total, ceiling, ceilingWarnMargin),
			closure: closure{total: total, files: 3, packages: []packageBytes{{importPath: "p", bytes: total, files: 3}}},
		}
	}

	t.Run("nothing in band writes nothing", func(t *testing.T) {
		var buf bytes.Buffer
		writeCeilingWarnings(&buf, []candidateMeasurement{
			measure("corvint-analyzer-quiet", 1_024, 65_536),
			measure("corvint-analyzer-edge", 65_536-ceilingWarnMargin-1, 65_536),
		})
		if buf.Len() != 0 {
			t.Fatalf("expected no output for out-of-band candidates, got %d bytes: %q", buf.Len(), buf.String())
		}
	})

	t.Run("over-ceiling candidates do not warn", func(t *testing.T) {
		var buf bytes.Buffer
		writeCeilingWarnings(&buf, []candidateMeasurement{measure("corvint-analyzer-over", 70_000, 65_536)})
		if buf.Len() != 0 {
			t.Fatalf("an OVER candidate must fail, not warn; got %q", buf.String())
		}
	})

	t.Run("in-band candidates each warn once by name", func(t *testing.T) {
		var buf bytes.Buffer
		writeCeilingWarnings(&buf, []candidateMeasurement{
			measure("corvint-analyzer-quiet", 1_024, 65_536),
			measure("corvint-analyzer-zero", 76_351, 76_351),
			measure("corvint-analyzer-near", 64_508, 65_536),
		})
		lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected 2 warning lines, got %d: %q", len(lines), buf.String())
		}
		for _, want := range []string{"corvint-analyzer-zero", "corvint-analyzer-near"} {
			if !strings.Contains(buf.String(), want) {
				t.Errorf("warning output does not name %s: %q", want, buf.String())
			}
		}
		if strings.Contains(buf.String(), "corvint-analyzer-quiet") {
			t.Errorf("out-of-band candidate must not be named: %q", buf.String())
		}
	})
}

func goListLines(t *testing.T, root string, args ...string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", args...)
	command.Dir = root
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go %s: %v", strings.Join(args, " "), err)
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n")
}
