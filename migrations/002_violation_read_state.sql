ALTER TABLE violations
    ADD COLUMN IF NOT EXISTS read_at timestamptz;

CREATE INDEX IF NOT EXISTS violations_unread_event_idx
    ON violations(event_at DESC, id)
    WHERE read_at IS NULL;
