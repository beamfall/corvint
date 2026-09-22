#!/usr/bin/env python3
"""Corvint-free, fixture-only CEM 0.1 external consumer runner."""

from __future__ import annotations

import argparse
import contextlib
import fcntl
import hashlib
import json
import os
import selectors
import shutil
import signal
import stat
import subprocess
import sys
import tempfile
import time
from pathlib import Path
from typing import Any, Mapping, Optional, Sequence


KIT_ROOT = Path(__file__).resolve().parent
MANIFEST_LIMIT = 1_048_576
ARTIFACT_LIMIT = 8_388_608
IMPLEMENTATION_LIMIT = 67_108_864
OBSERVATION_LIMIT = 131_072
CAPTURE_LIMIT = 65_536
CASE_TIMEOUT_SECONDS = 25.0
TOTAL_TIMEOUT_SECONDS = 900.0
LOCK_TIMEOUT_SECONDS = 5.0
MAX_RUNS = 2
MAX_JSON_DEPTH = 64
MAX_JSON_INTEGER_DIGITS = 128
MAX_PACKET_BYTES = 33_554_432
EXPECTED_COUNTS = {"valid": 7, "invalid": 19, "drift": 6}
EXPECTED_MANIFEST_SHA256 = "2655258e73d569e35dffb36cc6d3ae2e738801848d9f57decb774463b92f34bd"
EXPECTED_MATRIX = (
    ("valid", "supported-sha1"),
    ("valid", "supported-sha256"),
    ("valid", "unknown-create-delete-modified-rename"),
    ("valid", "mechanical-whitespace"),
    ("valid", "mechanical-line-ending-crlf-to-lf"),
    ("valid", "unicode-and-no-final-newline"),
    ("valid", "overlap-evidence-base"),
    ("invalid", "duplicate-json-key"),
    ("invalid", "unknown-field"),
    ("invalid", "unknown-spec"),
    ("invalid", "patch-digest"),
    ("invalid", "evidence-id"),
    ("invalid", "hunk-id"),
    ("invalid", "hunk-range"),
    ("invalid", "path-traversal"),
    ("invalid", "supported-without-basis"),
    ("invalid", "orphan-evidence"),
    ("invalid", "surplus-hunk-payload"),
    ("invalid", "mechanical-line-join"),
    ("invalid", "header-only-create"),
    ("invalid", "header-only-delete"),
    ("invalid", "special-index-mode"),
    ("invalid", "contradictory-create-index-mode"),
    ("invalid", "lf-tokenizer-line-overflow"),
    ("invalid", "integer-resource-bound"),
    ("invalid", "utf8-path-resource-bound"),
    ("drift", "stable"),
    ("drift", "relocated-whitespace-outside"),
    ("drift", "stale-whitespace-inside"),
    ("drift", "ambiguous-overlapping-matches"),
    ("drift", "deleted"),
    ("drift", "deleted-symlink-changed"),
)
FIXTURE_DIRECTORIES = ("maps", "patches", "repository", "targets")
PACKET_IDENTITY_FILES = (
    "ADAPTER.md", "ALGORITHMS.md", "IMPLEMENTATIONS.md", "START-HERE.md", "SUBMIT.md",
    "manifest.json", "observation.schema.json", "runner.py",
)
LF_OVERFLOW_RECIPE = "cem/0.1-lf-overflow-x-lines"
LF_OVERFLOW_BYTES = 524_290
LF_OVERFLOW_LINES = 262_145
LF_OVERFLOW_SHA256 = "cfecb16854630a4d141b429fdfdcdc73bb48124ac47375b5896c634037c9eee6"


class RunnerError(RuntimeError):
    def __init__(self, code: str):
        super().__init__(code)
        self.code = code


class RunnerInterrupted(BaseException):
    pass


def _sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def _json_resource_preflight(data: bytes, code: str) -> None:
    depth = 0
    digit_run = 0
    in_string = False
    escaped = False
    for byte in data:
        if in_string:
            if escaped:
                escaped = False
            elif byte == 0x5C:
                escaped = True
            elif byte == 0x22:
                in_string = False
            continue
        if byte == 0x22:
            in_string = True
            digit_run = 0
        elif byte in (0x7B, 0x5B):
            depth += 1
            digit_run = 0
            if depth > MAX_JSON_DEPTH:
                raise RunnerError(code + "-nesting")
        elif byte in (0x7D, 0x5D):
            depth = max(0, depth - 1)
            digit_run = 0
        elif 0x30 <= byte <= 0x39:
            digit_run += 1
            if digit_run > MAX_JSON_INTEGER_DIGITS:
                raise RunnerError(code + "-integer-too-long")
        else:
            digit_run = 0


def _strict_object(data: bytes, limit: int, code: str) -> dict[str, Any]:
    if len(data) > limit:
        raise RunnerError(code + "-too-large")
    _json_resource_preflight(data, code)

    def pairs(items: list[tuple[str, Any]]) -> dict[str, Any]:
        result: dict[str, Any] = {}
        for key, value in items:
            if key in result:
                raise RunnerError(code + "-duplicate-key")
            result[key] = value
        return result

    def invalid_constant(_value: str) -> None:
        raise RunnerError(code + "-invalid-number")

    try:
        text = data.decode("utf-8")
        value = json.loads(text, object_pairs_hook=pairs, parse_constant=invalid_constant)
    except RunnerError:
        raise
    except (UnicodeDecodeError, json.JSONDecodeError, RecursionError, ValueError,
            MemoryError, OverflowError):
        raise RunnerError(code + "-invalid-json") from None
    if not isinstance(value, dict):
        raise RunnerError(code + "-invalid-root")
    return value


def _read_regular(path: Path, limit: int, code: str) -> bytes:
    try:
        metadata = path.lstat()
    except OSError:
        raise RunnerError(code + "-unreadable") from None
    if not stat.S_ISREG(metadata.st_mode) or stat.S_ISLNK(metadata.st_mode):
        raise RunnerError(code + "-not-regular")
    if metadata.st_size > limit:
        raise RunnerError(code + "-too-large")
    try:
        data = path.read_bytes()
    except OSError:
        raise RunnerError(code + "-unreadable") from None
    if len(data) > limit:
        raise RunnerError(code + "-too-large")
    return data


