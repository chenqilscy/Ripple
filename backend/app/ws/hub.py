"""WebSocket Hub · 按湖分频道广播 · 参考 多人实时协作设计.md"""

from collections import defaultdict

from fastapi import WebSocket

from app.core.logging import logger


class LakeHub:
    """in-memory 最小实现；M2 升级为 Redis Pub/Sub 跨进程"""

    def __init__(self) -> None:
        self._channels: dict[str, set[WebSocket]] = defaultdict(set)

    async def join(self, lake_id: str, ws: WebSocket) -> None:
        await ws.accept()
        self._channels[lake_id].add(ws)
        logger.info("ws_joined", lake_id=lake_id, count=len(self._channels[lake_id]))

    def leave(self, lake_id: str, ws: WebSocket) -> None:
        self._channels[lake_id].discard(ws)
        if not self._channels[lake_id]:
            self._channels.pop(lake_id, None)

    async def broadcast(self, lake_id: str, payload: dict) -> None:
        dead: list[WebSocket] = []
        for ws in self._channels.get(lake_id, set()):
            try:
                await ws.send_json(payload)
            except Exception:
                dead.append(ws)
        for ws in dead:
            self.leave(lake_id, ws)


hub = LakeHub()
