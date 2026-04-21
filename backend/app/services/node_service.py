"""节点服务 · Neo4j (:Node) + 状态机

实现参考：
- G1 §二 Schema  §三 状态机 (MIST→DROP→FROZEN→VAPOR→ERASED/GHOST)
- 故事 5 onboarding  故事 6 云霓  故事 7 误删恢复
"""

import uuid
from datetime import UTC, datetime, timedelta
from typing import Any

from fastapi import HTTPException, status

from app.core.db import get_neo4j
from app.core.logging import logger

MIST_TTL_DAYS = 7
VAPOR_TTL_DAYS = 30


async def create_drop_node(
    lake_id: str, user_id: str, content: str, node_type: str, position: dict | None
) -> dict[str, Any]:
    node_id = f"node_{uuid.uuid4().hex[:16]}"
    driver = get_neo4j()
    async with driver.session() as s:
        await s.run(
            """MATCH (l:Lake {id:$lake_id})
               CREATE (n:Node {id:$id, lake_id:$lake_id, content:$content, type:$type,
                               state:'DROP', position:$pos, owner_id:$uid, created_at:datetime(),
                               updated_at:datetime()})
               MERGE (n)-[:BELONGS_TO]->(l)""",
            id=node_id,
            lake_id=lake_id,
            content=content,
            type=node_type,
            pos=position or {},
            uid=user_id,
        )
    logger.info("node_created", node_id=node_id, lake_id=lake_id, state="DROP")
    return {
        "id": node_id,
        "lake_id": lake_id,
        "content": content,
        "type": node_type,
        "state": "DROP",
        "position": position,
    }


async def create_mist_node(user_id: str, content: str, source: str) -> dict[str, Any]:
    node_id = f"mist_{uuid.uuid4().hex[:16]}"
    ttl_at = (datetime.now(UTC) + timedelta(days=MIST_TTL_DAYS)).isoformat()
    driver = get_neo4j()
    async with driver.session() as s:
        await s.run(
            """CREATE (n:Node {id:$id, content:$content, type:'TEXT', state:'MIST',
                               source:$source, owner_id:$uid, ttl_at:datetime($ttl),
                               created_at:datetime(), updated_at:datetime()})""",
            id=node_id,
            content=content,
            source=source,
            uid=user_id,
            ttl=ttl_at,
        )
    return {"id": node_id, "content": content, "state": "MIST", "ttl_at": ttl_at}


async def condense_mists(user_id: str, mist_ids: list[str], target_lake_id: str) -> int:
    """MIST → DROP，进入目标湖；TTL 清空"""
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run(
            """MATCH (n:Node) WHERE n.id IN $ids AND n.state='MIST' AND n.owner_id=$uid
               MATCH (l:Lake {id:$lake})
               SET n.state='DROP', n.lake_id=$lake, n.ttl_at=NULL, n.updated_at=datetime()
               MERGE (n)-[:BELONGS_TO]->(l)
               RETURN count(n) AS c""",
            ids=mist_ids,
            uid=user_id,
            lake=target_lake_id,
        )
        rec = await q.single()
        return rec["c"] if rec else 0


async def evaporate_node(node_id: str, user_id: str) -> None:
    """DROP/FROZEN → VAPOR (软删)"""
    ttl_at = (datetime.now(UTC) + timedelta(days=VAPOR_TTL_DAYS)).isoformat()
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run(
            """MATCH (n:Node {id:$id}) WHERE n.state IN ['DROP','FROZEN']
               SET n.state='VAPOR', n.deleted_at=datetime(), n.ttl_at=datetime($ttl),
                   n.vapor_by=$uid, n.updated_at=datetime()
               RETURN n.id AS id""",
            id=node_id,
            uid=user_id,
            ttl=ttl_at,
        )
        r = await q.single()
        if not r:
            raise HTTPException(status.HTTP_404_NOT_FOUND, "Node not found or not evaporable")
    logger.info("node_evaporated", node_id=node_id, user_id=user_id)


async def restore_node(node_id: str, user_id: str) -> dict[str, Any]:
    """VAPOR → DROP (凝露还原)"""
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run(
            """MATCH (n:Node {id:$id, state:'VAPOR'})
               WHERE n.owner_id=$uid OR exists {
                 MATCH (n)-[:BELONGS_TO]->(l:Lake) WHERE l.owner_id=$uid
               }
               SET n.state='DROP', n.deleted_at=NULL, n.ttl_at=NULL, n.updated_at=datetime()
               RETURN n""",
            id=node_id,
            uid=user_id,
        )
        r = await q.single()
        if not r:
            raise HTTPException(status.HTTP_404_NOT_FOUND, "Node not in mist zone or no permission")
    n = r["n"]
    logger.info("node_restored", node_id=node_id, user_id=user_id)
    return {
        "id": n["id"],
        "lake_id": n.get("lake_id"),
        "content": n["content"],
        "type": n["type"],
        "state": n["state"],
        "position": n.get("position"),
    }


async def list_mist_zone(user_id: str) -> list[dict[str, Any]]:
    """迷雾区：VAPOR 节点列表，按 deleted_at desc"""
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run(
            """MATCH (n:Node {state:'VAPOR'})
               WHERE n.owner_id=$uid OR n.vapor_by=$uid
               RETURN n ORDER BY n.deleted_at DESC LIMIT 200""",
            uid=user_id,
        )
        return [
            {
                "id": r["n"]["id"],
                "content": r["n"]["content"],
                "lake_id": r["n"].get("lake_id"),
                "deleted_at": str(r["n"]["deleted_at"]),
                "ttl_at": str(r["n"]["ttl_at"]),
            }
            async for r in q
        ]


async def get_node(node_id: str) -> dict[str, Any] | None:
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run("MATCH (n:Node {id:$id}) RETURN n", id=node_id)
        r = await q.single()
        if not r:
            return None
        n = r["n"]
        return {
            "id": n["id"],
            "lake_id": n.get("lake_id"),
            "content": n["content"],
            "type": n["type"],
            "state": n["state"],
            "position": n.get("position"),
        }


async def list_nodes_in_lake(lake_id: str) -> list[dict[str, Any]]:
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run(
            """MATCH (n:Node {lake_id:$id}) WHERE n.state IN ['DROP','FROZEN']
               RETURN n ORDER BY n.created_at DESC LIMIT 500""",
            id=lake_id,
        )
        return [
            {
                "id": r["n"]["id"],
                "lake_id": r["n"]["lake_id"],
                "content": r["n"]["content"],
                "type": r["n"]["type"],
                "state": r["n"]["state"],
                "position": r["n"].get("position"),
            }
            async for r in q
        ]
