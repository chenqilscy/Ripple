"""AI Weaver 嵌入层 · 离线确定性测试"""

import math

from app.ai.embeddings import DIM, cosine, embed


def test_embed_dim_and_normalized() -> None:
    v = embed("青萍之末")
    assert len(v) == DIM
    norm = math.sqrt(sum(x * x for x in v))
    assert abs(norm - 1.0) < 1e-6


def test_embed_is_deterministic() -> None:
    assert embed("石子入水") == embed("石子入水")


def test_embed_distinguishes_text() -> None:
    a = embed("清风吹皱")
    b = embed("一池春水")
    assert a != b
    # 不同文本的余弦通常显著小于 1
    assert cosine(a, b) < 0.95


def test_cosine_self_is_one() -> None:
    v = embed("涟漪")
    assert abs(cosine(v, v) - 1.0) < 1e-6
