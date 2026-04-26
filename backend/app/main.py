"""青萍 (Ripple) · FastAPI 应用入口

实现参考：
- docs/system-design/技术架构总览.md
- docs/system-design/API网关设计.md
- docs/system-design/多人实时协作设计.md
- docs/MVP-范围与里程碑.md  §M1 骨架
"""

from contextlib import asynccontextmanager

from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from fastapi.middleware.cors import CORSMiddleware

from app.api.v1 import api_router
from app.core.config import settings
from app.core.db import close_db, init_db
from app.core.logging import setup_logging
from app.core.security import decode_token
from app.ws.hub import hub


@asynccontextmanager
async def lifespan(app: FastAPI):
    setup_logging()
    await init_db()
    yield
    await close_db()


app = FastAPI(
    title="Ripple API",
    version="0.1.0",
    description="青萍 (Ripple) 水文生态创意系统 · 后端 API",
    lifespan=lifespan,
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.CORS_ORIGINS,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

app.include_router(api_router, prefix="/api/v1")


@app.get("/health", tags=["meta"])
async def health() -> dict[str, str]:
    return {"status": "ok", "service": "ripple-backend", "version": "0.1.0"}


@app.websocket("/ws/lakes/{lake_id}")
async def ws_lake(websocket: WebSocket, lake_id: str, token: str | None = None) -> None:
    """湖泊实时频道 · token 通过 query string 传递（?token=...）"""
    if not token:
        await websocket.close(code=4401, reason="Missing token")
        return
    try:
        decode_token(token)
    except Exception:
        await websocket.close(code=4403, reason="Invalid token")
        return
    await hub.join(lake_id, websocket)
    try:
        while True:
            msg = await websocket.receive_json()
            # 简单回显，真正的协作冲突解决在 D5
            await hub.broadcast(lake_id, {"type": "echo", "payload": msg})
    except WebSocketDisconnect:
        hub.leave(lake_id, websocket)
