"""云霓 (Cloud) API 骨架 · 实现参考 G2-云霓-灵感采集模块设计.md"""

from fastapi import APIRouter, Depends
from pydantic import BaseModel

from app.core.security import get_current_user

router = APIRouter()


class MistCreate(BaseModel):
    content: str
    source: str = "mobile"  # mobile | desktop-widget | web
    client_created_at: str | None = None


class MistResponse(BaseModel):
    id: str
    content: str
    state: str  # MIST
    ttl_at: str


@router.post("/mist", response_model=MistResponse, status_code=201)
async def create_mist(body: MistCreate, user: dict = Depends(get_current_user)) -> MistResponse:
    # TODO: CREATE (:Node {state:'MIST', ttl_at:NOW+7d, ...})
    return MistResponse(
        id="mist_stub", content=body.content, state="MIST", ttl_at="2026-05-01T00:00:00Z"
    )


class CondenseRequest(BaseModel):
    mist_ids: list[str]
    target_lake_id: str


@router.post("/condense", status_code=204)
async def condense(body: CondenseRequest, user: dict = Depends(get_current_user)) -> None:
    """凝露：MIST → DROP，TTL 清空，进入目标湖"""
    # TODO: 批量迁移状态
    pass
