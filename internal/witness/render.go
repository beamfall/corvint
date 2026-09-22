package witness

import (
	"fmt"
	"strings"
)

// Render writes the report as deterministic text. Identical reports produce
// identical bytes: every section is emitted in the order the report already
// sorted, and no map is ranged over.
func Render(report *Report) string {
	builder := &strings.Builder{}
	renderHeader(builder, report)
	renderSummary(builder, report)
	renderAuthorities(builder, report)
	renderSources(builder, report)
	renderPreconditions(builder, report)
	renderObligations(builder, report)
	return builder.String()
}

func renderHeader(builder *strings.Builder, report *Report) {
	fmt.Fprintf(builder, "WITNESS %s\n", report.Profile)
	fmt.Fprintf(builder, "range base=%s head=%s\n", report.Range.Base, report.Range.Head)
	fmt.Fprintf(builder, "trees baseTree=%s headTree=%s\n", report.Range.BaseTree, report.Range.HeadTree)
	fmt.Fprintf(builder, "universe changed-paths=%d admission=%s closure=%s\n",
		report.Range.ChangedPaths, report.Range.Admission, report.Range.Closure)
	if report.Range.AdmissionDetail != "" {
		fmt.Fprintf(builder, "  admission-detail %s\n", report.Range.AdmissionDetail)
	}
	if report.Range.ClosureDetail != "" {
		fmt.Fprintf(builder, "  closure-detail   %s\n", report.Range.ClosureDetail)
	}
}

func renderSummary(builder *strings.Builder, report *Report) {
	summary := report.Summary
	builder.WriteString("\nSUMMARY\n")
	fmt.Fprintf(builder, "  opened                 %d\n", summary.Opened)
	fmt.Fprintf(builder, "  closed-by-other-party  %d\n", summary.Closed)
	fmt.Fprintf(builder, "  unproven               %d\n", summary.Unproven)
	fmt.Fprintf(builder, "  not-run                %d\n", summary.NotRun)
	fmt.Fprintf(builder, "  witnesses-examined     %d (closing %d)\n", summary.WitnessesExamined, summary.WitnessesClosing)
	fmt.Fprintf(builder, "  determinable           %d/%d (%s)\n",
		summary.Analysed, summary.Opened, percent(summary.DeterminablePerMille))
	fmt.Fprintf(builder, "  proven                 %d/%d (%s)\n",
		summary.Closed, summary.Analysed, percent(summary.ProvenPerMille))
}

// percent renders a per-mille integer without floating point so the bytes are
// reproducible on every platform.
func percent(perMille int) string {
	return fmt.Sprintf("%d.%d%%", perMille/10, perMille%10)
}

func renderAuthorities(builder *strings.Builder, report *Report) {
	builder.WriteString("\nAUTHORITY TABLE\n")
	for _, item := range report.Authorities {
		fmt.Fprintf(builder, "  %-18s closing=%-5t %s\n", item.Class, item.Closing, item.Citation)
	}
}

func renderSources(builder *strings.Builder, report *Report) {
	builder.WriteString("\nWITNESS SOURCES\n")
	for _, item := range report.Sources {
		fmt.Fprintf(builder, "  %-5s status=%-8s path=%s hunks=%d basis=%d unattached=%d\n",
			item.Name, item.Status, item.Path, item.Hunks, item.Basis, item.Unattached)
		if item.Detail != "" {
			fmt.Fprintf(builder, "        %s\n", item.Detail)
		}
	}
}

func renderPreconditions(builder *strings.Builder, report *Report) {
	builder.WriteString("\nPRECONDITIONS\n")
	if len(report.Preconditions) == 0 {
		builder.WriteString("  (none)\n")
		return
	}
	for _, item := range report.Preconditions {
		fmt.Fprintf(builder, "  %s\n        %s\n", item.ID, item.Statement)
	}
}

func renderObligations(builder *strings.Builder, report *Report) {
	builder.WriteString("\nOBLIGATIONS\n")
	if len(report.Obligations) == 0 {
		builder.WriteString("  (none: the range changes no path)\n")
		return
	}
	for _, item := range report.Obligations {
		fmt.Fprintf(builder, "  %-8s %-6s cov=%-7s %-14s %s",
			item.Verdict, item.Kind, item.Coverage, item.Relation, displayPath(item))
		if item.Reason != "" {
			fmt.Fprintf(builder, "  [%s]", item.Reason)
		}
		builder.WriteString("\n")
		for _, candidate := range item.Witnesses {
			fmt.Fprintf(builder, "        witness %s hunk=%s relation=%s authority=%s closing=%t evidence=%s\n",
				candidate.Source, shortIdentity(candidate.HunkID), candidate.Relation,
				candidate.Authority, candidate.Closing, candidate.EvidencePath)
		}
	}
}

// shortIdentity abbreviates a content-addressed identity so two citations that
// differ only by which hunk raised them stay distinguishable in the report.
func shortIdentity(value string) string {
	if position := strings.LastIndex(value, ":"); position >= 0 {
		value = value[position+1:]
	}
	if len(value) > 12 {
		return value[:12]
	}
	if value == "" {
		return "-"
	}
	return value
}

func displayPath(item Obligation) string {
	if item.Path == "" {
		return "(unnamed)"
	}
	return item.Path
}
