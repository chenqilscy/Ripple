"""Graph Analysis Service · Path / Clustering / Planning

实现参考：
- G3 §一 路径分析  §二 聚类算法  §四 规划建议
"""

import uuid
from typing import Any

from app.core.db import get_neo4j
from app.core.logging import logger

_CLUSTER_COLORS = [
    "#4ecdc4",
    "#ff6b6b",
    "#a8e6cf",
    "#ffd93d",
    "#74b9ff",
    "#fd79a8",
    "#b2bec3",
    "#00b894",
]

# Edge types to traverse for path finding and clustering
EDGE_TYPES = ["RELATES_TO", "derives", "refines", "opposes", "groups"]


# ---------------------------------------------------------------------------
# Union-Find helper for clustering
# ---------------------------------------------------------------------------


class _UnionFind:
    """Simple Union-Find with path compression."""

    def __init__(self) -> None:
        self.parent: dict[str, str] = {}
        self.rank: dict[str, int] = {}

    def make_set(self, x: str) -> None:
        if x not in self.parent:
            self.parent[x] = x
            self.rank[x] = 0

    def find(self, x: str) -> str:
        if self.parent.get(x, x) != x:
            self.parent[x] = self.find(self.parent[x])
        return self.parent.get(x, x)

    def union(self, x: str, y: str) -> None:
        rx, ry = self.find(x), self.find(y)
        if rx == ry:
            return
        if self.rank[rx] < self.rank[ry]:
            self.parent[rx] = ry
        elif self.rank[rx] > self.rank[ry]:
            self.parent[ry] = rx
        else:
            self.parent[ry] = rx
            self.rank[rx] += 1


# ---------------------------------------------------------------------------
# 1. BFS shortest path
# ---------------------------------------------------------------------------


async def compute_path(source_id: str, target_id: str) -> dict[str, Any] | None:
    """BFS shortest path between two nodes in Neo4j.

    Traverses RELATES_TO / derives / refines / opposes / groups edges,
    excludes deleted nodes, and limits search to 10 hops.

    Returns a dict with source_id, target_id, nodes[], edges[], total_steps,
    or None if no path is found.
    """
    driver = get_neo4j()
    async with driver.session() as s:
        # Verify both endpoints exist and are not deleted
        q_check = await s.run(
            """MATCH (n:Node {id:$src}), (m:Node {id:$tgt})
               WHERE n.state IN ['DROP','FROZEN'] AND m.state IN ['DROP','FROZEN']
               RETURN n.id AS src_id, m.id AS tgt_id""",
            src=source_id,
            tgt=target_id,
        )
        if not await q_check.single():
            return None

        # BFS up to 10 hops using Cypher
        # We use a pattern that collects all paths up to length 10
        q_path = await s.run(
            f"""MATCH path = (start:Node {{id:$src}})-[
                   RELATES_TO|derives|refines|opposes|groups*1..10
                   ]->(end:Node {{id:$tgt}})
               WHERE start.state IN ['DROP','FROZEN']
                 AND end.state IN ['DROP','FROZEN']
               WITH path
                 WHERE all(n IN nodes(path) WHERE n.state IN ['DROP','FROZEN'])
               RETURN path
               ORDER BY length(path)
               LIMIT 1""",
            src=source_id,
            tgt=target_id,
        )
        record = await q_path.single()
        if not record:
            return None

        path = record["path"]
        path_nodes = list(path.nodes)
        path_rels = list(path.relationships)

        nodes_out = [
            {
                "id": n["id"],
                "title": n.get("content", "")[:50],
                "reason": "",
            }
            for n in path_nodes
        ]

        edges_out = [
            {
                "from": r.start_node["id"],
                "to": r.end_node["id"],
                "similarity": r.get("strength", 0.0) if r.type == "RELATES_TO" else 0.0,
                "kind": r.type,
            }
            for r in path_rels
        ]

        return {
            "source_id": source_id,
            "target_id": target_id,
            "nodes": nodes_out,
            "edges": edges_out,
            "total_steps": len(path_nodes) - 1,
        }


# ---------------------------------------------------------------------------
# 2. Jaccard similarity clustering with Union-Find
# ---------------------------------------------------------------------------


