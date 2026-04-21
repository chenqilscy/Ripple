"""密码服务单元测试"""

from app.services.password import hash_password, verify_password


def test_hash_and_verify() -> None:
    h = hash_password("correct-horse-battery")
    assert h != "correct-horse-battery"
    assert verify_password("correct-horse-battery", h)
    assert not verify_password("wrong", h)


def test_hashes_are_salted() -> None:
    assert hash_password("same") != hash_password("same")
