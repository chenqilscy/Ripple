"""认证 API · 参考 G7 §二 JWT + 故事 5 注册"""

import uuid

from fastapi import APIRouter, Depends
from pydantic import BaseModel, EmailStr, Field
from sqlalchemy.ext.asyncio import AsyncSession

from app.core.db import get_pg_session
from app.core.security import create_access_token
from app.services.auth_service import authenticate, register_user

router = APIRouter()


class RegisterRequest(BaseModel):
    email: EmailStr
    password: str = Field(..., min_length=8, max_length=128)
    display_name: str = Field(..., min_length=1, max_length=64)


class LoginRequest(BaseModel):
    email: EmailStr
    password: str


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"
    user_id: uuid.UUID
    email: str


@router.post("/register", response_model=TokenResponse, status_code=201)
async def register(
    body: RegisterRequest, session: AsyncSession = Depends(get_pg_session)
) -> TokenResponse:
    user = await register_user(session, body.email, body.password, body.display_name)
    token = create_access_token(str(user.id), {"email": user.email})
    return TokenResponse(access_token=token, user_id=user.id, email=user.email)


@router.post("/login", response_model=TokenResponse)
async def login(
    body: LoginRequest, session: AsyncSession = Depends(get_pg_session)
) -> TokenResponse:
    user = await authenticate(session, body.email, body.password)
    token = create_access_token(str(user.id), {"email": user.email})
    return TokenResponse(access_token=token, user_id=user.id, email=user.email)
