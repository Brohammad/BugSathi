-- +goose Up
CREATE TABLE share_links (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id    UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    project_id   UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    token        TEXT NOT NULL UNIQUE,
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_by   UUID NOT NULL REFERENCES users(id),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX share_links_report_id_idx ON share_links (report_id);
CREATE INDEX share_links_token_idx ON share_links (token);

-- +goose Down
DROP TABLE IF EXISTS share_links;
