//go:build darwin || linux

package verify

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/Beamfall/corvint/internal/cem/gitrun"
	"github.com/Beamfall/corvint/internal/groupreap"
)

// s0eRun is one synchronized Stable call over a freshly materialized fixture.
type s0eRun struct {
	t                         *testing.T
	base, repo, git, verified string // B, R, G and the path given to the verifier
	admin                     string // A, for T-LINKED
	artifacts                 string
	cancel                    context.CancelFunc
	mu                        sync.Mutex
	clock                     time.Time
	fakeClock                 bool
	hooks                     []func(gitrun.Event)
	commands                  map[int]func(string, []string) (string, []string)
	primitives                func(ordinal int, session bool) groupreap.Primitives
	fault                     func(path string) error
	diagnose                  func() // logs harness diagnostics when the case fails
	reserved                  int
	attempted                 int
}

func (r *s0eRun) now() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.clock
}

func (r *s0eRun) advance(d time.Duration) {
	r.mu.Lock()
	r.clock = r.clock.Add(d)
	r.mu.Unlock()
}

// at runs action once, at the first matching non-replay event. A zero ordinal
// matches a verifier checkpoint or the session close.
func (r *s0eRun) at(name string, ordinal int, action func()) {
	done := false
	r.hooks = append(r.hooks, func(event gitrun.Event) {
		if !done && event.Name == name && event.Ordinal == ordinal && !event.Replay {
			done = true
			action()
		}
	})
}

func (r *s0eRun) event(event gitrun.Event) {
	if event.Name == "reserved" {
		r.mu.Lock()
		r.reserved++
		r.attempted++
		r.mu.Unlock()
	}
	if event.Name == "refused-before-spawn" {
		r.mu.Lock()
		r.attempted++
		r.mu.Unlock()
	}
	for _, hook := range r.hooks {
		hook(event)
	}
}

func (r *s0eRun) command(ordinal int, replay bool, binary string, args []string) (string, []string) {
	if rewrite := r.commands[ordinal]; rewrite != nil {
		return rewrite(binary, args)
	}
	return binary, args
}

// sleeper replaces the child of one operation with a process that never
// answers, so the injected cause is the only way the operation ends.
func (r *s0eRun) sleeper(ordinal int) {
	r.commands[ordinal] = func(string, []string) (string, []string) {
		return "/bin/sh", []string{"-c", "exec /bin/sleep 600"}
	}
}

// script replaces the child of one operation with a shell script that
// receives the real Git argv as "$@". The script's PATH is minimal, so its Git
// is the absolute path the verifier would run: a bare "git" there would find
// /usr/bin/git, which on hosted macOS-15 rejects --attr-source.
func (r *s0eRun) script(ordinal int, body string) {
	path := filepath.Join(r.base, "shim.sh")
	if err := os.WriteFile(path, []byte("PATH=/bin:/usr/bin\n"+body+"\n"), 0o755); err != nil {
		r.t.Fatal(err)
	}
	r.commands[ordinal] = func(binary string, args []string) (string, []string) {
		if resolved, err := exec.LookPath(binary); err == nil {
			binary = resolved
		}
		return "/bin/sh", append([]string{path, binary}, args...)
	}
}

func (r *s0eRun) launchFailure(ordinal int) {
	r.commands[ordinal] = func(_ string, args []string) (string, []string) {
		return filepath.Join(r.base, "no-such-git"), args
	}
}

// lingeringDescendant makes one operation's child leave a background
// descendant in its process group before it execs the real Git. The shim's
// own diagnostics go to a side file: a host that refuses the fork makes bash
// 3.2 exit 128 before `exec "$@"`, which the product correctly reports as a
// failed Git, so a failing case logs that stderr to attribute it.
func (r *s0eRun) lingeringDescendant(ordinal int) {
	r.script(ordinal, `exec 3>&2 2>"$0.err"
/bin/sleep 600 </dev/null >/dev/null 2>&1 3>&- &
exec 2>&3 3>&-
exec "$@"`)
	shim := filepath.Join(r.base, "shim.sh")
	r.diagnose = func() {
		if diag, err := os.ReadFile(shim + ".err"); err == nil && len(diag) > 0 {
			r.t.Logf("harness: lingering-descendant shim stderr before exec: %q", strings.TrimSpace(string(diag)))
		}
	}
}

