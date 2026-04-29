import type { EdgeItem, NodeItem, NodeSearchResult } from '../api/types'

export interface RelatedNodeBatch {
  sourceNodeId: string
  related: NodeSearchResult[]
}

export interface RelationshipCandidate {
  src: string
  dst: string
  score: number
  snippet: string
}

export function relationshipKey(a: string, b: string): string {
  return [a, b].sort().join('::')
}

export function buildRelationshipCandidates(
  lakeId: string,
  nodes: NodeItem[],
  edges: EdgeItem[],
  batches: RelatedNodeBatch[],
): RelationshipCandidate[] {
  const nodeIds = new Set(
    nodes
      .filter(n => n.state !== 'ERASED' && n.state !== 'GHOST')
      .map(n => n.id),
  )
  const known = new Set(
    edges
      .filter(e => !e.deleted_at)
      .map(e => relationshipKey(e.src_node_id, e.dst_node_id)),
  )
  const candidates = new Map<string, RelationshipCandidate>()

  for (const batch of batches) {
    if (!nodeIds.has(batch.sourceNodeId)) continue
    for (const item of batch.related) {
      if (item.node_id === batch.sourceNodeId) continue
      if (item.lake_id !== lakeId || !nodeIds.has(item.node_id)) continue
      const key = relationshipKey(batch.sourceNodeId, item.node_id)
      if (known.has(key)) continue
      const next: RelationshipCandidate = {
        src: batch.sourceNodeId,
        dst: item.node_id,
        score: item.score,
        snippet: item.snippet,
      }
      const previous = candidates.get(key)
      if (!previous || next.score > previous.score) candidates.set(key, next)
    }
  }

  return [...candidates.values()].sort((a, b) => b.score - a.score)
}
