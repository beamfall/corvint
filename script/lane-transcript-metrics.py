#!/usr/bin/env python3
# SPDX-License-Identifier: Apache-2.0
"""Aggregate lane-procedure counts from Claude Code subagent transcripts (V1-1629, V1-1630).

usage: script/lane-transcript-metrics.py PROJECT_DIR --since YYYY-MM-DD --until YYYY-MM-DD

PROJECT_DIR is a Claude Code project transcript directory (~/.claude/projects/<slug>). Every
<session>/subagents/agent-*.jsonl whose first record falls in [since, until) is read. Only counts,
durations and agent ids are printed (JSON on stdout); no transcript text is retained.

Per subagent: Bash calls; dogfood-change and dogfood-finish invocations (make targets or the
scripts, not reads such as grep); seconds spent in polling tool calls (Bash with sleep, until/while
loops, --watch, gh run watch or tail -f, plus Monitor calls); tool calls the harness timed out
("Command timed out after", "did not complete within its Ns timeout"), split into those whose
limit was 600 s or more; tool calls that
ran 600 s or longer; and the wall time of single-package `go test` calls (through heavy.sh or not,
offloaded or not; it includes slot wait and compile time). A lane is a subagent that committed, pushed, or ran a dogfood target.
Commands are split at &&, ||, ;, | and newlines; a part that starts with a read or print command
(grep, sed, cat, echo, printf, git log/show/diff, ...) invokes nothing, and every invoking part counts.
"""
import argparse
import datetime as dt
import json
import pathlib
import re
import statistics
import sys

DOGFOOD_CHANGE = re.compile(r"(\bmake\b[^;&|\n]*\bdogfood-change\b|dogfood-change\.sh)")
DOGFOOD_FINISH = re.compile(r"(\bmake\b[^;&|\n]*\bdogfood-finish\b|dogfood-finish\.sh)")
READ_ONLY = re.compile(r"^\s*(grep|rg|sed|cat|head|tail|less|echo|printf|git (log|show|grep|diff))\b")
POLL = re.compile(r"(\bsleep\s+[0-9]|\buntil\b[^\n]*\bdo\b|\bwhile\b[^\n]*\bdo\b|--watch\b|\bgh run watch\b|\btail -f\b)")
DOGFOOD_ANY = re.compile(r"(\bmake\b[^;&|\n]*\bdogfood-(change|finish|check|seal)\b|dogfood-(change|finish|check|seal)\.sh)")
SPLIT = re.compile(r"&&|\|\||;|\||\n")
TIMEOUT = re.compile(r"Command timed out after (?:(?P<m>\d+)m )?(?P<s>\d+)s|timed out at (?P<a>\d+)s|did not complete within its (?P<b>\d+)s timeout")


def ts(record):
    t = record.get("timestamp")
    return dt.datetime.fromisoformat(t.replace("Z", "+00:00")) if t else None


def result_text(block):
    c = block.get("content")
    if isinstance(c, str):
        return c
    if isinstance(c, list):
        return "\n".join(x.get("text", "") for x in c if isinstance(x, dict))
    return ""


CLASSES = [
    (re.compile(r"dogfood-(finish|change|check|seal|batch-check)"), lambda m: "make dogfood-" + m.group(1)),
    (re.compile(r"\bgo test\b[^\n]*?\s(\./[\w./-]*[\w/.-])(?=[\s'\"]|$)(?<!\.log)(?<!\.txt)"), lambda m: "go test " + m.group(1).rstrip("/")),
    (re.compile(r"\bgo (build|vet)\b"), lambda m: "go " + m.group(1)),
    (re.compile(r"\bmake\s+(?:-\S+\s+)*([\w-]+)"), lambda m: "make " + m.group(1)),
    (re.compile(r"\bcodex exec\b"), lambda m: "codex exec"),
    (re.compile(r"\bcorvint-tasks\s+(\w[\w-]*)"), lambda m: "corvint-tasks " + m.group(1)),
    (re.compile(r"\bcorvint\s+(\w[\w-]*)"), lambda m: "corvint " + m.group(1)),
    (re.compile(r"\bgh\s+(\w+)"), lambda m: "gh " + m.group(1)),
    (re.compile(r"\bssh\b"), lambda m: "ssh (remote run)"),
]


