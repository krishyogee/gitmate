package agent

import (
	"github.com/krishyogee/gitmate/internal/ai"
	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/memory"
	"github.com/krishyogee/gitmate/internal/observability"
	"github.com/krishyogee/gitmate/internal/tools"
)

type Orchestrator struct {
	Planner       Planner
	Executor      *Executor
	Evaluator     Evaluator
	Approval      *approval.Manager
	Logger        *observability.Logger
	AI            *ai.Client
	LongTerm      *memory.Store
	SessionMemory *memory.Session
	MaxSteps      int
}

type Option func(*Orchestrator)

func New(opts ...Option) *Orchestrator {
	o := &Orchestrator{MaxSteps: 6}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

func WithPlanner(p Planner) Option           { return func(o *Orchestrator) { o.Planner = p } }
func WithExecutor(e *Executor) Option         { return func(o *Orchestrator) { o.Executor = e } }
func WithEvaluator(e Evaluator) Option         { return func(o *Orchestrator) { o.Evaluator = e } }
func WithApproval(a *approval.Manager) Option  { return func(o *Orchestrator) { o.Approval = a } }
func WithLogger(l *observability.Logger) Option { return func(o *Orchestrator) { o.Logger = l } }
func WithAI(c *ai.Client) Option                { return func(o *Orchestrator) { o.AI = c } }
func WithLongTerm(s *memory.Store) Option       { return func(o *Orchestrator) { o.LongTerm = s } }
func WithSession(s *memory.Session) Option      { return func(o *Orchestrator) { o.SessionMemory = s } }
func WithMaxSteps(n int) Option                 { return func(o *Orchestrator) { o.MaxSteps = n } }
func WithTools(reg *tools.Registry) Option {
	return func(o *Orchestrator) { o.Executor = NewExecutor(reg) }
}
