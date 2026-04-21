"""节点 (Node) API 骨架 · 实现参考 G1 §二 Neo4j Schema + §三状态机"""

from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel, Field

from app.core.security import get_current_user

router = APIRouter()

NodeState = str  # "MIST" | "DROP" | "FROZEN" | "VAPOR" | "ERASED" | "GHOST"


class NodeCreate(BaseModel):
    lake_id: str
    content: str = Field(..., min_length=1, max_length=10000)
    type: str = "TEXT"
    position: dict[str, float] | None = None


class NodeResponse(BaseModel):
    id: str
    lake_id: str
    content: str
    type: str
    state: NodeState
    position: dict[str, float] | None


@router.post("", response_model=NodeResponse, status_code=201)
async def create_node(body: NodeCreate, user: dict = Depends(get_current_user)) -> NodeResponse:
    # TODO: M1 Neo4j CREATE (:Node {state:'DROP', lake_id:$lake_id, ...})
    # TODO: 校验 user 对 lake_id 有写权限（参见 G7 §三权限矩阵）
    return NodeResponse(
        id="node_stub_id",
        lake_id=body.lake_id,
        content=body.content,
        type=body.type,
        state="DROP",
        position=body.position,
    )


@router.post("/{node_id}/evaporate", status_code=204)
async def evaporate_node(node_id: str, user: dict = Depends(get_current_user)) -> None:
    """蒸发：软删，进入 VAPOR 态，30 天后 ERASED（参见故事 7）"""
    # TODO: SET state='VAPOR', deleted_at=NOW()
    if not node_id:
        raise HTTPException(400, "node_id required")


@router.post("/{node_id}/restore", response_model=NodeResponse)
async def restore_node(node_id: str, user: dict = Depends(get_current_user)) -> NodeResponse:
    """凝露还原 · VAPOR → DROP + 还原全部连线"""
    # TODO: 校验 user 为原创者 or Owner
    return NodeResponse(
        id=node_id, lake_id="lake_stub_id", content="...", type="TEXT", state="DROP", position=None
    )
