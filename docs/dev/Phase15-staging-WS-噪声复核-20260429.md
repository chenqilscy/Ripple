# Phase15 staging WebSocket 噪声复核

日期：2026-04-29
文档层级：`docs/dev/*.md`
关联文档：`docs/dev/Phase14-6-运营化回归与准入设计-20260428.md`、`docs/launch/Phase14-准入清单.md`

## 1. 背景

此前在 staging 浏览器快照中看到多条 WebSocket `403 / 502` 控制台报错，怀疑 `LakeWS` 或 `yjs-bridge` 仍存在线上抖动。

本次复核目标不是重放历史日志，而是判断 **当前态** 是否仍能稳定完成握手。

## 2. 复核步骤

### 2.1 staging 基础探针

执行：

```powershell
$env:RIPPLE_STAGING_BASE='http://fn.cky:18000'
$env:RIPPLE_STAGING_FRONTEND_BASE='http://fn.cky:14173'
powershell -ExecutionPolicy Bypass -File scripts/smoke/phase14-6-acceptance.ps1 -SkipBackend -SkipFrontend -SkipE2E
```

结果：

- `staging: backend healthz` PASS
- `staging: /yjs returns 400 (lake required)` PASS
- transcript 已归档到 `docs/launch/acceptance-logs/acceptance-20260429-051231.log`

说明：frontend `/yjs` 当前没有错误回落到 SPA HTML，backend 也处于健康态。

### 2.2 浏览器当前态观察

直接打开 `http://fn.cky:14173/`，页面顶部可见“实时连接已建立”状态文案，说明至少有一条当前用户会话的 `LakeWS` 连接已经成功。

### 2.3 真实浏览器 Origin 双握手探针

在 `http://fn.cky:14173/` 页面上下文内执行只读探针：

1. 复用当前已登录会话 token；
2. 调 `POST /api/v1/ws_token` 换取 ws-only token；
3. 以浏览器真实 Origin 发起两条握手：
   - `LakeWS`: `ws://fn.cky:18000/api/v1/lakes/{lakeId}/ws?access_token=...`
   - `yjs-bridge`: `ws://fn.cky:14173/yjs?lake={lakeId}&node={probeNodeId}&token={wsToken}`

结果：

- `LakeWS`：`open`
- `yjs-bridge`：`open`

说明：在当前 staging 配置下，`OriginPatterns`、nginx `/yjs` 反代、ws-only token 链路均可完成真实浏览器握手。

## 3. 额外观察

- 试图用临时探针邮箱注册时返回 `403 permission denied: registration email not in graylist`，这与 WebSocket 无关，说明 staging 当前启用了注册灰度门。
- yjs-bridge 对不存在的 `node` 参数不会因快照 `404` 拒绝握手；快照缺失只会被忽略，因此本次探针能把“连接层失败”和“业务数据不存在”区分开。

## 4. 结论

本次复核 **未发现当前仍在发生的 WebSocket 403 / 502 故障**。

更合理的解释是：

1. 先前浏览器快照中的 `403 / 502` 属于历史控制台噪声，而非当前活跃错误；
2. 当前 staging 至少在以下 3 层均为绿色：
   - backend `/healthz`
   - frontend `/yjs` 反代
   - 真实浏览器 Origin 下的 `LakeWS` / `yjs-bridge` 握手

## 5. 后续建议

若后续再次出现浏览器中的 WS 报错，不要直接按线上事故处理，先按下面顺序复核：

1. 看日志或控制台时间戳，确认是不是历史残留。
2. 运行 `phase14-6-acceptance.ps1` 的 staging-only 检查。
3. 在浏览器上下文内复跑一次双握手探针，确认 `LakeWS` 与 `yjs` 是否都还是 `open`。
4. 仅当当前态探针失败时，再继续排查 `Origin` 配置、backend 健康和 yjs-bridge 可达性。