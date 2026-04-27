# TD-04 WS Cross-Machine Load Test Report

- Time: 20260427-135833
- Base: http://fn.cky:18000
- WS URL: ws://fn.cky:18000/api/v1/lakes/11d39365-fcd3-48d7-81da-96e6fafa715d/ws
- Concurrency: 1000
- Hold: 30s
- Elapsed: 43 s

## ws_connect output

\\\
=== WS Connect Stress ===
URL:           ws://fn.cky:18000/api/v1/lakes/11d39365-fcd3-48d7-81da-96e6fafa715d/ws
Concurrent:    1000
Hold:          30s
Total time:    37.119s
Dial OK:       739
Dial failed:   261
Alive @ mid:   739 (100.0% of OK)
Handshake p50: 1.072527s
Handshake p95: 7.109671s
Handshake p99: 7.114462s

\\\

## Metrics snapshots

- before: C:\Users\chenq\AppData\Local\Temp\td04-metrics-before-20260427-135833.txt
- after: C:\Users\chenq\AppData\Local\Temp\td04-metrics-after-20260427-135833.txt

## Resource sampling

- sampler log: `docs/dev/td04-sample-20260427-135833.log`
- backend CPU peak: `2.54%`
- backend RSS peak: `220.1 MiB` (baseline about `210.2 MiB`, delta about `+9.9 MiB`)
- `/metrics` peak `ripple_ws_connections`: `739`
- note: this sampled rerun did not reproduce the earlier 1000/1000 baseline and plateaued at `739` live connections; a later immediate rerun dropped to `516/1000`, so these peak numbers should be treated as a degraded-window sample rather than the final acceptance baseline.

Use Compare-Object or your metrics dashboard to inspect the delta.

## Checklist

- [ ] All dial attempts succeeded
- [ ] Backend RSS growth stayed within budget
- [ ] Peak CPU stayed within budget
- [ ] /metrics shows ripple_ws_connections near the target concurrency
