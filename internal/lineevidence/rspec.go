// SPDX-License-Identifier: Apache-2.0
package lineevidence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/testid"
	"github.com/Beamfall/corvint/internal/testinventory"
)

// The stock RSpec JSON report loses the group chain. This refresh-only formatter
// reads the native metadata instead of guessing a split of full_description.
const rspecFormatter = `require 'json'
class CorvintLineIDs
  RSpec::Core::Formatters.register self, :example_started, :close
  def initialize(output)
    @output = output
    @rows = []
  end
  def example_started(notification)
    example = notification.example
    groups = example.example_group.parent_groups.reverse.map(&:description)
    @rows << {id: example.id, file: example.metadata[:rerun_file_path],
              segments: groups + [example.description]}
  end
  def close(_notification)
    @output.write(JSON.generate(@rows))
  end
end
`

func rspecIDs(ctx context.Context, dir, root, work string, command []string) (map[string]string, error) {
	script := filepath.Join(work, "line_ids.rb")
	report := filepath.Join(work, "line_ids.json")
	if err := os.WriteFile(script, []byte(rspecFormatter), 0600); err != nil {
		return nil, err
	}
	if _, err := testinventory.RunLister(ctx, dir, append(append([]string(nil), command...), "--dry-run", "--require", script, "--format", "CorvintLineIDs", "--out", report)); err != nil {
		return nil, err
	}
	info, err := os.Stat(report)
	if err != nil {
		return nil, err
	}
	if info.Size() > testinventory.MaxBytes {
		return nil, fmt.Errorf("line-evidence: RSpec identity report exceeds bound")
	}
	data, err := os.ReadFile(report)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID, File string
		Segments []string
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return nil, err
	}
	result := map[string]string{}
	seen := map[string]bool{}
	for _, row := range rows {
		id, err := testid.New(testid.RSpec, joined(root, strings.TrimPrefix(row.File, "./")), row.Segments...)
		if err != nil {
			return nil, err
		}
		if seen[id.String()] || result[row.ID] != "" {
			return nil, fmt.Errorf("line-evidence: ambiguous RSpec identity")
		}
		seen[id.String()] = true
		result[row.ID] = id.String()
	}
	return result, nil
}
