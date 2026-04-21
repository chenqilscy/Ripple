"""审计日志服务 · 参考 G7 §三"""

import uuid
from typing import Any

from sqlalchemy.ext.asyncio import AsyncSession

from app.models.audit_event import AuditEvent


async def log_event(
    session: AsyncSession,
    actor_id: uuid.UUID,
    action: str,
    resource_type: str,
    resource_id: str,
    lake_id: str | None = None,
    metadata: dict[str, Any] | None = None,
) -> None:
    event = AuditEvent(
        actor_id=actor_id,
        action=action,
        resource_type=resource_type,
        resource_id=resource_id,
        lake_id=lake_id,
        metadata_json=metadata or {},
    )
    session.add(event)
    await session.commit()
