package e2e

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCmdMSWithOpenAIProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req struct {
			Messages []struct {
				Role    string      `json:"role"`
				Content interface{} `json:"content"`
			} `json:"messages"`
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		last := req.Messages[len(req.Messages)-1]
		text := ""
		if s, ok := last.Content.(string); ok {
			text = s
		}
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"created": time.Now().Unix(),
			"model":   req.Model,
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
						"content": "handled: " + text,
						"refusal": "",
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cmd := exec.Command("go", "run", "./cmd/ms", "--provider=openai", "--api-key=test", "--base-url", server.URL+"/")
	cmd.Dir = repoRoot(t)
	cmd.Stdin = strings.NewReader("hello\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("cmd failed: %v, stderr=%s", err, stderr.String())
	}

	out := strings.TrimSpace(stdout.String())
	if out != "handled: hello" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestCmdMSWithAnthropicProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}

		var req struct {
			Messages []struct {
				Role    string `json:"role"`
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"content"`
			} `json:"messages"`
			Model string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}

		last := req.Messages[len(req.Messages)-1]
		text := ""
		if len(last.Content) > 0 {
			text = last.Content[0].Text
		}

		now := time.Now()
		resp := map[string]any{
			"id":            "msg_123",
			"type":          "message",
			"role":          "assistant",
			"model":         req.Model,
			"stop_reason":   "end_turn",
			"stop_sequence": "",
			"container": map[string]any{
				"id":         "cont_123",
				"expires_at": now.Add(time.Hour).Format(time.RFC3339),
			},
			"content": []any{
				map[string]any{
					"type": "text",
					"text": "handled: " + text,
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

	cmd := exec.Command("go", "run", "./cmd/ms", "--provider=anthropic", "--api-key=test", "--base-url", server.URL+"/")
	cmd.Dir = repoRoot(t)
	cmd.Stdin = strings.NewReader("hello\n")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("cmd failed: %v, stderr=%s", err, stderr.String())
	}

	out := strings.TrimSpace(stdout.String())
	if out != "handled: hello" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}
