# TD-04 WS Cross-Machine Load Test Report

- Time: 20260427-112224
- Base: http://fn.cky:18000
- WS URL: ws://fn.cky:18000/api/v1/lakes/a63e6d1a-337a-4fb2-a098-03e20160a73d/ws
- Concurrency: 1000
- Hold: 30s
- Elapsed: 33 s

## ws_connect output

\\\
=== WS Connect Stress ===
URL:           ws://fn.cky:18000/api/v1/lakes/a63e6d1a-337a-4fb2-a098-03e20160a73d/ws
Concurrent:    1000
Hold:          30s
Total time:    30.193s
Dial OK:       1000
Dial failed:   0
Alive @ mid:   1000 (100.0% of OK)
Handshake p50: 132.543ms
Handshake p95: 183.743ms
Handshake p99: 189.327ms

\\\

## Metrics snapshots

- before: C:\Users\chenq\AppData\Local\Temp\td04-metrics-before-20260427-112224.txt
- after: C:\Users\chenq\AppData\Local\Temp\td04-metrics-after-20260427-112224.txt

Use Compare-Object or your metrics dashboard to inspect the delta.

## Checklist

- [ ] All dial attempts succeeded
- [ ] Backend RSS growth stayed within budget
- [ ] Peak CPU stayed within budget
- [ ] /metrics shows ripple_ws_connections near the target concurrency
