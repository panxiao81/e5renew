CREATE TABLE user_tokens (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id VARCHAR(40) NOT NULL,
    access_token TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expiry TIMESTAMP NOT NULL,
    token_type VARCHAR(10) NOT NULL,
    PRIMARY KEY (id, user_id)
);
