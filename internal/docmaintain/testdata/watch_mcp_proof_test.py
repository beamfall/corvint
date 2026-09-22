"""Cheap real-child refusal/cleanup regression for the qualification driver."""
import importlib.util
import os
import pathlib
import signal
import subprocess
import sys
import tempfile
import unittest

DRIVER = pathlib.Path(__file__).with_name("watch_mcp_proof.py")


class BoundedClientTest(unittest.TestCase):
    def test_bad_frames_refuse_and_join(self):
        for mode in ("partial", "oversized", "wrong-id", "stderr-overflow"):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                root = pathlib.Path(directory)
                fake = root / "fake.py"
                fake.write_text('''import os,sys,time
open(sys.argv[1],"w").write(str(os.getpid()))
sys.stdin.buffer.readline()
mode=sys.argv[2]
if mode=="partial": os.write(1,b'{"jsonrpc":')
if mode=="oversized": os.write(1,b'x'*(1048576+1))
if mode=="wrong-id": os.write(1,b'{"jsonrpc":"2.0","id":99,"result":{}}\\n')
if mode=="stderr-overflow": os.write(2,b'x'*(1048576+1))
time.sleep(30)
''')
                probe = '''import importlib.util,pathlib,sys
spec=importlib.util.spec_from_file_location("driver",sys.argv[1]);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
child=m.Child([sys.executable,sys.argv[2],sys.argv[3],sys.argv[4]])
try:m.MCP(child,pathlib.Path(sys.argv[5]),.2).request("server/discover",{},"response")
finally:child.close()
'''
                result = subprocess.run([sys.executable, "-c", probe, str(DRIVER), str(fake),
                                         str(root / "pid"), mode, str(root)],
                                        capture_output=True, timeout=5)
                self.assertNotEqual(result.returncode, 0, result.stdout)
                expected = {"partial": b"response-timeout", "oversized": b"child-output-overflow",
                            "wrong-id": b"response-id-or-result-mismatch", "stderr-overflow": b"child-output-overflow"}
                self.assertIn(expected[mode], result.stderr)
                pid = int((root / "pid").read_text())
                with self.assertRaises(ProcessLookupError):
                    os.kill(pid, 0)


if __name__ == "__main__":
    unittest.main()
