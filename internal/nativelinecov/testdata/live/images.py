# SPDX-License-Identifier: Apache-2.0
"""Reduce original launch events without erasing executable-image gaps.

The existing capture format carries a conservative process-wide conjunction.
The trace retains every image; PID-scoped JaCoCo sessions cannot prove repeated
Java images in one PID independently, so those also remain incomplete.
"""
from pathlib import Path


def capture_images(raw, family, java, exports, info):
    processes = {}
    images = {}
    complete = False
    for line in raw.splitlines():
        f = line.split(' ', 2)
        if f[0] == 'COMPLETE':
            complete = True
        elif f[0] == 'START':
            pid = int(f[1])
            if pid in processes:
                raise ValueError('duplicate process launch')
            processes[pid] = dict(ID=pid, Parent=int(f[2]), Exited=False,
                                  Instrumented=False, Reports=[])
            images[pid] = []
        elif f[0] == 'EXEC':
            pid = int(f[1])
            if pid not in processes:
                raise ValueError('exec without process launch')
            covered = (family == 'jvm' and Path(f[2]) == java
                       and len(exports) == 1
                       and ('Session "' + f[1] + '"').encode() in info
                       and not any(image[0] == f[2] for image in images[pid]))
            reports = ([dict(Format=exports[0][0], Data=exports[0][1])]
                       if covered else [])
            images[pid].append((f[2], covered, reports))
        elif f[0] == 'EXIT' and int(f[1]) in processes:
            processes[int(f[1])]['Exited'] = True
    for pid, p in processes.items():
        p['Instrumented'] = bool(images[pid]) and all(i[1] for i in images[pid])
        p['Reports'] = [report for i in images[pid] for report in i[2]]
    return dict(TreeComplete=complete, Processes=list(processes.values()))
