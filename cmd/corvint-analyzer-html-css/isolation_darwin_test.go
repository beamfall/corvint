//go:build darwin

package main

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBuiltCLIAllChannelDenialSpy(t *testing.T) {
	binary := buildCandidate(t)
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	home := filepath.Join(root, "home")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(root, "spy.log")
	library := filepath.Join(root, "deny.dylib")
	helper := filepath.Join(root, "negative-control")
	goHelper := filepath.Join(root, "go-runtime-positive-control")
	writeSpySource(t, filepath.Join(root, "deny.c"), denialSpySource)
	writeSpySource(t, filepath.Join(root, "negative-control.c"), denialSpyNegativeControl)
	compileSpy(t, "-dynamiclib", "-o", library, filepath.Join(root, "deny.c"))
	compileSpy(t, "-o", helper, filepath.Join(root, "negative-control.c"))
	buildGoRuntimeHelper(t, filepath.Join(root, "go-runtime-positive-control.go"), goHelper)
	for _, directory := range []string{cwd, home} {
		if err := os.WriteFile(filepath.Join(directory, "probe"), []byte("probe"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		_ = listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			_ = connection.Close()
		}
	}()

	env := appendWithout(os.Environ(), "DYLD_INSERT_LIBRARIES=", "DYLD_FORCE_FLAT_NAMESPACE=", "CORVINT_SPY_LOG=", "CORVINT_SPY_CWD=", "CORVINT_SPY_HOME=", "CORVINT_SPY_NETWORK=", "HOME=", "PWD=")
	env = append(env,
		"DYLD_INSERT_LIBRARIES="+library,
		"DYLD_FORCE_FLAT_NAMESPACE=1",
		"CORVINT_SPY_LOG="+logPath,
		"CORVINT_SPY_CWD="+cwd,
		"CORVINT_SPY_HOME="+home,
		"CORVINT_SPY_NETWORK="+listener.Addr().String(),
		"HOME="+home,
		// A matching PWD is what every shell-launched process sees; without it
		// the runtime resolves its working directory at startup via getcwd and
		// a parent-walk, which is launch noise, not a candidate channel.
		"PWD="+cwd,
	)
	negative := exec.Command(helper)
	negative.Dir, negative.Env = cwd, env
	if output, err := negative.CombinedOutput(); err != nil {
		t.Fatalf("negative control: %v\n%s", err, output)
	}
	markers, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"cwd\n", "home\n", "network\n", "process\n", "descriptor\n"} {
		if !bytes.Contains(markers, []byte(marker)) {
			t.Fatalf("spy negative control did not prove %q interception: %q", marker, markers)
		}
	}
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	positive := exec.Command(goHelper)
	positive.Dir, positive.Env = cwd, env
	if output, err := positive.CombinedOutput(); err != nil {
		t.Fatalf("Go-runtime positive control: %v\n%s", err, output)
	}
	markers, err = os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{"cwd\n", "home\n", "network\n", "process\n", "descriptor\n"} {
		if !bytes.Contains(markers, []byte(marker)) {
			t.Fatalf("spy hooks did not observe the Go runtime's %q channel: %q", marker, markers)
		}
	}
	if candidateIsolationClean(markers) {
		t.Fatalf("Go-runtime positive control did not fail the candidate isolation assertion: %q", markers)
	}
	if err := os.WriteFile(logPath, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	candidate := exec.Command(binary)
	candidate.Dir, candidate.Env = cwd, env
	candidate.Stdin = bytes.NewReader(cliFrame(t, `<html><head></head><body><a href="pages/about.html"></a></body></html>`))
	output, err := candidate.Output()
	if err != nil {
		t.Fatalf("denied candidate failed: %v", err)
	}
	if !bytes.Contains(output, []byte(`"status":"CANDIDATE"`)) {
		t.Fatalf("candidate output: %q", output)
	}
	if markers, err = os.ReadFile(logPath); err != nil {
		t.Fatal(err)
	} else if !candidateIsolationClean(markers) {
		t.Fatalf("candidate crossed a denied syscall/process/network/CWD/HOME channel: %q", markers)
	}
}

func candidateIsolationClean(markers []byte) bool { return string(markers) == "loaded\n" }

func writeSpySource(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}
func compileSpy(t *testing.T, args ...string) {
	t.Helper()
	command := exec.Command("cc", args...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compile spy: %v\n%s", err, output)
	}
}

func buildGoRuntimeHelper(t *testing.T, sourcePath, binary string) {
	t.Helper()
	writeSpySource(t, sourcePath, goRuntimePositiveControl)
	command := exec.Command("go", "build", "-o", binary, sourcePath)
	command.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOCACHE="+filepath.Join(t.TempDir(), "gocache"))
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build Go-runtime positive control: %v\n%s", err, output)
	}
}
func appendWithout(environment []string, prefixes ...string) []string {
	filtered := make([]string, 0, len(environment))
	for _, value := range environment {
		keep := true
		for _, prefix := range prefixes {
			if strings.HasPrefix(value, prefix) {
				keep = false
				break
			}
		}
		if keep {
			filtered = append(filtered, value)
		}
	}
	return filtered
}

