package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Beamfall/corvint/internal/cem/convert"
	"github.com/Beamfall/corvint/internal/cem/wire"
)

// runConvert is `corvint cem convert` (CEM-V1-102): it converts one cem/0.x
// map to cem/1.0 and prints the closed convert report. Exit 0 converted, 1 the
// map was refused, 2 an argument, input or output failure.
func runConvert(ctx context.Context, args []string, out io.Writer) int {
	values := map[string]string{}
	inPlace, bad := false, false
	for i := 0; i < len(args); i++ {
		key := args[i]
		if key == "--in-place" && !inPlace {
			inPlace = true
			continue
		}
		if key != "--map" && key != "--output" && key != "--bind-commit" && key != "--repository" {
			bad = true
			break
		}
		if _, ok := values[key]; ok || i+1 >= len(args) || strings.HasPrefix(args[i+1], "--") {
			bad = true
			break
		}
		i++
		values[key] = args[i]
	}
	_, hasOutput := values["--output"]
	source, repository := values["--map"], values["--repository"]
	absolute := func(p string) bool { return filepath.IsAbs(p) && filepath.Clean(p) == p }
	if bad || !absolute(source) || inPlace == hasOutput || hasOutput && !absolute(values["--output"]) || repository != "" && !absolute(repository) {
		return emitConvert(out, convertFailure("invalid-arguments"), 2)
	}
	bind, explicit := values["--bind-commit"]
	if !explicit {
		bind = convert.BindCommitFromName(source)
	}
	raw, err := readConvertSource(source)
	if err != nil {
		return emitConvert(out, convertFailure("map-unavailable"), 2)
	}
	in := convert.Inputs{BindCommit: bind}
	if repository != "" {
		in.Git = convert.NewGitReader(ctx, repository)
	}
	converted, report, err := convert.Convert(raw, in)
	if err != nil {
		return emitConvert(out, report, 1)
	}
	target := values["--output"]
	if inPlace {
		target = source
	}
	if err := publishConverted(target, converted, inPlace); err != nil {
		code := "publish-failed"
		report.Status, report.Code, report.Provenance, report.OutputSha256 = "REFUSED", &code, nil, nil
		return emitConvert(out, report, 2)
	}
	return emitConvert(out, report, 0)
}

func convertFailure(code string) convert.Report {
	return convert.Report{Profile: convert.ReportProfile, Status: "REFUSED", Code: &code, Converter: convert.Converter, SourceBinding: convert.BindingNotChecked}
}

func emitConvert(out io.Writer, report convert.Report, exit int) int {
	if json.NewEncoder(out).Encode(report) != nil {
		return 2
	}
	return exit
}

// readConvertSource reads one bounded regular file without following a final
// symbolic link, as the stable verifier reads its map.
func readConvertSource(path string) ([]byte, error) {
	f, err := openStableMap(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > wire.MaxMapBytes {
		return nil, os.ErrInvalid
	}
	raw, err := io.ReadAll(io.LimitReader(f, wire.MaxMapBytes+1))
	if err != nil || len(raw) > wire.MaxMapBytes {
		return nil, os.ErrInvalid
	}
	return raw, nil
}

// publishConverted writes through a sibling temporary file and one atomic
// publish step. A new --output never replaces an existing file: it is linked
// into place, which the filesystem refuses when the name exists, so no check
// and rename can race. --in-place replaces the source through one rename.
func publishConverted(target string, data []byte, replace bool) error {
	if !replace {
		if _, err := os.Lstat(target); err == nil || !os.IsNotExist(err) {
			return os.ErrExist
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(target), ".cem-convert-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, err = tmp.Write(data)
	if e := tmp.Chmod(0o644); err == nil {
		err = e
	}
	if e := tmp.Close(); err == nil {
		err = e
	}
	if err == nil {
		if replace {
			err = os.Rename(name, target)
		} else if err = os.Link(name, target); errors.Is(err, fs.ErrExist) {
			err = os.ErrExist
		}
	}
	if err != nil || !replace {
		os.Remove(name)
	}
	return err
}
