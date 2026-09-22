#!/usr/bin/env python3
import os


chunk = b"x" * 65536
for _ in range(64):
    os.write(1, chunk)

