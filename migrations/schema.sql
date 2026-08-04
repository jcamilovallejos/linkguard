CREATE TABLE IF NOT EXISTS urls (
    shortcode    TEXT PRIMARY KEY,
    original_url TEXT NOT NULL,
    clicks       BIGINT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
