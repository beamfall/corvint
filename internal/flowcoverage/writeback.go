package flowcoverage

import (
	"fmt"
	"github.com/Beamfall/corvint/internal/flowdocs"
	"sort"
	"strings"
)

func escape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "|", "&#124;", "`", "&#96;", "[", "&#91;", "]", "&#93;", "\\", "&#92;", "\n", " ", "\r", " ")
	return r.Replace(s)
}
func (r *Result) section(id string) []byte {
	var b strings.Builder
	b.WriteString("\n## E2E coverage\n\nByte-bound observations with caller-declared application identity; no attested deployment lineage. Declared negative controls observed failing do not independently prove control adequacy.\n\n| Variation | Status | Exact tests (file › title › project) | Last verified revision |\n| --- | --- | --- | --- |\n")
	for _, row := range r.Report.Rows {
		if row.DocumentedID != id {
			continue
		}
		tests := []string{}
		for _, t := range row.Tests {
			project := "(missing)"
			if t.Project != nil {
				project = *t.Project
			}
			tests = append(tests, t.File+" › "+t.FullTitle+" › "+project)
		}
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", escape(row.FlowID+":"+row.VariationID), row.Status, escape(strings.Join(tests, "; ")), escape(row.LastVerifiedRevision))
		for _, why := range row.Reasons {
			fmt.Fprintf(&b, "\nGap: %s.\n", escape(why))
		}
	}
	return []byte(b.String())
}
func (r *Result) Files() (map[string][]byte, error) {
	files := map[string][]byte{}
	if r.Generation != nil {
		for p, b := range r.Generation.Files {
			if p == "generation.json" {
				p = "source-generation.json"
			}
			files[p] = append([]byte{}, b...)
		}
		for _, f := range r.Generation.Manifest.Flows {
			found := false
			for p, b := range files {
				if strings.HasPrefix(p, strings.TrimPrefix(f.ID, "flowdocs:flow:")+"/") && strings.HasSuffix(p, "functional-overview.md") {
					files[p] = append(b, r.section(f.ID)...)
					found = true
				}
			}
			if !found {
				return nil, fmt.Errorf("generated flow page missing")
			}
		}
	} else {
		for _, id := range r.Documented {
			files[digest([]byte(id))+"/coverage.md"] = r.section(id)
		}
	}
	report, e := Encode(r.Report)
	if e != nil {
		return nil, e
	}
	files["coverage.json"] = report
	outputs := []flowdocs.Output{}
	names := []string{}
	for p := range files {
		names = append(names, p)
	}
	sort.Strings(names)
	total := 0
	for _, p := range names {
		total += len(files[p])
		outputs = append(outputs, flowdocs.Output{Path: p, SHA256: digest(files[p])})
	}
	manifest := struct {
		Schema      string            `json:"schema"`
		Denominator string            `json:"denominator_sha256"`
		Runs        string            `json:"runs_sha256"`
		Report      string            `json:"report_sha256"`
		Outputs     []flowdocs.Output `json:"outputs"`
	}{WritebackSchema, r.Report.DenominatorSHA256, r.Report.RunsSHA256, digest(report), outputs}
	b, e := Encode(manifest)
	if e != nil {
		return nil, e
	}
	files["coverage-manifest.json"] = b
	if total+len(b) > flowdocs.MaxOutputBytes || len(files) > flowdocs.MaxFlows*8+4 {
		return nil, fmt.Errorf("writeback bound exceeded")
	}
	return files, nil
}
func (r *Result) Write(destination string) error {
	files, e := r.Files()
	if e != nil {
		return e
	}
	return flowdocs.MaterializeCoverage(destination, files)
}
func (r *Result) Check(destination string) ([]string, error) {
	files, e := r.Files()
	if e != nil {
		return nil, e
	}
	return flowdocs.CompareOutput(destination, files)
}