var errInjectedProbe = errors.New("injected group probe failure")

func failingProbe(int) (groupreap.Probe, error) { return groupreap.ProbeLive, errInjectedProbe }

// unprovedTeardown makes the cleanup of one one-shot operation, or of the
// session, unobservable: its group probe fails, so the owner must HOLD.
func (r *s0eRun) unprovedTeardown(ordinal int, session bool) {
	r.primitives = func(o int, s bool) groupreap.Primitives {
		if s == session && (session || o == ordinal) {
			return groupreap.Primitives{ProbeGroup: failingProbe}
		}
		return groupreap.Primitives{}
	}
}

func (r *s0eRun) useClock() {
	r.fakeClock = true
	r.clock = time.Unix(1_800_000_000, 0)
}

// linked rewrites the fixture into topology T-LINKED and verifies W.
func (r *s0eRun) linked() {
	r.admin = filepath.Join(r.git, "worktrees", "wt")
	worktree := filepath.Join(r.base, "wt")
	for _, directory := range []string{r.admin, worktree} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			r.t.Fatal(err)
		}
	}
	r.write(filepath.Join(worktree, ".git"), "gitdir: "+r.admin+"\n")
	r.write(filepath.Join(r.admin, "gitdir"), worktree+"/.git\n")
	r.write(filepath.Join(r.admin, "commondir"), "../..\n")
	// Not inspected by admission; Git itself needs it for the control run.
	r.write(filepath.Join(r.admin, "HEAD"), "ref: refs/heads/main\n")
	r.verified = worktree
}

func (r *s0eRun) write(path, content string) {
	os.Remove(path)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
}

func (r *s0eRun) faultAt(path string, errno syscall.Errno) {
	r.fault = func(inspected string) error {
		if inspected == path {
			r.t.Logf("injected errno %d (%v) at the filesystem seam for %s", int(errno), errno, strings.TrimPrefix(path, r.base))
			return &fs.PathError{Op: "lstat", Path: path, Err: errno}
		}
		return nil
	}
}

const (
	s0eSealedOps      = 21
	s0eSidecarBlob    = "632d2d79485d3bb3b509fb0a309f5c2e244133d6"
	s0eEvidenceBlob   = "36e5ee3e49a2a60b602d0c7f2ce5453280af1066"
	s0eOpResolveBase  = 1
	s0eOpResolve      = 2
	s0eOpBaseSidecar  = 3
	s0eOpTargetEntry  = 4
	s0eOpSidecarAgain = 5
	s0eOpSidecarBlob  = 6
	s0eOpDiff         = 8
	s0eOpRootPair     = 9
	s0eOpEvidence     = 16
	s0eOpEvidenceBlob = 17
	s0eOpDriftResolve = 20
)

// blobInjection mutates one loose object when its normal body request is
// reserved, after the path lookup before it succeeded.
func (r *s0eRun) blobInjection(kind string, ordinal int, oid string) {
	switch kind {
	case "launch-failure":
		r.launchFailure(ordinal)
	case "missing-child-exit":
		r.at("reserved", ordinal, func() {
			if err := os.Remove(looseObjectPath(r.git, oid)); err != nil {
				r.t.Error(err)
			}
		})
	case "corrupt-compressed-child-exit":
		r.at("reserved", ordinal, func() { writeObjectFile(r.t, r.git, oid, []byte("not a zlib stream")) })
	case "wrong-complete-hash":
		r.at("reserved", ordinal, func() { writeLooseObject(r.t, r.git, oid, []byte("blob 6\x00wrong\n")) })
	}
}

const s0eOverflowDiff = "/bin/dd if=/dev/zero bs=1048576 count=9 2>/dev/null"

