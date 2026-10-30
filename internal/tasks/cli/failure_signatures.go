package cli

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Beamfall/corvint/internal/tasks/mutation"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// The failure-signature ledger (TOL-V0-051..055, GitHub #1048). It is
// disabled unless a run names --refuse-repeat N. It lives beside the
// preflight cache under the user cache directory, outside every
// repository, Tasks store and trace store, and only the verb that wrote an
// entry reads it back to decide whether to run; it is never evidence, a
// witness, a gate result or authority.
const (
	failureLedgerProfile    = "corvint-tasks-failure-signature/0"
	remediationAuditProfile = "corvint-tasks-remediation-audit/0"
	failureLedgerMaxEntries = 1024
	failureLedgerMaxAge     = 30 * 24 * time.Hour
	failureLedgerMaxBytes   = 64 << 10
	remediationAuditMax     = 4 << 20
	maxRefuseRepeat         = 20
	maxRemediationsPerEntry = 16
	maxRemediationBytes     = 256
	remediationAuditFile    = "remediations.jsonl"
	failureLedgerLock       = ".lock"
	failureLedgerLockWait   = 10 * time.Second
)

// Detail prefixes of the TOL-V0-052 and TOL-V0-053 refusals.
const (
	failureSignatureDetail = "FAILURE_SIGNATURE_REPEATED:"
	remediationUsedDetail  = "FAILURE_REMEDIATION_REUSED:"
)

// failureGuardArgs are the --refuse-repeat and --remediation flags.
type failureGuardArgs struct {
	refuseRepeat string
	remediation  string
}

func (a *failureGuardArgs) singles(m map[string]*string) map[string]*string {
	m["--refuse-repeat"], m["--remediation"] = &a.refuseRepeat, &a.remediation
	return m
}

// check validates the flags; it returns the threshold, 0 when disabled.
func (a *failureGuardArgs) check(cmd []string) (int, *wire.Result) {
	if a.refuseRepeat == "" {
		if a.remediation != "" {
			return 0, usage(cmd, "--remediation clears a failure-signature refusal: give it with --refuse-repeat N")
		}
		return 0, nil
	}
	n, err := strconv.Atoi(a.refuseRepeat)
	if err != nil || n < 1 || n > maxRefuseRepeat || strconv.Itoa(n) != a.refuseRepeat {
		return 0, usage(cmd, "--refuse-repeat is a count from 1 to "+strconv.Itoa(maxRefuseRepeat))
	}
	if a.remediation != "" {
		if len(a.remediation) > maxRemediationBytes || strings.TrimSpace(a.remediation) == "" || !utf8.ValidString(a.remediation) ||
			strings.ContainsFunc(a.remediation, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
			return 0, usage(cmd, "--remediation is 1.."+strconv.Itoa(maxRemediationBytes)+" bytes of single-line text naming the fix (a commit, ticket note or issue)")
		}
		if mutation.MatchesSecretScreen(a.remediation) {
			return 0, errorResult(cmd, wire.Errorf(wire.CodeSecretDetected, "--remediation", "--remediation matches the secret screen; name the fix without the secret"))
		}
	}
	return n, nil
}

// failureEntry is one ledger entry: the last failure signature at one
// verb, ticket and input digest, how many consecutive runs produced it,
// and the remediation references already spent on it.
type failureEntry struct {
	signature    string
	count        int
	remediations []string
}

// failureGuard is one run's use of the ledger.
type failureGuard struct {
	dir, verb, ticketID string
	input               wire.Digest
	threshold           int
	remediation         string
	prior               failureEntry
	admitted            bool   // a remediation admitted this run (TOL-V0-053)
	reason              string // non-empty: the ledger is unavailable for this run
}

// signatureTokens matches an identifier token (a letter, then letters and
// digits, with "-" or "_" joined parts, such as AC-1 or V1-1605), kept
// whole, or a bare digit run, folded to "#".
var signatureTokens = regexp.MustCompile(`[A-Za-z][A-Za-z0-9]*(?:[-_][A-Za-z0-9]+)*|[0-9]+`)

// normalizeFailure folds run-specific numbers and spacing out of an error
// line, so durations, ports and counts do not split one signature, while
// the digits of structural identifiers keep AC-1 and AC-2 apart
// (TOL-V0-051).
func normalizeFailure(s string) string {
	s = signatureTokens.ReplaceAllStringFunc(s, func(t string) string {
		if t[0] >= '0' && t[0] <= '9' {
			return "#"
		}
		return t
	})
	return strings.Join(strings.Fields(s), " ")
}

// failureSignature digests sorted normalized failure lines.
func failureSignature(lines []string) string {
	norm := make([]string, 0, len(lines))
	for _, l := range lines {
		norm = append(norm, normalizeFailure(l))
	}
	sort.Strings(norm)
	return string(wire.Sum([]byte(strings.Join(norm, "\n") + "\n")))
}

