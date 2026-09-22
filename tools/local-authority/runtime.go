package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
	"sort"
	"time"
)

var admittedImports = []string{"args_get", "args_sizes_get", "clock_time_get", "environ_get", "environ_sizes_get", "fd_close", "fd_fdstat_get", "fd_fdstat_set_flags", "fd_prestat_dir_name", "fd_prestat_get", "fd_read", "fd_write", "fd_write", "poll_oneoff", "proc_exit", "random_get", "sched_yield"}

func structuralCheck(wasm []byte) error {
	if len(wasm) < 8 || len(wasm) > maxModule {
		return errors.New("module size")
	}
	if !bytes.Equal(wasm[:8], []byte{0, 97, 115, 109, 1, 0, 0, 0}) {
		return errors.New("wasm header")
	}
	pos := 8
	sections := 0
	custom := 0
	for pos < len(wasm) {
		id := wasm[pos]
		pos++
		size, n := binary.Uvarint(wasm[pos:])
		if n <= 0 || n > 5 {
			return errors.New("section length")
		}
		pos += n
		if size > uint64(len(wasm)-pos) {
			return errors.New("truncated section")
		}
		sections++
		if sections > 64 {
			return errors.New("section count")
		}
		if id == 0 {
			custom += int(size)
			if custom > 1<<20 {
				return errors.New("custom section bound")
			}
		}
		if id > 12 {
			return errors.New("section id")
		}
		if id == 4 {
			if e := tableBound(wasm[pos : pos+int(size)]); e != nil {
				return e
			}
		}
		if id == 4 && size > 65536 {
			return errors.New("table section bound")
		}
		pos += int(size)
	}
	return nil
}
func execute(wasm, input []byte) ([]byte, error) { return executeProfile(wasm, input, true) }
func executeProfile(wasm, input []byte, exact bool) ([]byte, error) {
	if e := structuralCheck(wasm); e != nil {
		return nil, e
	}
	if len(input) > 65536 {
		return nil, errors.New("input bound")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4500*time.Millisecond)
	defer cancel()
	runtime := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfigInterpreter().WithCoreFeatures(api.CoreFeaturesV2).WithMemoryLimitPages(8192).WithCloseOnContextDone(true))
	defer runtime.Close(context.Background())
	compileCtx, cc := context.WithTimeout(ctx, 2*time.Second)
	compiled, e := runtime.CompileModule(compileCtx, wasm)
	cc()
	if e != nil {
		return nil, fmt.Errorf("compile: %w", e)
	}
	defer compiled.Close(context.Background())
	if len(compiled.ImportedMemories()) != 0 {
		return nil, errors.New("imported memory")
	}
	names := []string{}
	for _, f := range compiled.ImportedFunctions() {
		mod, name, _ := f.Import()
		if mod != "wasi_snapshot_preview1" {
			return nil, errors.New("foreign import")
		}
		names = append(names, name)
	}
	sort.Strings(names)
	expected := append([]string(nil), admittedImports...)
	sort.Strings(expected)
	for _, name := range names {
		idx := sort.SearchStrings(expected, name)
		if idx == len(expected) || expected[idx] != name {
			return nil, fmt.Errorf("forbidden import: %s", name)
		}
	}
	if exact && fmt.Sprint(names) != fmt.Sprint(expected) {
		return nil, fmt.Errorf("import set mismatch: %v", names)
	}
	if _, e = wasi_snapshot_preview1.Instantiate(ctx, runtime); e != nil {
		return nil, e
	}
	var stdout, stderr limitedBuffer
	stdout.max = maxOutput
	stderr.max = maxOutput
	// Default fake clocks/randomness; no filesystem, environment, arguments or real sleep.
	cfg := wazero.NewModuleConfig().WithStdin(bytes.NewReader(input)).WithStdout(&stdout).WithStderr(&stderr).WithStartFunctions()
	startCtx, sc := context.WithTimeout(ctx, time.Second)
	instance, e := runtime.InstantiateModule(startCtx, compiled, cfg)
	sc()
	if e != nil {
		return nil, fmt.Errorf("instantiate/start: %w", e)
	}
	fn := instance.ExportedFunction("_start")
	if fn == nil {
		return nil, errors.New("missing _start")
	}
	callCtx, fc := context.WithTimeout(ctx, 1500*time.Millisecond)
	_, e = fn.Call(callCtx)
	fc()
	var exit *sys.ExitError
	if errors.As(e, &exit) && exit.ExitCode() == 0 {
		e = nil
	}
	if e != nil {
		return nil, fmt.Errorf("function: %w", e)
	}
	if stdout.overflow || stderr.overflow {
		return nil, errors.New("guest output overflow")
	}
	if stderr.Len() != 0 {
		return nil, errors.New("guest stderr")
	}
	return stdout.Bytes(), nil
}

func tableBound(raw []byte) error {
	read := func() (uint64, error) {
		v, n := binary.Uvarint(raw)
		if n <= 0 || n > 5 {
			return 0, errors.New("table integer")
		}
		raw = raw[n:]
		return v, nil
	}
	count, e := read()
	if e != nil || count > 1 {
		return errors.New("table count")
	}
	for i := uint64(0); i < count; i++ {
		if len(raw) == 0 || raw[0] != 0x70 {
			return errors.New("table reference type")
		}
		raw = raw[1:]
		flags, e := read()
		if e != nil || flags > 1 {
			return errors.New("table flags")
		}
		minimum, e := read()
		if e != nil || minimum > 65536 {
			return errors.New("table element bound")
		}
		if flags == 1 {
			maximum, e := read()
			if e != nil || maximum > 65536 || maximum < minimum {
				return errors.New("table maximum")
			}
		}
	}
	if len(raw) != 0 {
		return errors.New("table trailing bytes")
	}
	return nil
}
