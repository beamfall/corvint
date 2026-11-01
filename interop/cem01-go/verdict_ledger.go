package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The portable reader of the native verdict ledger (CEM-V1-754, V1-2023). It
// reads the note of one commit under refs/notes/corvint-verdicts and reports
// the same closed state, in the same precedence, and the same records as the
// native `cem provenance --verdicts` listing (CEM-V1-276, CEM-V1-277). It is
// read-only and writes no ref, note, object, index, config or worktree file.
// Like the native ledger it carries no authority: no verify route reads it.
const (
	verdictLedgerRef      = "refs/notes/corvint-verdicts"
	verdictLedgerSchema   = "corvint-cem-verdict/1"
	verdictLedgerMax      = 256
	verdictLedgerNoteMax  = 1 << 20 // the native MaxNoteBytes
	verdictLedgerListMax  = 4 << 20 // the native MaxListBytes
	verdictLedgerTextMax  = 256
	verdictLedgerConsumer = "native"
)

// ledgerRecord is the closed corvint-cem-verdict/1 record (CEM-V1-272). Its
// canonical bytes are json.Marshal of the struct, members in this order.
type ledgerRecord struct {
	Schema    string `json:"schema"`
	MapSha256 string `json:"mapSha256"`
	Target    struct {
		GitCommit string `json:"gitCommit"`
		GitTree   string `json:"gitTree"`
	} `json:"target"`
	Outcome       string `json:"outcome"`
	VerdictSha256 string `json:"verdictSha256"`
	Consumer      string `json:"consumer"`
	Runtime       struct {
		GoVersion string `json:"goVersion"`
	} `json:"runtime"`
	RecordedAt           string  `json:"recordedAt"`
	PreviousRecordSha256 *string `json:"previousRecordSha256"`
}

// ledgerListing is the native `verdicts` member's members.
type ledgerListing struct {
	Ref       string         `json:"ref"`
	Trust     string         `json:"trust"`
	Authority string         `json:"authority"`
	State     string         `json:"state"`
	Records   []ledgerRecord `json:"records"`
}

// ledgerRefusal is the closed refusal line; its codes are the native reader's.
type ledgerRefusal struct {
	OK   bool   `json:"ok"`
	Code string `json:"code"`
}

func runVerdictLedgerCLI(argv []string) {
	ctx, cancel := context.WithTimeout(context.Background(), verificationBudget)
	defer cancel()
	listing, code := readVerdictLedgerCLI(ctx, argv)
	if code != "" {
		_ = json.NewEncoder(os.Stdout).Encode(ledgerRefusal{Code: code})
		if code == "invalid-arguments" {
			fmt.Fprintln(os.Stderr, "cem01-go read-verdict-ledger: usage: read-verdict-ledger --repository ABSOLUTE_PATH --commit REV")
		}
		os.Exit(2)
	}
	_ = json.NewEncoder(os.Stdout).Encode(listing)
	os.Exit(0)
}

func readVerdictLedgerCLI(ctx context.Context, argv []string) (ledgerListing, string) {
	flags := map[string]string{}
	for i := 0; i < len(argv); i += 2 {
		name := argv[i]
		if (name != "--repository" && name != "--commit") || i+1 >= len(argv) || argv[i+1] == "" {
			return ledgerListing{}, "invalid-arguments"
		}
		if _, repeated := flags[name]; repeated {
			return ledgerListing{}, "invalid-arguments"
		}
		flags[name] = argv[i+1]
	}
	repo, rev := flags["--repository"], flags["--commit"]
	if repo == "" || rev == "" || !filepath.IsAbs(repo) {
		return ledgerListing{}, "invalid-arguments"
	}
	return readVerdictLedger(&verifier{ctx: ctx, repo: repo}, rev)
}

// readVerdictLedger mirrors the native readLedger: absent, then malformed,
// chain-broken, scope-mismatch and conflict, else verified (CEM-V1-276).
// Records are listed only when the state is verified (CEM-V1-277).
func readVerdictLedger(v *verifier, rev string) (ledgerListing, string) {
	// Admit the repository as the native gitauth.Open does: the exact worktree
	// root (primary or reciprocal linked worktree), no object alternates and no
	// effective info/attributes. A subdirectory, a bare repository and an
	// alternates-backed clone are refused, never read (CEM-V1-754).
	if _, e := stableAdmit(v.repo, nil); e != nil {
		if e.code == "unsupported-repository-envelope" {
			return ledgerListing{}, "unsupported-repository-attributes"
		}
		return ledgerListing{}, e.code
	}
	out, err := v.git("rev-parse", "--verify", "--end-of-options", rev+"^{commit}")
	if err != nil {
		return ledgerListing{}, "repository-object-unavailable"
	}
	target := strings.TrimSpace(string(out))
	listing := ledgerListing{Ref: verdictLedgerRef, Trust: "repository-history", Authority: "git-history", Records: []ledgerRecord{}}
	state, records, code := verdictLedgerState(v, target)
	if code != "" {
		return ledgerListing{}, code
	}
	listing.State = state
	if state == "verified" {
		listing.Records = records
	}
	return listing, ""
}

