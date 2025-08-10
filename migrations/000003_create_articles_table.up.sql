CREATE TABLE articles
(
    id              SERIAL PRIMARY KEY,
    slug            VARCHAR(255) UNIQUE NOT NULL,
    title           VARCHAR(255)        NOT NULL,
    description     TEXT,
    body            TEXT                NOT NULL,
    tag_list        TEXT[], -- PostgreSQL array for tags
    created_at      TIMESTAMP DEFAULT (NOW() AT TIME ZONE 'UTC'),
    updated_at      TIMESTAMP DEFAULT (NOW() AT TIME ZONE 'UTC'),
    favorites_count INTEGER   DEFAULT 0,
    author_id       INTEGER             NOT NULL,
    FOREIGN KEY (author_id) REFERENCES users (id) ON DELETE CASCADE
);

-- Index for better query performance
CREATE INDEX idx_articles_slug ON articles (slug);
CREATE INDEX idx_articles_author_id ON articles (author_id);
CREATE INDEX idx_articles_created_at ON articles (created_at DESC);
