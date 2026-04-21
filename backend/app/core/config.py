"""全局配置 · 从环境变量加载"""

from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    model_config = SettingsConfigDict(env_file=".env", env_file_encoding="utf-8", extra="ignore")

    # App
    ENV: str = "development"
    DEBUG: bool = True
    SECRET_KEY: str = "change-me-in-production"
    CORS_ORIGINS: list[str] = ["http://localhost:3000", "http://localhost:5173"]

    # Neo4j
    NEO4J_URI: str = "bolt://localhost:7687"
    NEO4J_USER: str = "neo4j"
    NEO4J_PASSWORD: str = "ripple_dev_pwd"
    NEO4J_DATABASE: str = "neo4j"

    # PostgreSQL
    POSTGRES_DSN: str = "postgresql+psycopg://ripple:ripple_dev_pwd@localhost:5432/ripple"

    # Redis
    REDIS_URL: str = "redis://localhost:6379/0"

    # S3 / MinIO
    S3_ENDPOINT: str = "http://localhost:9000"
    S3_ACCESS_KEY: str = "minioadmin"
    S3_SECRET_KEY: str = "minioadmin"
    S3_BUCKET_RAW: str = "ripple-uploads-raw"
    S3_BUCKET_PROCESSED: str = "ripple-processed"
    S3_BUCKET_EXPORTS: str = "ripple-exports"
    S3_BUCKET_SNAPSHOTS: str = "ripple-snapshots"
    S3_BUCKET_PUBLIC: str = "ripple-public-shares"

    # JWT
    JWT_ALGORITHM: str = "HS256"
    JWT_ACCESS_TTL_MIN: int = 60 * 24
    JWT_REFRESH_TTL_DAY: int = 30

    # AI
    AI_ROUTER_ENDPOINT: str = "http://localhost:8001"
    AI_WEAVER_MODEL: str = "qwen2.5-72b"
    AI_CRITIC_MODEL: str = "claude-haiku"
    AI_EMBEDDING_MODEL: str = "bge-large-zh"

    # Rate Limits
    RATE_LIMIT_PER_MIN: int = 100
    AI_RECOVER_PER_LAKE_PER_SEC: int = 10
    AI_RECOVER_PER_USER_PER_MIN: int = 50


@lru_cache
def get_settings() -> Settings:
    return Settings()


settings = get_settings()
