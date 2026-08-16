-- +goose Up
ALTER TABLE recordings
    ADD COLUMN processing_owner      TEXT,
    ADD COLUMN processing_expires_at TIMESTAMPTZ;

-- A claim is only meaningful while the recording is PROCESSING; both columns
-- move together so a half-set row can never look like a live lease.
ALTER TABLE recordings
    ADD CONSTRAINT recordings_processing_claim_ck CHECK (
        (processing_owner IS NULL) = (processing_expires_at IS NULL)
    );

CREATE INDEX recordings_processing_expires_at_idx
    ON recordings (processing_expires_at)
    WHERE processing_expires_at IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS recordings_processing_expires_at_idx;
ALTER TABLE recordings DROP CONSTRAINT IF EXISTS recordings_processing_claim_ck;
ALTER TABLE recordings
    DROP COLUMN IF EXISTS processing_expires_at,
    DROP COLUMN IF EXISTS processing_owner;
