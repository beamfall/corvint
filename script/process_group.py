# SPDX-License-Identifier: Apache-2.0
"""Bounded process-group ownership for the development qualification scripts.

Python 3.9 has no waitid(WNOWAIT), so a script cannot hold its command's
exited leader unreaped the way internal/groupreap does. Instead each command
runs in a process group whose ID is pinned by a keeper: a sleeping child that
leads the group and is reaped only after the last group signal. While the
keeper is unreaped (alive or a zombie) its PID, and so the group ID, cannot be
reused, so every killpg names this group and no other (V1-1387, PGO-V0-019).
The command itself may be reaped at any time.

Retirement is bounded: SIGKILL to the group, repeated while the command's
output is still held, a bounded drain, one final SIGKILL, then the keeper
reap. Nothing is signalled after that reap. A group whose output is still
held at the bound raises GroupHold instead of returning silently. A
descendant that leaves the group (setsid or setpgid) is not reached.
"""
import os
import signal
import subprocess
import sys
import time

KEEPER = ['/bin/sleep', '86400']
RETIRE_BOUND = 10.0
KILL_INTERVAL = 0.2


class GroupHold(RuntimeError):
    """The owned group's retirement was not proved within its bound."""


class OwnedGroup:
    """Runs argv (Popen keyword arguments as usual, except preexec_fn and
    start_new_session) in a keeper-pinned process group."""

    def __init__(self, argv, **kwargs):
        if 'preexec_fn' in kwargs or kwargs.get('start_new_session'):
            raise ValueError('OwnedGroup places the command in its own group')
        kwargs.pop('start_new_session', None)
        self.keeper = subprocess.Popen(KEEPER, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL,
                                       stderr=subprocess.DEVNULL, preexec_fn=os.setpgrp)
        self.pgid = self.keeper.pid
        self.retired = False
        self.output = None
        self.drained = None
        pgid = self.pgid
        kwargs.setdefault('stdin', subprocess.DEVNULL)
        try:
            self.process = subprocess.Popen(argv, preexec_fn=lambda: os.setpgid(0, pgid), **kwargs)
        except BaseException:
            self._kill(signal.SIGKILL)
            self.retired = True
            self.keeper.wait(timeout=RETIRE_BOUND)
            raise

    @property
    def pid(self):
        return self.process.pid

    def _kill(self, sig):
        try:
            os.killpg(self.pgid, sig)
        except ProcessLookupError:
            pass
        except PermissionError:
            # Darwin answers EPERM for a group whose members are all zombies,
            # as internal/groupreap maps it; elsewhere EPERM stays an error.
            if sys.platform != 'darwin':
                raise

    def signal(self, sig):
        """Sends sig to the whole group; refused once retirement began."""
        if self.retired:
            raise GroupHold('group signal refused: the keeper may be reaped')
        self._kill(sig)

    def communicate(self, timeout=None):
        self.drained = self.process.communicate(timeout=timeout)
        return self.drained

    def retire(self, bound=RETIRE_BOUND):
        """Kills the group, drains the command's output and reaps it and the
        keeper, all within bound seconds. Returns (stdout, stderr) as Popen
        communicate does; a second call returns the first result."""
        if self.retired:
            if self.output is None:
                raise GroupHold('process group retirement was not proved')
            return self.output
        deadline = time.monotonic() + bound
        output = self.drained
        while output is None:
            self._kill(signal.SIGKILL)
            remaining = deadline - time.monotonic()
            try:
                output = self.process.communicate(timeout=max(0.0, min(KILL_INTERVAL, remaining)))
                break
            except subprocess.TimeoutExpired:
                if remaining <= KILL_INTERVAL:
                    break
        # Members that hold no pipe are reached by this last kill; the group ID
        # is still pinned because the keeper is not yet reaped.
        self._kill(signal.SIGKILL)
        self.retired = True
        try:
            self.keeper.wait(timeout=max(0.1, deadline - time.monotonic()))
        except subprocess.TimeoutExpired:
            raise GroupHold('keeper %d was not reaped after SIGKILL' % self.pgid)
        if output is None:
            raise GroupHold('process group %d still holds the output %.1fs after SIGKILL' % (self.pgid, bound))
        self.output = output
        return output

    def __enter__(self):
        return self

    def __exit__(self, kind, value, traceback):
        self.retire()
        return False