func (g *failureGuard) entryPath() string {
	name := wire.Sum([]byte(failureLedgerProfile + "\nverb " + g.verb + "\nticket " + strconv.Quote(g.ticketID) + "\ninput " + string(g.input) + "\n"))
	return filepath.Join(g.dir, string(name)+".json")
}

// startFailureGuard opens the ledger for verb, ticket and input digest. A
// threshold of 0 (the default) returns nil: the ledger is not read or
// written. An unavailable input digest or ledger runs the job without the
// guard and reports why (TOL-V0-054).
func startFailureGuard(verb, ticketID string, input wire.Digest, reason string, threshold int, remediation string) *failureGuard {
	if threshold == 0 {
		return nil
	}
	g := &failureGuard{verb: verb, ticketID: ticketID, input: input, threshold: threshold, remediation: remediation, reason: reason}
	if g.reason != "" {
		return g
	}
	dir, err := localCacheDir("failure-signatures")
	if err != nil {
		g.reason = "the failure-signature ledger is unavailable: " + err.Error()
		return g
	}
	g.dir = dir
	return g
}

// lock takes the ledger lock, so that each read-check-write of an entry
// and the audit append are serialized across concurrent runs (TOL-V0-054).
func (g *failureGuard) lock() (func(), error) {
	return lockFailureLedger(filepath.Join(g.dir, failureLedgerLock), failureLedgerLockWait)
}

// admit refuses the run when the stored signature has already repeated
// threshold times at this input digest (TOL-V0-052), unless an unspent
// --remediation admits it once and is audited (TOL-V0-053). The entry is
// read, checked and the remediation spent under the ledger lock.
func (g *failureGuard) admit() error {
	if g == nil || g.reason != "" {
		return nil
	}
	unlock, err := g.lock()
	if err != nil {
		g.reason = "the failure-signature ledger is unavailable: " + err.Error()
		return nil
	}
	defer unlock()
	e, problem := readFailureEntry(g.entryPath())
	if problem != "" {
		g.reason = problem
		return nil
	}
	g.prior = e
	if g.prior.count < g.threshold {
		return nil
	}
	if g.remediation == "" {
		return wire.Errorf(wire.CodeLoopDetected, "--refuse-repeat", "%s %s failed %d times with signature %s at unchanged input digest %s; change an input or declare the fix with --remediation REF",
			failureSignatureDetail, g.verb, g.prior.count, g.prior.signature, g.input)
	}
	for _, used := range g.prior.remediations {
		if used == g.remediation {
			return wire.Errorf(wire.CodeLoopDetected, "--remediation", "%s remediation %s was already spent on signature %s (%d failures) at input digest %s; a remediation clears the refusal once",
				remediationUsedDetail, strconv.Quote(g.remediation), g.prior.signature, g.prior.count, g.input)
		}
	}
	if len(g.prior.remediations) >= maxRemediationsPerEntry {
		return wire.Errorf(wire.CodeLimitExceeded, "--remediation", "%d remediations were already spent at input digest %s; change an input", maxRemediationsPerEntry, g.input)
	}
	if err := appendRemediationAudit(g.dir, g, localNow()); err != nil {
		return err
	}
	g.admitted = true
	// The count is kept: the admitted run is the one run the remediation
	// allows, so its identical failure refuses the next run at any N.
	g.prior.remediations = append(g.prior.remediations, g.remediation)
	return writeFailureEntry(g.entryPath(), g.prior)
}

// record folds the run's outcome into the ledger: a pass removes the
// entry, a failure with the stored signature counts one more, and a new
// signature starts at one (TOL-V0-051). It returns the item member.
func (g *failureGuard) record(failed bool, lines []string) wire.Value {
	if g == nil {
		return wire.Null()
	}
	o := wire.NewObject().Set("threshold", wire.String(strconv.Itoa(g.threshold)))
	if g.reason != "" {
		return wire.ObjectValue(o.Set("status", wire.String("SKIPPED")).Set("inputDigest", wire.Null()).
			Set("signature", wire.Null()).Set("count", wire.String("0")).Set("remediation", wire.Null()).Set("reason", wire.String(prose(g.reason))))
	}
	e, status, sig := g.prior, "CLEARED", ""
	// Re-read under the lock: a concurrent run may have counted a failure
	// or spent a remediation since admit (TOL-V0-054).
	unlock, err := g.lock()
	if err == nil {
		defer unlock()
		cur, problem := readFailureEntry(g.entryPath())
		if problem != "" {
			err = errors.New(problem)
		} else {
			e = cur
		}
	}
	if failed {
		sig, status = failureSignature(lines), "RECORDED"
		if e.signature == sig {
			e.count++
		} else {
			e.signature, e.count = sig, 1
		}
		if err == nil {
			err = writeFailureEntry(g.entryPath(), e)
		}
	} else {
		e.count = 0
		if err == nil {
			if rerr := os.Remove(g.entryPath()); rerr != nil && !errors.Is(rerr, fs.ErrNotExist) {
				err = rerr
			}
		}
	}
	reason := wire.Null()
	if err != nil {
		reason = wire.String(prose("the ledger was not updated: " + err.Error()))
	}
	sigValue, remediation := wire.Null(), wire.Null()
	if sig != "" {
		sigValue = wire.String(sig)
	}
	if g.admitted {
		remediation = wire.String(prose(g.remediation))
	}
	return wire.ObjectValue(o.Set("status", wire.String(status)).Set("inputDigest", wire.String(string(g.input))).
		Set("signature", sigValue).Set("count", wire.String(strconv.Itoa(e.count))).Set("remediation", remediation).Set("reason", reason))
}

