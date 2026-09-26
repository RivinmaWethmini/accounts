CREATE TABLE consented_redirects (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    host    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, host)
);