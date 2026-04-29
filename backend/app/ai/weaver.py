"""Weaver · 单 Agent 版本（M1）

根据 G5 §2.1：Diver 召回 → Weaver 打分 → Critic → Curator。
M1 简化为：纯向量相似度，阈值 0.78 以上生成 :RELATES_TO 建议。
M2 接入真 LLM 评分。
"""

from typing import Any

from app.ai.embeddings import cosine, embed
from app.core.db import get_neo4j
from app.core.logging import logger
from app.services.recommendation_service import add_recommendation

THRESHOLD = 0.78
TOP_K = 5


async def weave_for_node(node_id: str, content: str, lake_id: str) -> list[dict[str, Any]]:
    """对新凝露节点 node_id，找出同湖内相似度 ≥ THRESHOLD 的 Top-K 邻居，
    写入 :RELATES_TO（strength=cosine）。返回新建关系列表。"""

    src_vec = embed(content)
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run(
            """MATCH (n:Node {lake_id:$lake}) WHERE n.state IN ['DROP','FROZEN'] AND n.id <> $id
               RETURN n.id AS id, n.content AS content LIMIT 200""",
            lake=lake_id,
            id=node_id,
        )
        rows = [(r["id"], r["content"]) async for r in q]

    scored: list[tuple[str, float]] = []
    recommendations_candidates: list[tuple[str, float]] = []

    for nid, c in rows:
        sc = cosine(src_vec, embed(c))
        if sc >= THRESHOLD:
            scored.append((nid, sc))
        elif sc >= 0.60:
            recommendations_candidates.append((nid, sc))
    scored.sort(key=lambda x: x[1], reverse=True)
    recommendations_candidates.sort(key=lambda x: x[1], reverse=True)
    chosen = scored[:TOP_K]

    if not chosen:
        logger.info("weaver_no_match", node_id=node_id, candidates=len(rows))
        return []

    async with driver.session() as s:
        await s.run(
            """UNWIND $edges AS e
               MATCH (a:Node {id:$src}), (b:Node {id:e.target})
               MERGE (a)-[r:RELATES_TO]->(b)
               SET r.strength=e.strength, r.by='weaver', r.created_at=coalesce(r.created_at, datetime())""",
            src=node_id,
            edges=[{"target": nid, "strength": sc} for nid, sc in chosen],
        )
    logger.info("weaver_related", node_id=node_id, count=len(chosen))

    # Phase 2: Medium-similarity candidates → recommendations (0.6 <= sc < 0.78)
    # Note: We need the source node's lake_id and title for recommendations
    async with driver.session() as s:
        src_q = await s.run(
            """MATCH (n:Node {id:$id}) RETURN n.lake_id AS lake_id, n.title AS title LIMIT 1""",
            id=node_id,
        )
        src_rows = [r async for r in src_q]
        if src_rows:
            src_lake_id = src_rows[0]["lake_id"]
            src_title = src_rows[0]["title"]
            for nid, sc in recommendations_candidates:
                await add_recommendation(
                    lake_id=src_lake_id,
                    source_node_id=node_id,
                    target_node_id=nid,
                    reason=f"与「{src_title}」存在语义关联",
                    confidence=sc,
                    created_by="weaver",
                )
            if recommendations_candidates:
                logger.info("weaver_recommendations_created", node_id=node_id, count=len(recommendations_candidates))

    return [{"target": nid, "strength": sc} for nid, sc in chosen]
