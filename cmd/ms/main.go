package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
)

var (
	Name    string
	Model   string
	APIKey  string
	BaseURL string
)

func init() {
	flag.StringVar(&Name, "provider", "openai", "LLM provider: openai or anthropic")
	flag.StringVar(&Model, "model", "", "Model name (optional, provider default used if empty)")
	flag.StringVar(&APIKey, "api-key", "", "API key for the provider (env fallback OPENAI_API_KEY or ANTHROPIC_API_KEY)")
	flag.StringVar(&BaseURL, "base-url", "", "Optional base URL for the provider API")
	flag.Parse()
}

func main() {

	l, err := llm.NewLLM(
		llm.WithProvider(Name),
		llm.WithModel(Model),
		llm.WithAPIKey(APIKey),
		llm.WithBaseURL(BaseURL),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	session := llm.NewSession(l, llm.SessionConfig{
		SystemPrompt: "You are helpful",
	})

	p := prompt.New(
		func(text string) {
			text = strings.TrimSpace(text)
			if strings.HasPrefix(text, "/help") {
				fmt.Println("Enter your message to chat with the LLM. Use /reset to start a new session, /bye to exit.")
				return
			}
			if strings.HasPrefix(text, "/reset") {
				session.Reset()
				fmt.Println("Session cleared.")
				return
			}
			if strings.HasPrefix(text, "/bye") {
				fmt.Println("Goodbye!")
				os.Exit(0)
			}
			env, err := session.Complete(ctx, llm.Message{Role: llm.RoleUser, Content: text})
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}

			fmt.Println(env.Content)
		},
		func(in prompt.Document) []prompt.Suggest {
			s := []prompt.Suggest{
				{Text: "/help", Description: "Show the help message"},
				{Text: "/reset", Description: "Clear the current session"},
				{Text: "/bye", Description: "Exit the program"},
			}
			return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
		},
		prompt.OptionPrefix("> "),
	)
	p.Run()
}
