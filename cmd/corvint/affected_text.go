package main

import (
	"fmt"
	"strings"
)

// renderAffectedText is the `--format text` form of an affected-plan/1
// receipt (AFP-V0-928): the summary line, one `selected` line per unit with
// the reason it was selected, one `unknown` line per plan unknown, and the
// advice checks with their runnable command. It restates the receipt and
// never changes the selection; one line is one decision (textWord).
func renderAffectedText(receipt affectedCompactReceipt) string {
	var out strings.Builder
	fmt.Fprintf(&out, "affected %s revision=%s", receipt.Summary.Line, receipt.Revision)
	if receipt.Range.Base != "" {
		fmt.Fprintf(&out, " base=%s", receipt.Range.Base)
	}
	out.WriteString("\n")
	if receipt.Tiers != nil {
		fmt.Fprintf(&out, "tiers %s\n", receipt.Tiers.Summary)
	}
	for _, selection := range receipt.Plan.Selected {
		fmt.Fprintf(&out, "selected %s tests=%d %s", textWord(selection.UnitID), selection.TestCount, textWord(selection.Witness.Kind))
		if selection.Witness.DirtyPath != "" {
			fmt.Fprintf(&out, " <- %s", textWord(selection.Witness.DirtyPath))
		}
		if via := affectedTextVia(receipt.Plan.ViaLinks, selection); len(via) > 1 {
			fmt.Fprintf(&out, " via %s", textWord(strings.Join(via, " > ")))
		}
		if selection.Witness.Glob != "" {
			fmt.Fprintf(&out, " glob %s", textWord(selection.Witness.Glob))
		}
		out.WriteString("\n")
	}
	for _, exclusion := range receipt.Plan.GoAdditiveExcluded {
		fmt.Fprintf(&out, "narrowed %s through %s\n", textWord(exclusion.UnitID), textWord(exclusion.Through))
	}
	for _, unknown := range receipt.Plan.Unknown {
		fmt.Fprintf(&out, "unknown %s %s", textWord(unknown.Reason), textWord(unknown.Detail))
		if unknown.Hint != "" {
			fmt.Fprintf(&out, " hint %s", textWord(unknown.Hint))
		}
		out.WriteString("\n")
	}
	fmt.Fprintf(&out, "excluded %d %s\n", receipt.Plan.Excluded.Count, receipt.Plan.Excluded.Digest)
	for _, check := range receipt.Advice.Checks {
		command := check.Command
		if check.Arguments == affectedAdvisoryArguments && len(receipt.Provider.Go.Packages) != 0 {
			command += " " + shellQuoteJoin(receipt.Provider.Go.Packages)
		}
		fmt.Fprintf(&out, "check %s %s\n", check.Kind, textWord(command))
	}
	for _, unknown := range receipt.Advice.Unknown {
		fmt.Fprintf(&out, "advice-unknown %s\n", textWord(unknown))
	}
	return out.String()
}

// affectedTextVia is the selection's witness chain, from the dirty unit to
// the selected unit, read through plan.viaLinks when the witness holds a
// reference (AFP-V0-640).
func affectedTextVia(links map[string]string, selection affectedCompactSelection) []string {
	if len(selection.Witness.Via) != 0 {
		return selection.Witness.Via
	}
	chain := []string{selection.UnitID}
	for unit := selection.UnitID; len(chain) < selection.Witness.ViaLength; {
		previous, ok := links[unit]
		if !ok {
			break
		}
		chain = append(chain, previous)
		unit = previous
	}
	for left, right := 0, len(chain)-1; left < right; left, right = left+1, right-1 {
		chain[left], chain[right] = chain[right], chain[left]
	}
	return chain
}

// textWord prints value bare, or Go-quoted when it is empty or holds a
// control character, so a value never spans or ends a line.
func textWord(value string) string {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return fmt.Sprintf("%q", value)
		}
	}
	if value == "" {
		return `""`
	}
	return value
}
