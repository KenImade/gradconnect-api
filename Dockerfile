# ---- Build stage ----
FROM golang:1.25-bookworm AS builder
WORKDIR /src

# Copy the dependency manifests first, so this layer is cached
# and deps are only re-downloaded when go.mod/go.sum actually change.
COPY go.mod go.sum ./
RUN go mod download

# Now copy the source and compile a static binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/api

# ---- Runtime stage ----
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=builder /out/server /app/server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/server"]