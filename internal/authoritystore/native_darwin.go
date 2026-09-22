//go:build darwin

package authoritystore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

type processIdentity struct {
	PID, Parent          uint32
	Started, StartedUsec uint64
	Path                 string
}

func verifyRuntime(ctx context.Context, root RootDocument) error {
	if root.Profile == DirectRootProfile {
		if root.HostQualification != nil || !root.DirectQualification.valid() {
			return errUnavailable
		}
		return verifyDirectRuntime(ctx, root, root.DirectQualification.Runtime)
	}
	if root.Profile != RootProfile || root.DirectQualification != nil {
		return errUnavailable
	}
	q := root.HostQualification
	if q == nil || q.Profile != "corvint-native-qualified-host/0" || q.Surface != "codex-desktop" || !hexDigest(q.EvidenceSHA256) {
		return errUnavailable
	}
	if q.Topology == "shared-daemon" && !validSharedQualification(q) {
		return errUnavailable
	}
	return verifyRuntimePins(ctx, root, qualifiedPins(q))
}

func verifyRuntimePins(ctx context.Context, root RootDocument, q runtimePins) error {
	if q.Architecture != runtime.GOARCH {
		return errUnavailable
	}
	osBuild, err := syscall.Sysctl("kern.osversion")
	if err != nil || q.OSBuild != osBuild {
		return errUnavailable
	}

	boot, err := syscall.Sysctl("kern.bootsessionuuid")
	if err != nil || boot == "" || boot != q.BootSessionUUID {
		return errUnavailable
	}
	appBefore, err := inspectProcess(q.AppInstance.PID)
	if err != nil || appBefore.Started != q.AppInstance.Started || appBefore.StartedUsec != q.AppInstance.StartedUsec || appBefore.Path != q.App.Path {
		return errUnavailable
	}
	if q.Topology != "app-owned-stdio" && q.Topology != "shared-daemon" {
		return errUnavailable
	}
	if q.Topology == "shared-daemon" && !validControlSocket(q.ControlSocket) {
		return errUnavailable
	}
	if verifyProtectedRelease(ctx, root) != nil {
		return errUnavailable
	}
	// Bind actual kernel-reported parent executable identities, not argv, PATH
	// --version output, a caller PID, or a separately launched CLI observation.
	process, err := inspectProcess(uint32(os.Getpid()))
	if err != nil {
		return errUnavailable
	}
	seenEngine := false
	for depth := 0; depth < 8; depth++ {
		if process.Parent <= 1 {
			return errUnavailable
		}
		before, err := inspectProcess(process.Parent)
		if err != nil {
			return errUnavailable
		}
		image := q.Engine
		cdhash := q.EngineCDHash
		if seenEngine {
			image = q.App
			cdhash = q.AppCDHash
		}
		if before.Path == image.Path {
			if !seenEngine && !q.acceptsEngine(ProcessInstance{PID: before.PID, Started: before.Started, StartedUsec: before.StartedUsec}) {
				return errUnavailable
			}
			err := verifyHostImage(ctx, before.PID, image, cdhash)
			if err != nil {
				return errUnavailable
			}
			after, err := inspectProcess(before.PID)
			if err != nil || before != after {
				return errUnavailable
			}
			if seenEngine {
				if before != appBefore {
					return errUnavailable
				}
				return nil
			}
			if q.Topology == "shared-daemon" {
				if verifyHostImage(ctx, appBefore.PID, q.App, q.AppCDHash) != nil || verifySocketPair(ctx, appBefore.PID, before.PID, q.ControlSocket) != nil {
					return errUnavailable
				}
				appAfter, ea := inspectProcess(appBefore.PID)
				engineAfter, ee := inspectProcess(before.PID)
				if ea != nil || ee != nil || appAfter != appBefore || engineAfter != before {
					return errUnavailable
				}
				return nil
			}
			seenEngine = true
		}
		process = before
	}
	return errUnavailable
}

func verifyImage(ctx context.Context, image Image) (syscall.Stat_t, error) {
	var empty syscall.Stat_t
	if !hexDigest(image.SHA256) {
		return empty, errUnavailable
	}
	file, before, err := openAudited(image.Path, 0, false)
	if err != nil {
		return empty, errUnavailable
	}
	defer file.Close()
	if before.Size <= 0 || before.Size > 256<<20 {
		return empty, errUnavailable
	}
	sum := sha256.New()
	buffer := make([]byte, 64<<10)
	for {
		if ctx.Err() != nil {
			return empty, errUnavailable
		}
		n, readErr := file.Read(buffer)
		if n > 0 {
			sum.Write(buffer[:n])
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return empty, errUnavailable
		}
	}
	var after syscall.Stat_t
	if syscall.Fstat(int(file.Fd()), &after) != nil || !sameStat(before, after) {
		return empty, errUnavailable
	}
	if hex.EncodeToString(sum.Sum(nil)) != image.SHA256 {
		return empty, errUnavailable
	}
	return before, nil
}

