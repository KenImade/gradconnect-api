ARG GO_VERSION=1
FROM golang:${GO_VERSION}-alpine AS builder

WORKDIR /build

# Cache dependencies separately from source for fast incremental builds
COPY go.mod go.sum ./
RUN go mod download

# Install swag CLI inside the builder stage. Pinned to a specific version
# for reproducibility — bump when you upgrade locally.
RUN go install github.com/swaggo/swag/cmd/swag@v1.16.4


# Build the API binary
COPY . .

# Generate Swagger spec. --parseDependency is required because some of the
# response shapes reference types in internal/data.
RUN /go/bin/swag init -g cmd/api/main.go -o cmd/api/docs --parseDependency

RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-s -w' \
    -o /build/api \
    ./cmd/api

# Build the migrate CLI binary too — used by the Fly release_command
RUN go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@v4.17.0

# Runtime image — minimal, just enough to run a Go binary
FROM alpine:3.20

# ca-certificates: needed for HTTPS calls (Resend, Google OAuth, R2)
# tzdata: needed for time.LoadLocation("Africa/Lagos") in the cron worker
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /build/api /app/api
COPY --from=builder /go/bin/migrate /app/migrate
COPY --from=builder /build/migrations /app/migrations

EXPOSE 8080

CMD ["/app/api"]
