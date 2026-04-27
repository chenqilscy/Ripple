# TD-04 WS Cross-Machine Load Test Report

- Time: 20260427-142125
- Client: remote Linux host `fn.cky` (`docs/dev/ws_connect_linux_amd64`)
- Base: http://fn.cky:18000
- WS URL: ws://127.0.0.1:18000/api/v1/lakes/1f44f971-0660-426e-8f67-02504e0dfea5/ws
- Concurrency: 1000
- Hold: 30s
- Lake ready gate: polled `GET /api/v1/lakes/{id}` until `READY=1` before opening WS connections

## ws_connect output

\\\
=== WS Connect Stress ===
URL:           ws://127.0.0.1:18000/api/v1/lakes/1f44f971-0660-426e-8f67-02504e0dfea5/ws
Concurrent:    1000
Hold:          30s
Total time:    30.203s
Dial OK:       1000
Dial failed:   0
Alive @ mid:   1000 (100.0% of OK)
Handshake p50: 116.479ms
Handshake p95: 175.797ms
Handshake p99: 192.333ms

\\\

## Conclusion

- This clean rerun confirms the readiness-gated flow meets the strict Phase 13 WS gate: success rate `100%`, `p95 < 200ms`, `p99 < 200ms`.
- Together with the earlier Linux diagnosis run, the TD-04 incident is now closed as a test-flow issue rather than a backend WS defect.
- Remaining Phase 13 work shifts from WS pressure debugging to failure drills and rollback acceptance.