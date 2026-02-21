PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS posts (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    source       TEXT NOT NULL,
    channel      TEXT NOT NULL,
    external_id  TEXT NOT NULL,
    text         TEXT,
    snippet      TEXT NOT NULL,
    text_hash    TEXT NOT NULL,
    url          TEXT,
    posted_at    DATETIME NOT NULL,
    fetched_at   DATETIME NOT NULL,
    UNIQUE(source, channel, external_id)
);

CREATE TABLE IF NOT EXISTS scores (
    post_id      INTEGER PRIMARY KEY REFERENCES posts(id),
    score        INTEGER NOT NULL DEFAULT 0,
    labels       TEXT,
    tier         TEXT NOT NULL DEFAULT 'ignore',
    scored_at    DATETIME NOT NULL,
    explanation  TEXT
);

CREATE TABLE IF NOT EXISTS post_also_in (
    post_id  INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    source   TEXT NOT NULL,
    channel  TEXT NOT NULL,
    UNIQUE(post_id, source, channel)
);

CREATE TABLE IF NOT EXISTS metadata (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_posts_posted_at ON posts(posted_at);
CREATE INDEX IF NOT EXISTS idx_posts_text_hash ON posts(text_hash);
CREATE INDEX IF NOT EXISTS idx_posts_source_channel ON posts(source, channel);
CREATE INDEX IF NOT EXISTS idx_scores_tier ON scores(tier);
