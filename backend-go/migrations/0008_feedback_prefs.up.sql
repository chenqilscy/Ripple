-- M3-S3 数据基础：用户反馈事件 + 用户偏好
--
-- feedback_events:
--   记录用户对节点/边/perma 的反馈（点赞、点踩、评论、举报、稀有度评分等）。
--   用于：
--     1. AI Weaver 后续候选排序（A/B + 用户偏好加权）；
--     2. 推荐系统训练（社区亮点）；
--     3. 节点凝结优先级（高 reaction 的 mist 优先候选）。
--
-- user_preferences:
--   按用户存 KV 偏好（主题、节点默认 type、AI 风格、偏好的 LLM 等）。
--   用 JSONB 而非纵向表：偏好字段会持续扩张，schema-less 更合适。

CREATE TABLE IF NOT EXISTS feedback_events (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- 目标对象类型与 ID。string 是为了支持后续扩展（lake/node/edge/perma/comment...）
    target_type VARCHAR(32) NOT NULL,
    target_id UUID NOT NULL,
    -- 事件类型：LIKE / DISLIKE / RARE / REPORT / COMMENT / EMOJI ...
    event_type VARCHAR(32) NOT NULL,
    -- 数值或字符串负载（Like 可不填；Rare 可填 1-5；Comment 填文本；Emoji 填字符）
    payload JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 高频读：某 target 收到的反馈聚合
CREATE INDEX IF NOT EXISTS idx_feedback_target ON feedback_events(target_type, target_id, created_at DESC);
-- 用户行为流
CREATE INDEX IF NOT EXISTS idx_feedback_user ON feedback_events(user_id, created_at DESC);
-- 同一用户对同一 target 的同类型事件，业务侧可去重（PG 层不强约束，留给 service）

CREATE TABLE IF NOT EXISTS user_preferences (
    user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    prefs JSONB NOT NULL DEFAULT '{}'::JSONB,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TRIGGER trg_user_prefs_updated_at
BEFORE UPDATE ON user_preferences
FOR EACH ROW EXECUTE FUNCTION trg_set_updated_at();
