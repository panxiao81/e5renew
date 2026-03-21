package repository

import (
	"context"
	"database/sql"

	"github.com/panxiao81/e5renew/internal/db"
	mydb "github.com/panxiao81/e5renew/internal/db/mysql"
	pgdb "github.com/panxiao81/e5renew/internal/db/postgres"
)

type userTokenStore interface {
	CreateUserTokens(context.Context, db.CreateUserTokensParams) (sql.Result, error)
	DeleteUserTokens(context.Context, string) error
	GetUserToken(context.Context, string) (db.UserToken, error)
	ListUserTokens(context.Context) ([]db.UserToken, error)
	UpdateUserTokens(context.Context, db.UpdateUserTokensParams) (sql.Result, error)
}

func newUserTokenStore(engine db.Engine, conn db.DBTX) userTokenStore {
	if engine == db.EngineMySQL {
		return &mysqlUserTokenStore{q: mydb.New(conn)}
	}
	return &postgresUserTokenStore{q: pgdb.New(conn)}
}

type postgresUserTokenStore struct{ q *pgdb.Queries }

func (p *postgresUserTokenStore) CreateUserTokens(ctx context.Context, arg db.CreateUserTokensParams) (sql.Result, error) {
	return p.q.CreateUserTokens(ctx, pgdb.CreateUserTokensParams(arg))
}
func (p *postgresUserTokenStore) DeleteUserTokens(ctx context.Context, userID string) error {
	return p.q.DeleteUserTokens(ctx, userID)
}
func (p *postgresUserTokenStore) GetUserToken(ctx context.Context, userID string) (db.UserToken, error) {
	token, err := p.q.GetUserToken(ctx, userID)
	return db.UserToken(token), err
}
func (p *postgresUserTokenStore) ListUserTokens(ctx context.Context) ([]db.UserToken, error) {
	tokens, err := p.q.ListUserTokens(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]db.UserToken, len(tokens))
	for i := range tokens {
		result[i] = db.UserToken(tokens[i])
	}
	return result, nil
}
func (p *postgresUserTokenStore) UpdateUserTokens(ctx context.Context, arg db.UpdateUserTokensParams) (sql.Result, error) {
	return p.q.UpdateUserTokens(ctx, pgdb.UpdateUserTokensParams(arg))
}

type mysqlUserTokenStore struct{ q *mydb.Queries }

func (m *mysqlUserTokenStore) CreateUserTokens(ctx context.Context, arg db.CreateUserTokensParams) (sql.Result, error) {
	return m.q.CreateUserTokens(ctx, mydb.CreateUserTokensParams(arg))
}
func (m *mysqlUserTokenStore) DeleteUserTokens(ctx context.Context, userID string) error {
	return m.q.DeleteUserTokens(ctx, userID)
}
func (m *mysqlUserTokenStore) GetUserToken(ctx context.Context, userID string) (db.UserToken, error) {
	token, err := m.q.GetUserToken(ctx, userID)
	return db.UserToken(token), err
}
func (m *mysqlUserTokenStore) ListUserTokens(ctx context.Context) ([]db.UserToken, error) {
	tokens, err := m.q.ListUserTokens(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]db.UserToken, len(tokens))
	for i := range tokens {
		result[i] = db.UserToken(tokens[i])
	}
	return result, nil
}
func (m *mysqlUserTokenStore) UpdateUserTokens(ctx context.Context, arg db.UpdateUserTokensParams) (sql.Result, error) {
	return m.q.UpdateUserTokens(ctx, mydb.UpdateUserTokensParams(arg))
}
