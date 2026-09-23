package main

import (
	"fmt"

	"github.com/Beamfall/corvint/internal/contextindex"
)

// classifyTrust assigns the row's falsifier and stamps its trust class
// (FPK-V0-032, decision 0346), derived from the authority label alone. A
// tainted row -- fetched from a provider or produced by a tool -- can be a
// basis for nothing, whatever the falsifier tables would assign it: it keeps
// falsifier `none`, so it can never be `PASS` or count as proven, and its
// `refusal` names the row so the reader sees which citation was set aside.
func classifyTrust(row *proveRow) {
	row.Falsifier = falsifierFor(*row)
	row.Trust = contextindex.TrustClass(row.Authority)
	if !contextindex.TrustTainted(row.Trust) {
		return
	}
	row.Falsifier = falsifierNone
	row.Refusal = fmt.Sprintf("%s row %s %s at %s:%d cannot satisfy a basis", row.Trust, row.Kind, row.Result, row.Path, row.Line)
}