func verdictLedgerState(v *verifier, target string) (string, []ledgerRecord, string) {
	list, code := ledgerGit(v, verdictLedgerListMax, "notes", "--ref="+verdictLedgerRef, "list")
	if code != "" {
		return "", nil, code
	}
	noteOid := ""
	for _, line := range strings.Split(string(list), "\n") {
		if fields := strings.Fields(line); len(fields) == 2 && fields[1] == target {
			noteOid = fields[0]
		}
	}
	if noteOid == "" {
		return "absent", nil, ""
	}
	body, code := ledgerGit(v, verdictLedgerNoteMax, "cat-file", "blob", noteOid)
	if code != "" {
		return "", nil, code
	}
	if len(body) == 0 || body[len(body)-1] != '\n' {
		return "malformed", nil, ""
	}
	lines := bytes.Split(body[:len(body)-1], []byte("\n"))
	if len(lines) > verdictLedgerMax {
		return "malformed", nil, ""
	}
	records := make([]ledgerRecord, 0, len(lines))
	for _, line := range lines {
		var record ledgerRecord
		decoder := json.NewDecoder(bytes.NewReader(line))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&record) != nil || decoder.More() || !wellFormedLedgerRecord(record) {
			return "malformed", nil, ""
		}
		if canonical, e := json.Marshal(record); e != nil || !bytes.Equal(canonical, line) {
			return "malformed", nil, ""
		}
		records = append(records, record)
	}
	for i, record := range records {
		if i == 0 {
			if record.PreviousRecordSha256 != nil {
				return "chain-broken", nil, ""
			}
			continue
		}
		digest := sha256.Sum256(lines[i-1])
		if record.PreviousRecordSha256 == nil || *record.PreviousRecordSha256 != hex.EncodeToString(digest[:]) {
			return "chain-broken", nil, ""
		}
	}
	tree, err := v.git("rev-parse", "--verify", "--end-of-options", target+"^{tree}")
	if err != nil {
		return "", nil, "repository-io"
	}
	seen := map[string]bool{}
	for _, record := range records {
		if record.Target.GitCommit != target || record.Target.GitTree != strings.TrimSpace(string(tree)) {
			return "scope-mismatch", nil, ""
		}
		if seen[record.MapSha256] {
			return "conflict", nil, ""
		}
		seen[record.MapSha256] = true
	}
	return "verified", records, ""
}

// ledgerBuffer keeps at most limit bytes and records an overrun, so an
// oversized Git output refuses git-output-exceeded as the native gitrun bound does.
type ledgerBuffer struct {
	buffer   bytes.Buffer
	limit    int
	exceeded bool
}

func (b *ledgerBuffer) Write(p []byte) (int, error) {
	if len(p) > b.limit-b.buffer.Len() {
		b.exceeded = true
		return 0, errSizeLimit
	}
	return b.buffer.Write(p)
}

// ledgerGit runs one bounded read: more than limit bytes of standard output
// refuses git-output-exceeded, any other failure repository-io.
func ledgerGit(v *verifier, limit int, args ...string) ([]byte, string) {
	out := &ledgerBuffer{limit: limit}
	if e := v.gitTo(out, nil, args...); e != nil || out.exceeded {
		if out.exceeded {
			return nil, "git-output-exceeded"
		}
		return nil, "repository-io"
	}
	return out.buffer.Bytes(), ""
}

// wellFormedLedgerRecord checks every member's value domain (CEM-V1-272).
func wellFormedLedgerRecord(r ledgerRecord) bool {
	hex64 := func(s string) bool { return len(s) == 64 && lowerHex(s) }
	oid := func(s string) bool { return (len(s) == 40 || len(s) == 64) && lowerHex(s) }
	printable := func(s string) bool {
		for i := 0; i < len(s); i++ {
			if s[i] < 0x20 || s[i] > 0x7e {
				return false
			}
		}
		return true
	}
	recorded, err := time.Parse(time.RFC3339, r.RecordedAt)
	return r.Schema == verdictLedgerSchema && hex64(r.MapSha256) && hex64(r.VerdictSha256) &&
		oid(r.Target.GitCommit) && oid(r.Target.GitTree) &&
		(r.Outcome == "ACCEPT" || r.Outcome == "REJECT") && r.Consumer == verdictLedgerConsumer &&
		r.Runtime.GoVersion != "" && len(r.Runtime.GoVersion) <= verdictLedgerTextMax && printable(r.Runtime.GoVersion) &&
		err == nil && recorded.UTC().Format(time.RFC3339) == r.RecordedAt &&
		(r.PreviousRecordSha256 == nil || hex64(*r.PreviousRecordSha256))
}