def classify(name, command):
    if name != "Bash":
        return name or "unknown"
    for pattern, label in CLASSES:
        m = pattern.search(command)
        if m:
            return label(m)
    return "other Bash"


def invokes(pattern, command):
    """Number of command parts that run pattern (a read or print of it is not a run)."""
    return sum(1 for part in SPLIT.split(command) if pattern.search(part) and not READ_ONLY.search(part))


def invokes_git_write(command):
    """Count Git writes without backtracking across overlapping option/operand forms."""
    count = 0
    for part in SPLIT.split(command):
        if READ_ONLY.search(part):
            continue
        words = part.split()
        index = 0
        while index < len(words) and re.fullmatch(r"\w+=\S*", words[index]):
            index += 1
        if index == len(words) or words[index] != "git":
            continue
        index += 1
        while index < len(words):
            word = words[index]
            option, separator, operand = word.partition("=")
            if word in ("-c", "-C"):
                index += 2  # These options consume exactly one operand.
            elif re.fullmatch(r"--?[\w-]+", option) and (not separator or operand):
                index += 1
            else:
                break
        if index < len(words) and re.match(r"(?:commit|push)\b", words[index]):
            count += 1
    return count


def go_test_packages(command):
    """The ./ package operands of each `go test` part, one list per part."""
    out = []
    for part in SPLIT.split(command):
        m = re.search(r"\bgo test\b", part)
        if not m or READ_ONLY.search(part):
            continue
        ops = [t.strip("'\"").rstrip("/") for t in part[m.end():].split()]
        out.append([t for t in ops if t.startswith("./") and "..." not in t and not t.endswith((".log", ".txt"))])
    return out


def scan(path):
    calls = {}  # tool_use_id -> (name, command, started)
    row = {"agent": path.stem.removeprefix("agent-"), "bashCalls": 0, "dogfoodChange": 0, "dogfoodFinish": 0,
           "pollCalls": 0, "pollSeconds": 0.0, "harnessTimeouts": 0, "harnessTimeouts600s": 0, "calls600s": 0, "lane": False}
    long = []  # tool calls of 600 s or more, as a command class and duration only
    focused = []  # single-package go test calls: package, seconds, through heavy.sh, offloaded
    first = last = None
    with path.open(encoding="utf-8") as fh:
        for line in fh:
            try:
                rec = json.loads(line)
            except json.JSONDecodeError:
                continue
            t = ts(rec)
            if t:
                first = first or t
                last = t
            msg = rec.get("message")
            if not isinstance(msg, dict) or not isinstance(msg.get("content"), list):
                continue
            for block in msg["content"]:
                if not isinstance(block, dict):
                    continue
                if block.get("type") == "tool_use":
                    inp = block.get("input") or {}
                    cmd = inp.get("command", "") if block.get("name") == "Bash" else ""
                    calls[block.get("id")] = (block.get("name"), cmd, t)
                    if block.get("name") == "Bash":
                        row["bashCalls"] += 1
                        row["dogfoodChange"] += invokes(DOGFOOD_CHANGE, cmd)
                        row["dogfoodFinish"] += invokes(DOGFOOD_FINISH, cmd)
                        row["lane"] = row["lane"] or bool(invokes_git_write(cmd) or invokes(DOGFOOD_ANY, cmd))
                elif block.get("type") == "tool_result" and block.get("tool_use_id") in calls:
                    name, cmd, started = calls.pop(block["tool_use_id"])
                    secs = (t - started).total_seconds() if t and started else 0.0
                    if name == "Monitor" or (name == "Bash" and POLL.search(cmd)):
                        row["pollCalls"] += 1
                        row["pollSeconds"] += secs
                    hit = TIMEOUT.search(result_text(block))
                    if hit:
                        row["harnessTimeouts"] += 1
                        g = hit.groupdict()
                        limit = int(g["a"] or g["b"] or 0) or int(g["m"] or 0) * 60 + int(g["s"])
                        row["harnessTimeouts600s"] += limit >= 600
                    if name == "Bash":
                        runs = go_test_packages(cmd)
                        if len(runs) == 1 and len(set(runs[0])) == 1:
                            focused.append({"package": runs[0][0], "seconds": round(secs),
                                            "heavy": "heavy.sh" in cmd,
                                            "offloaded": "heavy.sh: running on" in result_text(block)})
                    if secs >= 600:
                        row["calls600s"] += 1
                        long.append({"class": classify(name, cmd), "seconds": round(secs)})
    row["pollSeconds"] = round(row["pollSeconds"])
    return first, last, row, long, focused


