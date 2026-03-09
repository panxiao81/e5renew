-- name: GetUserToken :one
select *
from user_tokens
where user_id = ?
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
values (?, ?, ?, ?, ?);
-- name: UpdateUserTokens :execresult
update user_tokens
set access_token = ?,
    refresh_token = ?,
    expiry = ?,
    token_type = ?
where user_id = ?;
-- name: DeleteUserTokens :exec
delete from user_tokens
where user_id = ?;

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
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetAPILogs :many
SELECT * FROM api_logs
ORDER BY request_time DESC
LIMIT ? OFFSET ?;

-- name: GetAPILogsByUser :many
SELECT * FROM api_logs
WHERE user_id = ?
ORDER BY request_time DESC
LIMIT ? OFFSET ?;

-- name: GetAPILogsByTimeRange :many
SELECT * FROM api_logs
WHERE request_time >= ? AND request_time <= ?
ORDER BY request_time DESC
LIMIT ? OFFSET ?;

-- name: GetAPILogsByJobType :many
SELECT * FROM api_logs
WHERE job_type = ?
ORDER BY request_time DESC
LIMIT ? OFFSET ?;

-- name: GetAPILogStats :one
SELECT 
    COUNT(*) as total_requests,
    COUNT(CASE WHEN success = true THEN 1 END) as successful_requests,
    COUNT(CASE WHEN success = false THEN 1 END) as failed_requests,
    AVG(duration_ms) as avg_duration_ms,
    MIN(duration_ms) as min_duration_ms,
    MAX(duration_ms) as max_duration_ms
FROM api_logs
WHERE request_time >= ? AND request_time <= ?;

-- name: GetAPILogStatsByEndpoint :many
SELECT 
    api_endpoint,
    COUNT(*) as total_requests,
    COUNT(CASE WHEN success = true THEN 1 END) as successful_requests,
    COUNT(CASE WHEN success = false THEN 1 END) as failed_requests,
    AVG(duration_ms) as avg_duration_ms
FROM api_logs
WHERE request_time >= ? AND request_time <= ?
GROUP BY api_endpoint
ORDER BY total_requests DESC;

-- name: DeleteOldAPILogs :exec
DELETE FROM api_logs
WHERE request_time < ?;