CREATE TABLE api_logs (
    id BIGSERIAL PRIMARY KEY,
    user_id VARCHAR(40) NULL,
    api_endpoint VARCHAR(255) NOT NULL,
    http_method VARCHAR(10) NOT NULL,
    http_status_code INT NOT NULL,
    request_time TIMESTAMPTZ NOT NULL,
    response_time TIMESTAMPTZ NOT NULL,
    duration_ms INT NOT NULL,
    request_size INT DEFAULT 0,
    response_size INT DEFAULT 0,
    error_message TEXT NULL,
    job_type VARCHAR(50) NOT NULL,
    success BOOLEAN NOT NULL
);

CREATE INDEX idx_user_id ON api_logs (user_id);
CREATE INDEX idx_request_time ON api_logs (request_time);
CREATE INDEX idx_api_endpoint ON api_logs (api_endpoint);
CREATE INDEX idx_job_type ON api_logs (job_type);
