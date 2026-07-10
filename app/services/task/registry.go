package task

import (
	"context"
	"encoding/json"
	"fmt"
)

// Handler runs one action. It must be idempotent: delivery is at-least-once, so
// a handler may see the same task twice.
type Handler func(ctx context.Context, data json.RawMessage, progress func(pct int32, msg string)) error

var registry = map[string]Handler{}

func Register(action string, h Handler) {
	registry[action] = h
}

func lookup(action string) (Handler, error) {
	h, ok := registry[action]
	if !ok {
		return nil, fmt.Errorf("unknown task action: %s", action)
	}
	return h, nil
}
