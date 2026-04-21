"""湖泊 (Lake) API · 实现参考 G1-数据模型与权限设计.md + 故事 2"""

import uuid

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.db import get_pg_session
from app.core.security import get_current_user
from app.services.lake_service import create_lake, list_lakes_for_user

router = APIRouter()


class LakeCreate(BaseModel):
    name: str = Field(..., min_length=1, max_length=64)
    description: str | None = None
    is_public: bool = False


class LakeResponse(BaseModel):
    id: str
    name: str
    description: str | None
    is_public: bool
    owner_id: str
    role: str


@router.post("", response_model=LakeResponse, status_code=201)
async def create_lake_endpoint(
    body: LakeCreate,
    user: dict = Depends(get_current_user),
    session: AsyncSession = Depends(get_pg_session),
) -> LakeResponse:
    data = await create_lake(
        session, uuid.UUID(user["user_id"]), body.name, body.description, body.is_public
    )
    return LakeResponse(**data)


@router.get("", response_model=list[LakeResponse])
async def list_lakes_endpoint(
    user: dict = Depends(get_current_user),
    session: AsyncSession = Depends(get_pg_session),
) -> list[LakeResponse]:
    rows = await list_lakes_for_user(session, uuid.UUID(user["user_id"]))
    return [LakeResponse(**r) for r in rows]
