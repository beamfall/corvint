package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/tasks/intent"
	"github.com/Beamfall/corvint/internal/tasks/journal"
	"github.com/Beamfall/corvint/internal/tasks/wire"
)

// Journal size and growth (JRT-V0-012). The doctor stats the live receipts
// directory (names and sizes only, no receipt is read) and derives the growth
// rate from the receipts its bounded window scan already verified.
var (
	// doctorJournalReceipts and doctorJournalBytes are the JOURNAL_GROWTH
	// thresholds: past either, every complete audit and cold start walks a
	// history large enough to dominate its cost (#1055).
	doctorJournalReceipts uint64 = 16384
	doctorJournalBytes    uint64 = 256 << 20
)

// doctorJournalEntries bounds the directory listing.
const doctorJournalEntries = 1 << 22

type doctorJournal struct {
	receipts, bytes uint64
	retiredThrough  uint64
	// perDay is the receipts/day over the scanned window, or "" when fewer
	// than two scanned receipts span a positive interval.
	perDay, bytesPerDay string
	err                 string
}

func (j *doctorJournal) value() wire.Value {
	o := wire.NewObject()
	if j.err != "" {
		o.Set("receipts", wire.String("NOT_OBSERVED")).Set("bytes", wire.String("NOT_OBSERVED"))
	} else {
		o.Set("receipts", wire.String(strconv.FormatUint(j.receipts, 10))).Set("bytes", wire.String(strconv.FormatUint(j.bytes, 10)))
	}
	o.Set("retiredThrough", wire.String(strconv.FormatUint(j.retiredThrough, 10)))
	rate := func(s string) wire.Value {
		if s == "" {
			return wire.String("NOT_OBSERVED")
		}
		return wire.String(s)
	}
	o.Set("receiptsPerDay", rate(j.perDay)).Set("bytesPerDay", rate(j.bytesPerDay))
	o.Set("thresholdReceipts", wire.String(strconv.FormatUint(doctorJournalReceipts, 10)))
	o.Set("thresholdBytes", wire.String(strconv.FormatUint(doctorJournalBytes, 10)))
	return wire.ObjectValue(o)
}

// doctorJournalSize records the live journal's size on scan before the
// window scan, so that scan ends at a retired prefix's anchor.
func doctorJournalSize(repo *intent.Repository, scan *doctorScan) {
	j := &doctorJournal{}
	scan.journal = j
	if raw, err := intent.ReadFile(filepath.Join(repo.StateDir, journal.RetiredPrefixFile), journal.MaxRetiredPrefixBytes); err == nil {
		if seal, err := journal.DecodeRetiredPrefix(raw); err == nil {
			j.retiredThrough = seal.Retired()
		}
	}
	dir, err := os.Open(filepath.Join(repo.StateDir, "receipts"))
	if err == nil {
		var entries []os.DirEntry
		entries, err = dir.ReadDir(doctorJournalEntries)
		dir.Close()
		for _, e := range entries {
			name := e.Name()
			if digits, ok := strings.CutSuffix(name, ".json"); !ok || len(digits) != 12 || !e.Type().IsRegular() {
				continue
			}
			info, ierr := e.Info()
			if ierr != nil {
				continue
			}
			j.receipts++
			j.bytes += uint64(info.Size())
		}
	}
	if err != nil {
		j.err = err.Error()
		scan.warnings = append(scan.warnings, prose(fmt.Sprintf("doctor could not size the journal: %v", err)))
	}
}

// doctorJournalGrowth derives the growth rate from the window scan and
// returns a JOURNAL_GROWTH finding when either threshold is passed.
func doctorJournalGrowth(scan *doctorScan) []doctorFinding {
	j := scan.journal
	if j == nil || j.err != "" {
		return nil
	}
	if n := len(scan.events); n >= 2 && j.receipts > 0 {
		if span := scan.events[n-1].at.Sub(scan.events[0].at); span > 0 {
			days := span.Hours() / 24
			per := float64(n-1) / days
			j.perDay = strconv.FormatUint(uint64(per+0.5), 10)
			j.bytesPerDay = strconv.FormatUint(uint64(per*float64(j.bytes)/float64(j.receipts)+0.5), 10)
		}
	}
	if j.receipts <= doctorJournalReceipts && j.bytes <= doctorJournalBytes {
		return nil
	}
	growth := "an unobserved rate"
	if j.perDay != "" {
		growth = j.perDay + " receipts/day"
	}
	return []doctorFinding{{
		kind:   "JOURNAL_GROWTH",
		who:    "journal",
		detail: fmt.Sprintf("the live journal holds %d receipts in %d bytes (thresholds %d receipts, %d bytes) and grows at %s; every complete audit and cold start walks it", j.receipts, j.bytes, doctorJournalReceipts, doctorJournalBytes, growth),
		remedy: "receipt compact reclaims superseded liveness bodies only; to bound the journal, run `receipt retire --dry-run`, then `receipt retire --archive FILE` once no attempt, reservation or pool slot is live (JRT-V0), and keep the archive.",
	}}
}

// doctorRetiredScanStop reports whether a receipt the window scan could not
// read lies inside a retired prefix, which ends the scan without truncation.
func doctorRetiredScanStop(scan *doctorScan, seq uint64) bool {
	return scan.journal != nil && seq <= scan.journal.retiredThrough
}
