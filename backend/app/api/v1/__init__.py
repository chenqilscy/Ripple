"""API v1 路由聚合 · 参考 API网关设计.md"""

from fastapi import APIRouter

from app.api.v1 import auth, cloud, iceberg, lakes, nodes, shares

api_router = APIRouter()
api_router.include_router(auth.router, prefix="/auth", tags=["auth"])
api_router.include_router(lakes.router, prefix="/lakes", tags=["lakes"])
api_router.include_router(nodes.router, prefix="/nodes", tags=["nodes"])
api_router.include_router(cloud.router, prefix="/cloud", tags=["cloud-云霓"])
api_router.include_router(iceberg.router, prefix="/icebergs", tags=["icebergs-冰山"])
api_router.include_router(shares.router, prefix="/shares", tags=["shares-抛饵"])
