# TD-04 WS Cross-Machine Load Test Report

- Time: 20260427-142214
- Client: remote Linux host `fn.cky` (`docs/dev/ws_connect_linux_amd64`)
- Base: http://fn.cky:18000
- WS URL: ws://127.0.0.1:18000/api/v1/lakes/98720714-7e59-4287-93a2-a679fc29874e/ws
- Concurrency: 1000
- Hold: 30s
- Lake ready gate: polled `GET /api/v1/lakes/{id}` until `READY=1` before opening WS connections

## ws_connect output

\\\
=== WS Connect Stress ===
URL:           ws://127.0.0.1:18000/api/v1/lakes/98720714-7e59-4287-93a2-a679fc29874e/ws
Concurrent:    1000
Hold:          30s
Total time:    30.233s
Dial OK:       1000
Dial failed:   0
Alive @ mid:   1000 (100.0% of OK)
Handshake p50: 128.175ms
Handshake p95: 206.634ms
Handshake p99: 217.664ms

\\\

## Conclusion

- The earlier `739/1000` and `516/1000` degraded windows were not caused by Linux client instability or a missing WS route.
- Root cause: the load test dialed `/api/v1/lakes/{id}/ws` immediately after lake creation, while `LakeWS` first checks `Lakes.Get(...)`; during the outbox projection delay this returned `404`, so part of the run failed before the lake became visible.
- After adding a readiness gate and re-running from a remote Linux client, connection success returned to `1000 / 1000` with no dial failures.
- This run still leaves a marginal latency gap against the strict Phase 13 gate (`p95 206.634ms` vs target `< 200ms`), so TD-04 status should remain `△` until one more clean rerun lands below the latency threshold.