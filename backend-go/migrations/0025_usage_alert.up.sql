-- Phase 15.2: 用量告警表

CREATE TABLE IF NOT EXISTS org_usage_alerts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id),
    threshold_percent INT NOT NULL DEFAULT 80 CHECK (threshold_percent >= 0 AND threshold_percent <= 100),
    enabled BOOLEAN NOT NULL DEFAULT true,
    last_triggered_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    UNIQUE(org_id)
);

CREATE INDEX IF NOT EXISTS idx_org_usage_alerts_org_id ON org_usage_alerts(org_id);

COMMENT ON TABLE org_usage_alerts IS '组织AI用量告警配置';
COMMENT ON COLUMN org_usage_alerts.threshold_percent IS '告警阈值百分比，如80表示80%';
COMMENT ON COLUMN org_usage_alerts.enabled IS '是否启用告警';