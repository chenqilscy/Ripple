CREATE TABLE IF NOT EXISTS node_tags (
    node_id    UUID        NOT NULL,
    lake_id    UUID        NOT NULL,
    tag        VARCHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (node_id, tag)
);

CREATE INDEX IF NOT EXISTS node_tags_lake_tag_idx ON node_tags (lake_id, tag);
