// SPDX-License-Identifier: Apache-2.0

package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/gitauth"
	"github.com/Beamfall/corvint/internal/proctrace"
	"github.com/Beamfall/corvint/internal/testevidence"
)

// TraceScriptCommand is the core executor for a script SPI command. The caller
// admits/pins q.Executable for cmd.Argv[0] and supplies repository/outside roots
// and caps, just as for other core execution. No PATH search or shell/glob
// expansion occurs here. A nil backend uses the platform tracing service.
// The returned decimal report is consumed by the command's bound SPI Parse.
func TraceScriptCommand(ctx context.Context, cmd CommandResponse, q proctrace.Request, backend proctrace.Backend) (proctrace.Result, string, error) {
	if len(cmd.Argv) == 0 || cmd.Argv[0] == "" || !filepath.IsAbs(q.Executable) || !filepath.IsAbs(q.Dir) || strings.ContainsRune(q.Executable+q.Dir, 0) || q.EventCap <= 0 || q.PathCap <= 0 || len(q.Repositories) == 0 || cmd.Cwd != q.Dir || !slices.Equal(cmd.Argv[1:], q.Args) {
		return proctrace.Result{}, "", fmt.Errorf("script-trace-command-mismatch")
	}
	env := q.Env
	if env == nil {
		env = os.Environ()
	}
	q.Env = make([]string, 0, len(env)+len(cmd.Env))
	for _, assignment := range env {
		key, _, _ := strings.Cut(assignment, "=")
		if _, replaced := cmd.Env[key]; !replaced {
			q.Env = append(q.Env, assignment)
		}
	}
	keys := make([]string, 0, len(cmd.Env))
	for key := range cmd.Env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		q.Env = append(q.Env, key+"="+cmd.Env[key])
	}
	if backend == nil {
		backend = proctrace.New()
	}
	trace := backend.Trace(ctx, q)
	if !trace.Outcome.Started || trace.Outcome.Error != "" || trace.Outcome.Signal != 0 || trace.Outcome.ExitCode < 0 || trace.Outcome.ExitCode > 255 {
		return trace, "", fmt.Errorf("script-trace-no-terminal-result")
	}
	return trace, strconv.Itoa(trace.Outcome.ExitCode), nil
}

// ScriptTraceRecord converts core trace observations to T1's file components,
// pinning every read and executable to the record's immutable commit. It never
// consumes coverage: all native components stay absent with script-kind. Missing
// paths, foreign links, unknown states and caps widen to incomplete, with a
// reason. The caller still owns freshness, qualification, admission and storage.
func ScriptTraceRecord(ctx context.Context, header testevidence.Record, trace proctrace.Result, repo *gitauth.Repository) (testevidence.Record, error) {
	if header.Runner.Plugin != "script" || !strings.HasPrefix(header.Test, "script:") {
		return testevidence.Record{}, fmt.Errorf("script-record-identity-mismatch")
	}
	return fileTraceRecord(ctx, header, trace, repo, scriptNative())
}

