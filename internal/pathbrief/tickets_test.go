package pathbrief

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/Beamfall/corvint/internal/procgroup"
)

// TBC-V1-010: which scope entries cover a path.
func TestScopeCovers(t *testing.T) {
	cases := []struct {
		entry, path string
		directory   bool
		want        bool
	}{
		{"src/x/y.go", "src/x/y.go", false, true},
		{"src", "src/x/y.go", false, true},
		{"src/", "src/x/y.go", false, true},
		{"./src", "src/x/y.go", false, true},
		{"src/x/y.go/", "src/x/y.go", false, true},
		{"srcx", "src/x/y.go", false, false},
		{"src/x/y.go", "src", true, true},
		{"src/x/y.go", "src", false, false},
		{"srcx/a", "src", true, false},
		{".", "a", false, true},
		{"", "a", false, false},
		{"a", "", true, true},
		{"", "", true, false},
		{"*.go", "a.go", false, false},
		{"src/*", "src/a", false, false},
	}
	for _, c := range cases {
		if got := scopeCovers(c.entry, c.path, c.directory); got != c.want {
			t.Errorf("scopeCovers(%q, %q, %v) = %v", c.entry, c.path, c.directory, got)
		}
	}
}

// TBC-V1-010: one page of `corvint-tasks ticket search`.
func TestParseTicketPage(t *testing.T) {
	const page = `{"outcome":"OK","page":{"truncated":%s},"items":[
	 {"ticketId":"V1-1","title":"t","declaredScope":["a/"],"record":{"effects":{"touchPaths":["b.go"]}}},
	 {"ticketId":"V1-2","title":"u"}]}`
	for _, truncated := range []string{"true", "false"} {
		tickets, more, err := parseTicketPage([]byte(strings.Replace(page, "%s", truncated, 1)))
		if err != nil || more != (truncated == "true") || len(tickets) != 2 {
			t.Fatalf("page: %v %v %v", tickets, more, err)
		}
		if tickets[0].ID != "V1-1" || tickets[0].Title != "t" || !slices.Equal(tickets[0].DeclaredScope, []string{"a/"}) || !slices.Equal(tickets[0].TouchPaths, []string{"b.go"}) {
			t.Errorf("ticket %+v", tickets[0])
		}
	}
	for name, data := range map[string]string{
		"outcome not OK": `{"outcome":"REFUSED","items":[]}`,
		"no outcome":     `{"items":[]}`,
		"missing id":     `{"outcome":"OK","items":[{"title":"x"}]}`,
		"not JSON":       `outcome OK`,
		"empty":          ``,
	} {
		if _, _, err := parseTicketPage([]byte(data)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TBC-V1-011: the Tasks source fails quietly when corvint-tasks is not installed.
func TestExecTicketsWithoutCompanion(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	tickets, err := ExecTickets(t.TempDir())(context.Background())
	if err == nil || tickets != nil {
		t.Fatalf("got %v, %v", tickets, err)
	}
}

// TBC-V1-010: ExecTickets pages through the store and treats any failure as a missing source.
func TestExecTicketsPages(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable lookup needs an extension")
	}
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, TasksExecutable), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	original := runProcess
	t.Cleanup(func() { runProcess = original })
	var offsets []string
	respond := func(stdout string, exit int) {
		runProcess = func(_ context.Context, spec procgroup.Spec) procgroup.Observation {
			i := slices.Index(spec.Argv, "--offset")
			offsets = append(offsets, spec.Argv[i+1])
			return procgroup.Observation{Stdout: []byte(stdout), ExitStatus: exit}
		}
	}
	first := `{"outcome":"OK","page":{"truncated":true},"items":[{"ticketId":"V1-1"}]}`
	last := `{"outcome":"OK","page":{"truncated":false},"items":[{"ticketId":"V1-2"}]}`
	calls := 0
	runProcess = func(_ context.Context, spec procgroup.Spec) procgroup.Observation {
		i := slices.Index(spec.Argv, "--offset")
		offsets = append(offsets, spec.Argv[i+1])
		if slices.Index(spec.Argv, "OPEN") < 0 {
			t.Errorf("argv %v does not ask for open tickets", spec.Argv)
		}
		calls++
		if calls == 1 {
			return procgroup.Observation{Stdout: []byte(first)}
		}
		return procgroup.Observation{Stdout: []byte(last)}
	}
	tickets, err := ExecTickets(t.TempDir())(context.Background())
	if err != nil || len(tickets) != 2 || !slices.Equal(offsets, []string{"0", "1000"}) {
		t.Fatalf("paged read: %v %v offsets %v", tickets, err, offsets)
	}
	for name, c := range map[string]struct {
		out  string
		exit int
	}{"exit status": {last, 1}, "bad page": {"nope", 0}} {
		respond(c.out, c.exit)
		if _, err := ExecTickets(t.TempDir())(context.Background()); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	// A ticket a later page repeats is kept once.
	pages := 0
	runProcess = func(context.Context, procgroup.Spec) procgroup.Observation {
		if pages++; pages == 1 {
			return procgroup.Observation{Stdout: []byte(first)}
		}
		return procgroup.Observation{Stdout: []byte(`{"outcome":"OK","page":{"truncated":false},"items":[{"ticketId":"V1-1"},{"ticketId":"V1-2"}]}`)}
	}
	if tickets, err := ExecTickets(t.TempDir())(context.Background()); err != nil || len(tickets) != 2 {
		t.Errorf("repeated ticket: %v %v", tickets, err)
	}
	// A store that always says "more" is cut at the page bound, as a missing source.
	runProcess = func(context.Context, procgroup.Spec) procgroup.Observation {
		return procgroup.Observation{Stdout: []byte(first)}
	}
	if _, err := ExecTickets(t.TempDir())(context.Background()); err == nil {
		t.Error("endless pages accepted")
	}
}
