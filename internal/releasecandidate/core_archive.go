package releasecandidate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"debug/buildinfo"
	"fmt"
	"io"
	"path"
	"strings"
)

type coreMember struct {
	mode int64
	data []byte
}

var verifyCoreBinary = verifyCoreArchiveBinary

func verifyCoreArchiveBinary(raw []byte, target coreTarget) ([]byte, error) {
	members, err := readCoreMembers(raw, target)
	if err != nil {
		return nil, err
	}
	root := strings.TrimSuffix(strings.TrimSuffix(target.ArchiveName, ".gz"), ".tar")
	if target.GOOS == "windows" {
		root = strings.TrimSuffix(target.ArchiveName, ".zip")
	}
	want := []string{target.BinaryName, "SHA256SUMS", "LICENSE", "LICENSE-APACHE-2.0", "LICENSING.md", "PROVENANCE.md"}
	if len(members) != len(want) {
		return nil, fmt.Errorf("core archive %s has %d members, expected %d", target.ArchiveName, len(members), len(want))
	}
	for _, name := range want {
		member, present := members[path.Join(root, name)]
		mode := int64(0o644)
		if name == target.BinaryName {
			mode = 0o755
		}
		if !present || member.mode != mode {
			return nil, fmt.Errorf("core archive %s member %s is missing or has wrong mode", target.ArchiveName, name)
		}
	}
	binary := members[path.Join(root, target.BinaryName)].data
	if digest(binary) != target.RetainedBinary.SHA256 || int64(len(binary)) != target.RetainedBinary.Bytes {
		return nil, fmt.Errorf("core archive %s binary disagrees with report", target.ArchiveName)
	}
	wantSum := digest(binary) + "  " + target.BinaryName + "\n"
	if string(members[path.Join(root, "SHA256SUMS")].data) != wantSum {
		return nil, fmt.Errorf("core archive %s internal checksum disagrees", target.ArchiveName)
	}
	info, err := buildinfo.Read(bytes.NewReader(binary))
	if err != nil || info.GoVersion != "go1.27.1" || info.Path != "github.com/Beamfall/corvint/cmd/corvint" {
		return nil, fmt.Errorf("core archive %s build identity disagrees", target.ArchiveName)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["GOOS"] != target.GOOS || settings["GOARCH"] != target.GOARCH || settings["CGO_ENABLED"] != "0" {
		return nil, fmt.Errorf("core archive %s target build identity disagrees", target.ArchiveName)
	}
	return binary, nil
}

func readCoreMembers(raw []byte, target coreTarget) (map[string]coreMember, error) {
	members := map[string]coreMember{}
	add := func(name string, mode int64, data []byte) error {
		if name == "" || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") {
			return fmt.Errorf("core archive %s has unsafe member", target.ArchiveName)
		}
		if _, exists := members[name]; exists {
			return fmt.Errorf("core archive %s has duplicate member", target.ArchiveName)
		}
		members[name] = coreMember{mode: mode, data: append([]byte(nil), data...)}
		return nil
	}
	if target.GOOS == "windows" {
		reader, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
		if err != nil {
			return nil, err
		}
		for _, file := range reader.File {
			if len(members) == 6 || !file.Mode().IsRegular() || file.UncompressedSize64 > 64<<20 {
				return nil, fmt.Errorf("core archive %s inventory is invalid", target.ArchiveName)
			}
			handle, err := file.Open()
			if err != nil {
				return nil, err
			}
			data, readErr := io.ReadAll(io.LimitReader(handle, 64<<20+1))
			closeErr := handle.Close()
			if readErr != nil || closeErr != nil || len(data) > 64<<20 {
				return nil, fmt.Errorf("core archive %s member read failed", target.ArchiveName)
			}
			if err := add(file.Name, int64(file.Mode().Perm()), data); err != nil {
				return nil, err
			}
		}
		return members, nil
	}
	gzipReader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()
	reader := tar.NewReader(gzipReader)
	for {
		header, nextErr := reader.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, nextErr
		}
		if len(members) == 6 || (header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA) || header.Size < 0 || header.Size > 64<<20 {
			return nil, fmt.Errorf("core archive %s inventory is invalid", target.ArchiveName)
		}
		data, err := io.ReadAll(io.LimitReader(reader, 64<<20+1))
		if err != nil || int64(len(data)) != header.Size {
			return nil, fmt.Errorf("core archive %s member read failed", target.ArchiveName)
		}
		if err := add(header.Name, header.Mode, data); err != nil {
			return nil, err
		}
	}
	return members, nil
}
