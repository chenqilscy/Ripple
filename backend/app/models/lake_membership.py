"""湖泊成员关系表 · 参考 G1 §四 + §五权限矩阵"""

import uuid

from sqlalchemy import ForeignKey, String, UniqueConstraint
from sqlalchemy.dialects.postgresql import UUID as PGUUID
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base


class LakeMembership(Base):
    """role ∈ {OWNER, NAVIGATOR, PASSENGER, OBSERVER}（舟子/副舟子/渡客/观潮）"""

    __tablename__ = "lake_memberships"
    __table_args__ = (UniqueConstraint("user_id", "lake_id", name="uq_lake_member"),)

    id: Mapped[uuid.UUID] = mapped_column(PGUUID(as_uuid=True), primary_key=True, default=uuid.uuid4)
    user_id: Mapped[uuid.UUID] = mapped_column(ForeignKey("users.id", ondelete="CASCADE"), index=True)
    lake_id: Mapped[str] = mapped_column(String(64), index=True)  # Neo4j Lake uuid
    role: Mapped[str] = mapped_column(String(16), default="PASSENGER")