// proc_info returns metadata only. No process arguments, environment, memory,
// transcript, task identifier or private app database is read or retained.
func inspectProcess(pid uint32) (processIdentity, error) {
	var identity processIdentity
	bsd := make([]byte, 136) // sizeof(proc_bsdinfo), current Darwin 64-bit ABI.
	if err := procInfo(pid, 3, bsd); err != nil {
		return identity, errUnavailable
	}
	identity.PID = binary.LittleEndian.Uint32(bsd[12:16])
	identity.Parent = binary.LittleEndian.Uint32(bsd[16:20])
	identity.Started = binary.LittleEndian.Uint64(bsd[120:128])
	identity.StartedUsec = binary.LittleEndian.Uint64(bsd[128:136])
	if identity.PID != pid || identity.Started == 0 {
		return identity, errUnavailable
	}
	path := make([]byte, 4096)
	if err := procInfo(pid, 11, path); err != nil {
		return identity, errUnavailable
	}
	end := bytes.IndexByte(path, 0)
	if end <= 0 {
		return identity, errUnavailable
	}
	identity.Path = string(path[:end])
	if !filepath.IsAbs(identity.Path) || filepath.Clean(identity.Path) != identity.Path {
		return identity, errUnavailable
	}
	return identity, nil
}

func procInfo(pid, flavor uint32, buffer []byte) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_PROC_INFO, 2, uintptr(pid), uintptr(flavor), 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))
	if errno != 0 {
		return errUnavailable
	}
	return nil
}

// Live csops status and CDHash bind the mapped signed executable, independently
// of a claimant-writable pathname. Full bundle/resource/fuse audit belongs
// to independent admission of the exact fresh AppInstance and boot session. This is native host provenance, never execution root admission.
// Source: Apple XNU bsd/sys/codesign.h and osfmk/kern/cs_blobs.h; read ops 0/5.
func verifyHostImage(ctx context.Context, pid uint32, image Image, expectedCDHash string) error {
	if len(expectedCDHash) != 40 || strings.ToLower(expectedCDHash) != expectedCDHash {
		return errUnavailable
	}
	if _, err := hex.DecodeString(expectedCDHash); err != nil {
		return errUnavailable
	}
	if !hexDigest(image.SHA256) || !filepath.IsAbs(image.Path) || filepath.Clean(image.Path) != image.Path {
		return errUnavailable
	}
	before, err := codeDirectoryHash(pid)
	if err != nil || before != expectedCDHash {
		return errUnavailable
	}
	if ctx.Err() != nil {
		return errUnavailable
	}
	after, err := codeDirectoryHash(pid)
	if err != nil || after != before {
		return errUnavailable
	}
	return nil
}

func codeDirectoryHash(pid uint32) (string, error) {
	status := make([]byte, 4)
	if csopsRead(pid, 0, status) != nil {
		return "", errUnavailable
	}
	flags := binary.LittleEndian.Uint32(status)
	// CS_VALID required; CS_DEBUGGED refuses invalid-page debugging provenance.
	if flags&0x00000001 == 0 || flags&0x10000000 != 0 {
		return "", errUnavailable
	}
	digest := make([]byte, 20)
	if csopsRead(pid, 5, digest) != nil {
		return "", errUnavailable
	}
	return hex.EncodeToString(digest), nil
}

func csopsRead(pid, operation uint32, buffer []byte) error {
	_, _, errno := syscall.Syscall6(syscall.SYS_CSOPS, uintptr(pid), uintptr(operation), uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)), 0, 0)
	if errno != 0 {
		return errUnavailable
	}
	return nil
}

func validSharedQualification(q *HostQualification) bool {
	if !validControlSocket(q.ControlSocket) || len(q.QualifiedSurfaces) < 1 || len(q.QualifiedSurfaces) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, s := range q.QualifiedSurfaces {
		if (s.Surface != "codex-desktop" && s.Surface != "codex-cli") || seen[s.Surface] || !hexDigest(s.EvidenceSHA256) {
			return false
		}
		seen[s.Surface] = true
	}
	return seen["codex-desktop"] && seen["codex-cli"]
}

func verifyProtectedRelease(ctx context.Context, root RootDocument) error {
	release := filepath.Dir(root.Consumer.Path)
	if !strings.HasPrefix(release, RootPath+"/versions/") || !hexDigest(filepath.Base(release)) {
		return errUnavailable
	}
	if root.Consumer.Path != release+"/corvint" || root.Adapter.Path != release+"/authority-hook.json" || root.Git.Path != release+"/bin/git" {
		return errUnavailable
	}
	executable, err := os.Executable()
	if err != nil || executable != root.Consumer.Path || os.Getenv("PATH") != release+"/bin" {
		return errUnavailable
	}
	for _, image := range []Image{root.Consumer, root.Adapter, root.Git} {
		if _, err = verifyImage(ctx, image); err != nil {
			return errUnavailable
		}
	}

	return nil
}
