#!/usr/bin/env python3
import subprocess
import sys
import time


subprocess.Popen([sys.executable, "-c", "import time; time.sleep(300)"])
time.sleep(300)

