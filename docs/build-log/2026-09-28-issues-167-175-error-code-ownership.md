# Issue167/175 emitted-code ownership

PR313 CI found ten emitted error codes that the selected local documentation checks had not audited. The ownership gate was missing from the local check set; the source tests and the earlier sealed change remain recorded at their original commits.

The owning BBF, PWP and AFU specifications now enumerate those existing refusal conditions. No runtime code, wire value, assertion or timeout changes. The repair runs the complete CI documentation job, use-case receipt checks and focused spec-index validation. It is isolated from the completed source enrollment and bound against its seal commit `0c0301fe53b70911c87483964bce5a21fbfe2ab1`.
