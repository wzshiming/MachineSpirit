package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

func TestOpenAIAgentSendsTranscriptAndReturnsReply(t *testing.T) {
	var captured struct {
		Model    string
		Messages []struct {
			Role    string      `json:"role"`
			Content interface{} `json:"content"`
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		last := captured.Messages[len(captured.Messages)-1]
		content := ""
		switch v := last.Content.(type) {
		case string:
			content = v
		default:
			content = "<unknown>"
		}

		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   captured.Model,
			"choices": []any{
				map[string]any{
					"index":         0,
					"finish_reason": "stop",
					"logprobs": map[string]any{
						"content": []any{},
						"refusal": []any{},
					},
					"message": map[string]any{
						"role":    "assistant",
						"content": "handled: " + content,
						"refusal": "",
					},
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 1,
				"total_tokens":      2,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL+"/v1"),
	)

	agent := OpenAIAgent{
		Client:       &client,
		Model:        shared.ChatModelGPT4_1Mini,
		SystemPrompt: "You are helpful",
	}

	reply, err := agent.Respond(context.Background(), AgentInput{
		Event: Event{
			Content: "current question",
		},
		Transcript: []Message{
			{Role: RoleAssistant, Content: "prior answer"},
		},
	})
	if err != nil {
		t.Fatalf("Respond returned error: %v", err)
	}

	if reply.Role != RoleAssistant {
		t.Fatalf("expected assistant role, got %s", reply.Role)
	}
	if reply.Content != "handled: current question" {
		t.Fatalf("unexpected reply content: %q", reply.Content)
	}

	if len(captured.Messages) != 3 {
		t.Fatalf("expected 3 messages sent (system + transcript + event), got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != "system" {
		t.Fatalf("expected system message first, got role %s", captured.Messages[0].Role)
	}
	if captured.Model != string(shared.ChatModelGPT4_1Mini) {
		t.Fatalf("unexpected model used: %s", captured.Model)
	}
}
