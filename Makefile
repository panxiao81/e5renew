dev:
	go run main.go --config config.dev.yaml

sqlc:
	sqlc generate

test:
	go test -v ./...

test-coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-race:
	go test -race -v ./...

bench:
	go test -bench=. -v ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

lint:
	golangci-lint run

lint-fix:
	golangci-lint run --fix

clean:
	rm -f coverage.out coverage.html bin/e5renew

build:
	go build -o bin/e5renew main.go

migrate-up:
	go run main.go migrate up

migrate-down:
	go run main.go migrate down

migrate-status:
	go run main.go migrate status

migrate-version:
	go run main.go migrate version

migrate-force:
	@read -p "Enter version to force: " version; \
	go run main.go migrate force $$version

.PHONY: dev sqlc test test-coverage test-race bench fmt vet lint lint-fix clean build migrate-up migrate-down migrate-status migrate-version migrate-force