def _relative_path(value: object, code: str) -> str:
    if not isinstance(value, str) or not value or len(value.encode("utf-8")) > 512:
        raise RunnerError(code)
    path = Path(value)
    if (path.is_absolute() or "\\" in value or
            any(ord(character) < 32 or ord(character) == 127 for character in value) or
            any(part in ("", ".", "..") for part in value.split("/"))):
        raise RunnerError(code)
    return value


def _validate_manifest(manifest: Mapping[str, Any]) -> None:
    repository = manifest.get("repository")
    if not isinstance(repository, dict) or set(repository) != {
            "author", "message", "revisions", "root", "timestamp"}:
        raise RunnerError("manifest-repository")
    if any(not isinstance(repository[field], str) for field in ("author", "message", "root", "timestamp")):
        raise RunnerError("manifest-repository")
    _relative_path(repository["root"], "repository-root")
    revisions = repository.get("revisions")
    if not isinstance(revisions, dict) or set(revisions) != {"sha1", "sha256"}:
        raise RunnerError("manifest-repository")
    if (not isinstance(revisions["sha1"], str) or len(revisions["sha1"]) != 40 or
            not isinstance(revisions["sha256"], str) or len(revisions["sha256"]) != 64 or
            any(character not in "0123456789abcdef" for value in revisions.values() for character in value)):
        raise RunnerError("manifest-repository")
    names: set[str] = set()
    for group in ("valid", "invalid"):
        for case in manifest[group]:
            if not isinstance(case, dict) or not isinstance(case.get("name"), str):
                raise RunnerError("manifest-case")
            if case["name"] in names:
                raise RunnerError("manifest-case")
            names.add(case["name"])
            _relative_path(case.get("map"), "map-path")
            if ("patch" in case) == ("patchRecipe" in case):
                raise RunnerError("manifest-case")
            if "patch" in case:
                _relative_path(case["patch"], "patch-path")
            elif case["patchRecipe"] != LF_OVERFLOW_RECIPE:
                raise RunnerError("manifest-case")
            if group == "valid" and case.get("objectFormat") not in ("sha1", "sha256"):
                raise RunnerError("manifest-case")
    drift_names: set[str] = set()
    for case in manifest["drift"]:
        required = {
            "accept", "map", "message", "name", "patch", "status", "targetBlobOid",
            "targetPatch", "targetRevision", "targetSpan", "timestamp",
        }
        if not isinstance(case, dict) or set(case) != required or not isinstance(case["name"], str):
            raise RunnerError("manifest-drift")
        if case["name"] in drift_names:
            raise RunnerError("manifest-drift")
        drift_names.add(case["name"])
        _relative_path(case["map"], "map-path")
        _relative_path(case["patch"], "patch-path")
        if case["targetPatch"] is not None:
            _relative_path(case["targetPatch"], "target-patch")
        if (not isinstance(case["accept"], bool) or
                case["status"] not in ("stable", "relocated", "stale", "ambiguous", "deleted") or
                not isinstance(case["targetRevision"], str) or len(case["targetRevision"]) != 40):
            raise RunnerError("manifest-drift")
    if not isinstance(manifest["producerJobs"], list) or len(manifest["producerJobs"]) != 5:
        raise RunnerError("manifest-producer")
    observed_matrix = tuple(
        (group, case["name"])
        for group in ("valid", "invalid", "drift")
        for case in manifest[group]
    )
    if observed_matrix != EXPECTED_MATRIX:
        raise RunnerError("manifest-matrix-identity")


def _read_packet_path(root_fd: int, relative: str, limit: int, code: str) -> bytes:
    value = _relative_path(relative, code + "-path")
    parts = value.split("/")
    if len(parts) > 16:
        raise RunnerError(code + "-path")
    directory_fd = os.dup(root_fd)
    try:
        for component in parts[:-1]:
            try:
                child_fd = os.open(
                    component,
                    os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) |
                    getattr(os, "O_DIRECTORY", 0) | getattr(os, "O_NOFOLLOW", 0),
                    dir_fd=directory_fd,
                )
            except OSError:
                raise RunnerError(code + "-unreadable") from None
            os.close(directory_fd)
            directory_fd = child_fd
        return _read_regular_at(directory_fd, parts[-1], limit, code)
    finally:
        os.close(directory_fd)


def _framed_packet_digest(files: Mapping[str, bytes]) -> str:
    digest = hashlib.sha256()
    for name in sorted(files):
        name_bytes = name.encode("utf-8")
        data = files[name]
        digest.update(len(name_bytes).to_bytes(4, "big"))
        digest.update(name_bytes)
        digest.update(len(data).to_bytes(8, "big"))
        digest.update(data)
    return digest.hexdigest()


def _manifest_and_artifacts() -> tuple[dict[str, Any], str, dict[str, bytes], str]:
    try:
        root_fd = os.open(
            KIT_ROOT,
            os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) |
            getattr(os, "O_DIRECTORY", 0) | getattr(os, "O_NOFOLLOW", 0),
        )
    except OSError:
        raise RunnerError("packet-unreadable") from None
    try:
        manifest_bytes = _read_packet_path(root_fd, "manifest.json", MANIFEST_LIMIT, "manifest")
        manifest_digest = _sha256(manifest_bytes)
        if manifest_digest != EXPECTED_MANIFEST_SHA256:
            raise RunnerError("manifest-digest")
        manifest = _strict_object(manifest_bytes, MANIFEST_LIMIT, "manifest")
        required = {
            "suite", "repository", "valid", "invalid", "drift", "producerJobs",
            "artifactSha256",
        }
        if set(manifest) != required or manifest.get("suite") != "cem/0.1-interop":
            raise RunnerError("manifest-shape")
        for group, expected in EXPECTED_COUNTS.items():
            if not isinstance(manifest.get(group), list) or len(manifest[group]) != expected:
                raise RunnerError("manifest-case-count")
        _validate_manifest(manifest)
        declared = manifest.get("artifactSha256")
        if not isinstance(declared, dict) or len(declared) != 51:
            raise RunnerError("manifest-artifact-set")
        artifacts: dict[str, bytes] = {}
        total_bytes = 0
        for relative, digest in declared.items():
            path = _relative_path(relative, "manifest-artifact-path")
            if path.split("/", 1)[0] not in FIXTURE_DIRECTORIES:
                raise RunnerError("manifest-artifact-path")
            if not isinstance(digest, str):
                raise RunnerError("artifact-digest")
            data = _read_packet_path(root_fd, path, ARTIFACT_LIMIT, "artifact")
            total_bytes += len(data)
            if total_bytes > MAX_PACKET_BYTES:
                raise RunnerError("packet-too-large")
            if _sha256(data) != digest:
                raise RunnerError("artifact-digest")
            artifacts[path] = data
        packet_files = {
            name: manifest_bytes if name == "manifest.json" else
            _read_packet_path(root_fd, name, MANIFEST_LIMIT, "packet-file")
            for name in PACKET_IDENTITY_FILES
        }
        if total_bytes + sum(len(data) for data in packet_files.values()) > MAX_PACKET_BYTES:
            raise RunnerError("packet-too-large")
        packet_digest = _framed_packet_digest(packet_files)
    finally:
        os.close(root_fd)
    return manifest, manifest_digest, artifacts, packet_digest


