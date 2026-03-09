# Postgres 兼容性测试依赖与运行方式

这些测试用于 TDD 第一阶段（先红灯）：

- `internal/db/conn_postgres_test.go`
  - 验证数据库连接工厂可连接 Postgres（Docker + 非默认端口）
- `cmd/postgres_support_test.go`
  - 验证 `run` 路径存在 Postgres session store 选择逻辑
  - 验证 `migrate` 路径支持 Postgres driver 判断与实例化

## 依赖

- Docker
- Go 1.24+
- 本地可用端口 `15432`（避免占用系统默认 Postgres 端口 `5432`）

## 启动测试用 Postgres（建议）

```bash
docker rm -f e5renew-pg-test 2>/dev/null || true
docker run -d \
  --name e5renew-pg-test \
  -e POSTGRES_USER=e5renew \
  -e POSTGRES_PASSWORD=e5renew \
  -e POSTGRES_DB=e5renew_test \
  -p 15432:5432 \
  postgres:16
```

可选：等待容器就绪

```bash
docker logs -f e5renew-pg-test
```

## 运行测试

在项目根目录 `e5renew/`：

```bash
E5RENEW_TEST_POSTGRES_DSN='postgres://e5renew:e5renew@127.0.0.1:15432/e5renew_test?sslmode=disable' \
  go test ./internal/db ./cmd
```

## 说明

- 当前阶段是 TDD 的「测试先行」，这些测试在功能尚未实现前应出现红灯。
- 后续开发子任务需要让以下点变绿：
  - `db.NewDB` 能根据引擎选择 Postgres driver（而非固定 MySQL）
  - `cmd/run.go` 按 `database.engine` 选择 `mysqlstore`/`postgresstore`
  - `cmd/migrate.go` 按 `database.engine` 选择 `mysql`/`postgres` 迁移驱动
