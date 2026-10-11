-- +goose Up
-- Reliable subscribers own durable queues, independent of live SSE delivery.
-- A completed row is retained until its source is deleted, preventing a
-- repeated source delivery from resurrecting an already acknowledged job.
CREATE TABLE policy_outbox (
    seq BIGSERIAL PRIMARY KEY,
    source_seq BIGINT NOT NULL,
    subscriber TEXT NOT NULL,
    type TEXT NOT NULL,
    payload JSONB NOT NULL,
    completed BOOLEAN NOT NULL DEFAULT false,
    UNIQUE (source_seq, subscriber)
);
CREATE INDEX policy_outbox_pending ON policy_outbox (subscriber, seq) WHERE NOT completed;

-- +goose Down
DROP TABLE policy_outbox;