def _minimal_environment(
    home: Path, extra: Optional[Mapping[str, str]] = None,
) -> dict[str, str]:
    environment = {
        "GIT_CONFIG_GLOBAL": os.devnull,
        "GIT_CONFIG_NOSYSTEM": "1",
        "GIT_NO_LAZY_FETCH": "1",
        "GIT_OPTIONAL_LOCKS": "0",
        "GIT_TERMINAL_PROMPT": "0",
        "HOME": str(home),
        "LANG": "C",
        "LC_ALL": "C",
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "TMPDIR": str(home),
    }
    if extra:
        environment.update(extra)
    return environment


def _stop_group(process: subprocess.Popen[Any]) -> bool:
    try:
        os.killpg(process.pid, signal.SIGKILL)
    except ProcessLookupError:
        return True
    except OSError:
        try:
            process.kill()
        except ProcessLookupError:
            return True
        except OSError:
            return False
    return True


def _capture_once(
    selector: selectors.BaseSelector,
    buffers: Mapping[str, bytearray],
    limit: int,
    timeout: float,
) -> Optional[str]:
    for key, _events in selector.select(timeout):
        buffer = buffers[str(key.data)]
        while len(buffer) <= limit:
            try:
                chunk = os.read(key.fd, min(65_536, limit + 1 - len(buffer)))
            except BlockingIOError:
                break
            if not chunk:
                selector.unregister(key.fileobj)
                key.fileobj.close()
                break
            buffer.extend(chunk)
            if len(buffer) > limit:
                return str(key.data)
    return None


def _close_capture(
    selector: Optional[selectors.BaseSelector], streams: Sequence[Any],
) -> None:
    for stream in streams:
        if stream.closed:
            continue
        if selector is not None:
            try:
                selector.unregister(stream)
            except KeyError:
                pass
        stream.close()
    if selector is not None:
        selector.close()


def _run_process(
    argv: Sequence[str],
    cwd: Path,
    environment: Mapping[str, str],
    timeout_seconds: float,
    capture_limit: int = CAPTURE_LIMIT,
) -> tuple[dict[str, Any], bytes, bytes]:
    if not hasattr(os, "killpg"):
        raise RunnerError("posix-required")
    started = time.monotonic_ns()
    process: Optional[subprocess.Popen[Any]] = None
    selector: Optional[selectors.BaseSelector] = None
    streams: tuple[Any, ...] = ()
    overflow = None
    timed_out = False
    try:
        try:
            previous_mask = signal.pthread_sigmask(
                signal.SIG_BLOCK, {signal.SIGINT, signal.SIGTERM},
            )
            try:
                process = subprocess.Popen(
                    list(argv), cwd=cwd, env=dict(environment), stdin=subprocess.DEVNULL,
                    stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True,
                )
                assert process.stdout is not None and process.stderr is not None
                streams = (process.stdout, process.stderr)
            finally:
                signal.pthread_sigmask(signal.SIG_SETMASK, previous_mask)
        except (OSError, subprocess.SubprocessError):
            return ({
                "durationNs": time.monotonic_ns() - started,
                "errorCode": "spawn-failed", "exitCode": None, "timedOut": False,
            }, b"", b"")
        buffers = {"stdout": bytearray(), "stderr": bytearray()}
        selector = selectors.DefaultSelector()
        deadline = time.monotonic() + max(0.0, timeout_seconds)
        for name, stream in zip(buffers, streams):
            os.set_blocking(stream.fileno(), False)
            selector.register(stream, selectors.EVENT_READ, name)
        while process.poll() is None and overflow is None:
            remaining = max(0.0, deadline - time.monotonic())
            overflow = _capture_once(selector, buffers, capture_limit, min(0.05, remaining))
            if time.monotonic() >= deadline:
                timed_out = True
                break
        if not _stop_group(process):
            raise RunnerError("process-cleanup")
        try:
            process.wait(timeout=1.0)
        except subprocess.TimeoutExpired:
            raise RunnerError("process-wait") from None
        drain_deadline = time.monotonic() + 0.5
        while selector.get_map() and overflow is None and time.monotonic() < drain_deadline:
            overflow = _capture_once(selector, buffers, capture_limit, 0.05)
        capture_closed = not selector.get_map()
        _close_capture(selector, streams)
    except BaseException as error:
        stop_failed = process is not None and not _stop_group(process)
        if process is not None:
            try:
                process.wait(timeout=1.0)
            except (OSError, subprocess.TimeoutExpired):
                pass
        try:
            _close_capture(selector, streams)
        except OSError:
            stop_failed = True
        if stop_failed:
            raise RunnerError("process-cleanup") from error
        raise
    assert process.returncode is not None
    error_code = None
    if overflow:
        error_code = overflow + "-bound-exceeded"
    elif timed_out:
        error_code = "timeout"
    elif not capture_closed:
        error_code = "capture-did-not-close"
    return ({
        "durationNs": time.monotonic_ns() - started,
        "errorCode": error_code, "exitCode": process.returncode, "timedOut": timed_out,
    }, bytes(buffers["stdout"]), bytes(buffers["stderr"]))


def _git(repo: Path, home: Path, *arguments: str, dated: Optional[str] = None) -> str:
    extra = None if dated is None else {
        "GIT_AUTHOR_DATE": dated, "GIT_COMMITTER_DATE": dated,
    }
    argv = [
        "git", "-C", str(repo), "-c", "core.hooksPath=/dev/null",
        "-c", "credential.helper=", "-c", "protocol.file.allow=never", *arguments,
    ]
    result, stdout, _stderr = _run_process(
        argv, repo.parent, _minimal_environment(home, extra), 20.0,
    )
    if result["errorCode"] is not None or result["exitCode"] != 0:
        raise RunnerError("git-command")
    try:
        return stdout.decode("ascii").strip()
    except UnicodeDecodeError:
        raise RunnerError("git-output") from None


