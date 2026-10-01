#!/usr/bin/env python3
"""Exercise compiled companions and a disposable, nonfixture native Tasks queue.

All writes and local Git integration are confined to a new output directory.
The test source is trusted executable code. This is feasibility evidence only.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import subprocess


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True,
                      separators=(",", ":")).encode() + b"\n"


def sha(data):
    return hashlib.sha256(data).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("source", "core", "tasks", "experiments", "go", "qualification", "out"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    out = Path(args.out).resolve()
    out.mkdir(mode=0o700)  # Refuse an existing directory; never reset a store.
    primary, candidate = out / "repository", out / "candidate"
    primary.mkdir()
    step = 0
    environment = dict(os.environ, CORVINT_TASKS_ACTOR="cem-experiment-demo")

    def call(argv, cwd=primary, data=None, expected=0):
        nonlocal step
        step += 1
        result = subprocess.run([str(x) for x in argv], cwd=cwd, input=data,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                env=environment, timeout=1800, check=False)
        (out / f"{step:03d}.stdout").write_bytes(result.stdout)
        (out / f"{step:03d}.stderr").write_bytes(result.stderr)
        if result.returncode != expected:
            raise RuntimeError(f"step {step} returned {result.returncode}, expected {expected}; inspect private logs")
        return result.stdout

    def git(*argv, cwd=primary):
        return call(["git", *argv], cwd).decode().strip()

    def task(*argv, cwd=primary, payload=None):
        return json.loads(call([args.tasks, *argv], cwd, payload))

    def commit(message):
        git("add", ".")
        git("commit", "-m", message)
        return git("rev-parse", "HEAD")

    git("init", "-b", "main")
    git("config", "user.name", "CEM experiment demo")
    git("config", "user.email", "cem-experiment@example.invalid")
    (primary / ".gitignore").write_text(".corvint/\n")
    (primary / "sample").mkdir()
    (primary / "sample/go.mod").write_text("module example.invalid/clamp\n\ngo 1.27.1\n")
    (primary / "sample/clamp.go").write_text("package clamp\n\nfunc Clamp(value int) int { return value }\n")
    criteria = ["Negative inputs clamp to zero.", "Inputs between zero and ten are preserved.",
                "Inputs above ten clamp to ten."]
    (primary / "contract.txt").write_text("Owner acceptance criteria\n" + "\n".join(criteria) + "\n")
    oracle = '''package clamp

import "testing"

func TestClampRepair(t *testing.T) {
    if Clamp(-1) != 0 { t.Fatal("CEX_REPAIR: negative input") }
}
func TestClampPreservation(t *testing.T) {
    if Clamp(5) != 5 { t.Fatal("CEX_PRESERVE: interior input") }
}
func TestClampNewBehavior(t *testing.T) {
    if Clamp(11) != 10 { t.Fatal("CEX_NEW: upper bound") }
}
'''
    (primary / "sample/oracle_test.go").write_text(oracle)
    templates = Path(args.source) / "internal/tasks/cli/testdata/external-agents"
    queue = json.loads((templates / "queue.json").read_bytes())
    queue.update(queueId="queue:experiment:main", repositoryAuthorityId="repo:experiment", prefix="CEX")
    policy = json.loads((templates / "policy.json").read_bytes())
    policy["gates"][0].update(
        argv=[str(Path(args.experiments).resolve()), "gate", "--repo", str(candidate),
              "--plan", str(out / "plan/plan.json"), "--receipt", str(out / "run/receipt.json")],
        timeoutSeconds="300")
    (primary / ".taskman").mkdir()
    (primary / ".taskman/queue.json").write_bytes(canonical(queue))
    (primary / ".taskman/policy.json").write_bytes(canonical(policy))
    commit("demo: declare owner criteria and native queue")
    task("init", "--request-id", "demo-init")
    task("cutover", "--execution", "--decision", "disposable-demo-only",
         "--qualification", str(Path(args.qualification).resolve()))
    payload = json.loads((templates / "ticket-create.json").read_bytes())
    payload.update(title="Clamp criterion experiment", acceptanceCriteria=criteria,
                   kind="FEATURE", effects={"coverage": "QUALIFIED", "externalUnbounded": False,
                                             "resources": [], "touchPaths": ["sample/clamp.go"]})
    payload["source"]["sourceQueueId"] = queue["queueId"]
    task("ticket", "create", "--request-id", "demo-create", "--payload-stdin", payload=canonical(payload))
    base = commit("demo: retain native execution cutover and ticket")
    ticket_id = "ticket:experiment:main:CEX-0001"
    controls = []
    bodies = ["if value > 10 { return 10 }; return value",
              "if value < 0 { return 0 }; if value > 10 { return 10 }; return 4",
              "if value < 0 { return 0 }; return value"]
    for index, body in enumerate(bodies):
        git("checkout", "-b", f"control-{index}", base)
        (primary / "sample/clamp.go").write_text(f"package clamp\n\nfunc Clamp(value int) int {{ {body} }}\n")
        controls.append(commit(f"demo: registered wrong control {index}"))
    git("checkout", "-b", "candidate", base)
    (primary / "sample/clamp.go").write_text(
        "package clamp\n\nfunc Clamp(value int) int {\n"
        "    if value < 0 { return 0 }\n    if value > 10 { return 10 }\n    return value\n}\n")
    target = commit("demo: satisfy clamp criteria")
    tree = git("rev-parse", "HEAD^{tree}")
    git("checkout", "main")
    git("worktree", "add", str(candidate), "candidate")
    claim = task("claim", ticket_id, "--holder", "experiment-demo", "--request-id", "demo-claim",
                 "--branch", "candidate", "--base", base, "--lease-minutes", "30",
                 "--scope", "sample/clamp.go", cwd=candidate)
    attempt = claim["items"][0]
    # Claim results retain the exact native attempt identity and generation.
    attempt_id, generation = attempt["attemptId"], attempt["generation"]
    call([args.core, "cem", "prepare", "--base", base, "--target", target], candidate)
    cem = candidate / ".corvint/change.cem.json"
    hunk = json.loads(cem.read_bytes())["hunks"][0]["id"]
    request = {"schema": "cem-criterion-experiments/0", "base": base, "target": target,
               "cem": str(cem), "ticket": ticket_id, "attempt": attempt_id, "module": "sample",
               "goBinary": str(Path(args.go).resolve()), "goSha256": sha(Path(args.go).read_bytes()),
               "timeoutSeconds": 120, "criteria": []}
    tests = ["TestClampRepair", "TestClampPreservation", "TestClampNewBehavior"]
    markers = ["CEX_REPAIR", "CEX_PRESERVE", "CEX_NEW"]
    for index, text in enumerate(criteria):
        request["criteria"].append({
            "index": index, "acceptanceSha256": sha(text.encode()),
            "relation": ["repair", "preservation", "new-behavior"][index],
            "test": tests[index], "package": ".", "assertion": markers[index], "hunks": [hunk],
            "oracle": {"commit": base, "path": "sample/oracle_test.go"},
            "authority": {"reviewer": "disposable-demo-owner", "anchorCommit": base,
                          "anchorPath": "contract.txt", "anchorSha256": sha((primary / "contract.txt").read_bytes()),
                          "independent": True}, "controls": [controls[index]]})
    request_path = out / "request.json"
    request_path.write_bytes(canonical(request))
    planned = json.loads(call([args.experiments, "plan", "--repo", candidate,
                              "--request", request_path, "--out", out / "plan"]))
    call([args.experiments, "run", "--repo", candidate, "--plan", out / "plan/plan.json",
          "--approve", planned["planSha256"], "--experimental", "--trusted-local", "--out", out / "run"])
    verification = json.loads(call([args.experiments, "verify", "--repo", candidate,
                                   "--plan", out / "plan/plan.json", "--receipt", out / "run/receipt.json"]))
    task("submit", "--attempt", attempt_id, "--generation", generation,
         "--request-id", "demo-submit", "--tree", tree, cwd=candidate)
    dirty = candidate / "demo-owned-untracked"
    dirty.write_text("force clean-candidate refusal\n")
    try:
        call([args.experiments, "gate", "--repo", candidate, "--plan", out / "plan/plan.json",
              "--receipt", out / "run/receipt.json"], expected=1)
    finally:
        dirty.unlink()
    gate = task("gate", "run", "--attempt", attempt_id, "--generation", generation,
                "--request-id", "demo-gate", "--gate", "verify", "--worktree", str(candidate), cwd=candidate)
    git("merge", "--ff-only", "candidate")  # Disposable repository only.
    completion = task("complete", "--attempt", attempt_id, "--generation", generation,
                      "--request-id", "demo-complete", "--commit", target)
    readback = task("ticket", "show", ticket_id)
    audit = task("receipt", "audit")
    # Historical verification is independent of the attempt becoming terminal.
    historical = json.loads(call([args.experiments, "verify", "--repo", candidate,
                                 "--plan", out / "plan/plan.json", "--receipt", out / "run/receipt.json"]))
    call([args.experiments, "gate", "--repo", candidate, "--plan", out / "plan/plan.json",
          "--receipt", out / "run/receipt.json"], expected=1)
    record = readback["items"][0]["record"]
    task("ticket", "reopen", "--target", ticket_id, "--expected-revision", record["revision"],
         "--request-id", "demo-reopen", "--payload-stdin", payload=canonical({"reason": "exercise historical acceptance change"}))
    reopened = task("ticket", "show", ticket_id)["items"][0]["record"]
    changed_criteria = [*criteria]
    changed_criteria[0] = "Negative inputs clamp to zero, including the minimum integer."
    task("ticket", "refine", "--target", ticket_id, "--expected-revision", reopened["revision"],
         "--request-id", "demo-refine", "--payload-stdin", payload=canonical({"acceptanceCriteria": changed_criteria}))
    historical_changed = json.loads(call([args.experiments, "verify", "--repo", candidate,
                                         "--plan", out / "plan/plan.json", "--receipt", out / "run/receipt.json"]))
    call([args.experiments, "gate", "--repo", candidate, "--plan", out / "plan/plan.json",
          "--receipt", out / "run/receipt.json"], expected=1)
    final_readback, final_audit = task("ticket", "show", ticket_id), task("receipt", "audit")
    summary = {"profile": "cem-criterion-experiments-demo/0", "fixture": False,
               "base": base, "target": target, "controls": controls, "plan": planned,
               "verification": verification, "gate": gate, "completion": completion,
               "readback": readback, "audit": audit, "historicalAfterCompletion": historical,
               "historicalAfterAcceptanceChange": historical_changed,
               "acceptanceChangeReadback": final_readback, "finalAudit": final_audit,
               "dirtyCandidateGateRefused": True, "terminalGateRefused": True,
               "acceptanceChangedGateRefused": True,
               "staleCaseLimit": "acceptance change occurs after terminal completion; isolated binding mismatches have focused unit coverage",
               "externalOutcomeEvaluation": "NOT_RUN"}
    (out / "summary.json").write_bytes(canonical(summary))
    print(json.dumps({"state": "DISPOSABLE_LIFECYCLE_OBSERVED", "summary": str(out / "summary.json"),
                      "sha256": sha(canonical(summary))}))


if __name__ == "__main__":
    main()
