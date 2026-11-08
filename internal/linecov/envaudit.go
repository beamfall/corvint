// SPDX-License-Identifier: Apache-2.0
package linecov

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"
)

// EnvBinding is one inherited environment variable a test's record was
// observed under (GLS-V0-017): the variable it read and the value it saw, as
// EnvHash of the value, or EnvUnset for a variable absent from the
// environment. A read of the whole environment (os.Environ) is the binding
// named EnvAll; it observes the collector's own variables, so its value is
// EnvModified and it never holds (an earlier collector bound EnvAllHash of
// the environment instead). A selection compares the
// bindings of a test it would leave out against the environment the tests
// will run under; any difference keeps the test (GLS-V0-018).
type EnvBinding struct {
	Name, Value string
}

// The EnvBinding.Value form of an absent variable, the EnvAll name, and the
// value of a binding the collector itself set: EnvModified is never equal to
// a current value, so a test that read what the collector changed is always
// kept (GLS-V0-017, GLS-V0-018).
const (
	EnvAll      = "*"
	EnvUnset    = "unset"
	EnvModified = "collector-modified"
)

// EnvAllHash is the EnvAll binding value of an environment: the SHA-256 of
// its NAME=VALUE pairs, sorted by name, newline-separated, leaving out the
// neutral and collector-modified names. A collector no longer writes it (a
// whole-environment read observes the collector's own variables, so it binds
// EnvModified). This remains the comparison of an individual legacy binding;
// index admission rejects the old audit's completeness claim before selection.
func EnvAllHash(values map[string]string) string {
	names := make([]string, 0, len(values))
	for name := range values {
		if !envNeutral(name) && !EnvCollectorModified(name) {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name + "=" + values[name] + "\n")
	}
	return EnvHash(b.String())
}

// EnvHash is the binding value of an environment variable's value: its
// SHA-256, so an index retains no secret it was collected under.
func EnvHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// EnvNeutral names the variables a binding never records: PWD is the package
// directory on every host, by go test's contract, and its absolute prefix is
// host placement, not test input; OLDPWD, SHLVL and _ are the invoking
// shell's bookkeeping of its own last command and directory, which differ
// between two invocations of the same run (GLS-V0-017).
var EnvNeutral = map[string]bool{"PWD": true, "OLDPWD": true, "SHLVL": true, "_": true}

// EnvCollectorPrefix is the prefix of the collector's own variables.
const EnvCollectorPrefix = "CORVINT_LINECOV_"

// EnvCollectorModified reports whether the collector sets or changes name
// for every collected process: GOFLAGS gains the coverage and overlay flags,
// GOCOVERDIR names a scratch directory, and the CORVINT_LINECOV_* variables
// exist only under collection. A test that reads one observed a value no
// selected run has, so its record binds EnvModified and the test is always
// kept; the coverage runtime's and the exec audit's own reads are not the
// test's (GLS-V0-017).
func EnvCollectorModified(name string) bool {
	return name == "GOFLAGS" || name == "GOCOVERDIR" || strings.HasPrefix(name, EnvCollectorPrefix)
}

// envNeutral reports whether name is never bound.
func envNeutral(name string) bool {
	return EnvNeutral[name]
}

