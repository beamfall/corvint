//go:build darwin

package main

import (
	"bytes"
	json "encoding/json/v2"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestProductionAttemptObservingSpy preloads a closed observer into the built
// production executable. Positive controls cover multiple caller paths,
// environment names, process forms, and network forms before the candidate is
// required to leave every observer channel empty.
func TestProductionAttemptObservingSpy(t *testing.T) {
	root := t.TempDir()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{}, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			conn.Close()
			accepted <- struct{}{}
		}
	}()
	repository := filepath.Join(root, "repository-decoy")
	if err := os.WriteFile(repository, []byte("caller bytes only"), 0600); err != nil {
		t.Fatal(err)
	}
	spy, probe := buildAttemptObserver(t, root)
	env := append(os.Environ(),
		"DYLD_INSERT_LIBRARIES="+spy,
		"DYLD_FORCE_FLAT_NAMESPACE=1",
		"CORVINT_SPY_FD=3",
		"CORVINT_SPY_ROOT="+root,
		"CORVINT_SPY_REPOSITORY="+repository,
		"CORVINT_SPY_NETWORK="+listener.Addr().String(),
		"CORVINT_SPY_PROCESS=/usr/bin/true",
		"CORVINT_SPY_PROCESS_ALT=/usr/bin/false",
		"CORVINT_PRODUCTION_ENV_CANARY_A=must-not-be-read",
		"CORVINT_PRODUCTION_ENV_CANARY_B=must-not-be-read",
		"CORVINT_PRODUCTION_ENV_CANARY_C=must-not-be-read",
	)
	negativeRead, negativeWrite := attemptPipe(t)
	negative := exec.Command(probe, repository, listener.Addr().String(), "/usr/bin/true")
	negative.Env = append(env, "CORVINT_SPY_DIAGNOSTIC=1")
	negative.ExtraFiles = []*os.File{negativeWrite}
	output, negativeErr := negative.CombinedOutput()
	if negativeErr != nil && len(output) > 0 {
		t.Logf("negative control exited %v: %s", negativeErr, output)
	}
	if err := negativeWrite.Close(); err != nil {
		t.Fatal(err)
	}
	gotAttempts, err := io.ReadAll(negativeRead)
	if err != nil || !containsAttemptChannels(string(gotAttempts), "LRCWNUEGKPFMT") {
		t.Fatalf("attempt observer channels=%q err=%v", gotAttempts, err)
	}
	if err := negativeRead.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-accepted:
	default:
		t.Fatal("network negative control did not reach the listener")
	}
	raw := []byte(compiledCLIFrozenSuccessRequest)
	want := []byte(compiledCLIFrozenSuccessResponse)
	cmd := exec.Command(buildCompiledCLI(t))
	cmd.Dir = root
	cmd.Env = env
	cmd.Stdin = bytes.NewReader(raw)
	productionRead, productionWrite := attemptPipe(t)
	cmd.ExtraFiles = []*os.File{productionWrite}
	got, err := cmd.CombinedOutput()
	if closeErr := productionWrite.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("production attempt spy err=%v want=%q got=%q", err, want, got)
	}
	productionAttempts, readErr := io.ReadAll(productionRead)
	if closeErr := productionRead.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if readErr != nil || len(productionAttempts) != 0 {
		t.Fatalf("production attempted prohibited capability: %q err=%v", productionAttempts, readErr)
	}
}

