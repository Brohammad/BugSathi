-- +goose Up
CREATE TABLE analyses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recording_id    UUID NOT NULL REFERENCES recordings(id) ON DELETE CASCADE,
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    prompt_version  TEXT NOT NULL,
    status          TEXT NOT NULL CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    title           TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    steps           JSONB NOT NULL DEFAULT '[]'::jsonb,
    provider        TEXT NOT NULL DEFAULT '',
    model           TEXT NOT NULL DEFAULT '',
    error_message   TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (recording_id, prompt_version)
);

CREATE TABLE reports (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recording_id    UUID NOT NULL UNIQUE REFERENCES recordings(id) ON DELETE CASCADE,
    project_id      UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    status          TEXT NOT NULL CHECK (status IN ('PENDING', 'GENERATING', 'READY', 'FAILED')),
    title           TEXT NOT NULL DEFAULT '',
    summary         TEXT NOT NULL DEFAULT '',
    steps           JSONB NOT NULL DEFAULT '[]'::jsonb,
    ai_status       TEXT NOT NULL DEFAULT '',
    prompt_version  TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX analyses_recording_id_idx ON analyses (recording_id);
CREATE INDEX reports_project_id_idx ON reports (project_id);

-- +goose Down
DROP TABLE IF EXISTS reports;
DROP TABLE IF EXISTS analyses;
