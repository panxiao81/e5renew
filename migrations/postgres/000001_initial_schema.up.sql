CREATE TABLE user_tokens (
    id BIGSERIAL NOT NULL,
    user_id VARCHAR(40) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expiry TIMESTAMPTZ NOT NULL,
    token_type VARCHAR(10) NOT NULL,
    PRIMARY KEY (id, user_id)
);
