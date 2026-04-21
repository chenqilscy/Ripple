"""云霓 (Cloud) API · 参考 G2 + 故事 6 移动端通勤捕梦"""

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.db import get_pg_session
from app.core.security import get_current_user
from app.services.node_service import condense_mists, create_mist_node

router = APIRouter()


class MistCreate(BaseModel):
    content: str = Field(..., min_length=1, max_length=4000)
    source: str = Field("mobile", pattern="^(mobile|desktop-widget|web)$")


class MistResponse(BaseModel):
    id: str
    content: str
    state: str
    ttl_at: str


@router.post("/mist", response_model=MistResponse, status_code=201)
async def create_mist(
    body: MistCreate, user: dict = Depends(get_current_user)
) -> MistResponse:
    data = await create_mist_node(user["user_id"], body.content, body.source)
    return MistResponse(**data)


class CondenseRequest(BaseModel):
    mist_ids: list[str]
    target_lake_id: str


class CondenseResponse(BaseModel):
    condensed: int


@router.post("/condense", response_model=CondenseResponse)
async def condense(
    body: CondenseRequest,
    user: dict = Depends(get_current_user),
    session: AsyncSession = Depends(get_pg_session),
) -> CondenseResponse:
    # 注意：condense_mists 已校验 owner_id
    count = await condense_mists(user["user_id"], body.mist_ids, body.target_lake_id)
    return CondenseResponse(condensed=count)
