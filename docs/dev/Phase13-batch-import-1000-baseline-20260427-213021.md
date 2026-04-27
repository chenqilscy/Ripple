# Phase13 Batch Import 1000 Baseline

- Time: 20260427-213021
- Base: http://fn.cky:18000
- Lake ID: 0acdf1f6-a4f3-4a3f-98d5-e00146f47441
- Node count: 1000
- Created: 1000
- Elapsed: 0.1506s
- Threshold: 5s
- Result: PASS

## Request

- Endpoint: POST /api/v1/lakes/{id}/nodes/batch
- Payload nodes: 1000
- Type: TEXT

## Acceptance

- created == 1000
- elapsed <= 5s
