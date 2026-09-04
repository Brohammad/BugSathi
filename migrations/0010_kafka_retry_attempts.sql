-- +goose Up
CREATE TABLE kafka_retry_attempts (
    topic      TEXT        NOT NULL,
    partition  INT         NOT NULL,
    "offset"   BIGINT      NOT NULL,
    attempts   INT         NOT NULL CHECK (attempts > 0),
    updated_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (topic, partition, "offset")
);

CREATE INDEX kafka_retry_attempts_updated_at_idx ON kafka_retry_attempts (updated_at);

-- +goose Down
DROP INDEX IF EXISTS kafka_retry_attempts_updated_at_idx;
DROP TABLE IF EXISTS kafka_retry_attempts;
