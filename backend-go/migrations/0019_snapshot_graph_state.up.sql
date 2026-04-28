-- P21: 图谱内容快照 diff（在布局快照基础上增加 graph_state 列）
ALTER TABLE lake_snapshots ADD COLUMN graph_state JSONB;
