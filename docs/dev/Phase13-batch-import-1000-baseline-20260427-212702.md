# Phase13 Batch Import 1000 Baseline

- Time: 20260427-212702
- Base: http://fn.cky:18000
- Lake ID: 96e64ccb-d288-4bfc-805c-c02f8a87e962
- Node count: 1000
- Created: 1000
- Elapsed: 0.2535s
- Threshold: 5s
- Result: PASS

## Request

- Endpoint: POST /api/v1/lakes/{id}/nodes/batch
- Payload nodes: 1000
- Type: TEXT

## Acceptance

- created == 1000
- elapsed <= 5s
