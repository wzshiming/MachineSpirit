package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/wzshiming/MachineSpirit/pkg/scheduler"
)

// CronTool lets the agent manage cron-scheduled jobs.
type CronTool struct {
	scheduler *scheduler.Scheduler
}

// NewCronTool creates a new Cron tool.
func NewCronTool(sched *scheduler.Scheduler) *CronTool {
	return &CronTool{scheduler: sched}
}

func (t *CronTool) Name() string {
	return "cron"
}

func (t *CronTool) Description() string {
	return `Manage cron-scheduled tasks. Actions: ` +
		`{"action": "add", "name": "status-check", "schedule": "0 */5 * * * *", "message": "run status check"} - Add a cron job (6-field with seconds). ` +
		`{"action": "remove", "id": "status-check"} - Remove a cron job by ID. ` +
		`{"action": "list"} - List all active cron jobs.`
}

func (t *CronTool) Enabled() bool {
	return t.scheduler != nil
}

func (t *CronTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Action   string `json:"action"`
		Name     string `json:"name"`
		Schedule string `json:"schedule"`
		Message  string `json:"message"`
		ID       string `json:"id"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	switch params.Action {
	case "add":
		if params.Name == "" {
			return nil, fmt.Errorf("name is required for add action")
		}
		if params.Schedule == "" {
			return nil, fmt.Errorf("schedule is required for add action")
		}
		if params.Message == "" {
			return nil, fmt.Errorf("message is required for add action")
		}
		id, err := t.scheduler.AddCron(params.Name, params.Schedule, params.Message)
		if err != nil {
			return nil, fmt.Errorf("failed to add cron job: %w", err)
		}
		return marshalResult(map[string]any{
			"status":   "success",
			"id":       id,
			"schedule": params.Schedule,
			"message":  params.Message,
		})

	case "remove":
		if params.ID == "" {
			return nil, fmt.Errorf("id is required for remove action")
		}
		if err := t.scheduler.Remove(params.ID); err != nil {
			return nil, fmt.Errorf("failed to remove cron job: %w", err)
		}
		return marshalResult(map[string]any{
			"status": "success",
			"id":     params.ID,
		})

	case "list":
		jobs := t.scheduler.List()
		return marshalResult(map[string]any{
			"status": "success",
			"jobs":   jobs,
			"count":  len(jobs),
		})

	default:
		return nil, fmt.Errorf("unknown action %q, expected add, remove, or list", params.Action)
	}
}

func marshalResult(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return json.RawMessage(data), nil
}
