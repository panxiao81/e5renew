-- name: GetUserToken :one
select *
from user_tokens
where user_id = $1
limit 1;

-- name: ListUserTokens :many
select *
from user_tokens
order by user_id;

-- name: CreateUserTokens :execresult
insert into user_tokens (
        user_id,
        access_token,
        refresh_token,
        expiry,
        token_type
    )
values ($1, $2, $3, $4, $5);

-- name: UpdateUserTokens :execresult
update user_tokens
set access_token = $1,
    refresh_token = $2,
    expiry = $3,
    token_type = $4
where user_id = $5;

-- name: DeleteUserTokens :exec
delete from user_tokens
where user_id = $1;

-- name: CreateAPILog :execresult
INSERT INTO api_logs (
    user_id,
    api_endpoint,
    http_method,
    http_status_code,
    request_time,
    response_time,
    duration_ms,
    request_size,
    response_size,
    error_message,
    job_type,
    success
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: GetAPILogs :many
SELECT * FROM api_logs
ORDER BY request_time DESC
LIMIT $1 OFFSET $2;

-- name: GetAPILogsByUser :many
SELECT * FROM api_logs
WHERE user_id = $1
ORDER BY request_time DESC
LIMIT $2 OFFSET $3;

-- name: GetAPILogsByTimeRange :many
SELECT * FROM api_logs
WHERE request_time >= $1 AND request_time <= $2
ORDER BY request_time DESC
LIMIT $3 OFFSET $4;

-- name: GetAPILogsByJobType :many
SELECT * FROM api_logs
WHERE job_type = $1
ORDER BY request_time DESC
LIMIT $2 OFFSET $3;

-- name: GetAPILogStats :one
SELECT
    COUNT(*) as total_requests,
    COUNT(CASE WHEN success = true THEN 1 END) as successful_requests,
    COUNT(CASE WHEN success = false THEN 1 END) as failed_requests,
    AVG(duration_ms) as avg_duration_ms,
    MIN(duration_ms) as min_duration_ms,
    MAX(duration_ms) as max_duration_ms
FROM api_logs
WHERE request_time >= $1 AND request_time <= $2;

-- name: GetAPILogStatsByEndpoint :many
SELECT
    api_endpoint,
    COUNT(*) as total_requests,
    COUNT(CASE WHEN success = true THEN 1 END) as successful_requests,
    COUNT(CASE WHEN success = false THEN 1 END) as failed_requests,
    AVG(duration_ms) as avg_duration_ms
FROM api_logs
WHERE request_time >= $1 AND request_time <= $2
GROUP BY api_endpoint
ORDER BY total_requests DESC;

-- name: DeleteOldAPILogs :exec
DELETE FROM api_logs
WHERE request_time < $1;