const denialSpySource = `
#include <dlfcn.h>
#include <errno.h>
#include <fcntl.h>
#include <limits.h>
#include <spawn.h>
#include <stdarg.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <unistd.h>

/* dyld __interpose rebinds every image's imports (except this library's own),
   so the Go runtime's libc trampolines are intercepted; flat-namespace symbol
   shadowing alone does not reach them under dyld4. */
#define INTERPOSE(replacement, original) \
  __attribute__((used)) static struct { const void *replacement_pointer; const void *original_pointer; } \
  interpose_##original __attribute__((section("__DATA,__interpose"))) = { (const void *)(replacement), (const void *)(original) };

static int logfd = -2;
static void note(const char *marker) {
  int saved = errno;
  if (logfd == -2) {
    const char *path = getenv("CORVINT_SPY_LOG");
    logfd = path ? open(path, O_WRONLY | O_CREAT | O_APPEND, 0600) : -1;
  }
  if (logfd >= 0) write(logfd, marker, strlen(marker));
  errno = saved;
}
__attribute__((constructor)) static void loaded_marker(void) { note("loaded\n"); }
static int starts(const char *path, const char *prefix) {
  if (!prefix) return 0;
  size_t n = strlen(prefix);
  return strncmp(path, prefix, n) == 0 && (path[n] == 0 || path[n] == '/');
}
static int blocked(const char *path) {
  if (!path) return 0;
  if (path[0] != '/') { note("cwd\n"); return 1; }
  if (starts(path, getenv("CORVINT_SPY_CWD"))) { note("cwd\n"); return 1; }
  if (starts(path, getenv("CORVINT_SPY_HOME"))) { note("home\n"); return 1; }
  return 0;
}
static int my_open(const char *path, int flags, ...) {
  if (blocked(path)) { errno = EACCES; return -1; }
  if (flags & O_CREAT) { va_list ap; va_start(ap, flags); mode_t mode = va_arg(ap, int); va_end(ap); return open(path, flags, mode); }
  return open(path, flags);
}
static int my_openat(int dirfd, const char *path, int flags, ...) {
  if (blocked(path)) { errno = EACCES; return -1; }
  if (flags & O_CREAT) { va_list ap; va_start(ap, flags); mode_t mode = va_arg(ap, int); va_end(ap); return openat(dirfd, path, flags, mode); }
  return openat(dirfd, path, flags);
}
static int my_connect(int fd, const struct sockaddr *addr, socklen_t size) { note("network\n"); errno = EPERM; return -1; }
static int my_socket(int domain, int type, int protocol) { note("network\n"); errno = EPERM; return -1; }
static ssize_t my_sendto(int fd, const void *buf, size_t n, int flags, const struct sockaddr *addr, socklen_t size) { note("network\n"); errno = EPERM; return -1; }
static int my_execve(const char *path, char *const argv[], char *const envp[]) { note("process\n"); errno = EPERM; return -1; }
static int my_posix_spawn(pid_t *pid, const char *path, const posix_spawn_file_actions_t *fa, const posix_spawnattr_t *attr, char *const argv[], char *const envp[]) { note("process\n"); return EPERM; }
static char *my_getcwd(char *buf, size_t size) { note("cwd\n"); errno = EACCES; return NULL; }
/* Descriptor plumbing is observed and passed through: denying pipe would make
   the Go runtime's fork/exec status pipe fail before execve can be witnessed. */
static int my_pipe(int fds[2]) { note("descriptor\n"); return pipe(fds); }
static int my_dup(int fd) { note("descriptor\n"); return dup(fd); }
INTERPOSE(my_open, open)
INTERPOSE(my_openat, openat)
INTERPOSE(my_connect, connect)
INTERPOSE(my_socket, socket)
INTERPOSE(my_sendto, sendto)
INTERPOSE(my_execve, execve)
INTERPOSE(my_posix_spawn, posix_spawn)
INTERPOSE(my_getcwd, getcwd)
INTERPOSE(my_pipe, pipe)
INTERPOSE(my_dup, dup)
void corvint_spy_negative(void) {
  char buffer[PATH_MAX];
  my_getcwd(buffer, sizeof(buffer));
  my_open("cwd-probe", O_RDONLY);
  const char *home = getenv("HOME");
  if (home) my_open(home, O_RDONLY);
  my_socket(AF_INET, SOCK_STREAM, 0);
  char *const argv[] = {"/bin/true", NULL};
  my_execve("/bin/true", argv, NULL);
  int fds[2];
  if (my_pipe(fds) == 0) { close(fds[0]); close(fds[1]); }
  int duplicate = my_dup(0);
  if (duplicate >= 0) close(duplicate);
}
`

const denialSpyNegativeControl = `
#include <dlfcn.h>
int main(void) {
  void (*negative)(void) = dlsym(RTLD_DEFAULT, "corvint_spy_negative");
  if (!negative) return 2;
  negative();
  return 0;
}
`

// The helper never touches the spy log: every marker in the log after this
// process runs was written by the spy's own libc hooks. The helper only
// performs the operations and proves each injected denial actually landed.
const goRuntimePositiveControl = `
package main

import (
  "net"
  "os"
  "os/exec"
  "path/filepath"
  "syscall"
  "time"
)

func require(ok bool, code int) {
  if !ok { os.Exit(code) }
}

func main() {
  first, err := os.Open(filepath.Join(os.Getenv("CORVINT_SPY_CWD"), "probe"))
  if first != nil { _ = first.Close() }
  require(err != nil, 10)
  second, err := os.Open(filepath.Join(os.Getenv("HOME"), "probe"))
  if second != nil { _ = second.Close() }
  require(err != nil, 11)
  connection, err := net.DialTimeout("tcp", os.Getenv("CORVINT_SPY_NETWORK"), 200*time.Millisecond)
  if connection != nil { _ = connection.Close() }
  require(err != nil, 12)
  require(exec.Command("/usr/bin/true").Run() != nil, 13)
  reader, writer, err := os.Pipe()
  require(err == nil, 14)
  _ = writer.Close()
  duplicate, err := syscall.Dup(int(reader.Fd()))
  require(err == nil, 15)
  _ = syscall.Close(duplicate)
  _ = reader.Close()
}
`
