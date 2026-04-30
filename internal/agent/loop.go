package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/krishyogee/gitmate/internal/approval"
	"github.com/krishyogee/gitmate/internal/memory"
)

var ErrUserDenied = errors.New("user denied action")
var ErrMaxStepsExceeded = errors.New("agent exceeded max steps without converging")

type AgentState struct {
	Task        string
	Steps       []Step
	Memory      *memory.Session
	Done        bool
	FinalOutput string
}

type Step struct {
	Thought     string
	Action      string
	ActionInput string
	Observation string
	Score       float64
	UserAction  string
	Time        time.Time
}

func (o *Orchestrator) Run(ctx context.Context, task string) (string, error) {
	state := &AgentState{
		Task:   task,
		Memory: o.SessionMemory,
	}
	if state.Memory == nil {
		state.Memory = memory.NewSession("")
	}

	for step := 0; step < o.MaxSteps; step++ {
		thought, action, input, err := o.Planner.Plan(ctx, task, state)
		if err != nil {
			return "", fmt.Errorf("planner: %w", err)
		}

		if action == "stop" {
			state.Done = true
			break
		}
		if action == "ask_user" {
			fmt.Println("\nAgent asks:", input)
			return input, nil
		}

		if !o.Executor.Has(action) {
			obs := fmt.Sprintf("Unknown action %q. Available: %v", action, o.Executor.Names())
			state.Steps = append(state.Steps, Step{Thought: thought, Action: action, ActionInput: input,
				Observation: obs, Score: 0, Time: time.Now()})
			state.Memory.Update(action, obs)
			continue
		}

		var result string
		var execErr error

		if o.Approval.IsRequired(action) {
			card := approval.Card{
				Action:      action,
				Input:       input,
				Description: fmt.Sprintf("step %d of agent plan: %s", step+1, thought),
			}
			dec, edited, derr := o.Approval.Request(card)
			if derr != nil {
				return "", fmt.Errorf("approval: %w", derr)
			}
			switch dec {
			case approval.DecisionNo:
				return "", ErrUserDenied
			case approval.DecisionEdit:
				input = edited
			}
			result, execErr = o.Executor.Execute(ctx, action, input)
		} else {
			result, execErr = o.Executor.Execute(ctx, action, input)
		}

		if execErr != nil {
			result = fmt.Sprintf("ERROR: %v\n%s", execErr, result)
		}

		score := o.Evaluator.Score(action, result)
		stepRec := Step{Thought: thought, Action: action, ActionInput: input,
			Observation: result, Score: score, Time: time.Now()}
		state.Steps = append(state.Steps, stepRec)
		state.Memory.Update(action, result)
		state.Memory.UpdateScore(score)

		o.Logger.LogStep(task, action, result, score)

		if score >= o.Evaluator.PassThreshold() {
			state.Done = true
			state.FinalOutput = result
			break
		}
		if score < o.Evaluator.FailThreshold() && step > 0 {
			o.AI.RotateModel()
		}
	}

	if o.LongTerm != nil && state.Memory != nil && state.Memory.RepoRoot != "" {
		o.LongTerm.PersistSession(state.Memory.RepoRoot, state.Memory)
	}

	if !state.Done {
		return state.FinalOutput, ErrMaxStepsExceeded
	}
	return state.FinalOutput, nil
}
