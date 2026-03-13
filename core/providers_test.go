package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	anthropicoption "github.com/anthropics/anthropic-sdk-go/option"
	openai "github.com/openai/openai-go/v3"
	openaioption "github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

func TestOpenAIProviderComplete(t *testing.T) {
	var captured struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string      `json:"role"`
			Content interface{} `json:"content"`
		} `json:"messages"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		last := captured.Messages[len(captured.Messages)-1]
		lastText, _ := last.Content.(string)

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
						"content": "handled: " + lastText,
						"refusal": "",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := openai.NewClient(
		openaioption.WithAPIKey("test-key"),
		openaioption.WithBaseURL(server.URL+"/"),
	)

	provider := OpenAIProvider{
		Client: &client,
		Model:  shared.ChatModelGPT4_1Mini,
	}

	resp, err := provider.Complete(context.Background(), ChatRequest{
		SystemPrompt: "You are helpful",
		Transcript: []Message{
			{Role: RoleAssistant, Content: "prior answer"},
		},
		Prompt: Message{Role: RoleUser, Content: "current question"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Content != "handled: current question" {
		t.Fatalf("unexpected reply content: %q", resp.Content)
	}

	if len(captured.Messages) != 3 {
		t.Fatalf("expected 3 messages sent (system + transcript + prompt), got %d", len(captured.Messages))
	}
	if captured.Messages[0].Role != "system" {
		t.Fatalf("expected system message first, got role %s", captured.Messages[0].Role)
	}
}

func TestAnthropicProviderComplete(t *testing.T) {
	var captured struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
		System []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"system"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}

		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		last := captured.Messages[len(captured.Messages)-1]
		lastText := ""
		if len(last.Content) > 0 {
			lastText = last.Content[0].Text
		}

		now := time.Now()
		resp := map[string]any{
			"id":            "msg_123",
			"type":          "message",
			"role":          "assistant",
			"model":         captured.Model,
			"stop_reason":   "end_turn",
			"stop_sequence": "",
			"container": map[string]any{
				"id":         "cont_123",
				"expires_at": now.Add(time.Hour).Format(time.RFC3339),
			},
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "handled: " + lastText,
				},
			},
			"usage": map[string]any{
				"cache_creation": map[string]any{
					"ephemeral_1h_input_tokens": 0,
					"ephemeral_5m_input_tokens": 0,
				},
				"cache_creation_input_tokens": 0,
				"cache_read_input_tokens":     0,
				"inference_geo":               "us",
				"input_tokens":                1,
				"output_tokens":               1,
				"server_tool_use": map[string]any{
					"web_fetch_requests":  0,
					"web_search_requests": 0,
				},
				"service_tier": "standard",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := anthropic.NewClient(
		anthropicoption.WithAPIKey("test-key"),
		anthropicoption.WithBaseURL(server.URL+"/"),
	)

	provider := AnthropicProvider{
		Client: &client,
		Model:  anthropic.ModelClaude3_7SonnetLatest,
	}

	resp, err := provider.Complete(context.Background(), ChatRequest{
		SystemPrompt: "You are helpful",
		Transcript: []Message{
			{Role: RoleAssistant, Content: "prior answer"},
		},
		Prompt: Message{Role: RoleUser, Content: "current question"},
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if resp.Content != "handled: current question" {
		t.Fatalf("unexpected reply content: %q", resp.Content)
	}

	if len(captured.Messages) != 2 {
		t.Fatalf("expected transcript + prompt messages sent, got %d", len(captured.Messages))
	}
	if len(captured.System) != 1 || captured.System[0].Text != "You are helpful" {
		t.Fatalf("expected system prompt forwarded, got %+v", captured.System)
	}
}
