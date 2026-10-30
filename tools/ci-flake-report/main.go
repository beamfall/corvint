// SPDX-License-Identifier: Apache-2.0
// ci-flake-report aggregates the AFP-V0-441 FLAKY records of one window into one Markdown report
// (AFP-V0-530, V1-2682 option A). It reads retained GitHub artifact listings and downloaded
// flaky.json files (corvint-ci-flaky/0), never contacts the network or runs a test, and names
// every selected record it could not read, so a lost download is uncertainty, never a lower rate.
//
//	ci-flake-report [-now RFC3339] [-days N] select LISTING...
//	ci-flake-report [-now RFC3339] [-days N] report -artifacts DIR [-run-url URL] LISTING...
//
// LISTING files hold `GET /repos/{owner}/{repo}/actions/artifacts?name=...` pages, concatenated
// as `gh api --paginate` prints them. select prints one `id<TAB>name<TAB>created_at<TAB>run_id`
// line per unexpired `ci-flaky-<event>-<shard>` artifact created in the window [now-days, now),
// oldest first. report reads flaky.json out of DIR/<id>.zip (the artifact archive as the API
// serves it) for each of them and prints the Markdown.
package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	profile   = "corvint-ci-flaky/0"
	maxRecord = 1 << 20
	maxRows   = 200
	maxUnread = 50
	maxBody   = 60000
)

var namePattern = regexp.MustCompile(`^ci-flaky-(pull_request|merge_group)-[0-9]+$`)

// artifact is one entry of the artifacts listing.
type artifact struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	SizeInBytes int64     `json:"size_in_bytes"`
	CreatedAt   time.Time `json:"created_at"`
	Expired     bool      `json:"expired"`
	WorkflowRun struct {
		ID int64 `json:"id"`
	} `json:"workflow_run"`
}

// record is the corvint-ci-flaky/0 document flaky.sh writes (AFP-V0-441).
type record struct {
	Profile        string   `json:"profile"`
	Verdict        string   `json:"verdict"`
	Event          string   `json:"event"`
	RunID          int64    `json:"runId"`
	Shard          int      `json:"shard"`
	FailedPackages []string `json:"failedPackages"`
	FailedTests    []struct {
		Package string `json:"package"`
		Test    string `json:"test"`
	} `json:"failedTests"`
}

type window struct{ since, until time.Time }

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("ci-flake-report", flag.ContinueOnError)
	flags.SetOutput(stderr)
	now := flags.String("now", "", "the window's end, RFC3339 (default: the current time)")
	days := flags.Int("days", 7, "the window's length in days")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	until := time.Now().UTC()
	if *now != "" {
		t, err := time.Parse(time.RFC3339, *now)
		if err != nil {
			fmt.Fprintf(stderr, "ci-flake-report: -now: %v\n", err)
			return 2
		}
		until = t.UTC()
	}
	if *days < 1 || *days > 90 {
		fmt.Fprintln(stderr, "ci-flake-report: -days must be 1 to 90")
		return 2
	}
	w := window{since: until.Add(-time.Duration(*days) * 24 * time.Hour), until: until}
	rest := flags.Args()
	if len(rest) == 0 {
		fmt.Fprintln(stderr, "ci-flake-report: usage: select LISTING... | report -artifacts DIR [-run-url URL] LISTING...")
		return 2
	}
	switch rest[0] {
	case "select":
		if len(rest) < 2 {
			fmt.Fprintln(stderr, "ci-flake-report: select needs at least one LISTING")
			return 2
		}
		selected, err := selectArtifacts(rest[1:], w)
		if err != nil {
			fmt.Fprintf(stderr, "ci-flake-report: %v\n", err)
			return 1
		}
		for _, a := range selected {
			fmt.Fprintf(stdout, "%d\t%s\t%s\t%d\n", a.ID, a.Name, a.CreatedAt.UTC().Format(time.RFC3339), a.WorkflowRun.ID)
		}
		return 0
	case "report":
		sub := flag.NewFlagSet("report", flag.ContinueOnError)
		sub.SetOutput(stderr)
		dir := sub.String("artifacts", "", "the directory holding <id>.zip per selected artifact")
		runURL := sub.String("run-url", "", "the generating run's URL, named in the footer")
		if err := sub.Parse(rest[1:]); err != nil {
			return 2
		}
		if *dir == "" || len(sub.Args()) == 0 {
			fmt.Fprintln(stderr, "ci-flake-report: report needs -artifacts DIR and at least one LISTING")
			return 2
		}
		selected, err := selectArtifacts(sub.Args(), w)
		if err != nil {
			fmt.Fprintf(stderr, "ci-flake-report: %v\n", err)
			return 1
		}
		io.WriteString(stdout, report(selected, *dir, w, *runURL))
		return 0
	}
	fmt.Fprintf(stderr, "ci-flake-report: unknown command %q\n", rest[0])
	return 2
}

// selectArtifacts reads the listing pages and keeps each unexpired ci-flaky artifact created in
// the window once, oldest first. A listing that cannot be read or parsed is an error: the caller
// must not report a quiet week it never observed.
func selectArtifacts(paths []string, w window) ([]artifact, error) {
	seen := map[int64]bool{}
	var out []artifact
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		dec := json.NewDecoder(io.LimitReader(f, 64<<20))
		for {
			var page struct {
				Artifacts []artifact `json:"artifacts"`
			}
			err := dec.Decode(&page)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				f.Close()
				return nil, fmt.Errorf("%s: %v", p, err)
			}
			for _, a := range page.Artifacts {
				if a.Expired || seen[a.ID] || !namePattern.MatchString(a.Name) || a.SizeInBytes > maxRecord ||
					a.CreatedAt.Before(w.since) || !a.CreatedAt.Before(w.until) {
					continue
				}
				seen[a.ID] = true
				out = append(out, a)
			}
		}
		f.Close()
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].CreatedAt.Before(out[j].CreatedAt)
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// entry aggregates the FLAKY passes of one test (or one package without a named test).
type entry struct {
	key         string
	passes      int
	runs        map[int64]bool
	byEvent     map[string]int
	first, last time.Time
}

