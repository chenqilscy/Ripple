"""图谱分析 API"""
from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel
from typing import Optional

from app.services.graph_service import (
    compute_path,
    compute_clusters,
    compute_planning_suggestions,
)
from app.services.recommendation_service import (
    list_recommendations,
    accept_recommendation,
    reject_recommendation,
    ignore_recommendation,
    accept_planning_suggestion,
)
from app.core.auth import get_current_user

router = APIRouter()

# Request/Response models
class AcceptRecRequest(BaseModel):
    rec_id: str
class RejectRecRequest(BaseModel):
    rec_id: str
class IgnoreRecRequest(BaseModel):
    rec_id: str
class AcceptPlanningRequest(BaseModel):
    suggestion_id: str
class PathRequest(BaseModel):
    source_id: str
    target_id: str
class ClusterRequest(BaseModel):
    lake_id: str
class PlanningRequest(BaseModel):
    lake_id: str

# GET /recommendations?lake_id={lake_id}
@router.get("/recommendations")
async def api_list_recommendations(lake_id: str, user=Depends(get_current_user)):
    return await list_recommendations(lake_id)

# POST /recommendations/accept
@router.post("/recommendations/accept")
async def api_accept_recommendation(body: AcceptRecRequest, user=Depends(get_current_user)):
    rec = await accept_recommendation(body.rec_id, user["id"])
    if not rec:
        raise HTTPException(404, "Recommendation not found")
    return rec

# POST /recommendations/reject
@router.post("/recommendations/reject")
async def api_reject_recommendation(body: RejectRecRequest, user=Depends(get_current_user)):
    rec = await reject_recommendation(body.rec_id)
    if not rec:
        raise HTTPException(404, "Recommendation not found")
    return rec

# POST /recommendations/ignore
@router.post("/recommendations/ignore")
async def api_ignore_recommendation(body: IgnoreRecRequest, user=Depends(get_current_user)):
    rec = await ignore_recommendation(body.rec_id)
    if not rec:
        raise HTTPException(404, "Recommendation not found")
    return rec

# GET /path?source_id={}&target_id={}
@router.get("/path")
async def api_get_path(source_id: str, target_id: str, user=Depends(get_current_user)):
    result = await compute_path(source_id, target_id)
    if result is None:
        raise HTTPException(404, "No path found")
    return result

# GET /clusters?lake_id={lake_id}
@router.get("/clusters")
async def api_get_clusters(lake_id: str, user=Depends(get_current_user)):
    return await compute_clusters(lake_id)

# GET /planning?lake_id={lake_id}
@router.get("/planning")
async def api_get_planning(lake_id: str, user=Depends(get_current_user)):
    return await compute_planning_suggestions(lake_id)

# POST /planning/accept
@router.post("/planning/accept")
async def api_accept_planning(body: AcceptPlanningRequest, user=Depends(get_current_user)):
    return await accept_planning_suggestion(body.suggestion_id, user["id"], None)
