package postmergeconnector

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"time"
)

func ValidatePolicy(p Policy) error {
	if p.Profile != Profile || !idPattern.MatchString(p.HierarchyLevel) || p.MaxFindings < 0 || p.MaxFindings > 64 {
		return fmt.Errorf("policy-invalid")
	}
	seen := map[string]bool{}
	for _, c := range p.AllowedClasses {
		if !slices.Contains([]string{"defect", "scope-mismatch", "coverage-gap", "security"}, c) || seen[c] {
			return fmt.Errorf("policy-classes-invalid")
		}
		seen[c] = true
	}
	seen = map[string]bool{}
	for _, origin := range p.URLOrigins {
		u, e := url.Parse(origin)
		if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.String() != origin || seen[origin] {
			return fmt.Errorf("policy-origins-invalid")
		}
		seen[origin] = true
	}
	return nil
}
func Read(ctx context.Context, root string, f Fixture, p Policy, source string) (Intake, error) {
	if err := ValidatePolicy(p); err != nil {
		return Intake{}, err
	}
	if f.Profile != Profile || f.Forge.Change.Binding != p.Expected {
		return Intake{}, fmt.Errorf("fixture-binding-mismatch")
	}
	change := f.Forge.Change
	files, err := ChangedFiles(ctx, root, change.Binding)
	if err != nil {
		return Intake{}, err
	}
	actual := slices.Clone(change.Files)
	slices.SortFunc(actual, func(a, b ChangedFile) int {
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		return 0
	})
	if !slices.Equal(files, actual) {
		return Intake{}, fmt.Errorf("changed-files-mismatch")
	}
	change.Files = files
	created, e1 := time.Parse(time.RFC3339, change.CreatedAt)
	merged, e2 := time.Parse(time.RFC3339, change.MergedAt)
	if e1 != nil || e2 != nil || !idPattern.MatchString(change.Author) || created.After(merged) || created.Location() != time.UTC || merged.Location() != time.UTC {
		return Intake{}, fmt.Errorf("change-metadata-invalid")
	}
	items := map[string]Item{}
	if len(f.Tracker.Items) > 1024 {
		return Intake{}, fmt.Errorf("tracker-too-large")
	}
	for _, i := range f.Tracker.Items {
		if !idPattern.MatchString(i.ID) || !idPattern.MatchString(i.Level) || (i.Parent != "" && !idPattern.MatchString(i.Parent)) {
			return Intake{}, fmt.Errorf("item-invalid")
		}
		if _, ok := items[i.ID]; ok {
			return Intake{}, fmt.Errorf("duplicate-item")
		}
		items[i.ID] = i
	}
	if !idPattern.MatchString(source) {
		return Intake{}, fmt.Errorf("source-invalid")
	}
	chain := []Item{}
	seen := map[string]bool{}
	for id := source; len(chain) < 32; {
		item, ok := items[id]
		if !ok {
			return Intake{}, fmt.Errorf("hierarchy-item-missing")
		}
		if seen[id] {
			return Intake{}, fmt.Errorf("hierarchy-cycle")
		}
		seen[id] = true
		chain = append(chain, item)
		if item.Level == p.HierarchyLevel {
			return Intake{Profile, change, chain}, nil
		}
		if item.Parent == "" {
			return Intake{}, fmt.Errorf("hierarchy-level-unresolved")
		}
		id = item.Parent
	}
	return Intake{}, fmt.Errorf("hierarchy-depth-exceeded")
}
