ALTER TABLE user_tokens
    ALTER COLUMN access_token TYPE VARCHAR(255),
    ALTER COLUMN refresh_token TYPE VARCHAR(255);