// s0eRecipes synchronizes each public case. A case without a recipe runs the
// unmodified fixture.
var s0eRecipes = map[string]func(r *s0eRun){
	"cancel-before-content":                 func(r *s0eRun) { r.at("metadata-admitted", 0, r.cancel) },
	"cancel-during-target-resolution":       func(r *s0eRun) { r.at("op-start", s0eOpResolve, r.cancel) },
	"cancel-during-base-sidecar":            func(r *s0eRun) { r.at("op-start", s0eOpBaseSidecar, r.cancel) },
	"cancel-during-initial-target-entry":    func(r *s0eRun) { r.at("op-start", s0eOpTargetEntry, r.cancel) },
	"cancel-during-target-sidecar-body":     func(r *s0eRun) { r.at("op-start", s0eOpSidecarBlob, r.cancel) },
	"repeated-target-sidecar-lookup-cancel": func(r *s0eRun) { r.at("op-start", s0eOpSidecarAgain, r.cancel) },
	"cancel-during-canonical-diff": func(r *s0eRun) {
		r.sleeper(s0eOpDiff)
		r.at("op-start", s0eOpDiff, r.cancel)
	},
	"cancel-during-evidence-or-structural": func(r *s0eRun) { r.at("op-start", s0eOpEvidence, r.cancel) },
	"cancel-before-first-drift-item":       func(r *s0eRun) { r.at("op-start", s0eOpDriftResolve, r.cancel) },
	"cancel-during-artifacts":              func(r *s0eRun) { r.at("artifacts-start", 0, r.cancel) },
	"cancel-at-final-publication":          func(r *s0eRun) { r.at("final-check", 0, r.cancel) },
	"root-replaced-before-final-check": func(r *s0eRun) {
		r.at("artifacts-complete", 0, func() {
			if err := os.Rename(r.repo, r.repo+".moved"); err != nil {
				r.t.Error(err)
			}
			if err := os.Mkdir(r.repo, 0o755); err != nil {
				r.t.Error(err)
			}
		})
	},
	"owner-failure-before-first-content-progress": func(r *s0eRun) {
		// Unproved teardown in the first content transaction: the session
		// child never answers, the operation reaches its deadline, and the
		// retirement cannot be observed.
		r.useClock()
		r.sleeper(s0eOpResolveBase)
		r.unprovedTeardown(0, true)
		r.at("op-start", s0eOpResolveBase, func() { r.advance(gitrun.DefaultPerOpTimeout) })
	},
	"owner-failure-after-provisional-complete":   func(r *s0eRun) { r.unprovedTeardown(0, true) },
	"normal-exit-lingering-descendant-contained": func(r *s0eRun) { r.lingeringDescendant(s0eOpDiff) },
	"canonical-per-op-timeout": func(r *s0eRun) {
		r.useClock()
		r.sleeper(s0eOpDiff)
		r.at("op-start", s0eOpDiff, func() { r.advance(gitrun.DefaultPerOpTimeout) })
	},
	"canonical-output-limit": func(r *s0eRun) { r.script(s0eOpDiff, s0eOverflowDiff+"\nexec /bin/sleep 600") },
	"canonical-nonzero-exit": func(r *s0eRun) { r.script(s0eOpDiff, "exit 3") },
	"simultaneous-outer-cancel-and-overflow": func(r *s0eRun) {
		r.script(s0eOpDiff, s0eOverflowDiff+"\nexec /bin/sleep 600")
		r.at("retire-started", s0eOpDiff, r.cancel)
	},
	"simultaneous-overflow-and-nonzero": func(r *s0eRun) { r.script(s0eOpDiff, s0eOverflowDiff+"\nexit 3") },
	"simultaneous-owner-failure-and-cancel": func(r *s0eRun) {
		r.sleeper(s0eOpDiff)
		r.unprovedTeardown(s0eOpDiff, false)
		r.at("op-start", s0eOpDiff, r.cancel)
	},
	"optional-attributes-EACCES": func(r *s0eRun) {
		if err := os.MkdirAll(filepath.Join(r.git, "info"), 0o755); err != nil {
			r.t.Fatal(err)
		}
		r.faultAt(filepath.Join(r.git, "info", "attributes"), syscall.EACCES)
	},
	"optional-objects-info-ELOOP": func(r *s0eRun) {
		r.faultAt(filepath.Join(r.git, "objects", "info"), syscall.ELOOP)
	},
	"optional-commondir-EIO": func(r *s0eRun) {
		r.linked()
		r.faultAt(filepath.Join(r.admin, "commondir"), syscall.EIO)
	},
	"metadata-leaf-replaced": func(r *s0eRun) {
		// The last ordered inspection is the synchronization point: an
		// already admitted metadata directory is replaced by another
		// ordinary directory before admission returns.
		leaf, last, done := filepath.Join(r.git, "objects", "info"), filepath.Join(r.git, "info", "attributes"), false
		if err := os.MkdirAll(filepath.Join(r.git, "info"), 0o755); err != nil {
			r.t.Fatal(err)
		}
		r.fault = func(path string) error {
			if path == last && !done {
				done = true
				if err := os.Rename(leaf, leaf+".moved"); err != nil {
					r.t.Error(err)
				}
				if err := os.Mkdir(leaf, 0o755); err != nil {
					r.t.Error(err)
				}
			}
			return nil
		}
	},
	"admin-ancestor-symlink": func(r *s0eRun) {
		// A primary administrative directory has only the root's ancestors:
		// the verified path reaches the unchanged repository through one
		// symbolic-link ancestor.
		if err := os.Symlink(r.base, filepath.Join(r.base, "link")); err != nil {
			r.t.Fatal(err)
		}
		r.verified = filepath.Join(r.base, "link", "repo")
	},
	"empty-commondir": func(r *s0eRun) {
		r.linked()
		r.write(filepath.Join(r.admin, "commondir"), "")
	},
	"absolute-commondir": func(r *s0eRun) {
		r.linked()
		r.write(filepath.Join(r.admin, "commondir"), r.git+"\n")
	},
	"metadata-CRLF": func(r *s0eRun) {
		r.linked()
		r.write(filepath.Join(r.verified, ".git"), "gitdir: "+r.admin+"\r\n")
	},
	"metadata-two-final-LF": func(r *s0eRun) {
		r.linked()
		r.write(filepath.Join(r.verified, ".git"), "gitdir: "+r.admin+"\n\n")
	},
	"metadata-invalid-UTF8": func(r *s0eRun) {
		r.linked()
		r.write(filepath.Join(r.verified, ".git"), "gitdir: "+r.admin+"/\xff/..\n")
	},
	"metadata-over4096": func(r *s0eRun) {
		r.linked()
		n := 4088 - len(r.admin)
		if n < 0 {
			r.t.Fatalf("base path is too long for the 4,097-byte marker")
		}
		q := strings.Repeat("/.", n/2)
		if n%2 == 1 {
			q = "/" + strings.Repeat("/.", (n-1)/2)
		}
		content := "gitdir: " + r.admin + q + "\n"
		if len(content) != 4097 {
			r.t.Fatalf("marker is %d bytes, want 4097", len(content))
		}
		r.write(filepath.Join(r.verified, ".git"), content)
	},
	"empty-http-alternates": func(r *s0eRun) {
		if err := os.MkdirAll(filepath.Join(r.git, "objects", "info"), 0o755); err != nil {
			r.t.Fatal(err)
		}
		r.write(filepath.Join(r.git, "objects", "info", "http-alternates"), "")
	},
	"canonical-complete-parser-and-nonzero": func(r *s0eRun) {
		r.script(s0eOpRootPair, "printf 'not a batch header\\n'\nexit 3")
	},
	"canonical-per-op-timeout-and-overflow": func(r *s0eRun) {
		// Overflow is latched first; the deadline is reached before the
		// causes are sampled, so both are present at arbitration.
		r.useClock()
		r.script(s0eOpRootPair, "/bin/dd if=/dev/zero bs=65536 count=4 >&2 2>/dev/null\nexec /bin/sleep 600")
		r.at("retire-started", s0eOpRootPair, func() { r.advance(gitrun.DefaultPerOpTimeout) })
	},
	"canonical-valid-framing-limit-plus-one-and-nonzero": func(r *s0eRun) {
		// Real records first, then only further valid tree records, enough of
		// them to cross the local bound.
		r.script(s0eOpRootPair, `out="$0.out"; tree="$0.tree"
"$@" >"$out" || exit 9
printf 'bff36ef7f94a44f0b43dd71ba7125e0185f01fe1^{tree}\0' | "$@" >"$tree" || exit 9
i=0
while [ $i -lt 17 ]; do /bin/cat "$tree" "$tree" >"$tree.next" && /bin/mv -f "$tree.next" "$tree"; i=$((i+1)); done
/bin/cat "$out" "$tree"
exit 3`)
	},
	"canonical-complete-malformed-before-limit-and-overflow": func(r *s0eRun) {
		r.script(s0eOpRootPair, "printf 'not a batch header\\n'\n/bin/dd if=/dev/zero bs=1048576 count=9 2>/dev/null\nexec /bin/sleep 600")
	},
	"outer-expiry-content-observed": func(r *s0eRun) {
		r.useClock()
		r.sleeper(s0eOpDiff)
		r.at("op-start", s0eOpDiff, func() { r.advance(StableOuterDeadline) })
	},
	"outer-expiry-content-unobserved": func(r *s0eRun) {
		r.useClock()
		r.sleeper(s0eOpDiff)
		r.unprovedTeardown(s0eOpDiff, false)
		r.at("op-start", s0eOpDiff, func() { r.advance(StableOuterDeadline) })
	},
	"outer-expiry-normal-session-close-observed": func(r *s0eRun) {
		r.useClock()
		r.at("session-close-start", 0, func() { r.advance(StableOuterDeadline) })
	},
	"outer-expiry-normal-session-close-unobserved": func(r *s0eRun) {
		r.useClock()
		r.unprovedTeardown(0, true)
		r.at("session-close-start", 0, func() { r.advance(StableOuterDeadline) })
	},
	"partial-drift-one-item-cancel": func(r *s0eRun) {
		// 337 evidence items over the sidecarless target: the 15-call
		// fixed prefix, two calls per evidence check, then one lookup per
		// sorted drift item. The second drift lookup is cancelled.
		r.at("op-start", 15+2*337+2, r.cancel)
	},
	"per-op-timeout-returned-then-outer-expiry-in-teardown": func(r *s0eRun) {
		r.useClock()
		r.sleeper(s0eOpDiff)
		r.at("op-start", s0eOpDiff, func() { r.advance(gitrun.DefaultPerOpTimeout) })
		r.at("cause-committed", s0eOpDiff, func() { r.advance(StableOuterDeadline - gitrun.DefaultPerOpTimeout) })
	},
	"per-op-timeout-unreturned-then-outer-expiry-in-teardown": func(r *s0eRun) {
		r.useClock()
		r.sleeper(s0eOpDiff)
		r.at("op-start", s0eOpDiff, func() { r.advance(gitrun.DefaultPerOpTimeout) })
		r.at("retire-started", s0eOpDiff, func() { r.advance(StableOuterDeadline - gitrun.DefaultPerOpTimeout) })
	},
}

