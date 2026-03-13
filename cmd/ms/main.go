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
	var flightContext struct {
		lastQuery string
		lastPick  string
	}

	agent := llm.NewAgent(session, llm.AgentConfig{
		MaxSteps: 8,
		Tools: []llm.Tool{
			{
				Name:        "search_flights",
				Short:       "Search flights",
				Description: "Search flights for a given route and date range or specific day.",
				Parameters: map[string]string{
					"route": "origin to destination (e.g., New York to London)",
					"date":  "date or range (e.g., tomorrow)",
				},
				Returns: map[string]string{
					"options": "list of options with time and price",
				},
				Fn: func(ctx context.Context, input map[string]string) (map[string]string, error) {
					route := strings.TrimSpace(input["route"])
					date := strings.TrimSpace(input["date"])
					query := strings.TrimSpace(strings.Join([]string{route, date}, " "))
					flightContext.lastQuery = query
					results := fmt.Sprintf("Options for %s: 08:00 $500; 12:00 $450; 18:00 $520", query)
					return map[string]string{"options": results}, nil
				},
			},
			{
				Name:        "reserve_flight",
				Short:       "Reserve flight",
				Description: "Reserve the selected flight option.",
				Parameters: map[string]string{
					"selection": "preferred option or reference",
				},
				Returns: map[string]string{
					"status": "reservation confirmation message",
				},
				Fn: func(ctx context.Context, input map[string]string) (map[string]string, error) {
					choice := strings.TrimSpace(input["selection"])
					if choice == "" {
						choice = flightContext.lastQuery
					}
					if choice == "" {
						return map[string]string{"status": "No prior search. Please provide route/date to reserve."}, nil
					}
					flightContext.lastPick = choice
					return map[string]string{"status": fmt.Sprintf("Reservation placed for: %s", choice)}, nil
				},
			},
		},
	})

	p := prompt.New(
		func(text string) {
			text = strings.TrimSpace(text)
			if strings.HasPrefix(text, "/help") {
				fmt.Println("Chat with the agent. Use /reset to start a new session, /bye to exit.")
				return
			}
			if strings.HasPrefix(text, "/reset") {
				session.Reset()
				flightContext = struct {
					lastQuery string
					lastPick  string
				}{}
				fmt.Println("Session cleared.")
				return
			}
			if strings.HasPrefix(text, "/bye") {
				fmt.Println("Goodbye!")
				os.Exit(0)
			}
			env, err := agent.Run(ctx, text)
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
