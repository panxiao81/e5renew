create table user_tokens (
    id bigint not null auto_increment,
    user_id varchar(40) not null,
    access_token text not null,
    refresh_token text not null,
    expiry timestamp not null,
    token_type varchar(10) not null,
    primary key (id, user_id)
);

CREATE TABLE sessions (
	token CHAR(43) PRIMARY KEY,
	data BLOB NOT NULL,
	expiry TIMESTAMP(6) NOT NULL
);

CREATE INDEX sessions_expiry_idx ON sessions (expiry);

CREATE TABLE api_logs (
    id BIGINT NOT NULL AUTO_INCREMENT,
    user_id VARCHAR(40) NULL,          -- NULL for client credential calls
    api_endpoint VARCHAR(255) NOT NULL, -- e.g., "users", "me/messages"
    http_method VARCHAR(10) NOT NULL,   -- GET, POST, etc.
    http_status_code INT NOT NULL,      -- 200, 401, 500, etc.
    request_time TIMESTAMP NOT NULL,    -- When the request was made
    response_time TIMESTAMP NOT NULL,   -- When the response was received
    duration_ms INT NOT NULL,           -- Request duration in milliseconds
    request_size INT DEFAULT 0,         -- Request body size
    response_size INT DEFAULT 0,        -- Response body size
    error_message TEXT NULL,            -- Error details if failed
    job_type VARCHAR(50) NOT NULL,      -- "client_credentials", "user_mail", etc.
    success BOOLEAN NOT NULL,           -- TRUE/FALSE
    PRIMARY KEY (id),
    INDEX idx_user_id (user_id),
    INDEX idx_request_time (request_time),
    INDEX idx_api_endpoint (api_endpoint),
    INDEX idx_job_type (job_type)
);
