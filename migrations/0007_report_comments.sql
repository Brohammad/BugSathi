-- +goose Up
CREATE TABLE report_comments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id   UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    project_id  UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    author_id   UUID NOT NULL REFERENCES users(id),
    body        TEXT NOT NULL CHECK (char_length(body) > 0 AND char_length(body) <= 4000),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX report_comments_report_id_idx ON report_comments (report_id, created_at);

-- +goose Down
DROP TABLE IF EXISTS report_comments;