// TestProductionGoRuntimeGetenvTrap overlays os.Getenv and os.LookupEnv in a
// private compiler invocation. The positive control proves that this catches
// Go runtime environment reads, which a libc getenv interposer cannot observe.
func TestProductionGoRuntimeGetenvTrap(t *testing.T) {
	root := t.TempDir()
	overlay := getenvTrapOverlay(t, root)
	probe := buildRuntimeGetenvProbe(t, root, overlay)
	control := exec.Command(probe)
	control.Env = []string{"CORVINT_PRODUCTION_ENV_CANARY_RUNTIME=visible"}
	output, err := control.CombinedOutput()
	if err == nil || !bytes.Contains(output, []byte("CORVINT_ANALYZER_RUNTIME_GETENV")) {
		t.Fatalf("Go runtime getenv positive control err=%v output=%q", err, output)
	}

	binary := buildCompiledCLIWithOptions(t, "-overlay="+overlay)
	production := exec.Command(binary)
	production.Env = []string{"CORVINT_PRODUCTION_ENV_CANARY_RUNTIME=visible"}
	production.Stdin = bytes.NewReader([]byte(compiledCLIFrozenSuccessRequest))
	got, err := production.CombinedOutput()
	if err != nil || !bytes.Equal(got, []byte(compiledCLIFrozenSuccessResponse)) {
		t.Fatalf("production Go runtime getenv trap err=%v want=%q got=%q", err, compiledCLIFrozenSuccessResponse, got)
	}
}

func getenvTrapOverlay(t testing.TB, root string) string {
	t.Helper()
	goRoot := filepath.Dir(filepath.Dir(pinnedGoTool))
	originalPath := filepath.Join(goRoot, "src", "os", "env.go")
	source, err := os.ReadFile(originalPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, function := range []struct{ original, replacement string }{
		{
			"func Getenv(key string) string {\n\ttestlog.Getenv(key)\n\tv, _ := syscall.Getenv(key)\n\treturn v\n}",
			"func Getenv(key string) string {\n\ttestlog.Getenv(key)\n\tif key == \"CORVINT_PRODUCTION_ENV_CANARY_RUNTIME\" { panic(\"CORVINT_ANALYZER_RUNTIME_GETENV\") }\n\tv, _ := syscall.Getenv(key)\n\treturn v\n}",
		},
		{
			"func LookupEnv(key string) (string, bool) {\n\ttestlog.Getenv(key)\n\treturn syscall.Getenv(key)\n}",
			"func LookupEnv(key string) (string, bool) {\n\ttestlog.Getenv(key)\n\tif key == \"CORVINT_PRODUCTION_ENV_CANARY_RUNTIME\" { panic(\"CORVINT_ANALYZER_RUNTIME_GETENV\") }\n\treturn syscall.Getenv(key)\n}",
		},
	} {
		if !bytes.Contains(source, []byte(function.original)) {
			t.Fatal("pinned Go environment implementation drifted")
		}
		source = bytes.Replace(source, []byte(function.original), []byte(function.replacement), 1)
	}
	replacementPath := filepath.Join(root, "os-env-trap.go")
	if err := os.WriteFile(replacementPath, source, 0600); err != nil {
		t.Fatal(err)
	}
	overlay, err := json.Marshal(map[string]map[string]string{"Replace": {originalPath: replacementPath}})
	if err != nil {
		t.Fatal(err)
	}
	overlayPath := filepath.Join(root, "runtime-getenv-overlay.json")
	if err := os.WriteFile(overlayPath, overlay, 0600); err != nil {
		t.Fatal(err)
	}
	return overlayPath
}

func buildRuntimeGetenvProbe(t testing.TB, root, overlay string) string {
	t.Helper()
	directory := filepath.Join(root, "runtime-getenv-probe")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module runtimegetenvprobe\ngo 1.27.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "main.go"), []byte("package main\nimport \"os\"\nfunc main() { _ = os.Getenv(\"CORVINT_PRODUCTION_ENV_CANARY_RUNTIME\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "runtime-getenv-probe-bin")
	tool, env := pinnedGoBuildEnvironment(t, filepath.Join(root, "runtime-getenv-build"))
	command := exec.Command(tool, "build", "-trimpath", "-buildvcs=false", "-overlay="+overlay, "-o", binary, ".")
	command.Dir = directory
	command.Env = env
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build Go runtime getenv positive control: %v\n%s", err, output)
	}
	return binary
}

