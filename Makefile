include .envrc.local

## help: print this help message
.PHONY: help
help:
	@echo 'Usage:'
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

.PHONY: confirm
confirm:
	@echo -n 'Are you sure? [y/N] ' && read ans && [ $${ans:-N} = y ]

## run/api: run the cmd/api application
.PHONY: run/api
run/api:
	go run ./cmd/api \
	  -port=${GRADCONNECT_PORT} \
	  -db-dsn="${GRADCONNECT_DB_DSN}" \
	  -cors-trusted-origins="${GRADCONNECT_CORS_TRUSTED_ORIGINS}"

## build/api: build the cmd/api application
.PHONY: build/api
build/api:
	@echo 'Building cmd/api...'
	go build -ldflags='-s' -o=./bin/api ./cmd/api

## db/psql: connect to the database using psql
.PHONY: db/psql
db/psql:
	docker exec -it gradconnect-db psql -U gradconnect -d gradconnect

## db/migrations/new name=$1: create a new database migration
.PHONY: db/migrations/new
db/migrations/new:
	@echo 'Creating migration files for ${name}...'
	migrate create -seq -ext=.sql -dir=./migrations ${name}

## db/migrations/up: apply all up database migrations
.PHONY: db/migrations/up
db/migrations/up: confirm
	@echo 'Running up migrations...'
	migrate -path ./migrations -database ${GRADCONNECT_DB_DSN} up

## db/migrations/down: apply all down database migrations
.PHONY: db/migrations/down
db/migrations/down: confirm
	@echo 'Running down migrations...'
	migrate -path ./migrations -database ${GRADCONNECT_DB_DSN} down

## db/seed/all: seed the database with all test data
.PHONY: db/seed/all
db/seed/all:
	@echo 'Seeding database...'
	docker exec -i gradconnect-db psql -U gradconnect -d gradconnect < ./migrations/seed/seed.sql

## db/seed/reviews: seed the database with reviews test data
.PHONY: db/seed/reviews
db/seed/reviews:
	@echo 'Seeding database...'
	docker exec -i gradconnect-db psql -U gradconnect -d gradconnect < ./migrations/seed/reviews_seed.sql

## docs/generate: generate Swagger/OpenAPI documentation
.PHONY: docs/generate
docs/generate:
	@echo 'Generating API documentation...'
	swag init -g main.go -d cmd/api,internal/app -o cmd/api/docs --parseDependency --parseInternal

## audit: tidy dependencies, vet, staticcheck, and test
.PHONY: audit
audit:
	@echo 'Tidying and verifying module dependencies...'
	go mod tidy
	go mod verify
	@echo 'Vetting code...'
	go vet ./...
	staticcheck ./...
	@echo 'Running tests...'
	go test -race -vet=off ./...

## test: run unit tests (no database required)
.PHONY: test
test:
	go test -race -vet=off $(shell go list ./... | grep -v test/integration)

## test/integration: run integration tests against the test database
.PHONY: test/integration
test/integration:
	go test -v -race ./test/integration/ -timeout 60s