package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/Beamfall/corvint/internal/worksource"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"testing"

	"github.com/Beamfall/corvint/internal/workqueue"
)

// WQO-V0-008..012: the producer retains complete summaries and emits exactly
// the requested detail set, including the policy's valid zero boundary.
func TestDocumentsRequestedDetails(t *testing.T) {
	items := []workItem{
		{ID: "z", Title: "Last alphabetically", Body: "body z", TouchPaths: []string{"z.go", "a.go"}},
		{ID: "a", Title: "First alphabetically", Body: "body a", TouchPaths: []string{"b.go"}},
		{ID: "m", Title: "Middle", Body: "body m", TouchPaths: []string{"c.go"}},
	}
	for _, count := range []int{0, 3} {
		t.Run("tickets="+strconv.Itoa(count), func(t *testing.T) {
			root, policy := queueFixture(t, items[:count])
			full, fullDetails, _, err := documents(root, policy)
			if err != nil {
				t.Fatal(err)
			}
			for _, limit := range []int{0, 1, 2, 3, 4, 512} {
				t.Run("limit="+strconv.Itoa(limit), func(t *testing.T) {
					policy.DetailLimit = strconv.Itoa(limit)
					policy.RefreshIdentity()
					parsedPolicy, err := workqueue.ParsePolicy(policy.Canonical())
					if err != nil {
						t.Fatal(err)
					}
					snapshot, details, checkpoint, err := documents(root, parsedPolicy)
					if err != nil {
						t.Fatal(err)
					}
					validateDocuments(t, snapshot, details, workqueue.StateValidated)
					parsedCheckpoint, err := workqueue.ParseCheckpoint(checkpoint.Canonical())
					if err != nil {
						t.Fatal(err)
					}
					if err := workqueue.ValidateCheckpoint(parsedCheckpoint, parsedPolicy, snapshot, snapshot.RepositorySource); err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(snapshot.Tickets, full.Tickets) || snapshot.Scope != full.Scope || snapshot.RepositorySource != full.RepositorySource {
						t.Fatal("detail limit changed summaries, scope, or source")
					}
					wantVersions := []string{}
					for index, ticket := range snapshot.Tickets {
						if ticket.TicketID != "ticket:corvint:worklist:"+items[index].ID || ticket.Rank != workqueue.Rank(index) {
							t.Fatal("ticket ordering changed")
						}
						if index < limit {
							wantVersions = append(wantVersions, ticket.TicketVersionID)
						}
					}
					sort.Strings(wantVersions)
					if !reflect.DeepEqual(append([]string{}, snapshot.DetailRequestTicketVersionIDs...), wantVersions) {
						t.Fatal("wrong requested versions")
					}
					expected := &workqueue.DetailsDocument{Details: []workqueue.Detail{}, SnapshotID: snapshot.ID}
					for _, detail := range fullDetails.Details {
						if containsVersion(wantVersions, detail.TicketVersionID) {
							expected.Details = append(expected.Details, detail)
						}
					}
					workqueue.RefreshDetails(expected)
					if !bytes.Equal(details.Canonical(), expected.Canonical()) {
						t.Fatal("detail membership, identities, payloads, or canonical ordering changed")
					}
					again, againDetails, againCheckpoint, err := documents(root, parsedPolicy)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(snapshot.Canonical(), again.Canonical()) || !bytes.Equal(details.Canonical(), againDetails.Canonical()) || !bytes.Equal(checkpoint.Canonical(), againCheckpoint.Canonical()) {
						t.Fatal("producer is not deterministic")
					}
				})
			}
		})
	}
}

func containsVersion(versions []string, version string) bool {
	index := sort.SearchStrings(versions, version)
	return index < len(versions) && versions[index] == version
}

