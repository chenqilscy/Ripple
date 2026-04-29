import { describe, expect, it } from 'vitest'
import type { EdgeItem, NodeItem, NodeSearchResult } from '../src/api/types'
import { buildRelationshipCandidates, relationshipKey } from '../src/utils/relationshipAnalysis'

function node(id: string, lakeId = 'lake-1', state: NodeItem['state'] = 'MIST'): NodeItem {
  return {
    id,
    lake_id: lakeId,
    owner_id: 'user-1',
    content: `node ${id}`,
    type: 'TEXT',
    state,
    position: { x: 0, y: 0, z: 0 },
    created_at: '2026-04-29T00:00:00Z',
    updated_at: '2026-04-29T00:00:00Z',
  }
}

function edge(src: string, dst: string, kind: EdgeItem['kind'] = 'relates'): EdgeItem {
  return {
    id: `${src}-${dst}`,
    lake_id: 'lake-1',
    src_node_id: src,
    dst_node_id: dst,
    kind,
    owner_id: 'user-1',
    created_at: '2026-04-29T00:00:00Z',
  }
}

function deletedEdge(src: string, dst: string): EdgeItem {
  return { ...edge(src, dst), deleted_at: '2026-04-29T01:00:00Z' }
}

function related(nodeId: string, score: number, lakeId = 'lake-1'): NodeSearchResult {
  return { node_id: nodeId, lake_id: lakeId, snippet: `snippet ${nodeId}`, score }
}

describe('关系分析候选构建', () => {
  it('用无向 key 去重关系', () => {
    expect(relationshipKey('b', 'a')).toBe(relationshipKey('a', 'b'))
  })

  it('过滤跨湖、自身、已连线和隐藏节点，并保留最高分候选', () => {
    const candidates = buildRelationshipCandidates(
      'lake-1',
      [node('a'), node('b'), node('c'), node('d', 'lake-1', 'GHOST'), node('x', 'lake-2')],
      [edge('a', 'c', 'summarizes')],
      [
        { sourceNodeId: 'a', related: [related('a', 10), related('b', 3), related('c', 9), related('d', 8), related('x', 7, 'lake-2')] },
        { sourceNodeId: 'b', related: [related('a', 5)] },
      ],
    )

    expect(candidates).toEqual([
      { src: 'b', dst: 'a', score: 5, snippet: 'snippet a' },
    ])
  })

  it('软删除边不阻止重新推荐候选关系', () => {
    const candidates = buildRelationshipCandidates(
      'lake-1',
      [node('a'), node('b')],
      [deletedEdge('a', 'b')],
      [{ sourceNodeId: 'a', related: [related('b', 6)] }],
    )

    expect(candidates).toEqual([
      { src: 'a', dst: 'b', score: 6, snippet: 'snippet b' },
    ])
  })
})