func init() {
	for _, kind := range []string{"missing-child-exit", "corrupt-compressed-child-exit", "wrong-complete-hash", "launch-failure"} {
		s0eRecipes["target-sidecar-blob-"+kind] = func(r *s0eRun) { r.blobInjection(kind, s0eOpSidecarBlob, s0eSidecarBlob) }
		s0eRecipes["evidence-blob-"+kind] = func(r *s0eRun) { r.blobInjection(kind, s0eOpEvidenceBlob, s0eEvidenceBlob) }
	}
}

// s0eLedger is the number of admitted logical operations of a complete run.
var s0eLedger = map[string]int{
	"normal-exit-lingering-descendant-contained": s0eSealedOps,
	"ledger-336-evidence":                        1023,
	"ledger-337-evidence":                        1025,
}

func runS0E(t *testing.T, packet *s0ePacket, c s0eCase, recipe func(*s0eRun)) (StableResult, int, *s0eRun) {
	t.Helper()
	raw := packet.maps[s0eMapDigest(c)]
	if raw == nil {
		t.Fatalf("public packet has no map with digest %s", s0eMapDigest(c))
	}
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := &s0eRun{t: t, base: base, cancel: cancel, commands: map[int]func(string, []string) (string, []string){}}
	run.repo, run.artifacts = packet.materialize(t, base)
	run.git, run.verified = filepath.Join(run.repo, ".git"), run.repo
	if recipe != nil {
		recipe(run)
	}
	seam := &stableSeam{git: gitrun.Seam{Event: run.event, Command: run.command, Primitives: run.primitives}}
	if run.fakeClock {
		seam.git.Now = run.now
	}
	if run.fault != nil {
		seam.fault = run.fault
	}
	expectedBase, _ := c.ExpectedResult["expectedBase"].(string)
	target, _ := c.ExpectedResult["targetRevision"].(string)
	result, exit := Stable(ctx, raw, StableOptions{Repository: run.verified, ExpectedBase: expectedBase, Target: target, ArtifactRoot: run.artifacts, seam: seam})
	return result, exit, run
}

