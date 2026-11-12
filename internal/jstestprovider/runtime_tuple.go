package jstestprovider

import (
	"slices"

	"github.com/Beamfall/corvint/internal/jstestreceipt"
)

// RuntimeTupleNotApplicable is jstestreceipt.RuntimeTupleNotApplicable, kept under its old name.
const RuntimeTupleNotApplicable = jstestreceipt.RuntimeTupleNotApplicable

// RuntimeTupleCandidate is jstestreceipt.RuntimeTupleCandidate, kept under its old name.
const RuntimeTupleCandidate = jstestreceipt.RuntimeTupleCandidate

// RuntimeTupleUnqualified is jstestreceipt.RuntimeTupleUnqualified, kept under its old name.
const RuntimeTupleUnqualified = jstestreceipt.RuntimeTupleUnqualified

// RuntimeTupleUnobserved is jstestreceipt.RuntimeTupleUnobserved, kept under its old name.
const RuntimeTupleUnobserved = jstestreceipt.RuntimeTupleUnobserved

// BrowserSourceBundled is jstestreceipt.BrowserSourceBundled, kept under its old name.
const BrowserSourceBundled = jstestreceipt.BrowserSourceBundled

// BrowserSourceSystem is jstestreceipt.BrowserSourceSystem, kept under its old name.
const BrowserSourceSystem = jstestreceipt.BrowserSourceSystem

// Tuple fields a mismatch names, checked in this order (PWP-V0-040).
const (
	TupleFieldRunner        = "runner"
	TupleFieldNode          = "node"
	TupleFieldBrowserSource = "browser-source"
)

// freshnessReporterTuples lists the table's bundled freshness (runner, Node)
// rows in table order. The provider passes them to the reporter, so its
// freshness pre-filter reads this table instead of a copied literal
// (PWP-V0-052).
func freshnessReporterTuples() [][2]string {
	rows := [][2]string{}
	for _, row := range jstestreceipt.AdmittedRuntimeTuples {
		if row.Profile == FreshnessProfile && row.Source == BrowserSourceBundled {
			rows = append(rows, [2]string{row.Runner, row.Node})
		}
	}
	return rows
}

// RuntimeTupleMismatch names the first tuple field that leaves a request or
// receipt outside the shared table, and the exact values the table admits
// for that field given the fields before it (PWP-V0-040).
type RuntimeTupleMismatch struct {
	Profile  string   `json:"profile"`
	Field    string   `json:"field"`
	Admitted []string `json:"admitted"`
}

// AdmittedRuntimeTupleMismatch checks runner, then Node, then browser source
// against the shared table for profile and returns nil when the tuple is a
// row. An empty Node or source is not checked: before a run the browser
// source is unobserved, and a caller may check the runner alone.
func AdmittedRuntimeTupleMismatch(profile, runner, node, source string) *RuntimeTupleMismatch {
	rows := jstestreceipt.AdmittedRuntimeTuples
	for _, step := range []struct {
		field, value string
		get          func(jstestreceipt.AdmittedRuntimeTuple) string
	}{
		{TupleFieldRunner, runner, func(r jstestreceipt.AdmittedRuntimeTuple) string { return r.Runner }},
		{TupleFieldNode, node, func(r jstestreceipt.AdmittedRuntimeTuple) string { return r.Node }},
		{TupleFieldBrowserSource, source, func(r jstestreceipt.AdmittedRuntimeTuple) string { return r.Source }},
	} {
		if step.value == "" && step.field != TupleFieldRunner {
			continue
		}
		var admitted []string
		var kept []jstestreceipt.AdmittedRuntimeTuple
		for _, row := range rows {
			if row.Profile != profile {
				continue
			}
			if !slices.Contains(admitted, step.get(row)) {
				admitted = append(admitted, step.get(row))
			}
			if step.get(row) == step.value {
				kept = append(kept, row)
			}
		}
		if len(kept) == 0 {
			slices.Sort(admitted)
			if admitted == nil {
				admitted = []string{}
			}
			return &RuntimeTupleMismatch{Profile: profile, Field: step.field, Admitted: admitted}
		}
		rows = kept
	}
	return nil
}

// ReceiptRuntimeTuple is jstestreceipt.ReceiptRuntimeTuple, kept under its old name.
func ReceiptRuntimeTuple(r Receipt) string { return jstestreceipt.ReceiptRuntimeTuple(r) }