def _artifact_bytes(
    artifacts: Mapping[str, bytes], relative: object, code: str,
) -> bytes:
    path = _relative_path(relative, code + "-path")
    try:
        return artifacts[path]
    except KeyError:
        raise RunnerError(code + "-missing") from None


def _copy_fixture_tree(
    artifacts: Mapping[str, bytes], source: str, destination: Path,
) -> None:
    destination.mkdir(mode=0o700)
    prefix = _relative_path(source, "repository-root") + "/"
    selected = [path for path in sorted(artifacts) if path.startswith(prefix)]
    if not selected or len(selected) > 50:
        raise RunnerError("repository-fixture-set")
    for path in selected:
        relative = Path(path[len(prefix):])
        data = artifacts[path]
        target = destination / relative
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        target.write_bytes(data)
        target.chmod(0o600)


def _initialize_repository(
    repository: Path,
    home: Path,
    manifest: Mapping[str, Any],
    artifacts: Mapping[str, bytes],
    object_format: str,
) -> Path:
    _copy_fixture_tree(artifacts, manifest["repository"]["root"], repository)
    try:
        _git(repository, home, "init", "-q", "--object-format=" + object_format)
    except RunnerError:
        if object_format == "sha256":
            raise RunnerError("sha256-unsupported") from None
        raise
    _git(repository, home, "config", "user.name", "CEM Interop")
    _git(repository, home, "config", "user.email", "cem@example.invalid")
    _git(repository, home, "config", "core.autocrlf", "false")
    _git(repository, home, "config", "core.fileMode", "false")
    _git(repository, home, "add", ".")
    recipe = manifest["repository"]
    _git(
        repository, home, "commit", "--no-gpg-sign", "-qm", recipe["message"],
        dated=recipe["timestamp"],
    )
    return repository


def _make_repository(
    root: Path,
    home: Path,
    manifest: Mapping[str, Any],
    artifacts: Mapping[str, bytes],
    object_format: str,
) -> Path:
    repository = _initialize_repository(
        root / ("repo-" + object_format), home, manifest, artifacts, object_format,
    )
    actual = _git(repository, home, "rev-parse", "HEAD")
    if actual != manifest["repository"]["revisions"][object_format]:
        raise RunnerError("repository-identity")
    return repository


def _make_drift_repository(
    root: Path,
    home: Path,
    manifest: Mapping[str, Any],
    artifacts: Mapping[str, bytes],
    case: Mapping[str, Any],
    index: int,
) -> Path:
    repository = _initialize_repository(
        root / ("drift-" + str(index)), home, manifest, artifacts, "sha1",
    )
    if case["targetPatch"] is not None:
        target_patch = root / ("target-" + str(index) + ".patch")
        target_patch.write_bytes(_artifact_bytes(artifacts, case["targetPatch"], "target-patch"))
        target_patch.chmod(0o400)
        _git(repository, home, "apply", "--whitespace=nowarn", str(target_patch))
        _git(repository, home, "add", "-A")
        _git(
            repository, home, "commit", "--no-gpg-sign", "-qm", case["message"],
            dated=case["timestamp"],
        )
    if _git(repository, home, "rev-parse", "HEAD") != case["targetRevision"]:
        raise RunnerError("target-identity")
    return repository


def _patch_bytes(case: Mapping[str, Any], artifacts: Mapping[str, bytes]) -> bytes:
    has_patch = "patch" in case
    has_recipe = "patchRecipe" in case
    if has_patch == has_recipe:
        raise RunnerError("patch-selector")
    if has_patch:
        return _artifact_bytes(artifacts, case["patch"], "patch")
    if case["patchRecipe"] != LF_OVERFLOW_RECIPE:
        raise RunnerError("patch-recipe")
    payload = b"x\n" * LF_OVERFLOW_LINES
    if (len(payload) != LF_OVERFLOW_BYTES or payload.count(b"\n") != LF_OVERFLOW_LINES or
            _sha256(payload) != LF_OVERFLOW_SHA256):
        raise RunnerError("patch-recipe")
    return payload


def _corvint_packet_source(home: Path) -> tuple[Optional[str], str]:
    result, stdout, _stderr = _run_process(
        ["git", "-C", str(KIT_ROOT), "rev-parse", "--show-toplevel", "HEAD"],
        KIT_ROOT, _minimal_environment(home), 20.0,
    )
    if result["errorCode"] is not None or result["exitCode"] != 0:
        return None, "standalone"
    try:
        lines = stdout.decode("utf-8").splitlines()
    except UnicodeDecodeError:
        return None, "standalone"
    if len(lines) != 2:
        return None, "standalone"
    checkout = Path(lines[0])
    commit = lines[1]
    if (not checkout.is_absolute() or
            len(commit) not in (40, 64) or
            any(character not in "0123456789abcdef" for character in commit)):
        return None, "standalone"
    try:
        relative = KIT_ROOT.relative_to(checkout).as_posix()
    except ValueError:
        return None, "standalone"
    status, status_stdout, _status_stderr = _run_process(
        [
            "git", "-C", str(checkout), "status", "--porcelain=v1",
            "--untracked-files=all", "--", relative,
        ],
        checkout, _minimal_environment(home), 20.0,
    )
    if status["errorCode"] is not None or status["exitCode"] != 0:
        return commit, "modified"
    return commit, "clean" if not status_stdout else "modified"


def _doctor() -> dict[str, Any]:
    started = time.monotonic_ns()
    manifest, manifest_digest, artifacts, packet_digest = _manifest_and_artifacts()
    with tempfile.TemporaryDirectory(prefix="cem-interop-doctor-") as temporary_name:
        root = Path(temporary_name)
        home = root / "home"
        home.mkdir(mode=0o700)
        corvint_commit, corvint_tree_state = _corvint_packet_source(home)
        _make_repository(root, home, manifest, artifacts, "sha1")
        _make_repository(root, home, manifest, artifacts, "sha256")
        for index, case in enumerate(manifest["drift"]):
            _make_drift_repository(root, home, manifest, artifacts, case, index)
    return {
        "artifactCount": len(artifacts),
        "corvintCommit": corvint_commit,
        "corvintTreeState": corvint_tree_state,
        "durationNs": time.monotonic_ns() - started,
        "gitObjectFormats": {"sha1": "PASS", "sha256": "PASS"},
        "manifestSha256": manifest_digest,
        "packetSha256": packet_digest,
        "required": dict(EXPECTED_COUNTS),
        "state": "PASS",
    }


