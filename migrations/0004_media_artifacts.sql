-- +goose Up
CREATE TABLE media_artifacts (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recording_id  UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    kind          TEXT NOT NULL CHECK (kind IN ('frame', 'thumb')),
    storage_key   TEXT NOT NULL,
    ordinal       INT NOT NULL DEFAULT 0,
    content_type  TEXT NOT NULL DEFAULT 'image/jpeg',
    byte_size     BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (recording_id, kind, ordinal)
);

CREATE INDEX media_artifacts_recording_id_idx ON media_artifacts (recording_id);

-- +goose Down
DROP TABLE IF EXISTS media_artifacts;
