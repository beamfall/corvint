// SPDX-License-Identifier: AGPL-3.0-or-later
// Command conformance builds one authored provider from a copy of the kit's
// main.go and exercises it through Corvint's file and contained command
// transports. It uses only Go's standard library, so it builds from a checkout
// of the kit directory alone (EEP-V0-021, EEP-V0-022).
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const kitVersion = "0.2.0"

// changedPath is the one tracked path every case relates to its entity.
const changedPath = "capability.md"

type harness struct {
	scratch, corvint, provider, repo string
	id, version, first, head         string
}

// testCase is one labelled class. A nil edit launches the provider itself on
// the command transport; an edit rewrites the provider's valid /1 bytes, and
// the command transport then replays them with cat.
type testCase struct {
	name string
	args []string
	edit func(map[string]any) []byte
	want string
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	kit := flags.String("kit", ".", "directory holding the provider main.go")
	corvint := flags.String("corvint", "corvint", "Corvint binary")
	id := flags.String("provider-id", "kit-example", "exact provider id the source declares")
	version := flags.String("provider-version", "0.1.0", "exact provider revision the source declares")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	binary, err := exec.LookPath(*corvint)
	if err != nil {
		return err
	}
	binary, err = filepath.Abs(binary)
	if err != nil {
		return err
	}
	scratch, err := os.MkdirTemp("", "corvint-kit-conformance-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	h := &harness{scratch: scratch, corvint: binary, id: *id, version: *version}
	if err := h.build(ctx, *kit); err != nil {
		return fmt.Errorf("offline provider build: %w", err)
	}
	if err := h.repository(ctx); err != nil {
		return fmt.Errorf("scratch repository: %w", err)
	}
	baseline, err := h.impact(ctx)
	if err != nil {
		return err
	}
	cases := h.cases()
	for _, c := range cases {
		got, err := h.exercise(ctx, c, baseline)
		if err != nil {
			return fmt.Errorf("%s: %w", c.name, err)
		}
		if got != c.want {
			return fmt.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
		fmt.Fprintf(out, "pass %s: %s\n", c.name, got)
	}
	if err := h.refusal(ctx); err != nil {
		return fmt.Errorf("unsupported profile request: %w", err)
	}
	fmt.Fprintf(out, "pass unsupported profile request: provider refused, command unavailable\n")
	fmt.Fprintf(out, "kit %s conformance: %d cases agree over file and command transports\n", kitVersion, len(cases)+1)
	return nil
}

func (h *harness) cases() []testCase {
	loaded := "state=loaded provider=" + h.id + "@" + h.version
	absent := "state=invalid provider=-@- freshness=- binding=- results=0 relation=-"
	return []testCase{
		{"valid /0", h.args("0", h.head, ""), nil, loaded + " freshness=equal binding=- results=1 relation=-"},
		{"valid /1", h.args("1", h.head, h.first), nil, loaded + " freshness=equal binding=root results=1 relation=fresh"},
		{"valid /2", h.args("2", h.head, h.first), nil, loaded + " freshness=equal binding=root results=1 relation=fresh"},
		{"stale", h.args("1", h.first, h.first), nil, loaded + " freshness=repository-ahead binding=root results=1 relation=stale"},
		{"repository mismatch", h.args("1", h.head, h.head), nil, loaded + " freshness=not-evaluated binding=unbound results=0 relation=-"},
		{"malformed", nil, truncated, absent},
		{"ambiguous", nil, ambiguous, loaded + " freshness=identity-ambiguous binding=unresolved results=0 relation=-"},
		{"unsupported profile record", nil, unsupported, absent},
	}
}

func (h *harness) args(profile, revision, origin string) []string {
	args := []string{"--provider-version", h.version, "--profile", "external-evidence-provider/" + profile, "--revision", revision, "--path", changedPath}
	if origin != "" {
		args = append(args, "--origin", origin)
	}
	return args
}

func truncated(record map[string]any) []byte {
	data, _ := json.Marshal(record)
	return data[:len(data)/2]
}

func ambiguous(record map[string]any) []byte {
	repositories := record["repositories"].([]any)
	fork := map[string]any{}
	for key, value := range repositories[0].(map[string]any) {
		fork[key] = value
	}
	fork["id"] = "app-fork"
	record["repositories"] = append(repositories, fork)
	data, _ := json.Marshal(record)
	return data
}

func unsupported(record map[string]any) []byte {
	record["schema"] = "external-evidence-provider/3"
	data, _ := json.Marshal(record)
	return data
}

// build compiles only a copy of main.go, offline, with an empty build cache.
func (h *harness) build(ctx context.Context, kit string) error {
	source, err := os.ReadFile(filepath.Join(kit, "main.go"))
	if err != nil {
		return err
	}
	author := filepath.Join(h.scratch, "author")
	if err := os.Mkdir(author, 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(author, "main.go"), source, 0o600); err != nil {
		return err
	}
	h.provider = filepath.Join(h.scratch, "provider")
	env := append(h.env(), "GOCACHE="+filepath.Join(h.scratch, "gocache"), "GOTOOLCHAIN=local", "GOENV=off", "GOWORK=off", "GO111MODULE=off", "GOFLAGS=", "GOPROXY=off", "GOSUMDB=off")
	_, err = h.command(ctx, author, env, "go", "build", "-trimpath", "-o", h.provider, "main.go")
	return err
}

// repository commits changedPath twice so the root commit is a real ancestor.
func (h *harness) repository(ctx context.Context) error {
	h.repo = filepath.Join(h.scratch, "repository")
	if err := os.Mkdir(h.repo, 0o700); err != nil {
		return err
	}
	git := []string{"git", "-c", "user.name=kit", "-c", "user.email=kit@example.invalid", "-c", "commit.gpgsign=false"}
	steps := [][]string{{"init", "-q"}, {"add", changedPath}, {"commit", "-qm", "first"}, {"commit", "-qam", "second"}}
	for i, step := range steps {
		content := "first\n"
		if i == 3 {
			content += "second\n"
		}
		if err := os.WriteFile(filepath.Join(h.repo, changedPath), []byte(content), 0o600); err != nil {
			return err
		}
		if _, err := h.command(ctx, h.repo, h.env(), append(git, step...)...); err != nil {
			return err
		}
	}
	first, err := h.command(ctx, h.repo, h.env(), "git", "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return err
	}
	head, err := h.command(ctx, h.repo, h.env(), "git", "rev-parse", "HEAD")
	h.first, h.head = strings.TrimSpace(string(first)), strings.TrimSpace(string(head))
	return err
}

// exercise compares the file and command receipts apart from the provider
// source, checks the core receipt against the no-provider baseline, and
// returns the outcome summary both transports share.
func (h *harness) exercise(ctx context.Context, c testCase, baseline map[string]any) (string, error) {
	argv := append([]string{h.provider}, c.args...)
	if c.edit != nil {
		argv = append([]string{h.provider}, h.args("1", h.head, h.first)...)
	}
	data, err := h.command(ctx, h.repo, h.env(), argv...)
	if err != nil {
		return "", err
	}
	if c.edit != nil {
		var record map[string]any
		if err := json.Unmarshal(data, &record); err != nil {
			return "", err
		}
		data = c.edit(record)
	}
	file := filepath.Join(h.scratch, "record.json")
	if err := os.WriteFile(file, data, 0o600); err != nil {
		return "", err
	}
	if c.edit != nil {
		argv = []string{"/bin/cat", file}
	}
	encoded, _ := json.Marshal(argv)
	fromFile, err := h.impact(ctx, "--provider", file)
	if err != nil {
		return "", err
	}
	fromCommand, err := h.impact(ctx, "--provider-command", string(encoded))
	if err != nil {
		return "", err
	}
	fileExternal := external(fromFile)
	commandExternal := external(fromCommand)
	for _, section := range []map[string]any{fileExternal, commandExternal} {
		first(section["providers"])["source"] = "neutral"
	}
	if canonical(fileExternal) != canonical(commandExternal) {
		return "", errors.New("external section differs by transport")
	}
	delete(fromFile["context"].(map[string]any), "external")
	if canonical(fromFile) != canonical(baseline) {
		return "", errors.New("core receipt changed by the provider")
	}
	return summary(fileExternal), nil
}

// refusal requests an unsupported profile: the provider must exit nonzero
// with no record, and the command transport must report it unavailable.
func (h *harness) refusal(ctx context.Context) error {
	args := h.args("3", h.head, h.first)
	var stdout bytes.Buffer
	command := exec.CommandContext(ctx, h.provider, args...)
	command.Stdout = &stdout
	if err := command.Run(); err == nil || stdout.Len() != 0 {
		return fmt.Errorf("provider emitted %d bytes, exit error %v", stdout.Len(), err)
	}
	encoded, _ := json.Marshal(append([]string{h.provider}, args...))
	receipt, err := h.impact(ctx, "--provider-command", string(encoded))
	if err != nil {
		return err
	}
	if got := summary(external(receipt)); got != "state=unavailable provider=-@- freshness=- binding=- results=0 relation=-" {
		return fmt.Errorf("command transport outcome %s", got)
	}
	return nil
}

// impact runs one read-only impact receipt over changedPath.
func (h *harness) impact(ctx context.Context, provider ...string) (map[string]any, error) {
	args := append(append([]string{h.corvint, "--root", h.repo, "impact"}, provider...), changedPath)
	data, err := h.command(ctx, h.repo, h.env(), args...)
	if err != nil {
		return nil, err
	}
	var receipt map[string]any
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, err
	}
	if receipt["mutates"] != false {
		return nil, errors.New("impact receipt must report mutates false")
	}
	return receipt, nil
}

// env isolates tools from the caller's home, global Git config and caches.
func (h *harness) env() []string {
	return []string{"PATH=" + os.Getenv("PATH"), "HOME=" + h.scratch, "TMPDIR=" + os.TempDir(), "LANG=C", "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1"}
}

func (h *harness) command(ctx context.Context, dir string, env []string, argv ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, argv[0], argv[1:]...)
	command.Dir, command.Env = dir, env
	command.Cancel = func() error { return command.Process.Signal(os.Interrupt) }
	command.WaitDelay = 10 * time.Second
	var stderr bytes.Buffer
	command.Stderr = &stderr
	out, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", filepath.Base(argv[0]), err, bytes.TrimSpace(stderr.Bytes()))
	}
	return out, nil
}

func summary(section map[string]any) string {
	provider := first(section["providers"])
	repository := first(provider["repositories"])
	result := first(section["results"])
	freshness := provider["freshness"]
	if repository != nil {
		freshness = repository["freshness"]
	}
	results, _ := section["results"].([]any)
	return fmt.Sprintf("state=%s provider=%s@%s freshness=%s binding=%s results=%d relation=%s",
		text(provider["state"]), text(provider["id"]), text(provider["revision"]), text(freshness),
		text(repository["binding"]), len(results), text(result["relation_state"]))
}

func external(receipt map[string]any) map[string]any {
	section, _ := receipt["context"].(map[string]any)["external"].(map[string]any)
	return section
}

func first(value any) map[string]any {
	items, _ := value.([]any)
	if len(items) == 0 {
		return nil
	}
	item, _ := items[0].(map[string]any)
	return item
}

func text(value any) string {
	if value == nil || value == "" {
		return "-"
	}
	return fmt.Sprint(value)
}

func canonical(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
