"""密码哈希 · bcrypt 直接调用

绕开 passlib（4.x bcrypt 兼容问题），手动截断到 bcrypt 的 72 字节上限。
"""

import bcrypt

_BCRYPT_MAX = 72


def hash_password(plain: str) -> str:
    pw = plain.encode("utf-8")[:_BCRYPT_MAX]
    return bcrypt.hashpw(pw, bcrypt.gensalt()).decode("utf-8")


def verify_password(plain: str, hashed: str) -> bool:
    pw = plain.encode("utf-8")[:_BCRYPT_MAX]
    try:
        return bcrypt.checkpw(pw, hashed.encode("utf-8"))
    except ValueError:
        return False