def by_class(long):
    groups = {}
    for c in long:
        groups.setdefault(c["class"], []).append(c["seconds"])
    return [{"class": k, "count": len(v), "medianSeconds": statistics.median(v), "maxSeconds": max(v)}
            for k, v in sorted(groups.items(), key=lambda kv: -len(kv[1]))]


def focused_summary(calls):
    def med(rs):
        return statistics.median([c["seconds"] for c in rs]) if rs else None
    groups = {}
    for c in calls:
        groups.setdefault(c["package"], []).append(c)
    return {
        "calls": len(calls), "medianSeconds": med(calls),
        "viaHeavy": sum(c["heavy"] for c in calls), "offloaded": sum(c["offloaded"] for c in calls),
        "medianSecondsViaHeavyLocal": med([c for c in calls if c["heavy"] and not c["offloaded"]]),
        "medianSecondsOffloaded": med([c for c in calls if c["offloaded"]]),
        "medianSecondsWithoutHeavy": med([c for c in calls if not c["heavy"]]),
        "byPackage": [{"package": k, "count": len(v), "medianSeconds": med(v)}
                      for k, v in sorted(groups.items(), key=lambda kv: -len(kv[1])) if len(v) >= 3][:15],
    }


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("project_dir")
    ap.add_argument("--since", required=True)
    ap.add_argument("--until", required=True)
    a = ap.parse_args()
    lo = dt.datetime.fromisoformat(a.since).replace(tzinfo=dt.timezone.utc)
    hi = dt.datetime.fromisoformat(a.until).replace(tzinfo=dt.timezone.utc)
    rows, long, focused = [], [], []
    for p in sorted(pathlib.Path(a.project_dir).glob("*/subagents/agent-*.jsonl")):
        first, last, row, calls, tests = scan(p)
        if first and lo <= first < hi:
            long += calls
            focused += tests
            row["start"] = first.strftime("%Y-%m-%dT%H:%MZ")
            row["hours"] = round((last - first).total_seconds() / 3600, 2)
            rows.append(row)
    lanes = [r for r in rows if r["lane"]]
    dogfood_lanes = [r for r in lanes if r["dogfoodChange"] or r["dogfoodFinish"]]

    def total(key, rs):
        return sum(r[key] for r in rs)

    out = {
        "profile": "corvint-lane-transcript-metrics/0",
        "window": {"since": a.since, "until": a.until},
        "subagents": len(rows),
        "lanes": len(lanes),
        "lanesRunningDogfood": len(dogfood_lanes),
        "dogfoodChangePerDogfoodLane": {
            "median": statistics.median([r["dogfoodChange"] for r in dogfood_lanes]) if dogfood_lanes else None,
            "max": max([r["dogfoodChange"] for r in dogfood_lanes], default=None),
            "total": total("dogfoodChange", dogfood_lanes)},
        "dogfoodChangePlusFinishPerDogfoodLane": {
            "median": statistics.median([r["dogfoodChange"] + r["dogfoodFinish"] for r in dogfood_lanes]) if dogfood_lanes else None},
        "pollingHoursAllSubagents": round(total("pollSeconds", rows) / 3600, 3),
        "pollCallsAllSubagents": total("pollCalls", rows),
        "harnessTimeoutsAllSubagents": total("harnessTimeouts", rows),
        "harnessTimeouts600sAllSubagents": total("harnessTimeouts600s", rows),
        "harnessTimeouts600sLanes": total("harnessTimeouts600s", lanes),
        "toolCalls600sAllSubagents": total("calls600s", rows),
        "toolCalls600sByClass": by_class(long),
        "focusedGoTests": focused_summary(focused),
        "perSubagent": rows,
    }
    json.dump(out, sys.stdout, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