// readFailureEntry reads one entry; a missing or expired entry is empty
// and anything else unreadable is a problem that disables the guard.
func readFailureEntry(p string) (failureEntry, string) {
	var e failureEntry
	st, err := os.Lstat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return e, ""
	}
	if err != nil || !st.Mode().IsRegular() || st.Size() > failureLedgerMaxBytes {
		return e, "the failure-signature entry is not a readable regular file within the bound"
	}
	if localNow().Sub(st.ModTime()) > failureLedgerMaxAge {
		return e, ""
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return e, "the failure-signature entry could not be read"
	}
	v, err := wire.Parse(raw)
	if err != nil {
		return e, "the failure-signature entry is not a canonical document"
	}
	r := wire.NewReader(v, "/")
	r.Closed("profile", "signature", "count", "remediations")
	_ = r.Profile(failureLedgerProfile)
	sig := r.Field("signature").String()
	count := r.Field("count").String()
	rem := r.Field("remediations").Value()
	n, cerr := strconv.Atoi(count)
	if r.Err() != nil || cerr != nil || n < 0 || rem.Kind != wire.KindArray || len(rem.Arr) > maxRemediationsPerEntry {
		return e, "the failure-signature entry does not match its profile"
	}
	for _, x := range rem.Arr {
		if x.Kind != wire.KindString {
			return e, "the failure-signature entry does not match its profile"
		}
		e.remediations = append(e.remediations, x.Str)
	}
	e.signature, e.count = sig, n
	return e, ""
}

// writeFailureEntry writes atomically and prunes the ledger to its bounds.
func writeFailureEntry(p string, e failureEntry) error {
	rem := make([]wire.Value, 0, len(e.remediations))
	for _, r := range e.remediations {
		rem = append(rem, wire.String(r))
	}
	raw := wire.EncodeFile(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(failureLedgerProfile)).
		Set("signature", wire.String(e.signature)).Set("count", wire.String(strconv.Itoa(e.count))).Set("remediations", wire.Array(rem...))))
	dir := filepath.Dir(p)
	f, err := os.CreateTemp(dir, ".entry-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(raw)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp, p)
	}
	if werr != nil {
		_ = os.Remove(tmp)
		return werr
	}
	pruneCacheDir(dir, failureLedgerMaxEntries, failureLedgerMaxAge)
	return nil
}

// appendRemediationAudit appends one line to the remediation audit log
// before the remediation takes effect. A log at its bound refuses the
// remediation rather than dropping the record.
func appendRemediationAudit(dir string, g *failureGuard, at time.Time) error {
	p := filepath.Join(dir, remediationAuditFile)
	line := wire.Encode(wire.ObjectValue(wire.NewObject().Set("profile", wire.String(remediationAuditProfile)).
		Set("at", wire.String(at.UTC().Format(time.RFC3339))).Set("verb", wire.String(g.verb)).Set("ticketId", wire.String(g.ticketID)).
		Set("inputDigest", wire.String(string(g.input))).Set("signature", wire.String(g.prior.signature)).
		Set("count", wire.String(strconv.Itoa(g.prior.count))).Set("remediation", wire.String(g.remediation))))
	line = append(bytes.TrimRight(line, "\n"), '\n')
	if st, err := os.Lstat(p); err == nil {
		if !st.Mode().IsRegular() {
			return wire.Errorf(wire.CodeUnsupported, "--remediation", "the remediation audit log is not a regular file")
		}
		if st.Size()+int64(len(line)) > remediationAuditMax {
			return wire.Errorf(wire.CodeLimitExceeded, "--remediation", "the remediation audit log %s reached %d bytes; archive it to record another remediation", p, remediationAuditMax)
		}
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return wire.Errorf(wire.CodeUnsupported, "--remediation", "cannot open the remediation audit log: %v", err)
	}
	_, werr := io.Copy(f, bytes.NewReader(line))
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		return wire.Errorf(wire.CodeUnsupported, "--remediation", "cannot write the remediation audit log: %v", werr)
	}
	return nil
}
