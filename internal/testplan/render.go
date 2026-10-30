package testplan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/gokernel"
)

const (
	// MaxOutputBytes bounds the printed plan in either format (TCN-V0-009).
	MaxOutputBytes = 8 << 20
	// TableHeaderPrefix starts the one header line of a pasted table (TCN-V0-010).
	TableHeaderPrefix = "<!-- corvint test-consolidation-table/0"
	tableColumns      = "| test | spec | variation ids | isolated reason |"
	tableRule         = "| --- | --- | --- | --- |"
)

func invalidArguments(format string, args ...any) error {
	return &gokernel.Error{Code: "test-plan-invalid-arguments", Message: fmt.Sprintf(format, args...)}
}

func mismatch(format string, args ...any) error {
	return &gokernel.Error{Code: "test-plan-mismatch", Message: fmt.Sprintf(format, args...)}
}

// render fixes the table bytes and their digest; both formats refuse past MaxOutputBytes.
func (p *Plan) render() error {
	var b bytes.Buffer
	fmt.Fprintf(&b, "%s input=%s tests=%s maps=%s revision=%s anchors=%s max-steps=%d status=%s -->\n",
		TableHeaderPrefix, p.InputDigest, p.TestsDigest, p.MapsDigest, p.EvaluatedRevision, p.AnchorValidation, p.MaxSteps, p.Status)
	b.WriteString(tableColumns + "\n" + tableRule + "\n")
	for _, t := range p.Tests {
		ids := make([]string, len(t.Steps))
		for i, s := range t.Steps {
			ids[i] = s.VariationID
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", t.Test, t.Spec, strings.Join(ids, ", "), t.Reason)
	}
	for _, r := range p.Reused {
		w := r.Witnesses[0]
		name := w.TestID
		if w.Project != "" {
			name += "@" + w.Project
		}
		fmt.Fprintf(&b, "| REUSE %s | %s | %s | reused strength=%s |\n", name, p.specs[r.VariationID], r.VariationID, w.Projection.Strength.State)
	}
	for _, d := range p.Duplicates {
		fmt.Fprintf(&b, "| DUPLICATE %s | %s | %s | duplicate |\n", d.DuplicateOf, p.specs[d.VariationID], d.VariationID)
	}
	for _, a := range p.Abstained {
		fmt.Fprintf(&b, "| ABSTAIN | %s | %s | %s |\n", p.specs[a.VariationID], a.VariationID, a.Reason)
	}
	if b.Len() > MaxOutputBytes {
		return boundExceeded("the plan table exceeds 8 MiB")
	}
	p.table = b.Bytes()
	p.TableDigest = sha(p.table)
	encoded, err := p.encode()
	if err != nil {
		return err
	}
	if len(encoded) > MaxOutputBytes {
		return boundExceeded("the plan document exceeds 8 MiB")
	}
	return nil
}

func (p *Plan) encode() ([]byte, error) {
	var b bytes.Buffer
	encoder := json.NewEncoder(&b)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(p); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Table is the TCN-V0-010 table, LF-terminated.
func (p *Plan) Table() []byte { return append([]byte(nil), p.table...) }

// JSON is the TCN-V0-009 document followed by LF.
func (p *Plan) JSON() []byte {
	encoded, _ := p.encode() // render already encoded it once
	return encoded
}

// Check compares a pasted plan file with the recomputed plan (TCN-V0-011). It returns nil only
// when the table bytes match and the plan is COMPLETE.
func Check(p *Plan, file []byte) error {
	if len(file) > MaxInputBytes {
		return invalidInput("plan file exceeds %d bytes", MaxInputBytes)
	}
	lines := strings.Split(string(file), "\n")
	header := -1
	for i, line := range lines {
		if strings.HasPrefix(line, TableHeaderPrefix) {
			if header >= 0 {
				return &gokernel.Error{Code: "test-plan-header-missing", Message: "the plan file holds more than one table header"}
			}
			header = i
		}
	}
	if header < 0 {
		return &gokernel.Error{Code: "test-plan-header-missing", Message: "the plan file holds no table header"}
	}
	got := []string{trimTrailing(lines[header])}
	for _, line := range lines[header+1:] {
		if !strings.HasPrefix(line, "|") {
			break
		}
		got = append(got, trimTrailing(line))
	}
	want := strings.Split(strings.TrimSuffix(string(p.table), "\n"), "\n")
	if steps, ok := headerMaxSteps(got[0]); !ok || steps != p.MaxSteps {
		return mismatch("the table header's max-steps does not equal --max-steps %d", p.MaxSteps)
	}
	if missing, extra := idDifference(got[1:], want[1:]); missing != "" || extra != "" {
		if missing != "" {
			return mismatch("variation %s is missing from the table", missing)
		}
		return mismatch("variation %s is extra in the table", extra)
	}
	for i := 0; i < len(got) || i < len(want); i++ {
		if i >= len(got) || i >= len(want) || got[i] != want[i] {
			return mismatch("table line %d (file line %d) differs from the recomputed plan", i+1, header+i+1)
		}
	}
	if p.Status != statusComplete {
		return &gokernel.Error{Code: "test-plan-incomplete", Message: fmt.Sprintf("the table matches but %d variations are abstained", p.Counts.Abstained)}
	}
	return nil
}

func trimTrailing(line string) string { return strings.TrimRight(line, " \t") }

func headerMaxSteps(header string) (int, bool) {
	for _, field := range strings.Fields(header) {
		if value, ok := strings.CutPrefix(field, "max-steps="); ok {
			n, err := strconv.Atoi(value)
			return n, err == nil
		}
	}
	return 0, false
}

// idDifference returns the smallest variation ID the recomputed rows hold and the pasted rows do
// not, and the smallest the pasted rows hold and the recomputed rows do not.
func idDifference(got, want []string) (string, string) {
	gotIDs, wantIDs := rowIDs(got), rowIDs(want)
	missing, extra := "", ""
	for id := range wantIDs {
		if !gotIDs[id] && (missing == "" || id < missing) {
			missing = id
		}
	}
	for id := range gotIDs {
		if !wantIDs[id] && (extra == "" || id < extra) {
			extra = id
		}
	}
	return missing, extra
}

// rowIDs reads the variation IDs column of each table row; the column rule and column names are
// not rows.
func rowIDs(rows []string) map[string]bool {
	ids := map[string]bool{}
	for _, row := range rows {
		if row == tableColumns || row == tableRule {
			continue
		}
		cells := strings.Split(row, "|")
		if len(cells) < 4 {
			continue
		}
		for _, id := range strings.Split(cells[3], ",") {
			if id = strings.TrimSpace(id); id != "" {
				ids[id] = true
			}
		}
	}
	return ids
}
