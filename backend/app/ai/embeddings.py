"""本地 Embedding · M1 占位实现（hash-based 确定性伪向量）

M2 将替换为 BGE-large-zh (sentence-transformers)。
占位保留同一接口：embed(text) -> list[float]，维度 256。

参考：G5 §6.3 混合推理。
"""

import hashlib
import math

DIM = 256


def embed(text: str) -> list[float]:
    """确定性 hash embedding（只为 M1 冷启通路和单测稳定）"""
    if not text:
        return [0.0] * DIM
    h = hashlib.sha256(text.encode("utf-8")).digest()
    # 重复 hash 摘要到 DIM*4 bytes，每 4 bytes 读 1 float
    seed = h
    buf = bytearray()
    while len(buf) < DIM * 4:
        seed = hashlib.sha256(seed).digest()
        buf.extend(seed)
    vec = [
        int.from_bytes(buf[i * 4 : (i + 1) * 4], "big", signed=False) / 2**32 - 0.5
        for i in range(DIM)
    ]
    norm = math.sqrt(sum(x * x for x in vec)) or 1.0
    return [x / norm for x in vec]


def cosine(a: list[float], b: list[float]) -> float:
    if len(a) != len(b):
        return 0.0
    dot = sum(x * y for x, y in zip(a, b, strict=False))
    # 已经归一化，dot 即 cosine
    return max(-1.0, min(1.0, dot))
