"""冰山 (Iceberg) API 骨架 · 参考 G3-冰山-资产沉淀模块设计.md"""

from fastapi import APIRouter, Depends
from pydantic import BaseModel

from app.core.security import get_current_user

router = APIRouter()


class IcebergCreate(BaseModel):
    lake_id: str
    title: str
    description: str | None = None
    node_ids: list[str]


class IcebergResponse(BaseModel):
    id: str
    title: str
    lake_id: str
    node_count: int


@router.post("", response_model=IcebergResponse, status_code=201)
async def create_iceberg(
    body: IcebergCreate, user: dict = Depends(get_current_user)
) -> IcebergResponse:
    # TODO: 快照节点 + 生成灵感三角洲索引
    return IcebergResponse(
        id="iceberg_stub", title=body.title, lake_id=body.lake_id, node_count=len(body.node_ids)
    )
