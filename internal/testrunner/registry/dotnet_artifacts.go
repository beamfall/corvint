// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/Beamfall/corvint/internal/testrunner"
)

// DotnetVSTestArtifacts resolves coverage attachments from a core-captured TRX
// (RSP-V0-005). VSTest places Coverlet exports in generated subdirectories, not
// beside results.trx. After capturing the command's results.trx, core calls this
// function and captures these run-directory-relative paths as artifact handles.
// Core must reject symlink escapes when opening them. This resolver performs no IO.
func DotnetVSTestArtifacts(report []byte) ([]string, error) {
	if len(report) == 0 || len(report) > testrunner.MaxReportBytes {
		return nil, fmt.Errorf("dotnet-evidence-artifacts-missing-or-capped")
	}
	d := xml.NewDecoder(bytes.NewReader(report))
	var stack []string
	seen := map[string]bool{}
	nodes, roots := 0, 0
	for {
		token, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("dotnet-coverage-xml-invalid:%w", err)
		}
		switch t := token.(type) {
		case xml.StartElement:
			nodes++
			if nodes > 100000 || len(stack) >= 32 || t.Name.Space != "" && t.Name.Space != "http://microsoft.com/schemas/VisualStudio/TeamTest/2010" {
				return nil, fmt.Errorf("dotnet-coverage-xml-bounds")
			}
			if len(stack) == 0 {
				roots++
				if roots != 1 || t.Name.Local != "TestRun" {
					return nil, fmt.Errorf("dotnet-evidence-artifact-invalid")
				}
			}
			stack = append(stack, t.Name.Local)
			if strings.Join(stack, "/") != "TestRun/ResultSummary/CollectorDataEntries/Collector/UriAttachments/UriAttachment/A" {
				continue
			}
			href := ""
			for _, a := range t.Attr {
				if a.Name.Local == "href" && a.Name.Space == "" {
					if href != "" {
						return nil, fmt.Errorf("dotnet-evidence-artifact-invalid")
					}
					href = a.Value
				}
			}
			// TRX uses Windows separators even when consumed on another OS.
			href = strings.ReplaceAll(href, "\\", "/")
			if !dotnetPath(href) || strings.ContainsAny(href, "%?#") || path.Base(href) != "coverage.cobertura.xml" || seen[href] {
				return nil, fmt.Errorf("dotnet-evidence-artifact-invalid")
			}
			seen[href] = true
			if len(seen) > testrunner.MaxReports {
				return nil, fmt.Errorf("dotnet-evidence-artifacts-missing-or-capped")
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		}
	}
	if roots != 1 || len(stack) != 0 || len(seen) != 1 {
		return nil, fmt.Errorf("dotnet-evidence-artifacts-missing-or-capped")
	}
	paths := make([]string, 0, len(seen))
	for name := range seen {
		paths = append(paths, name)
	}
	sort.Strings(paths)
	return paths, nil
}
