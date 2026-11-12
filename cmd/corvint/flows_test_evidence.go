// SPDX-License-Identifier: Apache-2.0
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/appflows"
	"github.com/Beamfall/corvint/internal/gokernel"
	"github.com/Beamfall/corvint/internal/testevidencedoc"
)

const flowsTestEvidenceHelp = `
Read retained test evidence linked to application flows (RSP-V0-051).
Usage:
  corvint [--root PATH] flows test-evidence --flows DIR PATH [--lines START:END] [--revision COMMIT] [--index FILE] [--format json|text]
  corvint [--root PATH] flows test-evidence --flows DIR --test TEST [--revision COMMIT] [--index FILE] [--format json|text]
  corvint [--root PATH] flows test-evidence --flows DIR --explain TEST --base COMMIT [--revision COMMIT] [--index FILE] [--format json|text]
Requires --flows and one query. Links are evaluated at HEAD; a different resolved
revision refuses the join. UNKNOWN evidence never upgrades flow authority.
Output: application-flow-test-evidence/1, containing the existing flow lookup and
test evidence query. Writes nothing. Example:
  corvint flows test-evidence --flows flows src/main.go
`

// runFlowsTestEvidence is `corvint flows test-evidence` (RSP-V0-051), the one flows verb that
// stays in this binary (CMS-V0-914): it joins the `corvint test-evidence` projection, which the
// tests layer owns, to the flow lookup the shared flow reader (internal/appflows) answers.
// Refusals exit 2 with the invalid-arguments envelope, as they did under the flows dispatcher.
func runFlowsTestEvidence(ctx context.Context, root string, args []string, out, diagnostic io.Writer) int {
	root, err := filepath.Abs(root)
	if err == nil {
		err = flowsTestEvidence(ctx, root, args, out)
	}
	if err == nil {
		return 0
	}
	var coded *gokernel.Error
	if !errors.As(err, &coded) {
		err = argumentError(err.Error())
	}
	emitError(diagnostic, err)
	return 2
}

// The additive flow projection preserves the existing lookup document and query
// document, without upgrading a source/test link into execution authority.
func flowsTestEvidence(ctx context.Context, root string, args []string, out io.Writer) error {
	dir := ""
	rest := []string{}
	for i := 0; i < len(args); i++ {
		if args[i] == "--flows" {
			if dir != "" || i+1 == len(args) {
				return errors.New("flows test-evidence requires --flows DIR")
			}
			i++
			dir = args[i]
		} else if strings.HasPrefix(args[i], "--flows=") {
			if dir != "" {
				return errors.New("repeated --flows")
			}
			dir = strings.TrimPrefix(args[i], "--flows=")
		} else {
			rest = append(rest, args[i])
		}
	}
	if dir == "" {
		return errors.New("flows test-evidence requires --flows DIR")
	}
	// FSV-V1-005: a flows declaration the shared reader refuses ends the
	// report with the reader's reason before anything is written.
	if refusal := appflows.ReportRefusal(root); refusal != nil {
		return &gokernel.Error{Code: refusal.Code(), Message: refusal.Reason + ": " + refusal.Error()}
	}
	q, format, err := testevidencedoc.Parse(rest)
	if err != nil {
		return err
	}
	d := testevidencedoc.Read(ctx, root, q)
	// FlowLookup evaluates HEAD links; refuse a historical join instead of
	// silently joining different revisions. The evidence query itself supports it.
	set, err := appflows.LoadIntentsAt(ctx, root, dir, "HEAD")
	if err != nil {
		return err
	}
	test := q.Test
	if test == "" {
		test = q.Explain
	}
	lookup, err := appflows.FlowLookup(ctx, root, set, q.Path, test)
	if err != nil {
		return err
	}
	var linked appflows.LookupReport
	if err = json.Unmarshal(lookup, &linked); err != nil {
		return err
	}
	if linked.Revision != d.Query.Revision {
		return errors.New("flow evidence revision mismatch")
	}
	if format == "text" {
		if err = testevidencedoc.Write(out, d, format); err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(lookup))
		return err
	}
	return json.NewEncoder(out).Encode(struct {
		Schema   string                `json:"schema"`
		Flows    appflows.LookupReport `json:"flows"`
		Evidence any                   `json:"evidence"`
	}{"application-flow-test-evidence/1", linked, d})
}