async def compute_clusters(lake_id: str) -> list[dict[str, Any]]:
    """Jaccard-similarity-based community detection.

    - Fetches all DROP/FROZEN nodes in the lake.
    - For each pair computes Jaccard = |intersection| / |union| of neighbor sets.
    - Pairs with similarity >= 0.3 are linked via Union-Find.
    - Returns clusters with color, bridge nodes (max 5), and density.
    """
    driver = get_neo4j()
    async with driver.session() as s:
        # Fetch all nodes in lake
        q_nodes = await s.run(
            """MATCH (n:Node {lake_id:$lake}) WHERE n.state IN ['DROP','FROZEN']
               RETURN n.id AS id, n.content AS content""",
            lake=lake_id,
        )
        nodes = [(r["id"], r["content"]) async for r in q_nodes]

        if not nodes:
            return []

        node_ids = [nid for nid, _ in nodes]
        node_contents = {nid: content for nid, content in nodes}

        # Fetch all edges between lake nodes (both directions)
        q_edges = await s.run(
            """MATCH (n:Node {lake_id:$lake})-[r]->(m:Node {lake_id:$lake})
               WHERE n.state IN ['DROP','FROZEN'] AND m.state IN ['DROP','FROZEN']
                 AND type(r) IN ['RELATES_TO','derives','refines','opposes','groups']
               RETURN n.id AS src, m.id AS dst, r.strength AS strength, type(r) AS kind""",
            lake=lake_id,
        )
        raw_edges = [(r["src"], r["dst"], r.get("strength", 0.0), r["kind"]) async for r in q_edges]

    # Build neighbor sets (undirected for clustering purposes)
    neighbors: dict[str, set[str]] = {nid: set() for nid in node_ids}
    edge_list: list[tuple[str, str, float, str]] = []

    for src, dst, strength, kind in raw_edges:
        neighbors[src].add(dst)
        neighbors[dst].add(src)
        edge_list.append((src, dst, strength, kind))

    # Compute Jaccard similarities and Union-Find
    uf = _UnionFind()
    for nid in node_ids:
        uf.make_set(nid)

    for i, ida in enumerate(node_ids):
        for idb in node_ids[i + 1 :]:
            inter = len(neighbors[ida] & neighbors[idb])
            union = len(neighbors[ida] | neighbors[idb])
            if union > 0 and (inter / union) >= 0.3:
                uf.union(ida, idb)

    # Group nodes by root
    clusters_raw: dict[str, list[str]] = {}
    for nid in node_ids:
        root = uf.find(nid)
        clusters_raw.setdefault(root, []).append(nid)

    # Build edge index for internal edge counting
    edge_set: set[tuple[str, str]] = set()
    for src, dst, _, _ in edge_list:
        edge_set.add((src, dst))
        edge_set.add((dst, src))

    results: list[dict[str, Any]] = []
    for idx, (root, members) in enumerate(clusters_raw.items()):
        node_count = len(members)
        internal_edges = sum(
            1 for a in members for b in members if a != b and (a, b) in edge_set
        ) // 2  # undirected, so divide by 2

        # Bridge nodes: nodes with external connections (max 5)
        bridge_ids: list[str] = []
        for nid in members:
            ext = neighbors[nid] - set(members)
            if ext:
                bridge_ids.append(nid)
                if len(bridge_ids) >= 5:
                    break

        # Label from first node's content
        first_content = node_contents.get(members[0], "")[:15]

        results.append(
            {
                "id": f"cluster_{idx + 1}",
                "label": first_content,
                "node_ids": members,
                "color": _CLUSTER_COLORS[idx % len(_CLUSTER_COLORS)],
                "bridge_node_ids": bridge_ids,
                "density": round(internal_edges / node_count, 4) if node_count > 0 else 0.0,
            }
        )

    logger.info("clusters_computed", lake_id=lake_id, cluster_count=len(results))
    return results


# ---------------------------------------------------------------------------
# 3. Planning suggestions via knowledge gap analysis
# ---------------------------------------------------------------------------


async def compute_planning_suggestions(lake_id: str) -> list[dict[str, Any]]:
    """Knowledge gap analysis for a lake.

    Generates suggestions based on graph topology:
    - Isolated nodes (0 in-edges AND 0 out-edges) → "建立关联" high priority
    - Low-degree nodes (1-2 connections) → "扩展关联" medium priority
    - If lake has 20+ nodes → "AI 探索补充" low priority
    """
    driver = get_neo4j()
    async with driver.session() as s:
        # Fetch all nodes and their degree stats
        q_stats = await s.run(
            """MATCH (n:Node {lake_id:$lake}) WHERE n.state IN ['DROP','FROZEN']
               OPTIONAL MATCH (n)-[r]->(m:Node)
               WHERE n.state IN ['DROP','FROZEN'] AND m.state IN ['DROP','FROZEN']
                 AND type(r) IN ['RELATES_TO','derives','refines','opposes','groups']
               WITH n, count(r) AS out_degree
               OPTIONAL MATCH (p:Node)-[r2]->(n)
               WHERE p.state IN ['DROP','FROZEN']
                 AND type(r2) IN ['RELATES_TO','derives','refines','opposes','groups']
               RETURN n.id AS id, n.content AS content, out_degree,
                      count(r2) AS in_degree, out_degree + count(r2) AS total_degree""",
            lake=lake_id,
        )
        rows = [
            {
                "id": r["id"],
                "content": r["content"],
                "in_degree": r["in_degree"],
                "out_degree": r["out_degree"],
                "total_degree": r["total_degree"],
            }
            async for r in q_stats
        ]

    suggestions: list[dict[str, Any]] = []
    seen_related: set[frozenset[str]] = set()

    for row in rows:
        total = row["total_degree"]
        if total == 0:
            key = frozenset([row["id"]])
            if key not in seen_related:
                seen_related.add(key)
                suggestions.append(
                    {
                        "id": f"suggest_{uuid.uuid4().hex[:12]}",
                        "type": "build_connection",
                        "title": "建立关联",
                        "description": f"节点「{row['content'][:20]}」完全孤立，建议与其他节点建立关联。",
                        "priority": "high",
                        "related_node_ids": [row["id"]],
                    }
                )
        elif 1 <= total <= 2:
            key = frozenset([row["id"]])
            if key not in seen_related:
                seen_related.add(key)
                suggestions.append(
                    {
                        "id": f"suggest_{uuid.uuid4().hex[:12]}",
                        "type": "expand_connection",
                        "title": "扩展关联",
                        "description": f"节点「{row['content'][:20]}」连接较少（{total}条），建议扩展关联。",
                        "priority": "medium",
                        "related_node_ids": [row["id"]],
                    }
                )

    # AI exploration suggestion if lake has many nodes
    if len(rows) >= 20:
        suggestions.append(
            {
                "id": f"suggest_{uuid.uuid4().hex[:12]}",
                "type": "ai_explore",
                "title": "AI 探索补充",
                "description": f"湖内已有 {len(rows)} 个节点，AI 可帮助发现潜在知识关联并补充图谱。",
                "priority": "low",
                "related_node_ids": [r["id"] for r in rows[:10]],  # sample 10 related
            }
        )

    logger.info(
        "planning_suggestions_computed",
        lake_id=lake_id,
        suggestion_count=len(suggestions),
    )
    return suggestions