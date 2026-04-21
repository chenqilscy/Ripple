"""湖泊 (Lake) API 骨架 · 实现参考 G1-数据模型与权限设计.md"""

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from app.core.security import get_current_user

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


@router.post("", response_model=LakeResponse, status_code=201)
async def create_lake(body: LakeCreate, user: dict = Depends(get_current_user)) -> LakeResponse:
    # TODO: M1 写入 Neo4j (:Lake) + PG lake_memberships(role=OWNER)
    return LakeResponse(
        id="lake_stub_id",
        name=body.name,
        description=body.description,
        is_public=body.is_public,
        owner_id=user["user_id"],
    )


@router.get("", response_model=list[LakeResponse])
async def list_lakes(user: dict = Depends(get_current_user)) -> list[LakeResponse]:
    # TODO: M1 查询 lake_memberships WHERE user_id = user["user_id"]
    return []
