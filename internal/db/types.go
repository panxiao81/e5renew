package db

import pgdb "github.com/panxiao81/e5renew/internal/db/postgres"

type ApiLog = pgdb.ApiLog
type Session = pgdb.Session
type UserToken = pgdb.UserToken

type CreateAPILogParams = pgdb.CreateAPILogParams
type CreateUserTokensParams = pgdb.CreateUserTokensParams
type GetAPILogStatsParams = pgdb.GetAPILogStatsParams
type GetAPILogStatsRow = pgdb.GetAPILogStatsRow
type GetAPILogStatsByEndpointParams = pgdb.GetAPILogStatsByEndpointParams
type GetAPILogStatsByEndpointRow = pgdb.GetAPILogStatsByEndpointRow
type GetAPILogsParams = pgdb.GetAPILogsParams
type GetAPILogsByJobTypeParams = pgdb.GetAPILogsByJobTypeParams
type GetAPILogsByTimeRangeParams = pgdb.GetAPILogsByTimeRangeParams
type GetAPILogsByUserParams = pgdb.GetAPILogsByUserParams
type UpdateUserTokensParams = pgdb.UpdateUserTokensParams