// The negative controls exercise native parsers and the summary-bound payload
// digest, preserving existing failure semantics rather than weakening validators.
func TestDocumentsDetailFailures(t *testing.T) {
	root, policy := queueFixture(t, []workItem{{ID: "one", Title: "One", Body: "original", TouchPaths: []string{"one.go"}}, {ID: "two", Title: "Two", Body: "second", TouchPaths: []string{"two.go"}}})
	_, all, _, err := documents(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	policy.DetailLimit = "1"
	policy.RefreshIdentity()
	snapshot, details, _, err := documents(root, policy)
	if err != nil {
		t.Fatal(err)
	}
	validateDocuments(t, snapshot, details, workqueue.StateValidated)
	t.Run("missing", func(t *testing.T) {
		missing := *details
		missing.Details = []workqueue.Detail{}
		workqueue.RefreshDetails(&missing)
		result := validateDocuments(t, snapshot, &missing, workqueue.StatePartial)
		if !reflect.DeepEqual(result.Unknowns, []string{workqueue.UnknownDetailMissing}) {
			t.Fatalf("missing detail unknowns: %v", result.Unknowns)
		}
	})
	t.Run("tampered", func(t *testing.T) {
		changed := *details
		changed.Details = append([]workqueue.Detail{}, details.Details...)
		body := "tampered"
		changed.Details[0].Payload.Body = &body
		workqueue.RefreshDetail(&changed.Details[0])
		workqueue.RefreshDetails(&changed)
		validateDocuments(t, snapshot, &changed, workqueue.StateConflicted)
	})
	t.Run("surplus", func(t *testing.T) {
		extra := *all
		extra.SnapshotID = snapshot.ID
		workqueue.RefreshDetails(&extra)
		validateDocuments(t, snapshot, &extra, workqueue.StateConflicted)
	})
}

// documentsFromSource must not silently swallow a malformed DetailLimit: an
// invalid policy value should fail the run, not fall back to limit=0 and
// quietly emit zero details (matching the sibling ParseCount error handling
// in internal/workqueue/detail_requests.go).
func TestDocumentsRejectsInvalidDetailLimit(t *testing.T) {
	root, policy := queueFixture(t, []workItem{{ID: "one", Title: "One", Body: "body", TouchPaths: []string{"one.go"}}})
	policy.DetailLimit = "not-a-number"
	if _, _, _, err := documents(root, policy); err == nil {
		t.Fatal("documents: want error for invalid detailLimit, got nil")
	}
}

// The self-dogfood worklist is exactly one JSON document; trailing data is
// malformed framing (WQO-V0-001), not a complete snapshot of the first value.
func TestDocumentsRejectTrailingWorklistData(t *testing.T) {
	for name, trailer := range map[string]string{"second-document": `{"profile":"corvint-worklist/0","tickets":[]}`, "garbage": "x"} {
		t.Run(name, func(t *testing.T) {
			root, policy := queueFixture(t, []workItem{{ID: "one", Title: "One", Body: "body", TouchPaths: []string{"one.go"}}})
			path := filepath.Join(root, "docs/worklist.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(raw, trailer...), 0600); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"add", "docs/worklist.json"}, {"-c", "user.name=Queue test", "-c", "user.email=queue@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "Trailing data"}} {
				if _, err := git(root, args...); err != nil {
					t.Fatal(err)
				}
			}
			if _, _, _, err := documents(root, policy); err == nil || err.Error() != "invalid worklist" {
				t.Fatalf("documents error = %v, want invalid worklist", err)
			}
		})
	}
}

func validateDocuments(t *testing.T, snapshot *workqueue.Snapshot, details *workqueue.DetailsDocument, state string) workqueue.ValidationResult {
	t.Helper()
	parsedSnapshot, err := workqueue.ParseSnapshot(snapshot.Canonical())
	if err != nil {
		t.Fatal(err)
	}
	if result := workqueue.ValidateSnapshot(parsedSnapshot); result.State != workqueue.StateValidated {
		t.Fatalf("snapshot: %+v", result)
	}
	parsedDetails, err := workqueue.ParseDetails(details.Canonical())
	if err != nil {
		t.Fatal(err)
	}
	result := workqueue.ValidateDetailCoverage(parsedSnapshot, parsedDetails)
	if result.State != state {
		t.Fatalf("coverage: %+v; want %s", result, state)
	}
	return result
}

func queueFixture(t *testing.T, items []workItem) (string, *workqueue.Policy) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "docs"), 0700); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(worklist{Profile: "corvint-worklist/0", Tickets: items})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/worklist.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "docs/worklist.json"}, {"-c", "user.name=Queue test", "-c", "user.email=queue@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "Frozen queue"}} {
		if _, err := git(root, args...); err != nil {
			t.Fatal(err)
		}
	}
	raw, err = os.ReadFile("../../.corvint/work-queue-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := workqueue.ParsePolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	return root, policy
}

func git(root string, arguments ...string) ([]byte, error) {
	// Fixtures mutate only their t.TempDir repository.
	executable, err := qualifiedGitExecutable()
	if err != nil {
		return nil, err
	}
	source := &worksource.Source{Root: root, GitPath: executable, GitEnvironment: []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_SYSTEM=" + os.DevNull}}
	return source.Git(context.Background(), 4<<20, arguments...)
}

// qualifiedGitExecutable mirrors the production fixed-path selection in
// internal/worksource (unexported there): resolve /usr/bin/git via
// worksource.PlatformPath, then on Darwin bypass Apple's shim through the
// root-owned xcode-select link so fixture mutation does not depend on the
// shim's confstr()-dependent stderr output. It never weakens Source.Git's
// stderr rejection; it only selects which real Git binary is qualified.
func qualifiedGitExecutable() (string, error) {
	fixed, err := worksource.PlatformPath()
	if err != nil {
		return "", err
	}
	var executable string
	for _, directory := range filepath.SplitList(fixed) {
		candidate := filepath.Join(directory, "git")
		info, statErr := os.Stat(candidate)
		if statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
			executable, err = filepath.EvalSymlinks(candidate)
			break
		}
	}
	if executable == "" || err != nil {
		return "", errors.New("fixed-path Git unavailable")
	}
	if runtime.GOOS == "darwin" && executable == "/usr/bin/git" {
		if developer, readErr := os.Readlink("/var/db/xcode_select_link"); readErr == nil && filepath.IsAbs(developer) {
			if resolved, evalErr := filepath.EvalSymlinks(filepath.Join(developer, "usr", "bin", "git")); evalErr == nil {
				if info, statErr := os.Stat(resolved); statErr == nil && info.Mode().IsRegular() && info.Mode().Perm()&0111 != 0 {
					executable = resolved
				}
			}
		}
	}
	return executable, nil
}