// parseEnvAudit reads the env-audit file the overlaid os package of a test
// process and its instrumented children append to: `read NAME QUOTED`,
// `unset NAME`, `modified NAME` and `all`, tab-separated, each key once per
// process. It returns the sorted, deduplicated bindings with neutral names
// left out. A read of a collector-modified name, a `modified` line and an
// `all` line (the whole environment includes the collector's variables) bind
// EnvModified, which no environment holds, so the test is kept: what it did
// under the collector's values says nothing of a run without them. A line it
// cannot parse is an error, so a damaged audit yields no record.
func parseEnvAudit(data []byte) ([]EnvBinding, error) {
	seen := map[string]string{}
	bind := func(name, value string) {
		switch {
		case envNeutral(name):
		case name == EnvAll || EnvCollectorModified(name):
			seen[name] = EnvModified
		default:
			if _, dup := seen[name]; !dup {
				seen[name] = value
			}
		}
	}
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		f := strings.SplitN(line, "\t", 3)
		switch {
		case f[0] == "all" && len(f) == 1:
			bind(EnvAll, EnvModified)
		case f[0] == "modified" && len(f) == 2 && f[1] != "":
			seen[f[1]] = EnvModified
		case f[0] == "unset" && len(f) == 2 && f[1] != "":
			bind(f[1], EnvUnset)
		case f[0] == "read" && len(f) == 3 && f[1] != "":
			v, err := strconv.Unquote(f[2])
			if err != nil {
				return nil, &envAuditError{line}
			}
			bind(f[1], EnvHash(v))
		default:
			return nil, &envAuditError{line}
		}
	}
	out := make([]EnvBinding, 0, len(seen))
	for name, value := range seen {
		out = append(out, EnvBinding{Name: name, Value: value})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

type envAuditError struct{ line string }

func (e *envAuditError) Error() string {
	return "linecov: malformed env-audit line " + strconv.Quote(e.line)
}

// EnvValues is the lookup form of an environment list: each name's last value.
func EnvValues(environ []string) map[string]string {
	values := make(map[string]string, len(environ))
	for _, kv := range environ {
		if name, value, ok := strings.Cut(kv, "="); ok {
			values[name] = value
		}
	}
	return values
}

// EnvUnbound reports whether a record with bindings is unbound under the
// environment values: the first binding whose current value differs, as a
// `NAME: recorded, current` detail, or "" when every binding holds. A
// collector-modified binding never holds.
func EnvUnbound(bindings []EnvBinding, values map[string]string) string {
	for _, b := range bindings {
		if b.Value == EnvModified {
			return b.Name + ": read a value the collector set"
		}
		current := EnvUnset
		if b.Name == EnvAll {
			current = EnvAllHash(values)
		} else if v, ok := values[b.Name]; ok {
			current = EnvHash(v)
		}
		if current != b.Value {
			return b.Name + ": recorded " + short(b.Value) + ", current " + short(current)
		}
	}
	return ""
}

func short(v string) string {
	if len(v) > 19 {
		return v[:19]
	}
	return v
}

// envBindings is a test's bindings resolved through the index table.
func (ix *Index) envBindings(id int32) []EnvBinding {
	t := ix.Tests[id]
	out := make([]EnvBinding, 0, len(t.Env))
	for _, i := range t.Env {
		if int(i) < len(ix.Env) {
			out = append(out, ix.Env[i])
		}
	}
	return out
}

// The os overlay (GLS-V0-017). envHelper is added to package os in the
// collector's scratch overlay; envEdits rewrite its accessors to report each
// inherited read once per process to the file CORVINT_LINECOV_ENVAUDIT names.
// A read whose value differs from the process's initial environment, or of a
// name CORVINT_LINECOV_ENVSELF lists (set by the parent for a child), is the
// program's own setting, not inherited, and is not reported. An inherited
// read of a collector-modified name is reported as `modified` unless the
// first caller outside package os is the coverage runtime or the exec audit,
// whose reads are the collector's own and leave the name unreported.
var envEdits = [][2]string{
	{"func Getenv(key string) string {\n\ttestlog.Getenv(key)\n\tv, _ := syscall.Getenv(key)\n\treturn v\n}",
		"func Getenv(key string) string {\n\ttestlog.Getenv(key)\n\tv, ok := syscall.CorvintLinecovGetenvQuiet(key)\n\tcorvintLinecovEnvRead(key, v, ok)\n\treturn v\n}"},
	{"func LookupEnv(key string) (string, bool) {\n\ttestlog.Getenv(key)\n\treturn syscall.Getenv(key)\n}",
		"func LookupEnv(key string) (string, bool) {\n\ttestlog.Getenv(key)\n\tv, ok := syscall.CorvintLinecovGetenvQuiet(key)\n\tcorvintLinecovEnvRead(key, v, ok)\n\treturn v, ok\n}"},
	{"func Environ() []string {\n\treturn syscall.Environ()\n}",
		"func Environ() []string {\n\tcorvintLinecovEnvAll()\n\treturn syscall.CorvintLinecovEnvironQuiet()\n}"},
}

const envHelper = `// SPDX-License-Identifier: Apache-2.0
// Present only in the line coverage collector's scratch overlay (GLS-V0-017).
package os

import (
	"runtime"
	"strconv"
	"sync"
	"syscall"
)

var (
	corvintLCEnvMu      sync.Mutex
	corvintLCEnvSeen    = map[string]bool{}
	corvintLCEnvFile, _ = syscall.CorvintLinecovGetenvQuiet("CORVINT_LINECOV_ENVAUDIT")
	corvintLCEnvInitial = corvintLinecovEnvSnapshot()
	corvintLCEnvSelf    = corvintLinecovEnvSelfSet()
)

func init() {
 syscall.CorvintLinecovEnvObserve(corvintLinecovEnvRead, corvintLinecovEnvAll)
}

// corvintLinecovEnvSnapshot is the environment this process started with,
// taken when package os initializes, before any importer can change it.
func corvintLinecovEnvSnapshot() map[string]string {
	initial := map[string]string{}
	for _, kv := range syscall.CorvintLinecovEnvironQuiet() {
		for i := 0; i < len(kv); i++ {
			if kv[i] == '=' {
				if _, dup := initial[kv[:i]]; !dup {
					initial[kv[:i]] = kv[i+1:]
				}
				break
			}
		}
	}
	return initial
}

func corvintLinecovEnvSelfSet() map[string]bool {
	self := map[string]bool{}
	list, _ := syscall.CorvintLinecovGetenvQuiet("CORVINT_LINECOV_ENVSELF")
	for len(list) > 0 {
		name := list
		if i := indexByte(list, ','); i >= 0 {
			name, list = list[:i], list[i+1:]
		} else {
			list = ""
		}
		if name != "" {
			self[name] = true
		}
	}
	return self
}

func indexByte(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func corvintLinecovEnvWrite(line string) {
	f, err := OpenFile(corvintLCEnvFile, O_CREATE|O_APPEND|O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	if _, err = f.WriteString(line); err != nil {
		panic(err)
	}
	if err = f.Close(); err != nil {
		panic(err)
	}
}

// corvintLinecovEnvRead reports an inherited read of key, once.
func corvintLinecovEnvRead(key, value string, ok bool) {
	if corvintLCEnvFile == "" {
		return
	}
	corvintLCEnvMu.Lock()
	defer corvintLCEnvMu.Unlock()
	if corvintLCEnvSeen[key] || corvintLCEnvSelf[key] {
		return
	}
	initial, had := corvintLCEnvInitial[key]
	if had != ok || ok && initial != value {
		return // the program's own setting
	}
	if corvintLinecovCollectorName(key) {
		if corvintLinecovCollectorCaller() {
			return // the collector's own read; a later test read still reports
		}
		corvintLCEnvSeen[key] = true
		corvintLinecovEnvWrite("modified\t" + key + "\n")
		return
	}
	corvintLCEnvSeen[key] = true
	if ok {
		corvintLinecovEnvWrite("read\t" + key + "\t" + strconv.Quote(value) + "\n")
	} else {
		corvintLinecovEnvWrite("unset\t" + key + "\n")
	}
}

// corvintLinecovCollectorName reports whether the collector sets or changes
// key for every collected process (linecov.EnvCollectorModified).
func corvintLinecovCollectorName(key string) bool {
	const prefix = "CORVINT_LINECOV_"
	return key == "GOFLAGS" || key == "GOCOVERDIR" || len(key) >= len(prefix) && key[:len(prefix)] == prefix
}

// corvintLinecovCollectorCaller reports whether the first caller outside
// package os is the coverage runtime or the exec audit. An unknown frame is
// not: its read is reported.
func corvintLinecovCollectorCaller() bool {
	pcs := make([]uintptr, 32)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	for {
		f, more := frames.Next()
		if pkg := corvintLinecovFuncPackage(f.Function); pkg != "os" {
			return pkg == "internal/coverage/cfile" || pkg == "os/exec"
		}
		if !more {
			return false
		}
	}
}

// corvintLinecovFuncPackage is the import path of a frame's function name,
// such as os/exec of os/exec.(*Cmd).Start.
func corvintLinecovFuncPackage(fn string) string {
	if i := indexByte(fn, '['); i >= 0 {
		fn = fn[:i]
	}
	slash := -1
	for i := 0; i < len(fn); i++ {
		if fn[i] == '/' {
			slash = i
		}
	}
	if i := indexByte(fn[slash+1:], '.'); i >= 0 {
		return fn[:slash+1+i]
	}
	return fn
}

// corvintLinecovEnvAll reports a read of the whole environment, once.
func corvintLinecovEnvAll() {
	if corvintLCEnvFile == "" || corvintLinecovExecInheritance() {
		return
	}
	corvintLCEnvMu.Lock()
	defer corvintLCEnvMu.Unlock()
	if corvintLCEnvSeen["\x00all"] {
		return
	}
	corvintLCEnvSeen["\x00all"] = true
	corvintLinecovEnvWrite("all\n")
}

// Only the audited Cmd.Start inheritance path passes the environment without
// consuming it. Cmd.Environ, direct syscalls and unknown stacks are reads.
func corvintLinecovExecInheritance() bool {
 pcs := make([]uintptr, 32)
 frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
 want := []string{"syscall.Environ", "internal/syscall/execenv.Default", "os/exec.(*Cmd).environ", "os/exec.(*Cmd).Start"}
 i := 0
 for {
  f, more := frames.Next()
  if i == 0 && corvintLinecovFuncPackage(f.Function) == "os" {
   if !more { return false }; continue
  }
  if f.Function != want[i] { return false }
  i++
  if i == len(want) { return true }
  if !more { return false }
 }
}

// CorvintLinecovEnvironQuiet is the environment without reporting a read:
// for the collector's own exec audit, which passes it to a child whose own
// reads are reported, and which must not bind the parent to everything.
func CorvintLinecovEnvironQuiet() []string { return syscall.CorvintLinecovEnvironQuiet() }

// CorvintLinecovEnvSelfFor is the CORVINT_LINECOV_ENVSELF value of a child
// started with env: every name whose value differs from this process's
// initial environment, or that this process itself was told is its own.
func CorvintLinecovEnvSelfFor(env []string) string {
	names := map[string]bool{}
	for n := range corvintLCEnvSelf {
		names[n] = true
	}
	seen := map[string]bool{}
	for _, kv := range env {
		i := indexByte(kv, '=')
		if i < 0 {
			continue
		}
		k, v := kv[:i], kv[i+1:]
		if seen[k] {
			continue
		}
		seen[k] = true
		if initial, had := corvintLCEnvInitial[k]; !had || initial != v {
			names[k] = true
		}
	}
	for k := range corvintLCEnvInitial {
		if !seen[k] {
			names[k] = true
		}
	}
	list := make([]string, 0, len(names))
	for n := range names {
		list = append(list, n)
	}
	// Sorted, so a child's variable is deterministic.
	for i := 1; i < len(list); i++ {
		for j := i; j > 0 && list[j] < list[j-1]; j-- {
			list[j], list[j-1] = list[j-1], list[j]
		}
	}
	out := ""
	for i, n := range list {
		if i > 0 {
			out += ","
		}
		out += n
	}
	return out
}
`
