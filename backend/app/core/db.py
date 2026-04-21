"""数据库连接 · Neo4j + PostgreSQL + Redis

实现参考：docs/system-design/G1-数据模型与权限设计.md
"""

from collections.abc import AsyncIterator

from neo4j import AsyncDriver, AsyncGraphDatabase
from redis.asyncio import Redis
from sqlalchemy.ext.asyncio import (
    AsyncEngine,
    AsyncSession,
    async_sessionmaker,
    create_async_engine,
)

from app.core.config import settings
from app.core.logging import logger

_neo4j_driver: AsyncDriver | None = None
_pg_engine: AsyncEngine | None = None
_pg_sessionmaker: async_sessionmaker[AsyncSession] | None = None
_redis: Redis | None = None


NEO4J_CONSTRAINTS = [
    "CREATE CONSTRAINT lake_id IF NOT EXISTS FOR (l:Lake) REQUIRE l.id IS UNIQUE",
    "CREATE CONSTRAINT node_id IF NOT EXISTS FOR (n:Node) REQUIRE n.id IS UNIQUE",
    "CREATE CONSTRAINT iceberg_id IF NOT EXISTS FOR (i:Iceberg) REQUIRE i.id IS UNIQUE",
    "CREATE INDEX node_lake_state IF NOT EXISTS FOR (n:Node) ON (n.lake_id, n.state)",
    "CREATE INDEX node_ttl IF NOT EXISTS FOR (n:Node) ON (n.ttl_at)",
]


async def init_db() -> None:
    global _neo4j_driver, _pg_engine, _pg_sessionmaker, _redis

    _neo4j_driver = AsyncGraphDatabase.driver(
        settings.NEO4J_URI,
        auth=(settings.NEO4J_USER, settings.NEO4J_PASSWORD),
    )
    await _neo4j_driver.verify_connectivity()
    async with _neo4j_driver.session(database=settings.NEO4J_DATABASE) as s:
        for stmt in NEO4J_CONSTRAINTS:
            await s.run(stmt)  # type: ignore[arg-type]
    logger.info("neo4j_initialized")

    _pg_engine = create_async_engine(
        settings.POSTGRES_DSN,
        echo=settings.DEBUG,
        pool_pre_ping=True,
    )
    _pg_sessionmaker = async_sessionmaker(_pg_engine, expire_on_commit=False)

    # 开发模式自动建表；生产应走 Alembic
    if settings.ENV == "development":
        from app.models import Base

        async with _pg_engine.begin() as conn:
            await conn.run_sync(Base.metadata.create_all)
        logger.info("pg_schema_created")

    _redis = Redis.from_url(settings.REDIS_URL, decode_responses=True)
    await _redis.ping()
    logger.info("redis_connected")


async def close_db() -> None:
    if _neo4j_driver:
        await _neo4j_driver.close()
    if _pg_engine:
        await _pg_engine.dispose()
    if _redis:
        await _redis.close()


def get_neo4j() -> AsyncDriver:
    if _neo4j_driver is None:
        raise RuntimeError("Neo4j driver not initialized; call init_db() first")
    return _neo4j_driver


async def get_pg_session() -> AsyncIterator[AsyncSession]:
    if _pg_sessionmaker is None:
        raise RuntimeError("PG sessionmaker not initialized; call init_db() first")
    async with _pg_sessionmaker() as session:
        yield session


def get_redis() -> Redis:
    if _redis is None:
        raise RuntimeError("Redis not initialized; call init_db() first")
    return _redis
