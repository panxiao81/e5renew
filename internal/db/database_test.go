package db

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type statsPingDB struct {
	pingErr error
	stats   sql.DBStats
}

func (s *statsPingDB) ExecContext(_ context.Context, _ string, _ ...interface{}) (sql.Result, error) {
	return nil, nil
}

func (s *statsPingDB) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error) {
	return nil, nil
}

func (s *statsPingDB) QueryContext(_ context.Context, _ string, _ ...interface{}) (*sql.Rows, error) {
	return nil, nil
}

func (s *statsPingDB) QueryRowContext(_ context.Context, _ string, _ ...interface{}) *sql.Row {
	return &sql.Row{}
}

func (s *statsPingDB) PingContext(_ context.Context) error {
	return s.pingErr
}

func (s *statsPingDB) Stats() sql.DBStats {
	return s.stats
}

type dbWithoutStatsPing struct{}

func (d *dbWithoutStatsPing) ExecContext(_ context.Context, _ string, _ ...interface{}) (sql.Result, error) {
	return nil, nil
}
func (d *dbWithoutStatsPing) PrepareContext(_ context.Context, _ string) (*sql.Stmt, error) {
	return nil, nil
}
func (d *dbWithoutStatsPing) QueryContext(_ context.Context, _ string, _ ...interface{}) (*sql.Rows, error) {
	return nil, nil
}
func (d *dbWithoutStatsPing) QueryRowContext(_ context.Context, _ string, _ ...interface{}) *sql.Row {
	return &sql.Row{}
}

func TestParseEngine(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  Engine
	}{
		{name: "empty defaults mysql", input: "", want: EngineMySQL},
		{name: "mysql explicit", input: "mysql", want: EngineMySQL},
		{name: "postgres keyword", input: "postgres", want: EnginePostgres},
		{name: "postgres alias", input: " pg ", want: EnginePostgres},
		{name: "postgresql", input: "postgresql", want: EnginePostgres},
		{name: "unknown defaults mysql", input: "sqlite", want: EngineMySQL},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ParseEngine(tt.input))
		})
	}
}

