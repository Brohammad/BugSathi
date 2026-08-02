-- +goose Up
CREATE TABLE recordings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    created_by      UUID NOT NULL REFERENCES users(id),
    status          TEXT NOT NULL CHECK (status IN (
        'UPLOADING', 'UPLOADED', 'PROCESSING', 'READY', 'FAILED'
    )),
    storage_key     TEXT NOT NULL,
    content_type    TEXT NOT NULL DEFAULT 'application/octet-stream',
    byte_size       BIGINT,
    checksum        TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    correlation_id  TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX recordings_project_id_idx ON recordings (project_id);
CREATE INDEX recordings_status_idx ON recordings (status);

CREATE TABLE outbox (
    id              BIGSERIAL PRIMARY KEY,
    topic           TEXT NOT NULL,
    partition_key   TEXT NOT NULL,
    payload         JSONB NOT NULL,
    correlation_id  TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ
);

CREATE INDEX outbox_unpublished_idx ON outbox (id) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS recordings;
