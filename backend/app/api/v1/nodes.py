"""节点 API · 参考 G1 §三状态机 + 故事 5/6/7

功能：创建 (DROP)、列出、蒸发、还原、迷雾区、触发织网
"""

import uuid

from fastapi import APIRouter, BackgroundTasks, Depends, HTTPException
from pydantic import BaseModel, Field
from sqlalchemy.ext.asyncio import AsyncSession

from app.ai.weaver import weave_for_node
from app.core.db import get_pg_session
from app.core.security import get_current_user
from app.services.audit_service import log_event
from app.services.lake_service import assert_access
from app.services.node_service import (
    create_drop_node,
    evaporate_node,
    get_node,
    list_mist_zone,
    list_nodes_in_lake,
    restore_node,
)
from app.ws.hub import hub

router = APIRouter()


class NodeCreate(BaseModel):
    lake_id: str
    content: str = Field(..., min_length=1, max_length=10000)
    type: str = "TEXT"
    position: dict[str, float] | None = None


class NodeResponse(BaseModel):
    id: str
    lake_id: str | None
    content: str
    type: str
    state: str
    position: dict[str, float] | None = None


@router.post("", response_model=NodeResponse, status_code=201)
async def create_node(
    body: NodeCreate,
    bg: BackgroundTasks,
    user: dict = Depends(get_current_user),
    session: AsyncSession = Depends(get_pg_session),
) -> NodeResponse:
    await assert_access(session, uuid.UUID(user["user_id"]), body.lake_id, min_role="PASSENGER")
    data = await create_drop_node(
        body.lake_id, user["user_id"], body.content, body.type, body.position
    )
    await log_event(
        session,
        uuid.UUID(user["user_id"]),
        "node.create",
        "node",
        data["id"],
        lake_id=body.lake_id,
    )
    await hub.broadcast(body.lake_id, {"type": "node.created", "node": data})
    bg.add_task(_weave_and_broadcast, data["id"], body.content, body.lake_id)
    return NodeResponse(**data)


async def _weave_and_broadcast(node_id: str, content: str, lake_id: str) -> None:
    edges = await weave_for_node(node_id, content, lake_id)
    if edges:
        await hub.broadcast(
            lake_id,
            {"type": "edges.proposed", "source": node_id, "edges": edges},
        )


@router.get("/by-lake/{lake_id}", response_model=list[NodeResponse])
async def list_by_lake(
    lake_id: str,
    user: dict = Depends(get_current_user),
    session: AsyncSession = Depends(get_pg_session),
) -> list[NodeResponse]:
    await assert_access(session, uuid.UUID(user["user_id"]), lake_id, min_role="OBSERVER")
    return [NodeResponse(**r) for r in await list_nodes_in_lake(lake_id)]


@router.get("/{node_id}", response_model=NodeResponse)
async def get_node_endpoint(
    node_id: str, user: dict = Depends(get_current_user)
) -> NodeResponse:
    data = await get_node(node_id)
    if not data:
        raise HTTPException(404, "Node not found")
    return NodeResponse(**data)


@router.post("/{node_id}/evaporate", status_code=204)
async def evaporate(
    node_id: str,
    user: dict = Depends(get_current_user),
    session: AsyncSession = Depends(get_pg_session),
) -> None:
    """蒸发：软删（VAPOR），30 天后物理删除"""
    await evaporate_node(node_id, user["user_id"])
    await log_event(session, uuid.UUID(user["user_id"]), "node.evaporate", "node", node_id)
    node = await get_node(node_id)
    if node and node.get("lake_id"):
        await hub.broadcast(node["lake_id"], {"type": "node.evaporated", "id": node_id})


@router.post("/{node_id}/restore", response_model=NodeResponse)
async def restore(
    node_id: str,
    user: dict = Depends(get_current_user),
    session: AsyncSession = Depends(get_pg_session),
) -> NodeResponse:
    data = await restore_node(node_id, user["user_id"])
    await log_event(session, uuid.UUID(user["user_id"]), "node.restore", "node", node_id)
    if data.get("lake_id"):
        await hub.broadcast(data["lake_id"], {"type": "node.restored", "node": data})
    return NodeResponse(**data)


@router.get("/mist-zone/mine", response_model=list[dict])
async def mist_zone(user: dict = Depends(get_current_user)) -> list[dict]:
    """迷雾区：我可还原的蒸发节点"""
    return await list_mist_zone(user["user_id"])
