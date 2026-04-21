"""分享 (Share) API 骨架 · 参考 G3 §六决堤口 + 故事 8"""

from fastapi import APIRouter, Depends
from pydantic import BaseModel, Field

from app.core.security import get_current_user

router = APIRouter()


class ShareCreate(BaseModel):
    iceberg_id: str
    mode: str = Field("blur", pattern="^(blur|plain)$")  # 朦胧版 | 明文
    ttl_hours: int = 168  # 7 天
    password: str | None = None


class ShareResponse(BaseModel):
    id: str
    url: str
    mode: str
    expires_at: str


@router.post("", response_model=ShareResponse, status_code=201)
async def create_share(body: ShareCreate, user: dict = Depends(get_current_user)) -> ShareResponse:
    # TODO: 生成 signed URL + 记录 share_links
    return ShareResponse(
        id="share_stub",
        url=f"https://ripple.dev/s/stub?mode={body.mode}",
        mode=body.mode,
        expires_at="2026-04-28T00:00:00Z",
    )