// fileTraceRecord shares core trace pinning with shell runners. Native pieces
// remain runner-owned; no plugin-supplied file observations enter this path.
func fileTraceRecord(ctx context.Context, header testevidence.Record, trace proctrace.Result, repo *gitauth.Repository, native testevidence.Native) (testevidence.Record, error) {
	record := header
	record.Executed, record.Regions, record.Loaded = native.Executed, native.Regions, native.Loaded
	tree := ""
	var treeErr error
	if repo == nil {
		treeErr = fmt.Errorf("script-repository-absent")
	} else {
		tree, treeErr = scriptCommitTree(ctx, repo, header.Commit)
	}
	convertPath := func(p proctrace.Path) (testevidence.Path, error) {
		if p.Repository != header.Repository || p.ResolvedRepository != p.Repository {
			return testevidence.Path{}, fmt.Errorf("script-trace-repository-unsupported")
		}
		return testevidence.Path{Repository: p.Repository, Lexical: p.Path, Resolved: p.ResolvedPath}, nil
	}
	pin := func(p proctrace.Path) (testevidence.Blob, error) {
		if treeErr != nil {
			return testevidence.Blob{}, treeErr
		}
		path, err := convertPath(p)
		if err != nil {
			return testevidence.Blob{}, err
		}
		entry, exists, err := repo.LookupTreeEntry(ctx, tree, path.Resolved)
		if err != nil || !exists || entry.Type != "blob" || (entry.Mode != "100644" && entry.Mode != "100755") {
			return testevidence.Blob{}, fmt.Errorf("script-trace-blob-unavailable")
		}
		b := testevidence.Blob{Path: path, OID: entry.OID}
		if path.Lexical != path.Resolved {
			entry, exists, err = repo.LookupTreeEntry(ctx, tree, path.Lexical)
			if err != nil || !exists || entry.Type != "blob" || entry.Mode != "120000" {
				return testevidence.Blob{}, fmt.Errorf("script-trace-link-unavailable")
			}
			b.LexicalOID = entry.OID
		}
		return b, nil
	}
	record.Reads = scriptComponent(trace.Reads, pin)
	record.Listings = scriptComponent(trace.Listings, convertPath)
	record.Lookups = scriptComponent(trace.Lookups, convertPath)
	record.Writes = scriptComponent(trace.Writes, func(w proctrace.Write) (testevidence.Write, error) {
		p, err := convertPath(w.Path)
		return testevidence.Write{Path: p, Kind: testevidence.WriteKind(w.Kind)}, err
	})
	record.Execs = scriptComponent(trace.Execs, func(e proctrace.Exec) (testevidence.Executable, error) {
		b, err := pin(e.Path)
		return testevidence.Executable{Blob: b, Digest: e.Digest, Instrumented: false}, err
	})
	record.External = scriptComponent(trace.External, func(kind string) (testevidence.Contact, error) { return testevidence.Contact{Kind: kind}, nil })
	record.Outside = testevidence.OutsideCounts{}
	for class, count := range trace.Outside {
		if count < 0 {
			record.Reads.State, record.Reads.Reason = testevidence.Incomplete, "script-outside-count-invalid"
			continue
		}
		switch class {
		case "toolchain":
			record.Outside.Toolchain = uint64(count)
		case "cache":
			record.Outside.Cache = uint64(count)
		case "temporary":
			record.Outside.Temporary = uint64(count)
		default:
			record.Outside.Other += uint64(count)
			record.Reads.State, record.Reads.Reason = testevidence.Incomplete, "script-outside-class-unsupported"
		}
	}
	// A refused or malformed backend result cannot become present-empty evidence.
	if treeErr != nil || trace.Backend == "" || trace.Events < 0 || trace.Duration < 0 || !trace.Outcome.Started || trace.Outcome.Error != "" || trace.Outcome.Signal != 0 || trace.Outcome.ExitCode < 0 || trace.Outcome.ExitCode > 255 {
		invalidateScriptTrace(&record, "script-trace-unavailable")
	}
	if err := record.ValidateComponents(); err != nil {
		invalidateScriptTrace(&record, "script-trace-component-malformed")
	}
	if _, err := testevidence.Encode(record); err != nil {
		return testevidence.Record{}, err
	}
	return record, nil
}

func scriptComponent[T, U any](c proctrace.Component[T], convert func(T) (U, error)) testevidence.Component[U] {
	out := testevidence.Component[U]{State: testevidence.State(c.State), Reason: strings.Join(c.Reasons, ";")}
	switch c.State {
	case proctrace.Present:
		if len(c.Reasons) != 0 {
			out.State, out.Reason = testevidence.Incomplete, "script-trace-conflicting-state"
		}
	case proctrace.Absent, proctrace.Incomplete:
		if out.Reason == "" {
			out.State, out.Reason = testevidence.Incomplete, "script-trace-reason-missing"
		}
	default:
		out.State, out.Reason = testevidence.Incomplete, "script-trace-state-unknown"
	}
	if len(c.Entries) > 4096 {
		out.State, out.Reason = testevidence.Incomplete, "script-trace-path-cap"
		return out
	}
	if c.State == proctrace.Absent && len(c.Entries) != 0 {
		out.State, out.Reason = testevidence.Incomplete, "script-trace-conflicting-state"
	}
	for _, item := range c.Entries {
		value, err := convert(item)
		if err != nil {
			out.State, out.Reason = testevidence.Incomplete, err.Error()
			continue
		}
		out.Items = append(out.Items, value)
	}
	return out
}

func invalidateScriptTrace(r *testevidence.Record, reason string) {
	r.Reads = testevidence.Component[testevidence.Blob]{State: testevidence.Incomplete, Reason: reason}
	r.Listings = testevidence.Component[testevidence.Path]{State: testevidence.Incomplete, Reason: reason}
	r.Lookups = testevidence.Component[testevidence.Path]{State: testevidence.Incomplete, Reason: reason}
	r.Writes = testevidence.Component[testevidence.Write]{State: testevidence.Incomplete, Reason: reason}
	r.Execs = testevidence.Component[testevidence.Executable]{State: testevidence.Incomplete, Reason: reason}
	r.External = testevidence.Component[testevidence.Contact]{State: testevidence.Incomplete, Reason: reason}
}
