"""Outbox 生产者 · AI 工作流异步触发"""

from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.models.outbox import OutboxEvent


async def publish(
    session: AsyncSession, event_type: str, payload: dict[str, Any]
) -> None:
    session.add(OutboxEvent(event_type=event_type, payload=payload))
    await session.commit()
