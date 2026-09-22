# SPDX-License-Identifier: AGPL-3.0-or-later
"""Drive native Pi TUI prompts, reload, and session replacement and reap the complete owned PTY group."""
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
    phase = 0
    response_at = None
    first = int(os.environ["CORVINT_PI_TUI_FIRST"])
    ready_at = None
    while time.monotonic() - start < 40:
        if ready_at is None and b'fixture' in output:
            ready_at = time.monotonic()
        if not sent and ready_at is not None and time.monotonic() - ready_at > 0.5:
            os.write(fd, b'inspect main.go\r')
            sent = True
        if phase in (0, 2, 4) and response_at is None and ('PROTECTED_NATIVE_PI_OK_' + str(first + phase // 2)).encode() in output:
            response_at = time.monotonic()
        # A displayed token precedes agent_settled. Wait past the bounded Stop child deadline.
        ready = response_at is not None and time.monotonic() - response_at > 2.2
        if phase == 0 and ready:
            os.write(fd, b'/reload\r')
            phase = 1
            output.clear()
            response_at = None
        elif phase == 1 and b'Reloaded keybindings' in output:
            os.write(fd, b'inspect One after reload\r')
            phase = 2
            output.clear()
            response_at = None
        elif phase == 2 and ready:
            os.write(fd, b'/new\r')
            phase = 3
            output.clear()
            response_at = None
        elif phase == 3 and b'New session started' in output:
            os.write(fd, b'inspect One after replacement\r')
            phase = 4
            output.clear()
            response_at = None
        elif phase == 4 and ready:
            os.write(fd, b'\x04')
            phase = 5
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