// readRecord reads flaky.json out of the downloaded artifact archive DIR/<id>.zip.
func readRecord(dir string, a artifact) (record, error) {
	var r record
	z, err := zip.OpenReader(filepath.Join(dir, fmt.Sprint(a.ID)+".zip"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return r, errors.New("archive missing: not downloaded")
		}
		return r, fmt.Errorf("archive: %v", err)
	}
	defer z.Close()
	var b []byte
	for _, f := range z.File {
		if filepath.Base(f.Name) != "flaky.json" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return r, fmt.Errorf("flaky.json: %v", err)
		}
		b, err = io.ReadAll(io.LimitReader(rc, maxRecord+1))
		rc.Close()
		if err != nil {
			return r, fmt.Errorf("flaky.json: %v", err)
		}
		break
	}
	if b == nil {
		return r, errors.New("archive has no flaky.json")
	}
	if len(b) > maxRecord {
		return r, errors.New("flaky.json exceeds 1 MiB")
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return r, fmt.Errorf("flaky.json: %v", err)
	}
	if r.Profile != profile || r.Verdict != "FLAKY" {
		return r, fmt.Errorf("flaky.json profile %q verdict %q, want %s FLAKY", r.Profile, r.Verdict, profile)
	}
	return r, nil
}

// report builds the Markdown for the window: one row per test, FLAKY passes first, then the
// records it could not read. It is bounded for an issue body.
func report(selected []artifact, dir string, w window, runURL string) string {
	entries := map[string]*entry{}
	runs := map[int64]bool{}
	byEvent := map[string]int{}
	var unread []string
	read := 0
	for _, a := range selected {
		r, err := readRecord(dir, a)
		if err != nil {
			unread = append(unread, fmt.Sprintf("- artifact %d `%s` (%s): %v", a.ID, a.Name, a.CreatedAt.UTC().Format("2006-01-02"), err))
			continue
		}
		read++
		runs[r.RunID] = true
		byEvent[r.Event]++
		keys := make([]string, 0, len(r.FailedTests))
		for _, ft := range r.FailedTests {
			keys = append(keys, ft.Package+"."+ft.Test)
		}
		if len(keys) == 0 {
			for _, p := range r.FailedPackages {
				keys = append(keys, p+" (package, no test named)")
			}
		}
		sort.Strings(keys)
		for i, k := range keys {
			if i > 0 && keys[i-1] == k {
				continue
			}
			e := entries[k]
			if e == nil {
				e = &entry{key: k, runs: map[int64]bool{}, byEvent: map[string]int{}, first: a.CreatedAt, last: a.CreatedAt}
				entries[k] = e
			}
			e.passes++
			e.runs[r.RunID] = true
			e.byEvent[r.Event]++
			if a.CreatedAt.Before(e.first) {
				e.first = a.CreatedAt
			}
			if a.CreatedAt.After(e.last) {
				e.last = a.CreatedAt
			}
		}
	}
	rows := make([]*entry, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, e)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].passes != rows[j].passes {
			return rows[i].passes > rows[j].passes
		}
		if !rows[i].last.Equal(rows[j].last) {
			return rows[i].last.After(rows[j].last)
		}
		return rows[i].key < rows[j].key
	})
	day := func(t time.Time) string { return t.UTC().Format("2006-01-02") }
	var b strings.Builder
	fmt.Fprintf(&b, "## CI flake report: %s to %s\n\n", day(w.since), day(w.until))
	fmt.Fprintf(&b, "%d FLAKY shard pass(es) in %d CI run(s): merge_group %d, pull_request %d. "+
		"Source: the `ci-flaky-<event>-<shard>` records of go-product-shard reruns (AFP-V0-441); %d of %d selected record(s) read.\n\n",
		read, len(runs), byEvent["merge_group"], byEvent["pull_request"], read, len(selected))
	if len(rows) == 0 {
		b.WriteString("No FLAKY record in the window.\n")
	} else {
		b.WriteString("| Test | FLAKY passes | Runs | merge_group | pull_request | First seen | Last seen |\n|---|---:|---:|---:|---:|---|---|\n")
		for i, e := range rows {
			if i == maxRows || b.Len() > maxBody {
				fmt.Fprintf(&b, "\n... and %d more test(s).\n", len(rows)-i)
				break
			}
			fmt.Fprintf(&b, "| `%s` | %d | %d | %d | %d | %s | %s |\n", e.key, e.passes, len(e.runs),
				e.byEvent["merge_group"], e.byEvent["pull_request"], day(e.first), day(e.last))
		}
	}
	if len(unread) > 0 {
		fmt.Fprintf(&b, "\nNot read (%d): the rate above is a lower bound.\n\n", len(unread))
		for i, u := range unread {
			if i == maxUnread {
				fmt.Fprintf(&b, "- ... and %d more.\n", len(unread)-i)
				break
			}
			b.WriteString(u + "\n")
		}
	}
	b.WriteString("\n_Updated in place each week by the flake-report workflow")
	if runURL != "" {
		fmt.Fprintf(&b, " (%s)", runURL)
	}
	fmt.Fprintf(&b, " at %s; the records expire after 90 days._\n", w.until.UTC().Format(time.RFC3339))
	return b.String()
}
