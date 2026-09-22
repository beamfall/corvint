# SPDX-License-Identifier: AGPL-3.0-or-later
"""Drive one offline native Pi TUI prompt and reap the complete owned PTY group."""
import errno
import fcntl
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

pid = None
fd = None
reaped = False


def alive():
    try:
        os.killpg(pid, 0)
        return True
    except ProcessLookupError:
        return False


def cleanup():
    global reaped
    if pid is None:
        return
    try:
        os.killpg(pid, signal.SIGTERM)
    except ProcessLookupError:
        pass
    until = time.monotonic() + 1
    while alive() and time.monotonic() < until:
        if not reaped:
            reaped = os.waitpid(pid, os.WNOHANG)[0] == pid
        select.select([], [], [], 0.01)
    if alive():
        os.killpg(pid, signal.SIGKILL)
    if not reaped:
        os.waitpid(pid, 0)
        reaped = True
    if fd is not None:
        os.close(fd)


def interrupted(number, _frame):
    raise SystemExit(128 + number)


signal.signal(signal.SIGINT, interrupted)
signal.signal(signal.SIGTERM, interrupted)
try:
    pid, fd = pty.fork()
    if pid == 0:
        os.execvpe(sys.argv[1], sys.argv[1:], os.environ)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack('HHHH', 30, 100, 0, 0))
    if os.environ.get('CORVINT_PI_PTY_WITNESS'):
        with open(os.environ['CORVINT_PI_PTY_WITNESS'], 'w') as witness:
            witness.write(str(pid))
    output = bytearray()
    start = time.monotonic()
    sent = stopped = False
    while time.monotonic() - start < 25:
        if not sent and time.monotonic() - start > 1:
            os.write(fd, b'inspect main.go\r')
            sent = True
        if not stopped and b'fixture response' in output:
            os.write(fd, b'\x04')
            stopped = True
        readable, _, _ = select.select([fd], [], [], 0.05)
        if readable:
            try:
                chunk = os.read(fd, 16384)
                if chunk:
                    output.extend(chunk)
                if len(output) > 1048576:
                    raise RuntimeError('TUI output bound')
            except OSError as error:
                if error.errno != errno.EIO:
                    raise
        waited, status = os.waitpid(pid, os.WNOHANG)
        if waited:
            reaped = True
            if not stopped or os.waitstatus_to_exitcode(status) != 0:
                raise RuntimeError('TUI exited before successful prompt/shutdown: ' + output[-3000:].decode(errors='replace'))
            print('Native Pi TUI prompt and clean shutdown passed')
            break
    else:
        raise RuntimeError('TUI deadline: ' + output[-3000:].decode(errors='replace'))
finally:
    cleanup()
