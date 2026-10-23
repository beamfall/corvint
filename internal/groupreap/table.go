//go:build darwin || linux

package groupreap

// ReadProcessTable reads every process's pid, parent, process group, kernel
// start time and state, each process's fields from one read of that process
// (sysctl kern.proc.all on Darwin, /proc/<pid>/stat on Linux). A process that
// ends during the read is left out; any other read failure is an error, never
// an empty table. It sends no signal and grants no signal authority.
func ReadProcessTable() (map[int]Process, error) { return readProcessTable() }