func TestNewWithEngine_UsesPostgresQueries(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	queries := NewWithEngine(EnginePostgres, sqlDB)
	now := time.Now()

	mock.ExpectExec(regexp.QuoteMeta("values ($1, $2, $3, $4, $5)")).
		WithArgs("u1", "a", "r", now, "Bearer").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err = queries.CreateUserTokens(context.Background(), CreateUserTokensParams{
		UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: now, TokenType: "Bearer",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewWithEngine_UsesMySQLQueries(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	queries := NewWithEngine(EngineMySQL, sqlDB)
	now := time.Now()

	mock.ExpectExec(regexp.QuoteMeta("values (?, ?, ?, ?, ?)")).
		WithArgs("u1", "a", "r", now, "Bearer").
		WillReturnResult(sqlmock.NewResult(1, 1))

	_, err = queries.CreateUserTokens(context.Background(), CreateUserTokensParams{
		UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: now, TokenType: "Bearer",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestStoreConstructorsUseRequestedEngine(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	now := time.Now().UTC()

	mock.ExpectExec(regexp.QuoteMeta("values (?, ?, ?, ?, ?)")).
		WithArgs("u1", "a", "r", now, "Bearer").
		WillReturnResult(sqlmock.NewResult(1, 1))
	_, err = newUserTokenStore(EngineMySQL, sqlDB).CreateUserTokens(context.Background(), CreateUserTokensParams{
		UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: now, TokenType: "Bearer",
	})
	require.NoError(t, err)

	statsRows := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(int64(1), int64(1), int64(0), []byte("10.5"), int32(5), int32(15))
	mock.ExpectQuery(`(?is)count\(\*\).*from api_logs`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(statsRows)
	stats, err := newAPILogStore(EngineMySQL, sqlDB).GetAPILogStats(context.Background(), GetAPILogStatsParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
	require.NoError(t, err)
	require.Equal(t, 10.5, stats.AvgDurationMs)

	health := NewHealthStore(EngineMySQL, &statsPingDB{stats: sql.DBStats{OpenConnections: 3}})
	healthStats, err := health.Stats()
	require.NoError(t, err)
	require.Equal(t, 3, healthStats.OpenConnections)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNew_DefaultsToPostgresAPILogStore(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	now := time.Now().UTC()
	statsRows := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(int64(2), int64(1), int64(1), float64(12.5), int32(5), int32(20))
	mock.ExpectQuery(`(?is)count\(\*\).*from api_logs`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(statsRows)

	stats, err := New(sqlDB).GetAPILogStats(context.Background(), GetAPILogStatsParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
	require.NoError(t, err)
	require.Equal(t, 12.5, stats.AvgDurationMs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewQueriesAliasesPreserveBehavior(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	now := time.Now().UTC()
	statsRows := sqlmock.NewRows([]string{"total_requests", "successful_requests", "failed_requests", "avg_duration_ms", "min_duration_ms", "max_duration_ms"}).
		AddRow(int64(2), int64(1), int64(1), float64(15.5), int32(5), int32(20))
	mock.ExpectQuery(`(?is)count\(\*\).*from api_logs`).WithArgs(now.Add(-time.Hour), now).WillReturnRows(statsRows)

	stats, err := NewQueries(sqlDB).GetAPILogStats(context.Background(), GetAPILogStatsParams{RequestTime: now.Add(-time.Hour), RequestTime_2: now})
	require.NoError(t, err)
	require.Equal(t, 15.5, stats.AvgDurationMs)

	mock.ExpectExec(regexp.QuoteMeta("values (?, ?, ?, ?, ?)")).
		WithArgs("u1", "a", "r", now, "Bearer").
		WillReturnResult(sqlmock.NewResult(1, 1))
	_, err = NewQueriesWithEngine(EngineMySQL, sqlDB).CreateUserTokens(context.Background(), CreateUserTokensParams{
		UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: now, TokenType: "Bearer",
	})
	require.NoError(t, err)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewHealthStore_OptionalMethodErrors(t *testing.T) {
	health := NewHealthStore(EngineMySQL, &dbWithoutStatsPing{})

	err := health.PingContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not implement PingContext")

	_, err = health.Stats()
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not expose Stats")
}

func TestNewWithEngine_ExecErrorIsPropagated(t *testing.T) {
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	defer sqlDB.Close()

	errBoom := errors.New("boom")
	mock.ExpectExec(regexp.QuoteMeta("values (?, ?, ?, ?, ?)")).
		WithArgs("u1", "a", "r", sqlmock.AnyArg(), "Bearer").
		WillReturnError(errBoom)

	_, err = NewWithEngine(EngineMySQL, sqlDB).CreateUserTokens(context.Background(), CreateUserTokensParams{
		UserID: "u1", AccessToken: "a", RefreshToken: "r", Expiry: time.Now(), TokenType: "Bearer",
	})
	require.ErrorIs(t, err, errBoom)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestQueriesPingContextAndStats(t *testing.T) {
	db := &statsPingDB{stats: sql.DBStats{OpenConnections: 7}}
	queries := New(db)

	require.NoError(t, queries.PingContext(context.Background()))
	stats, err := queries.Stats()
	require.NoError(t, err)
	require.Equal(t, 7, stats.OpenConnections)

	db.pingErr = errors.New("ping failed")
	err = queries.PingContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "ping failed")
}

func TestQueriesPingContextAndStatsWithoutOptionalMethods(t *testing.T) {
	queries := New(&dbWithoutStatsPing{})

	err := queries.PingContext(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not implement PingContext")

	_, err = queries.Stats()
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not expose Stats")
}

func TestWithTxPreservesEngine(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	defer sqlDB.Close()

	queries := NewWithEngine(EngineMySQL, sqlDB)
	tx := &sql.Tx{}

	txDB := queries.WithTx(tx)
	require.NotNil(t, txDB)
	require.IsType(t, &Queries{}, txDB)
	require.Equal(t, EngineMySQL, txDB.engine)
	require.NotNil(t, txDB.apilog)
	require.NotNil(t, txDB.tokens)
	require.NotNil(t, txDB.health)
}

func TestNewDBWithEngine_Constructors(t *testing.T) {
	t.Run("mysql constructor returns db handle", func(t *testing.T) {
		dbConn, err := NewDBWithEngine(EngineMySQL, "user:pass@tcp(127.0.0.1:3306)/dbname")
		require.NoError(t, err)
		require.NotNil(t, dbConn)
		require.NoError(t, dbConn.Close())
	})

	t.Run("postgres constructor returns db handle", func(t *testing.T) {
		dbConn, err := NewDBWithEngine(EnginePostgres, "postgres://user:pass@127.0.0.1:5432/dbname?sslmode=disable")
		require.NoError(t, err)
		require.NotNil(t, dbConn)
		require.NoError(t, dbConn.Close())
	})

	t.Run("default constructor uses mysql contract", func(t *testing.T) {
		dbConn, err := NewDB("user:pass@tcp(127.0.0.1:3306)/dbname")
		require.NoError(t, err)
		require.NotNil(t, dbConn)
		require.NoError(t, dbConn.Close())
	})
}