// requireStableLifecyclePlatform admits the platforms whose real owned-group
// lifecycle is observed: Darwin, and Linux through the /proc quiet proof
// (PGO-V0-006, V1-0668).
func requireStableLifecyclePlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
		t.Skip("NOT_RUN: Stable lifecycle conformance is observed on Darwin and Linux only")
	}
}

// TestStableS0EPublicCases runs every public full-result case against the
// native verifier and compares the complete closed result and the exit code.
// Each subtest prints one S0E-CASE line; a case that needs another Go release
// is NOT_RUN, never rewritten.
func TestStableS0EPublicCases(t *testing.T) {
	packet := loadS0EPacket(t)
	requireStableLifecyclePlatform(t)
	if len(packet.cases) != 56 {
		t.Fatalf("public packet has %d cases, want 56", len(packet.cases))
	}
	for _, c := range packet.cases {
		t.Run(c.ID, func(t *testing.T) {
			runtimeField, _ := c.ExpectedResult["runtime"].(map[string]any)
			if version, _ := runtimeField["goVersion"].(string); version != runtime.Version() {
				t.Logf("S0E-CASE %s NOT_RUN requires %s, running %s", c.ID, version, runtime.Version())
				t.Skipf("NOT_RUN: case requires %s", version)
			}
			result, exit, run := runS0E(t, packet, c, s0eRecipes[c.ID])
			ok := requireS0EResult(t, c, result, exit)
			if want, bound := s0eLedger[c.ID]; bound && run.attempted != want {
				ok = false
				t.Errorf("attempted logical operations = %d, want %d", run.attempted, want)
			}
			status := "PASS"
			if !ok {
				status = "FAIL"
				if run.diagnose != nil {
					run.diagnose()
				}
			}
			t.Logf("S0E-CASE %s %s exit=%d stage=%s operations=%d", c.ID, status, exit, result.Stage, run.attempted)
		})
	}
}

// TestStableS0ELinkedControl proves that topology T-LINKED with no injection
// passes admission and verifies exactly like the primary repository, so each
// T-LINKED case's injection is its only defect.
func TestStableS0ELinkedControl(t *testing.T) {
	packet := loadS0EPacket(t)
	requireStableLifecyclePlatform(t)
	c := packet.find(t, "normal-exit-lingering-descendant-contained")
	result, exit, run := runS0E(t, packet, c, func(r *s0eRun) { r.linked() })
	if requireS0EResult(t, c, result, exit) && run.attempted != s0eSealedOps {
		t.Errorf("attempted logical operations = %d, want %d", run.attempted, s0eSealedOps)
	}
}
