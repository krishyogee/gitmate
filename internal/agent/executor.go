package agent

import (
	"context"
	"fmt"

	"github.com/krishyogee/gitmate/internal/tools"
)

type Executor struct {
	registry *tools.Registry
}

func NewExecutor(reg *tools.Registry) *Executor {
	return &Executor{registry: reg}
}

func (e *Executor) RegisterTool(t tools.Tool) { e.registry.Register(t) }

func (e *Executor) Execute(ctx context.Context, action, input string) (string, error) {
	t, ok := e.registry.Get(action)
	if !ok {
		return "", fmt.Errorf("unknown action: %s", action)
	}
	return t.Execute(ctx, input)
}

func (e *Executor) Has(action string) bool {
	_, ok := e.registry.Get(action)
	return ok
}

func (e *Executor) Names() []string { return e.registry.Names() }
