# TD-04 WS Cross-Machine Load Test Report

- Time: 20260427-140045
- Base: http://fn.cky:18000
- WS URL: ws://fn.cky:18000/api/v1/lakes/b5ce9898-21ff-4bb6-95e0-9f7b1d8b695f/ws
- Concurrency: 1000
- Hold: 30s
- Elapsed: 41 s

## ws_connect output

\\\
=== WS Connect Stress ===
URL:           ws://fn.cky:18000/api/v1/lakes/b5ce9898-21ff-4bb6-95e0-9f7b1d8b695f/ws
Concurrent:    1000
Hold:          30s
Total time:    37.12s
Dial OK:       516
Dial failed:   484
Alive @ mid:   516 (100.0% of OK)
Handshake p50: 3.078773s
Handshake p95: 7.099752s
Handshake p99: 7.106061s

\\\

## Metrics snapshots

- before: C:\Users\chenq\AppData\Local\Temp\td04-metrics-before-20260427-140045.txt
- after: C:\Users\chenq\AppData\Local\Temp\td04-metrics-after-20260427-140045.txt

Use Compare-Object or your metrics dashboard to inspect the delta.

## Checklist

- [ ] All dial attempts succeeded
- [ ] Backend RSS growth stayed within budget
- [ ] Peak CPU stayed within budget
- [ ] /metrics shows ripple_ws_connections near the target concurrency
