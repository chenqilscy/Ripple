"""湖泊服务 · Neo4j (:Lake) + PG lake_memberships

实现参考：
- G1 §二 Neo4j Schema  §四 PG DDL  §五 权限矩阵
- 故事 2 跨湖协作
"""

import uuid
from typing import Any

from fastapi import HTTPException, status
from sqlalchemy import select
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.db import get_neo4j
from app.core.logging import logger
from app.models.lake_membership import LakeMembership

Role = str  # OWNER | NAVIGATOR | PASSENGER | OBSERVER


async def create_lake(
    session: AsyncSession, user_id: uuid.UUID, name: str, description: str | None, is_public: bool
) -> dict[str, Any]:
    lake_id = f"lake_{uuid.uuid4().hex[:16]}"
    driver = get_neo4j()
    async with driver.session() as s:
        await s.run(
            """CREATE (l:Lake {id:$id, name:$name, description:$desc, is_public:$pub,
               owner_id:$uid, created_at:datetime()}) RETURN l""",
            id=lake_id,
            name=name,
            desc=description or "",
            pub=is_public,
            uid=str(user_id),
        )
    membership = LakeMembership(user_id=user_id, lake_id=lake_id, role="OWNER")
    session.add(membership)
    await session.commit()
    logger.info("lake_created", lake_id=lake_id, user_id=str(user_id))
    return {
        "id": lake_id,
        "name": name,
        "description": description,
        "is_public": is_public,
        "owner_id": str(user_id),
        "role": "OWNER",
    }


async def list_lakes_for_user(session: AsyncSession, user_id: uuid.UUID) -> list[dict[str, Any]]:
    result = await session.execute(
        select(LakeMembership).where(LakeMembership.user_id == user_id)
    )
    memberships = result.scalars().all()
    if not memberships:
        return []
    driver = get_neo4j()
    async with driver.session() as s:
        q = await s.run(
            "MATCH (l:Lake) WHERE l.id IN $ids RETURN l",
            ids=[m.lake_id for m in memberships],
        )
        records = [r async for r in q]
    role_map = {m.lake_id: m.role for m in memberships}
    return [
        {
            "id": r["l"]["id"],
            "name": r["l"]["name"],
            "description": r["l"].get("description") or None,
            "is_public": r["l"]["is_public"],
            "owner_id": r["l"]["owner_id"],
            "role": role_map.get(r["l"]["id"], "OBSERVER"),
        }
        for r in records
    ]


async def assert_access(
    session: AsyncSession, user_id: uuid.UUID, lake_id: str, min_role: Role = "PASSENGER"
) -> Role:
    """参见 G7 §三权限矩阵"""
    rank = {"OBSERVER": 0, "PASSENGER": 1, "NAVIGATOR": 2, "OWNER": 3}
    result = await session.execute(
        select(LakeMembership).where(
            LakeMembership.user_id == user_id, LakeMembership.lake_id == lake_id
        )
    )
    m = result.scalar_one_or_none()
    if not m or rank.get(m.role, -1) < rank[min_role]:
        raise HTTPException(status.HTTP_403_FORBIDDEN, "Insufficient lake permission")
    return m.role
