-- +goose Up
-- Legacy share links stored raw tokens; M22 stores SHA-256 hex only.
DELETE FROM share_links
WHERE length(token) <> 64 OR token !~ '^[0-9a-f]+$';

CREATE INDEX IF NOT EXISTS reports_project_created_id_idx
    ON reports (project_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS share_links_report_created_id_idx
    ON share_links (project_id, report_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS report_comments_report_created_id_idx
    ON report_comments (project_id, report_id, created_at ASC, id ASC);

CREATE INDEX IF NOT EXISTS project_members_project_created_user_idx
    ON project_members (project_id, created_at ASC, user_id ASC);

CREATE INDEX IF NOT EXISTS projects_created_id_idx
    ON projects (created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS projects_created_id_idx;
DROP INDEX IF EXISTS project_members_project_created_user_idx;
DROP INDEX IF EXISTS report_comments_report_created_id_idx;
DROP INDEX IF EXISTS share_links_report_created_id_idx;
DROP INDEX IF EXISTS reports_project_created_id_idx;
