"""Recommendation management service for Ripple backend.

Phase 1: In-memory storage with Neo4j edge creation on acceptance.
"""

import uuid
from datetime import datetime, timezone
from dataclasses import dataclass, field
from typing import Optional

from app.core.db import get_neo4j
from app.core.logging import logger


# Phase 1: In-memory storage for recommendations
_RECOMMENDATIONS: dict[str, list] = {}


@dataclass
class RecommendationRecord:
    """Represents a recommendation record."""

    id: str
    lake_id: str
    source_node_id: str
    target_node_id: str
    reason: str
    confidence: float
    status: str = "pending"
    created_by: Optional[str] = None
    created_at: datetime = field(default_factory=lambda: datetime.now(timezone.utc))

    def to_dict(self) -> dict:
        """Convert record to dictionary representation."""
        return {
            "id": self.id,
            "lake_id": self.lake_id,
            "source_node_id": self.source_node_id,
            "target_node_id": self.target_node_id,
            "reason": self.reason,
            "confidence": self.confidence,
            "status": self.status,
            "created_by": self.created_by,
            "created_at": self.created_at.isoformat() if self.created_at else None,
        }


async def list_recommendations(lake_id: str) -> list[dict]:
    """Return all pending recommendations for a lake.

    Args:
        lake_id: The ID of the lake to list recommendations for.

    Returns:
        List of recommendation dicts with status == "pending".
    """
    recommendations = _RECOMMENDATIONS.get(lake_id, [])
    return [rec.to_dict() for rec in recommendations if rec.status == "pending"]


async def accept_recommendation(rec_id: str, user_id: str) -> dict:
    """Accept a recommendation and create Neo4j RELATES_TO edge.

    Args:
        rec_id: The recommendation ID to accept.
        user_id: The user ID accepting the recommendation.

    Returns:
        The updated recommendation dict.

    Raises:
        ValueError: If recommendation not found.
    """
    # Find the recommendation across all lakes
    recommendation = None
    for lake_id, recs in _RECOMMENDATIONS.items():
        for rec in recs:
            if rec.id == rec_id:
                recommendation = rec
                break
        if recommendation:
            break

    if not recommendation:
        raise ValueError(f"Recommendation not found: {rec_id}")

    # Update status to accepted
    recommendation.status = "accepted"

    # Create Neo4j RELATES_TO edge
    try:
        neo4j = await get_neo4j()
        await neo4j.execute_query(
            """
            MATCH (source:Node {id: $source_node_id})
            MATCH (target:Node {id: $target_node_id})
            CREATE (source)-[:RELATES_TO {strength: $confidence, by: 'recommendation'}]->(target)
            """,
            {
                "source_node_id": recommendation.source_node_id,
                "target_node_id": recommendation.target_node_id,
                "confidence": recommendation.confidence,
            },
        )
    except Exception as e:
        logger.error("Failed to create Neo4j edge", rec_id=rec_id, error=str(e))
        raise

    logger.info("recommendation_accepted", rec_id=rec_id, user_id=user_id)

    return recommendation.to_dict()


async def reject_recommendation(rec_id: str) -> dict:
    """Mark a recommendation as rejected.

    Args:
        rec_id: The recommendation ID to reject.

    Returns:
        The updated recommendation dict.

    Raises:
        ValueError: If recommendation not found.
    """
    # Find the recommendation across all lakes
    recommendation = None
    for lake_id, recs in _RECOMMENDATIONS.items():
        for rec in recs:
            if rec.id == rec_id:
                recommendation = rec
                break
        if recommendation:
            break

    if not recommendation:
        raise ValueError(f"Recommendation not found: {rec_id}")

    recommendation.status = "rejected"
    return recommendation.to_dict()


async def ignore_recommendation(rec_id: str) -> dict:
    """Mark a recommendation as ignored (for analytics).

    Args:
        rec_id: The recommendation ID to ignore.

    Returns:
        The updated recommendation dict.

    Raises:
        ValueError: If recommendation not found.
    """
    # Find the recommendation across all lakes
    recommendation = None
    for lake_id, recs in _RECOMMENDATIONS.items():
        for rec in recs:
            if rec.id == rec_id:
                recommendation = rec
                break
        if recommendation:
            break

    if not recommendation:
        raise ValueError(f"Recommendation not found: {rec_id}")

    recommendation.status = "ignored"
    return recommendation.to_dict()


async def accept_planning_suggestion(
    suggestion_id: str, user_id: str, session: Optional[dict] = None
) -> dict:
    """Accept a planning suggestion (Phase 1: log only).

    Args:
        suggestion_id: The suggestion ID to accept.
        user_id: The user ID accepting the suggestion.
        session: Optional session data.

    Returns:
        Dict with acceptance confirmation.
    """
    # Phase 1: Log only, no actual processing
    logger.info(
        "planning_suggestion_accepted",
        suggestion_id=suggestion_id,
        user_id=user_id,
    )

    return {
        "suggestion_id": suggestion_id,
        "status": "accepted",
        "user_id": user_id,
        "message": "Planning suggestion accepted (Phase 1)",
    }


async def add_recommendation(
    lake_id: str,
    source_node_id: str,
    target_node_id: str,
    reason: str,
    confidence: float,
    created_by: Optional[str] = None,
) -> dict:
    """Add a new recommendation.

    Args:
        lake_id: The lake ID this recommendation belongs to.
        source_node_id: The source node ID.
        target_node_id: The target node ID.
        reason: The reason for the recommendation.
        confidence: The confidence score (0-1).
        created_by: Optional user ID of who created this.

    Returns:
        The newly created recommendation dict.
    """
    rec_id = str(uuid.uuid4())

    record = RecommendationRecord(
        id=rec_id,
        lake_id=lake_id,
        source_node_id=source_node_id,
        target_node_id=target_node_id,
        reason=reason,
        confidence=confidence,
        created_by=created_by,
    )

    # Store in memory
    if lake_id not in _RECOMMENDATIONS:
        _RECOMMENDATIONS[lake_id] = []

    _RECOMMENDATIONS[lake_id].append(record)

    logger.info("recommendation_added", rec_id=rec_id, lake_id=lake_id)

    return record.to_dict()