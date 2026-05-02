include .envrc

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
  -port=4000 \
  -db-dsn="postgres://gradconnect:gradconnect_dev_pw@localhost:5432/gradconnect?sslmode=disable" \
  -cors-trusted-origins=http://localhost:3000 \
  -frontend-url=http://localhost:3000 \
  -base-url=http://localhost:4000 \
  -smtp-sender='GradConnect <no-reply@gradconnect.ng>'

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
	swag init -g cmd/api/main.go -d cmd/api,internal/app -o cmd/api/docs --parseDependency --parseInternal

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

## test: run all tests
.PHONY: test
test:
 	go test -race -vet=off ./...