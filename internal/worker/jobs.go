package worker

import (
	"context"
	"encoding/json"
	"fmt"
)

// JobHandler executes the actual work for a job. It's an interface so
// the worker package stays independent of cmd/api — main.go provides
// the concrete implementation that knows about models, mailer, etc.
type JobHandler interface {
	Handle(ctx context.Context, jobType string, payload []byte) error
}

// HandlerFunc adapts a plain function to the JobHandler interface,
// the same pattern net/http uses for handlers.
type HandlerFunc func(ctx context.Context, jobType string, payload []byte) error

func (f HandlerFunc) Handle(ctx context.Context, jobType string, payload []byte) error {
	return f(ctx, jobType, payload)
}

// Dispatcher routes jobs by type to type-specific handlers. Built in
// main.go and passed to the Pool. Centralizes the switch statement
// that used to live in processTask.
type Dispatcher struct {
	handlers map[string]HandlerFunc
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{handlers: make(map[string]HandlerFunc)}
}

// Register binds a job type to its handler function. Panics on duplicate
// registration — that's a programmer error worth catching loudly at startup.
func (d *Dispatcher) Register(jobType string, handler HandlerFunc) {
	if _, exists := d.handlers[jobType]; exists {
		panic(fmt.Sprintf("worker: job type %q already registered", jobType))
	}
	d.handlers[jobType] = handler
}

// Handle implements JobHandler.
func (d *Dispatcher) Handle(ctx context.Context, jobType string, payload []byte) error {
	handler, ok := d.handlers[jobType]
	if !ok {
		return fmt.Errorf("unknown job type: %s", jobType)
	}
	return handler(ctx, jobType, payload)
}

// UnmarshalPayload is a small helper for handlers that need to deserialize
// the JSON payload into a typed struct. Reduces boilerplate in main.go.
func UnmarshalPayload[T any](payload []byte) (T, error) {
	var data T
	if err := json.Unmarshal(payload, &data); err != nil {
		return data, fmt.Errorf("unmarshaling job payload: %w", err)
	}
	return data, nil
}
