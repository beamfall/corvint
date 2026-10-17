//go:build darwin || linux

package cli_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/tasks/cli"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// TestJRTV0012_RetireCLIAndDoctorJournal (JRT-V0-001, JRT-V0-012,
// JRT-V0-014): compact help says it reclaims superseded liveness bodies only
// and points to retire; doctor reports the journal size, growth rate and
// thresholds and raises JOURNAL_GROWTH past one; `receipt retire` needs an
// archive outside the store, its dry run writes nothing, and retire and
// unretire round-trip with doctor naming the retired prefix in between.
func TestJRTV0012_RetireCLIAndDoctorJournal(t *testing.T) {
	root, _ := leaseCLIStore(t, 0, time.Now().UTC().Add(-time.Minute))
	help := atm(t, root, nil, "receipt", "compact", "--help", "--verbose")
	if note := field(help.res.Items[0], "note").Str; !strings.Contains(note, "superseded liveness bodies only") || !strings.Contains(note, "receipt retire") {
		t.Fatalf("compact help: %s", help.stdout)
	}
	for _, verb := range []string{"retire", "unretire"} {
		if x := atm(t, root, nil, "receipt", verb, "--help"); x.res.Outcome != wire.OutcomeOK || !strings.Contains(field(x.res.Items[0], "usage").Str, "--archive") {
			t.Fatalf("%s help: %s", verb, x.stdout)
		}
	}

	_, item := doctorOK(t, root)
	j := field(item, "journal")
	receipts, err := strconv.ParseUint(field(j, "receipts").Str, 10, 64)
	if err != nil || receipts == 0 || field(j, "bytes").Str == "0" || field(j, "retiredThrough").Str != "0" || field(j, "thresholdReceipts").Str != "16384" {
		t.Fatalf("doctor journal: %s", wire.Encode(item))
	}
	if len(findingsOf(item, "JOURNAL_GROWTH")) != 0 {
		t.Fatalf("small journal raised JOURNAL_GROWTH: %s", wire.Encode(item))
	}
	restore := cli.SetDoctorJournalThresholds(0, 1<<40)
	_, item = doctorOK(t, root)
	restore()
	if f := findingsOf(item, "JOURNAL_GROWTH"); len(f) != 1 || !strings.Contains(field(f[0], "remedy").Str, "receipt retire") {
		t.Fatalf("JOURNAL_GROWTH past the receipt threshold: %s", wire.Encode(item))
	}

	if x := atm(t, root, nil, "receipt", "retire", "--dry-run"); x.res.Outcome == wire.OutcomeOK || !hasCode(x.res, wire.CodeUnsupported) {
		t.Fatalf("retire of a one-receipt journal: %s", x.stdout)
	}
	planTicket(t, root, "first", "P1", `["src/"]`)
	planTicket(t, root, "second", "P2", `["doc/"]`)
	_, item = doctorOK(t, root)
	if receipts, err = strconv.ParseUint(field(field(item, "journal"), "receipts").Str, 10, 64); err != nil || receipts < 3 {
		t.Fatalf("doctor journal after two tickets: %s", wire.Encode(item))
	}
	if x := atm(t, root, nil, "receipt", "retire"); x.code == 0 || x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("retire without --archive: %s", x.stdout)
	}
	if x := atm(t, root, nil, "receipt", "unretire", "--dry-run", "--archive", "a.tar"); x.code == 0 || x.res.Outcome == wire.OutcomeOK {
		t.Fatalf("unretire accepted --dry-run: %s", x.stdout)
	}
	dry := atm(t, root, nil, "receipt", "retire", "--dry-run")
	if dry.res.Outcome != wire.OutcomeOK || field(dry.res.Items[0], "dryRun").Kind != wire.KindBool || !field(dry.res.Items[0], "dryRun").Bool {
		t.Fatalf("retire dry run: %s", dry.stdout)
	}
	path := filepath.Join(t.TempDir(), "retired.tar")
	done := atm(t, root, nil, "receipt", "retire", "--archive", path)
	if done.code != 0 || done.res.Outcome != wire.OutcomeOK || field(done.res.Items[0], "anchorSeq").Str != strconv.FormatUint(receipts, 10) {
		t.Fatalf("retire: %s", done.stdout)
	}
	_, item = doctorOK(t, root)
	if j := field(item, "journal"); field(j, "receipts").Str != "1" || field(j, "retiredThrough").Str != strconv.FormatUint(receipts-1, 10) {
		t.Fatalf("doctor on a sealed store: %s", wire.Encode(item))
	}
	if a := atm(t, root, nil, "receipt", "audit"); a.code != 0 || a.res.Outcome != wire.OutcomeOK {
		t.Fatalf("audit of a sealed store: %s", a.stdout)
	}
	un := atm(t, root, nil, "receipt", "unretire", "--archive", path)
	if un.code != 0 || un.res.Outcome != wire.OutcomeOK {
		t.Fatalf("unretire: %s", un.stdout)
	}
	_, item = doctorOK(t, root)
	if j := field(item, "journal"); field(j, "receipts").Str != strconv.FormatUint(receipts, 10) || field(j, "retiredThrough").Str != "0" {
		t.Fatalf("doctor after unretire: %s", wire.Encode(item))
	}
}
