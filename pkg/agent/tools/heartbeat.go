package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/scheduler"
)

// HeartbeatTool lets the agent manage periodic heartbeat jobs.
type HeartbeatTool struct {
	scheduler *scheduler.Scheduler
}

// NewHeartbeatTool creates a new Heartbeat tool.
func NewHeartbeatTool(sched *scheduler.Scheduler) *HeartbeatTool {
	return &HeartbeatTool{scheduler: sched}
}

func (t *HeartbeatTool) Name() string {
	return "heartbeat"
}

func (t *HeartbeatTool) Description() string {
	return `Manage periodic heartbeat tasks. Actions: ` +
		`{"action": "start", "interval": "30s", "message": "check system status"} - Start a heartbeat that fires every interval. ` +
		`{"action": "stop", "id": "heartbeat-1"} - Stop a heartbeat by ID. ` +
		`{"action": "list"} - List all active heartbeats.`
}

func (t *HeartbeatTool) Enabled() bool {
	return t.scheduler != nil
}

func (t *HeartbeatTool) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	var params struct {
		Action   string `json:"action"`
		Interval string `json:"interval"`
		Message  string `json:"message"`
		ID       string `json:"id"`
	}

	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("invalid input: %w", err)
	}

	switch params.Action {
	case "start":
		if params.Interval == "" {
			return nil, fmt.Errorf("interval is required for start action")
		}
		dur, err := time.ParseDuration(params.Interval)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %q: %w", params.Interval, err)
		}
		if params.Message == "" {
			return nil, fmt.Errorf("message is required for start action")
		}
		id, err := t.scheduler.AddHeartbeat(dur, params.Message)
		if err != nil {
			return nil, fmt.Errorf("failed to start heartbeat: %w", err)
		}
		return marshalResult(map[string]any{
			"status":   "success",
			"id":       id,
			"interval": params.Interval,
			"message":  params.Message,
		})

	case "stop":
		if params.ID == "" {
			return nil, fmt.Errorf("id is required for stop action")
		}
		if err := t.scheduler.Remove(params.ID); err != nil {
			return nil, fmt.Errorf("failed to stop heartbeat: %w", err)
		}
		return marshalResult(map[string]any{
			"status": "success",
			"id":     params.ID,
		})

	case "list":
		jobs := t.scheduler.List()
		heartbeats := make([]scheduler.Job, 0)
		for _, j := range jobs {
			if j.Type == scheduler.JobTypeHeartbeat {
				heartbeats = append(heartbeats, j)
			}
		}
		return marshalResult(map[string]any{
			"status":     "success",
			"heartbeats": heartbeats,
			"count":      len(heartbeats),
		})

	default:
		return nil, fmt.Errorf("unknown action %q, expected start, stop, or list", params.Action)
	}
}

func marshalResult(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return json.RawMessage(data), nil
}