def _safe_parent(path: Path) -> Path:
    absolute = path.absolute()
    parent = absolute.parent
    try:
        if not parent.is_dir() or parent.is_symlink():
            raise RunnerError("output-parent")
        current = parent
        while current != current.parent:
            if current.is_symlink():
                raise RunnerError("output-parent")
            current = current.parent
    except OSError:
        raise RunnerError("output-parent") from None
    return parent


def _open_directory(path: Path, code: str) -> int:
    try:
        return os.open(
            path,
            os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) |
            getattr(os, "O_DIRECTORY", 0) | getattr(os, "O_NOFOLLOW", 0),
        )
    except OSError:
        raise RunnerError(code) from None


def _read_regular_at(
    directory_fd: int, name: str, limit: int, code: str,
    owner_only: bool = False,
) -> bytes:
    descriptor = None
    try:
        descriptor = os.open(
            name,
            os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) | getattr(os, "O_NOFOLLOW", 0),
            dir_fd=directory_fd,
        )
        metadata = os.fstat(descriptor)
        if not stat.S_ISREG(metadata.st_mode):
            raise RunnerError(code + "-not-regular")
        if owner_only and stat.S_IMODE(metadata.st_mode) != 0o600:
            raise RunnerError(code + "-not-private")
        if metadata.st_size > limit:
            raise RunnerError(code + "-too-large")
        chunks: list[bytes] = []
        size = 0
        while True:
            chunk = os.read(descriptor, min(65_536, limit + 1 - size))
            if not chunk:
                break
            chunks.append(chunk)
            size += len(chunk)
            if size > limit:
                raise RunnerError(code + "-too-large")
        return b"".join(chunks)
    except RunnerError:
        raise
    except OSError:
        raise RunnerError(code + "-unreadable") from None
    finally:
        if descriptor is not None:
            os.close(descriptor)


@contextlib.contextmanager
def _observation_lock(path: Path):
    parent = _safe_parent(path)
    directory_fd = _open_directory(parent, "observation-lock-open")
    lock_fd = None
    try:
        for attempt in range(3):
            try:
                lock_fd = os.open(
                    ".cem-observation.lock",
                    os.O_RDWR | os.O_CREAT | getattr(os, "O_CLOEXEC", 0) |
                    getattr(os, "O_NOFOLLOW", 0),
                    0o600,
                    dir_fd=directory_fd,
                )
                break
            except FileNotFoundError:
                if attempt == 2:
                    raise RunnerError("observation-lock-open") from None
            except OSError:
                raise RunnerError("observation-lock-open") from None
        if lock_fd is None:
            raise RunnerError("observation-lock-open")
        try:
            metadata = os.fstat(lock_fd)
            if not stat.S_ISREG(metadata.st_mode) or metadata.st_uid != os.geteuid():
                raise RunnerError("observation-lock-owner")
            os.fchmod(lock_fd, 0o600)
            if stat.S_IMODE(os.fstat(lock_fd).st_mode) != 0o600:
                raise RunnerError("observation-lock-mode")
        except RunnerError:
            raise
        except OSError:
            raise RunnerError("observation-lock-mode") from None
        deadline = time.monotonic() + LOCK_TIMEOUT_SECONDS
        while True:
            try:
                fcntl.flock(lock_fd, fcntl.LOCK_EX | fcntl.LOCK_NB)
                break
            except BlockingIOError:
                if time.monotonic() >= deadline:
                    raise RunnerError("observation-lock-timeout") from None
                time.sleep(0.05)
            except OSError:
                raise RunnerError("observation-lock-acquire") from None
        yield
    finally:
        if lock_fd is not None:
            try:
                fcntl.flock(lock_fd, fcntl.LOCK_UN)
            except OSError:
                pass
            os.close(lock_fd)
        os.close(directory_fd)


