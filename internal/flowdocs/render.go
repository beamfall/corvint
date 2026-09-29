package flowdocs

import (
	"fmt"
	"html"
	"sort"
	"strings"
)

func quote(s string) string {
	s = html.EscapeString(s)
	var b strings.Builder
	for _, r := range s {
		if r < 32 || r == 127 {
			fmt.Fprintf(&b, "&#%d;", r)
			continue
		}
		if strings.ContainsRune("\\`*_{}[]()#+-.!|", r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
func render(m Manifest) map[string][]byte {
	files := map[string][]byte{}
	for _, f := range m.Flows {
		for _, section := range sections {
			var b strings.Builder
			fmt.Fprintf(&b, "# %s — %s\n\n", quote(f.Name), section)
			fmt.Fprintf(&b, "Source revision: %s. Trust: generated. Freshness: pinned source; run docs flows check for a later revision.\n\n", m.Source.Revision)
			for _, p := range f.Paragraphs {
				if p.Section != section {
					continue
				}
				fmt.Fprintf(&b, "<!-- %s -->\n%s\n\nSource: %s:%d; symbol: %s; span SHA256: %s.\n\n", p.ID, quote(p.Text), quote(p.Anchor.Path), p.Anchor.Start, quote(p.Anchor.Symbol), p.Anchor.SpanSHA256)
			}
			switch section {
			case "flow-diagram":
				b.WriteString("```mermaid\nsequenceDiagram\n    participant S as Static source\n    participant U as Unresolved runtime\n    Note over S,U: Lexical inventory only - no witnessed order\n")
				fmt.Fprintf(&b, "    Note over S: Flow %s\n", strings.TrimPrefix(f.ID, "flowdocs:flow:"))
				for _, c := range f.Calls {
					fmt.Fprintf(&b, "    Note over S,U: Source contains call %s at line %d - order unresolved\n", mermaidName(c.Name), c.Anchor.Start)
				}
				b.WriteString("```\n")
			case "functional-overview":
				b.WriteString("| Variation inventory | Runtime coverage | Denominator rule |\n| --- | --- | --- |\n| Unknown unless supplied below | Unknown unless supplied below | Explicit accepted corpus membership only |\n")
			case "readme":
				b.WriteString("\nSecurity summaries: ")
				if m.Corpus == nil || len(m.Corpus.RestrictedSummaries) == 0 {
					b.WriteString("not collected; absence is not a clean security finding.\n")
				} else {
					for _, s := range m.Corpus.RestrictedSummaries {
						fmt.Fprintf(&b, "%s total %d; ", quote(s.Provider), s.Total)
						keys := []string{}
						for k := range s.BySeverity {
							keys = append(keys, k)
						}
						sort.Strings(keys)
						for _, k := range keys {
							fmt.Fprintf(&b, "%s=%d ", quote(k), s.BySeverity[k])
						}
					}
					b.WriteByte('\n')
				}
				for _, g := range m.Gaps {
					fmt.Fprintf(&b, "\n- %s\n", quote(g))
				}
			case "related-flows":
				for _, r := range m.Provider.Relations {
					if (r.Type == "related_to" || r.Type == "depends_on") && (r.From == f.ID || r.To == f.ID) {
						other := r.To
						if other == f.ID {
							other = r.From
						}
						fmt.Fprintf(&b, "\n- [%s](../%s/README.md): %s source candidate; runtime dependency unresolved.\n", other, strings.TrimPrefix(other, "flowdocs:flow:"), r.Type)
					}
				}
			}
			name := section + ".md"
			if section == "readme" {
				name = "README.md"
			}
			files[strings.TrimPrefix(f.ID, "flowdocs:flow:")+"/"+name] = []byte(b.String())
		}
	}
	return files
}
func mermaidName(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}
