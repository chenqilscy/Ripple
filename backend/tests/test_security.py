"""JWT 安全层单元测试"""

import pytest
from fastapi import HTTPException

from app.core.security import create_access_token, decode_token


def test_roundtrip() -> None:
    tok = create_access_token("user-42", {"email": "a@b.c"})
    claims = decode_token(tok)
    assert claims["sub"] == "user-42"
    assert claims["email"] == "a@b.c"
    assert claims["typ"] == "access"


def test_bad_token_rejected() -> None:
    with pytest.raises(HTTPException) as exc:
        decode_token("not.a.jwt")
    assert exc.value.status_code == 401
