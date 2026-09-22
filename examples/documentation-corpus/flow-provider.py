#!/usr/bin/env python3
"""Independent example consumer: authored screens/flows -> corpus provider JSON.

No Corvint imports, source scanning, network calls or invented journey steps.
Input anchors and ordered steps must have been collected explicitly.
"""
import json
import sys


def evidence(anchors):
    return {
        "derivation": "declared", "trust": "generated", "reported_trust": "",
        "state": "supported" if anchors else "unknown", "freshness": "fresh",
        "anchors": anchors, "unknown": "" if anchors else "flow evidence not collected",
        "limitations": ["Authored flow declaration; navigation and passing tests do not establish assertions."],
    }


def compile_flow(source):
    provider = source["provider"]
    subjects, claims, relations, journeys = [], [], [], []
    for screen in source["screens"]:
        identity = provider + ":screen:" + screen["id"]
        proof = evidence(screen["anchors"])
        subjects.append({"id": identity, "kind": "ui_surface", "name": screen["name"],
                         "provider": provider, "evidence": proof})
        claims.append({"id": identity + ":route", "subject": identity,
                       "text": "Declared route: " + screen["route"], "provider": provider, "evidence": proof})
    for flow in source["flows"]:
        identity = provider + ":flow:" + flow["id"]
        subject = provider + ":screen:" + flow["screen"]
        proof = evidence(flow["anchors"])
        steps = []
        for step in flow["steps"]:
            steps.append({"id": step["id"], "action": step["action"], "operation": step["operation"],
                          "expected": step["expected"], "observation": step.get("observation", ""),
                          "evidence": evidence(step["anchors"])})
        journeys.append({"id": identity, "subject": subject, "provider": provider,
                         "status": "generated_not_verified" if steps else "missing_journey",
                         "preconditions": flow["preconditions"], "cleanup": flow["cleanup"],
                         "steps": steps, "evidence": proof})
    capabilities = [{"name": name, "state": "present", "reason": "Explicit authored example input"}
                    for name in ("subjects", "claims", "relations", "journeys")]
    return {"schema": "corvint-corpus-provider/1", "id": provider, "version": "1",
            "source": source["repository"], "subjects": subjects, "claims": claims,
            "relations": relations, "journeys": journeys, "observations": [], "capabilities": capabilities}


if __name__ == "__main__":
    raw = sys.stdin.buffer.read((4 << 20) + 1)
    if len(raw) > 4 << 20:
        raise SystemExit("example input exceeds 4 MiB")
    output = json.dumps(compile_flow(json.loads(raw)), ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    if len(output.encode()) + 1 > 4 << 20:
        raise SystemExit("example output exceeds 4 MiB")
    print(output)
