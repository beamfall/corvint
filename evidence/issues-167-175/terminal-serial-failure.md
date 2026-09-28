# Serial terminal failure and PID publication repair

The serial full Go run at `1bc4d48a82bf3bff58a38704838d6d9f954daa0a` failed only `TestContainedRunnerKillsDescendantOnCancellation` in `conformance/release-artifact-v0`. Every other package passed. The failure reported a 30-second startup timeout after only 0.01 seconds.

The polling loop broke as soon as the PID file existed and ignored `strconv.Atoi` errors. `os.WriteFile` can expose the file before its contents are written, so an empty observation became PID zero and the test failed immediately. The poll now continues until a positive PID parses successfully, retaining the same deadline and cancellation/descendant assertions. Production code is unchanged.

The existing cancellation regression passed 100 consecutive executions after this repair (`pid-poll-regression.log`). The full failure log remains in the private task checkpoint as `serial-go-test-stdout.log`; its SHA-256 is `2ffde96891ea904a4a3c00ffc68498d03fbbd9e08c362ef69f0686843e5c48e1`. This failure is not acceptance evidence. The next terminal run must bind the repaired test source; prior successful source checks are not substituted for the full enrolled check.