def _atomic_write(
    path: Path,
    document: Mapping[str, Any],
    must_be_absent: bool,
    expected_digest: Optional[str] = None,
) -> None:
    payload = (json.dumps(document, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")
    if len(payload) > OBSERVATION_LIMIT:
        raise RunnerError("observation-too-large")
    parent = _safe_parent(path)
    if must_be_absent and expected_digest is not None:
        raise RunnerError("observation-write-contract")
    if not must_be_absent and expected_digest is None:
        raise RunnerError("observation-write-contract")
    directory_fd = _open_directory(parent, "observation-write")
    temporary_name = ".cem-observation-{}-{}".format(os.getpid(), time.monotonic_ns())
    file_fd = None
    publication_attempted = False

    def publication_matches() -> bool:
        try:
            published = _read_regular_at(
                directory_fd, path.name, OBSERVATION_LIMIT,
                "observation", owner_only=True,
            )
        except RunnerError:
            return False
        return published == payload

    try:
        file_fd = os.open(
            temporary_name,
            os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_NOFOLLOW", 0),
            0o600,
            dir_fd=directory_fd,
        )
        os.fchmod(file_fd, 0o600)
        with os.fdopen(file_fd, "wb", closefd=True) as stream:
            file_fd = None
            stream.write(payload)
            stream.flush()
            os.fsync(stream.fileno())
        if must_be_absent:
            publication_attempted = True
            try:
                os.link(
                    temporary_name,
                    path.name,
                    src_dir_fd=directory_fd,
                    dst_dir_fd=directory_fd,
                    follow_symlinks=False,
                )
            except FileExistsError:
                raise RunnerError("observation-exists")
            os.unlink(temporary_name, dir_fd=directory_fd)
        else:
            current = _read_regular_at(
                directory_fd, path.name, OBSERVATION_LIMIT, "observation", owner_only=True,
            )
            if _sha256(current) != expected_digest:
                raise RunnerError("observation-changed")
            publication_attempted = True
            os.replace(
                temporary_name,
                path.name,
                src_dir_fd=directory_fd,
                dst_dir_fd=directory_fd,
            )
        os.fsync(directory_fd)
    except RunnerError:
        raise
    except (KeyboardInterrupt, RunnerInterrupted) as error:
        if publication_attempted:
            if publication_matches():
                return
            raise RunnerError("observation-commit-uncertain") from error
        raise
    except OSError:
        if publication_attempted:
            if publication_matches():
                return
            raise RunnerError("observation-commit-uncertain") from None
        raise RunnerError("observation-write") from None
    finally:
        if file_fd is not None:
            os.close(file_fd)
        try:
            os.unlink(temporary_name, dir_fd=directory_fd)
        except OSError:
            pass
        os.close(directory_fd)


def _load_observation_with_digest(path: Path) -> tuple[dict[str, Any], str]:
    parent = _safe_parent(path)
    directory_fd = _open_directory(parent, "observation-unreadable")
    try:
        data = _read_regular_at(
            directory_fd, path.name, OBSERVATION_LIMIT, "observation", owner_only=True,
        )
    finally:
        os.close(directory_fd)
    document = _strict_object(data, OBSERVATION_LIMIT, "observation")
    _validate_observation(document)
    return document, _sha256(data)


def _load_observation(path: Path) -> dict[str, Any]:
    document, _digest_value = _load_observation_with_digest(path)
    return document


def _closed(value: object, keys: set[str]) -> dict[str, Any]:
    if not isinstance(value, dict) or set(value) != keys:
        raise RunnerError("observation-shape")
    return value


def _integer(value: object, maximum: Optional[int] = None) -> int:
    if not isinstance(value, int) or isinstance(value, bool) or value < 0:
        raise RunnerError("observation-shape")
    if maximum is not None and value > maximum:
        raise RunnerError("observation-shape")
    return value


def _digest(value: object) -> str:
    if (not isinstance(value, str) or len(value) != 64 or
            any(character not in "0123456789abcdef" for character in value)):
        raise RunnerError("observation-shape")
    return value


def _token(value: object, maximum: int) -> str:
    if (not isinstance(value, str) or not 1 <= len(value) <= maximum or
            any(character not in "abcdefghijklmnopqrstuvwxyz0123456789-" for character in value)):
        raise RunnerError("observation-shape")
    return value


def _validate_observation(document: Mapping[str, Any]) -> None:
    _closed(document, {"doctor", "lane", "profile", "publication", "runs", "state"})
    if (document["profile"] != "cem-external-interop-observation/1" or
            document["publication"] != "COMMITTED" or
            document["lane"] != "consumer" or document["state"] not in (
                "STARTED", "PASS", "FAIL")):
        raise RunnerError("observation-shape")
    doctor = _closed(document["doctor"], {
        "artifactCount", "corvintCommit", "corvintTreeState", "durationNs", "gitObjectFormats",
        "manifestSha256", "packetSha256", "required", "state",
    })
    if (doctor["artifactCount"] != 51 or doctor["state"] != "PASS" or
            doctor["corvintTreeState"] not in ("clean", "modified", "standalone") or
            doctor["required"] != EXPECTED_COUNTS or
            doctor["gitObjectFormats"] != {"sha1": "PASS", "sha256": "PASS"}):
        raise RunnerError("observation-shape")
    corvint_commit = doctor["corvintCommit"]
    if corvint_commit is not None and (
            not isinstance(corvint_commit, str) or len(corvint_commit) not in (40, 64) or
            any(character not in "0123456789abcdef" for character in corvint_commit)):
        raise RunnerError("observation-shape")
    if (doctor["corvintTreeState"] == "standalone") != (corvint_commit is None):
        raise RunnerError("observation-shape")
    _integer(doctor["durationNs"])
    _digest(doctor["manifestSha256"])
    _digest(doctor["packetSha256"])
    runs = document["runs"]
    if not isinstance(runs, list) or len(runs) > MAX_RUNS:
        raise RunnerError("observation-shape")
    for expected_sequence, run_value in enumerate(runs, 1):
        run = _closed(run_value, {
            "cases", "counts", "durationNs", "implementationSha256", "manifestSha256",
            "packetSha256", "sequence", "state",
        })
        if (run["sequence"] != expected_sequence or run["state"] not in ("PASS", "FAIL") or
                run["manifestSha256"] != doctor["manifestSha256"] or
                run["packetSha256"] != doctor["packetSha256"]):
            raise RunnerError("observation-shape")
        _integer(run["durationNs"])
        _digest(run["implementationSha256"])
        cases = run["cases"]
        if not isinstance(cases, list) or len(cases) != 32:
            raise RunnerError("observation-shape")
        observed_groups = {group: 0 for group in EXPECTED_COUNTS}
        observed_states = {"pass": 0, "fail": 0, "unsupported": 0}
        observed_matrix: list[tuple[str, str]] = []
        for case_value in cases:
            case = _closed(case_value, {
                "durationNs", "exitCode", "failureCode", "group", "name", "state",
            })
            if case["group"] not in EXPECTED_COUNTS or case["state"] not in (
                    "PASS", "FAIL", "UNSUPPORTED"):
                raise RunnerError("observation-shape")
            _token(case["name"], 80)
            _integer(case["durationNs"])
            if case["exitCode"] is not None and (
                    not isinstance(case["exitCode"], int) or isinstance(case["exitCode"], bool)):
                raise RunnerError("observation-shape")
            if case["failureCode"] is not None:
                _token(case["failureCode"], 64)
            if ((case["state"] == "PASS" and case["failureCode"] is not None) or
                    (case["state"] in ("FAIL", "UNSUPPORTED") and
                     case["failureCode"] is None)):
                raise RunnerError("observation-shape")
            observed_matrix.append((case["group"], case["name"]))
            observed_groups[case["group"]] += 1
            observed_states[case["state"].lower()] += 1
        if tuple(observed_matrix) != EXPECTED_MATRIX:
            raise RunnerError("observation-shape")
        counts = _closed(run["counts"], {"fail", "pass", "unsupported"})
        for value in counts.values():
            _integer(value, 32)
        if observed_groups != EXPECTED_COUNTS or counts != observed_states:
            raise RunnerError("observation-shape")
        expected_state = "PASS" if counts == {"pass": 32, "fail": 0, "unsupported": 0} else "FAIL"
        if run["state"] != expected_state:
            raise RunnerError("observation-shape")
    expected_top_state = "STARTED" if not runs else runs[-1]["state"]
    if document["state"] != expected_top_state:
        raise RunnerError("observation-shape")


def _start(path: Path) -> dict[str, Any]:
    with _observation_lock(path):
        try:
            path.lstat()
        except FileNotFoundError:
            pass
        except OSError:
            raise RunnerError("observation-unreadable") from None
        else:
            raise RunnerError("observation-exists")
        doctor = _doctor()
        observation = {
            "doctor": doctor,
            "lane": "consumer",
            "profile": "cem-external-interop-observation/1",
            "publication": "COMMITTED",
            "runs": [],
            "state": "STARTED",
        }
        _atomic_write(path, observation, must_be_absent=True)
    return observation


def _implementation(path: Path) -> tuple[bytes, str]:
    if not path.is_absolute():
        raise RunnerError("implementation-not-absolute")
    descriptor = None
    try:
        descriptor = os.open(
            path,
            os.O_RDONLY | getattr(os, "O_CLOEXEC", 0) | getattr(os, "O_NOFOLLOW", 0),
        )
        metadata = os.fstat(descriptor)
        if not stat.S_ISREG(metadata.st_mode):
            raise RunnerError("implementation-not-regular")
        if metadata.st_size > IMPLEMENTATION_LIMIT:
            raise RunnerError("implementation-too-large")
        if metadata.st_mode & 0o111 == 0:
            raise RunnerError("implementation-not-executable")
        chunks: list[bytes] = []
        size = 0
        while True:
            chunk = os.read(descriptor, min(65_536, IMPLEMENTATION_LIMIT + 1 - size))
            if not chunk:
                break
            chunks.append(chunk)
            size += len(chunk)
            if size > IMPLEMENTATION_LIMIT:
                raise RunnerError("implementation-too-large")
        data = b"".join(chunks)
    except RunnerError:
        raise
    except OSError:
        raise RunnerError("implementation-unreadable") from None
    finally:
        if descriptor is not None:
            os.close(descriptor)
    return data, _sha256(data)


def _private_implementation(root: Path, data: bytes) -> Path:
    path = root / "implementation"
    descriptor = None
    try:
        descriptor = os.open(
            path,
            os.O_WRONLY | os.O_CREAT | os.O_EXCL | getattr(os, "O_CLOEXEC", 0) |
            getattr(os, "O_NOFOLLOW", 0),
            0o500,
        )
        offset = 0
        while offset < len(data):
            offset += os.write(descriptor, data[offset:])
        os.fchmod(descriptor, 0o500)
    except OSError:
        raise RunnerError("implementation-copy") from None
    finally:
        if descriptor is not None:
            os.close(descriptor)
    return path


def _protocol_output(data: bytes) -> dict[str, Any]:
    return _strict_object(data, CAPTURE_LIMIT, "protocol-output")


def _case_result(
    group: str,
    name: str,
    process: Mapping[str, Any],
    output: Optional[dict[str, Any]],
    expected_accept: bool,
    expected_drift: list[dict[str, Any]],
) -> dict[str, Any]:
    failure = process["errorCode"]
    state = "PASS"
    if process["exitCode"] == 2 and failure is None:
        state, failure = "UNSUPPORTED", "implementation-unsupported"
    elif failure is not None:
        state = "FAIL"
    elif process["exitCode"] != (0 if expected_accept else 1):
        state, failure = "FAIL", "exit-mismatch"
    elif output is None:
        state, failure = "FAIL", "protocol-output"
    elif output.get("spec") != "cem/0.1" or output.get("accept") is not expected_accept:
        state, failure = "FAIL", "decision-mismatch"
    elif output.get("drift") != expected_drift:
        state, failure = "FAIL", "drift-mismatch"
    return {
        "durationNs": process["durationNs"],
        "exitCode": process["exitCode"],
        "failureCode": failure,
        "group": group,
        "name": name,
        "state": state,
    }


def _invoke_case(
    implementation_bytes: bytes,
    implementation_digest: str,
    repository: Path,
    map_bytes: bytes,
    patch_bytes: bytes,
    home: Path,
    group: str,
    case: Mapping[str, Any],
    target: Optional[str] = None,
) -> dict[str, Any]:
    with tempfile.TemporaryDirectory(prefix="case-input-", dir=home.parent) as input_name:
        input_root = Path(input_name)
        executable = _private_implementation(input_root, implementation_bytes)
        private_map = input_root / "change.cem.json"
        private_patch = input_root / "change.patch"
        private_map.write_bytes(map_bytes)
        private_patch.write_bytes(patch_bytes)
        private_map.chmod(0o400)
        private_patch.chmod(0o400)
        argv = [
            str(executable), "verify", "--repository", str(repository),
            "--map", str(private_map), "--patch", str(private_patch),
        ]
        if target is not None:
            argv.extend(("--target", target))
        process, stdout, _stderr = _run_process(
            argv, repository, _minimal_environment(home), CASE_TIMEOUT_SECONDS,
        )
        try:
            inputs_unchanged = (
                _sha256(_read_regular(
                    executable, IMPLEMENTATION_LIMIT, "implementation",
                )) == implementation_digest and
                _read_regular(private_map, MANIFEST_LIMIT, "map") == map_bytes and
                _read_regular(private_patch, ARTIFACT_LIMIT, "patch") == patch_bytes
            )
        except RunnerError:
            inputs_unchanged = False
    if not inputs_unchanged:
        process = dict(process)
        process["errorCode"] = "input-mutated"
    output = None
    output_failure = None
    if process["errorCode"] is None:
        try:
            output = _protocol_output(stdout)
        except RunnerError as error:
            output_failure = error.code
    if output_failure is not None:
        process = dict(process)
        process["errorCode"] = output_failure
    expected_accept = group == "valid" or (group == "drift" and bool(case["accept"]))
    expected_drift: list[dict[str, Any]] = []
    if group == "drift":
        map_document = _strict_object(map_bytes, MANIFEST_LIMIT, "map")
        evidence = map_document.get("evidence")
        if not isinstance(evidence, list) or len(evidence) != 1:
            raise RunnerError("drift-fixture")
        expected_drift = [{
            "evidenceId": evidence[0]["id"],
            "path": evidence[0]["path"],
            "status": case["status"],
            "targetBlobOid": case["targetBlobOid"],
            "targetSpan": case["targetSpan"],
        }]
    return _case_result(
        group, case["name"], process, output, expected_accept, expected_drift,
    )


def _consumer_run(implementation: Path) -> dict[str, Any]:
    run_started = time.monotonic_ns()
    manifest, manifest_digest, artifacts, packet_digest = _manifest_and_artifacts()
    cases: list[dict[str, Any]] = []
    with tempfile.TemporaryDirectory(prefix="cem-interop-consumer-") as temporary_name:
        root = Path(temporary_name)
        home = root / "home"
        home.mkdir(mode=0o700)
        implementation_bytes, implementation_digest = _implementation(implementation)
        repositories = {
            "sha1": _make_repository(root, home, manifest, artifacts, "sha1"),
            "sha256": _make_repository(root, home, manifest, artifacts, "sha256"),
        }
        drift_repositories = [
            _make_drift_repository(root, home, manifest, artifacts, case, index)
            for index, case in enumerate(manifest["drift"])
        ]
        for group in ("valid", "invalid"):
            for case in manifest[group]:
                patch_bytes = _patch_bytes(case, artifacts)
                map_bytes = _artifact_bytes(artifacts, case["map"], "map")
                object_format = case.get("objectFormat", "sha1")
                cases.append(_invoke_case(
                    implementation_bytes, implementation_digest,
                    repositories[object_format], map_bytes, patch_bytes, home,
                    group, case,
                ))
                if time.monotonic_ns() - run_started > int(TOTAL_TIMEOUT_SECONDS * 1_000_000_000):
                    raise RunnerError("total-timeout")
        for index, case in enumerate(manifest["drift"]):
            patch_bytes = _patch_bytes(case, artifacts)
            map_bytes = _artifact_bytes(artifacts, case["map"], "map")
            cases.append(_invoke_case(
                implementation_bytes, implementation_digest,
                drift_repositories[index], map_bytes, patch_bytes, home,
                "drift", case, case["targetRevision"],
            ))
            if time.monotonic_ns() - run_started > int(TOTAL_TIMEOUT_SECONDS * 1_000_000_000):
                raise RunnerError("total-timeout")
    counts = {
        state.lower(): sum(item["state"] == state for item in cases)
        for state in ("PASS", "FAIL", "UNSUPPORTED")
    }
    return {
        "cases": cases,
        "counts": counts,
        "durationNs": time.monotonic_ns() - run_started,
        "implementationSha256": implementation_digest,
        "manifestSha256": manifest_digest,
        "packetSha256": packet_digest,
        "sequence": 0,
        "state": "PASS" if counts == {"pass": 32, "fail": 0, "unsupported": 0} else "FAIL",
    }


def _consumer(observation_path: Path, implementation: Path, retry: bool) -> dict[str, Any]:
    with _observation_lock(observation_path):
        observation, source_digest = _load_observation_with_digest(observation_path)
        runs = observation["runs"]
        if retry and not runs:
            raise RunnerError("retry-without-first-run")
        if runs and not retry:
            raise RunnerError("retry-required")
        if len(runs) >= MAX_RUNS:
            raise RunnerError("run-limit")
        run = _consumer_run(implementation)
        if (observation["doctor"]["manifestSha256"] != run["manifestSha256"] or
                observation["doctor"]["packetSha256"] != run["packetSha256"]):
            raise RunnerError("observation-kit-mismatch")
        run["sequence"] = len(runs) + 1
        runs.append(run)
        observation["state"] = run["state"]
        _atomic_write(
            observation_path,
            observation,
            must_be_absent=False,
            expected_digest=source_digest,
        )
    return observation


def _summary(observation: Mapping[str, Any]) -> dict[str, Any]:
    runs = observation["runs"]
    latest = None if not runs else {
        "counts": runs[-1]["counts"],
        "durationNs": runs[-1]["durationNs"],
        "sequence": runs[-1]["sequence"],
        "state": runs[-1]["state"],
    }
    return {
        "corvintCommit": observation["doctor"]["corvintCommit"],
        "corvintTreeState": observation["doctor"]["corvintTreeState"],
        "doctorState": observation["doctor"]["state"],
        "latestRun": latest,
        "ok": observation["state"] in ("STARTED", "PASS"),
        "profile": observation["profile"],
        "packetSha256": observation["doctor"]["packetSha256"],
        "publication": observation["publication"],
        "runCount": len(runs),
        "state": observation["state"],
    }


def _emit(value: Mapping[str, Any], stream: Any = sys.stdout) -> None:
    payload = json.dumps(value, sort_keys=True, separators=(",", ":"))
    if len(payload.encode("utf-8")) > CAPTURE_LIMIT:
        raise RunnerError("output-too-large")
    print(payload, file=stream)


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    subcommands = parser.add_subparsers(dest="command", required=True)
    subcommands.add_parser("doctor")
    start = subcommands.add_parser("start")
    start.add_argument("--lane", choices=("consumer",), default="consumer")
    start.add_argument("--output", type=Path, required=True)
    consumer = subcommands.add_parser("consumer")
    consumer.add_argument("--implementation", type=Path, required=True)
    consumer.add_argument("--observation", type=Path, required=True)
    consumer.add_argument("--retry", action="store_true")
    inspect = subcommands.add_parser("inspect")
    inspect.add_argument("--observation", type=Path, required=True)
    return parser


def _termination_signal(_signum: int, _frame: object) -> None:
    raise RunnerInterrupted


def main(argv: Optional[Sequence[str]] = None) -> int:
    previous_sigterm = None
    try:
        try:
            previous_sigterm = signal.signal(signal.SIGTERM, _termination_signal)
        except (AttributeError, ValueError):
            previous_sigterm = None
        arguments = _parser().parse_args(argv)
        if arguments.command == "doctor":
            _emit({"ok": True, "profile": "cem-interop-doctor/0", **_doctor()})
            return 0
        if arguments.command == "start":
            _emit(_summary(_start(arguments.output)))
            return 0
        if arguments.command == "consumer":
            observation = _consumer(
                arguments.observation, arguments.implementation, arguments.retry,
            )
            _emit(_summary(observation))
            return 0 if observation["state"] == "PASS" else 1
        _emit(_summary(_load_observation(arguments.observation)))
        return 0
    except (KeyboardInterrupt, RunnerInterrupted):
        _emit({"error": {"code": "interrupted"}, "ok": False}, sys.stderr)
        return 130
    except RunnerError as error:
        _emit({"error": {"code": error.code}, "ok": False}, sys.stderr)
        return 2
    finally:
        if previous_sigterm is not None:
            signal.signal(signal.SIGTERM, previous_sigterm)


if __name__ == "__main__":
    raise SystemExit(main())