func attemptPipe(t testing.TB) (*os.File, *os.File) {
	t.Helper()
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	return read, write
}

func containsAttemptChannels(got, want string) bool {
	for _, channel := range want {
		if !strings.ContainsRune(got, channel) {
			return false
		}
	}
	return true
}

func buildAttemptObserver(t testing.TB, root string) (string, string) {
	t.Helper()
	spySource := filepath.Join(root, "attempt_spy.c")
	probeSource := filepath.Join(root, "attempt_probe.c")
	spy := filepath.Join(root, "attempt_spy.dylib")
	probe := filepath.Join(root, "attempt_probe")
	if err := os.WriteFile(spySource, []byte(`
#include <arpa/inet.h>
#include <dlfcn.h>
#include <fcntl.h>
#include <netdb.h>
#include <netinet/in.h>
#include <spawn.h>
#include <stdio.h>
#include <stdarg.h>
#include <sys/stat.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/wait.h>
#include <unistd.h>

static char *(*next_getenv)(const char *);
static int (*next_setenv)(const char *, const char *, int);
static int (*next_unsetenv)(const char *);
static int (*next_open)(const char *, int, ...);
static int (*next_openat)(int, const char *, int, ...);
static int (*next_creat)(const char *, mode_t);
static FILE *(*next_fopen)(const char *, const char *);
static int (*next_access)(const char *, int);
static int (*next_stat)(const char *, struct stat *);
static int (*next_rename)(const char *, const char *);
static int (*next_renameat)(int, const char *, int, const char *);
static int (*next_unlink)(const char *);
static int (*next_unlinkat)(int, const char *, int);
static int (*next_mkdir)(const char *, mode_t);
static int (*next_mkdirat)(int, const char *, mode_t);
static int (*next_rmdir)(const char *);
static int (*next_chmod)(const char *, mode_t);
static int (*next_truncate)(const char *, off_t);
static int (*next_symlink)(const char *, const char *);
static int (*next_link)(const char *, const char *);
static ssize_t (*next_write)(int, const void *, size_t);
static int (*next_getaddrinfo)(const char *, const char *, const struct addrinfo *, struct addrinfo **);
static int (*next_socket)(int, int, int);
static int (*next_connect)(int, const struct sockaddr *, socklen_t);
static ssize_t (*next_sendto)(int, const void *, size_t, int, const struct sockaddr *, socklen_t);
static int (*next_execve)(const char *, char *const [], char *const []);
static pid_t (*next_fork)(void);
static int (*next_posix_spawn)(pid_t *, const char *, const posix_spawn_file_actions_t *, const posix_spawnattr_t *, char *const [], char *const []);
static int spy_fd = -2;
static int watched_fd = -1;
extern char **environ;
static const char *env(const char *name) {
  size_t n = strlen(name);
  for (char **entry = environ; entry && *entry; entry++) if (!strncmp(*entry, name, n) && (*entry)[n] == '=') return *entry + n + 1;
  return 0;
}
static void mark(char channel) {
  if (spy_fd == -2) { const char *value = env("CORVINT_SPY_FD"); spy_fd = value ? atoi(value) : -1; }
  if (spy_fd >= 0) (void)(next_write ? next_write(spy_fd, &channel, 1) : write(spy_fd, &channel, 1));
}
static int observed_path(const char *path) {
  return path && path[0] && strcmp(path, "/dev/null");
}
__attribute__((constructor)) static void setup(void) {
  next_getenv = dlsym(RTLD_NEXT, "getenv");
  next_setenv = dlsym(RTLD_NEXT, "setenv");
  next_unsetenv = dlsym(RTLD_NEXT, "unsetenv");
  next_open = dlsym(RTLD_NEXT, "open");
  next_openat = dlsym(RTLD_NEXT, "openat");
  next_creat = dlsym(RTLD_NEXT, "creat");
  next_fopen = dlsym(RTLD_NEXT, "fopen");
  next_access = dlsym(RTLD_NEXT, "access");
  next_stat = dlsym(RTLD_NEXT, "stat");
  next_rename = dlsym(RTLD_NEXT, "rename");
  next_renameat = dlsym(RTLD_NEXT, "renameat");
  next_unlink = dlsym(RTLD_NEXT, "unlink");
  next_unlinkat = dlsym(RTLD_NEXT, "unlinkat");
  next_mkdir = dlsym(RTLD_NEXT, "mkdir");
  next_mkdirat = dlsym(RTLD_NEXT, "mkdirat");
  next_rmdir = dlsym(RTLD_NEXT, "rmdir");
  next_chmod = dlsym(RTLD_NEXT, "chmod");
  next_truncate = dlsym(RTLD_NEXT, "truncate");
  next_symlink = dlsym(RTLD_NEXT, "symlink");
  next_link = dlsym(RTLD_NEXT, "link");
  next_write = dlsym(RTLD_NEXT, "write");
  next_getaddrinfo = dlsym(RTLD_NEXT, "getaddrinfo");
  next_socket = dlsym(RTLD_NEXT, "socket");
  next_connect = dlsym(RTLD_NEXT, "connect");
  next_sendto = dlsym(RTLD_NEXT, "sendto");
  next_execve = dlsym(RTLD_NEXT, "execve");
  next_fork = dlsym(RTLD_NEXT, "fork");
  next_posix_spawn = dlsym(RTLD_NEXT, "posix_spawn");
  if (env("CORVINT_SPY_DIAGNOSTIC")) mark('L');
}
char *getenv(const char *name) {
  char *value = next_getenv ? next_getenv(name) : 0;
  if (name && !strncmp(name, "CORVINT_PRODUCTION_ENV_", 21)) mark('E');
  return value;
}
int setenv(const char *name, const char *value, int overwrite) {
  if (name && !strncmp(name, "CORVINT_PRODUCTION_ENV_", 21)) mark('E');
  return next_setenv ? next_setenv(name, value, overwrite) : -1;
}
int unsetenv(const char *name) {
  if (name && !strncmp(name, "CORVINT_PRODUCTION_ENV_", 21)) mark('E');
  return next_unsetenv ? next_unsetenv(name) : -1;
}
int open(const char *path, int flags, ...) {
  if (observed_path(path)) { if ((flags & O_ACCMODE) == O_RDONLY) mark('R'); else { mark('W'); watched_fd = -2; } }
  int mode = 0; if (flags & O_CREAT) { va_list args; va_start(args, flags); mode = va_arg(args, int); va_end(args); }
  if (!next_open) return -1;
  int fd = (flags & O_CREAT) ? next_open(path, flags, mode) : next_open(path, flags);
  if (observed_path(path) && (flags & O_ACCMODE) != O_RDONLY) watched_fd = fd;
  return fd;
}
int openat(int dirfd, const char *path, int flags, ...) {
  if (observed_path(path)) { if ((flags & O_ACCMODE) == O_RDONLY) mark('R'); else mark('W'); }
  int mode = 0; if (flags & O_CREAT) { va_list args; va_start(args, flags); mode = va_arg(args, int); va_end(args); }
  if (!next_openat) return -1;
  int fd = (flags & O_CREAT) ? next_openat(dirfd, path, flags, mode) : next_openat(dirfd, path, flags);
  if (observed_path(path) && (flags & O_ACCMODE) != O_RDONLY) watched_fd = fd;
  return fd;
}
int creat(const char *path, mode_t mode) { if (observed_path(path)) mark('C'); return next_creat ? next_creat(path, mode) : -1; }
FILE *fopen(const char *path, const char *mode) { if (observed_path(path)) mark('R'); return next_fopen ? next_fopen(path, mode) : 0; }
int access(const char *path, int mode) { if (observed_path(path)) mark('R'); return next_access ? next_access(path, mode) : -1; }
int stat(const char *path, struct stat *value) { if (observed_path(path)) mark('R'); return next_stat ? next_stat(path, value) : -1; }
int rename(const char *old, const char *new) { if (observed_path(old) || observed_path(new)) mark('N'); return next_rename ? next_rename(old, new) : -1; }
int renameat(int olddirfd, const char *old, int newdirfd, const char *new) { if (observed_path(old) || observed_path(new)) mark('N'); return next_renameat ? next_renameat(olddirfd, old, newdirfd, new) : -1; }
int unlink(const char *path) { if (observed_path(path)) mark('U'); return next_unlink ? next_unlink(path) : -1; }
int unlinkat(int dirfd, const char *path, int flags) { if (observed_path(path)) mark('U'); return next_unlinkat ? next_unlinkat(dirfd, path, flags) : -1; }
int mkdir(const char *path, mode_t mode) { if (observed_path(path)) mark('C'); return next_mkdir ? next_mkdir(path, mode) : -1; }
int mkdirat(int dirfd, const char *path, mode_t mode) { if (observed_path(path)) mark('C'); return next_mkdirat ? next_mkdirat(dirfd, path, mode) : -1; }
int rmdir(const char *path) { if (observed_path(path)) mark('U'); return next_rmdir ? next_rmdir(path) : -1; }
int chmod(const char *path, mode_t mode) { if (observed_path(path)) mark('M'); return next_chmod ? next_chmod(path, mode) : -1; }
int truncate(const char *path, off_t length) { if (observed_path(path)) mark('M'); return next_truncate ? next_truncate(path, length) : -1; }
int symlink(const char *old, const char *new) { if (observed_path(old) || observed_path(new)) mark('M'); return next_symlink ? next_symlink(old, new) : -1; }
int link(const char *old, const char *new) { if (observed_path(old) || observed_path(new)) mark('M'); return next_link ? next_link(old, new) : -1; }
ssize_t write(int fd, const void *buffer, size_t count) { if (fd == watched_fd) mark('W'); return next_write ? next_write(fd, buffer, count) : -1; }
int getaddrinfo(const char *node, const char *service, const struct addrinfo *hints, struct addrinfo **result) { if (node || service) mark('G'); return next_getaddrinfo ? next_getaddrinfo(node, service, hints, result) : EAI_FAIL; }
int socket(int domain, int type, int protocol) { if (domain == AF_INET || domain == AF_INET6) mark('K'); return next_socket ? next_socket(domain, type, protocol) : -1; }
int connect(int fd, const struct sockaddr *address, socklen_t length) {
  if (address && (address->sa_family == AF_INET || address->sa_family == AF_INET6)) mark('K');
  return next_connect ? next_connect(fd, address, length) : -1;
}
ssize_t sendto(int fd, const void *buffer, size_t count, int flags, const struct sockaddr *address, socklen_t length) { if (address && (address->sa_family == AF_INET || address->sa_family == AF_INET6)) mark('T'); return next_sendto ? next_sendto(fd, buffer, count, flags, address, length) : -1; }
pid_t fork(void) { mark('F'); return next_fork ? next_fork() : -1; }
int posix_spawn(pid_t *pid, const char *path, const posix_spawn_file_actions_t *actions, const posix_spawnattr_t *attrs, char *const argv[], char *const envp[]) { if (path) mark('P'); return next_posix_spawn ? next_posix_spawn(pid, path, actions, attrs, argv, envp) : -1; }
int execve(const char *path, char *const argv[], char *const envp[]) {
  if (path) mark('P');
  return next_execve ? next_execve(path, argv, envp) : -1;
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(probeSource, []byte(`
#include <arpa/inet.h>
#include <fcntl.h>
#include <netdb.h>
#include <netinet/in.h>
#include <spawn.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/socket.h>
#include <sys/stat.h>
#include <sys/wait.h>
#include <unistd.h>
extern char **environ;
int main(int argc, char **argv) {
  if (argc != 4) return 2;
  (void)getenv("CORVINT_PRODUCTION_ENV_CANARY_A");
  (void)setenv("CORVINT_PRODUCTION_ENV_CANARY_B", "changed", 1);
  (void)unsetenv("CORVINT_PRODUCTION_ENV_CANARY_C");
  int fd = open(argv[1], O_RDONLY); if (fd >= 0) close(fd);
  (void)access(argv[1], R_OK); struct stat st; (void)stat(argv[1], &st); FILE *file = fopen(argv[1], "r"); if (file) fclose(file);
  const char *root = getenv("CORVINT_SPY_ROOT"); char write_path[1024], renamed_path[1024], renamed_again_path[1024], dir_path[1024];
  snprintf(write_path, sizeof(write_path), "%s/write", root); snprintf(renamed_path, sizeof(renamed_path), "%s/renamed", root); snprintf(renamed_again_path, sizeof(renamed_again_path), "%s/renamed-again", root); snprintf(dir_path, sizeof(dir_path), "%s/created-dir", root);
  fd = open(write_path, O_WRONLY | O_CREAT | O_TRUNC, 0600); if (fd >= 0) { (void)write(fd, "x", 1); close(fd); }
  fd = openat(AT_FDCWD, write_path, O_WRONLY | O_APPEND); if (fd >= 0) { (void)write(fd, "y", 1); close(fd); }
  fd = creat(renamed_path, 0600); if (fd >= 0) close(fd);
  (void)chmod(renamed_path, 0600); (void)truncate(renamed_path, 0);
  char link_path[1024], symlink_path[1024]; snprintf(link_path, sizeof(link_path), "%s/link", root); snprintf(symlink_path, sizeof(symlink_path), "%s/symlink", root);
  (void)link(renamed_path, link_path); (void)symlink(renamed_path, symlink_path);
  (void)rename(write_path, renamed_path); (void)renameat(AT_FDCWD, renamed_path, AT_FDCWD, renamed_again_path);
  (void)unlink(renamed_again_path); (void)unlinkat(AT_FDCWD, renamed_path, 0); (void)unlink(link_path); (void)unlink(symlink_path);
  (void)mkdir(dir_path, 0700); (void)mkdirat(AT_FDCWD, dir_path, 0700); (void)rmdir(dir_path);
  struct addrinfo *info = 0; (void)getaddrinfo("localhost", "80", 0, &info); if (info) freeaddrinfo(info);
  const char *target = argv[2];
  const char *colon = strrchr(target, ':');
  int socket_fd = socket(AF_INET, SOCK_STREAM, 0);
  struct sockaddr_in address = {0}; address.sin_family = AF_INET; address.sin_addr.s_addr = htonl(INADDR_LOOPBACK); address.sin_port = htons(atoi(colon + 1));
  (void)connect(socket_fd, (struct sockaddr *)&address, sizeof(address)); (void)sendto(socket_fd, "x", 1, 0, (struct sockaddr *)&address, sizeof(address)); close(socket_fd);
  pid_t child = fork(); if (child == 0) _exit(0); if (child > 0) (void)waitpid(child, 0, 0);
  char *process_argv[] = { argv[3], 0 };
  pid_t spawned = 0; (void)posix_spawn(&spawned, process_argv[0], 0, 0, process_argv, environ); if (spawned > 0) (void)waitpid(spawned, 0, 0);
  char *alternate_argv[] = { "/usr/bin/false", 0 }; spawned = 0; (void)posix_spawn(&spawned, alternate_argv[0], 0, 0, alternate_argv, environ); if (spawned > 0) (void)waitpid(spawned, 0, 0);
  execve(process_argv[0], process_argv, environ);
  return 1;
}
`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, command := range [][]string{
		{"cc", "-dynamiclib", "-o", spy, spySource},
		{"cc", "-Wl,-interposable", "-o", probe, probeSource, spy},
	} {
		cmd := exec.Command(command[0], command[1:]...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build observer: %v\n%s", err, output)
		}
	}
	return spy, probe
}
