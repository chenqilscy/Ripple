"""认证骨架 · TODO: M1 完整实现"""

from fastapi import APIRouter, HTTPException, status
from pydantic import BaseModel, EmailStr

from app.core.security import create_access_token

router = APIRouter()


class LoginRequest(BaseModel):
    email: EmailStr
    password: str


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"


@router.post("/login", response_model=TokenResponse)
async def login(body: LoginRequest) -> TokenResponse:
    # TODO: M1 实现用户查库 + bcrypt 校验
    if body.email != "demo@ripple.dev" or body.password != "demo":
        raise HTTPException(status_code=status.HTTP_401_UNAUTHORIZED, detail="Bad credentials")
    token = create_access_token(subject="demo-user-id", extra_claims={"email": body.email})
    return TokenResponse(access_token=token)
