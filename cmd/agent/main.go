package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/wzshiming/MachineSpirit/pkg/agent"
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
		SystemPrompt: "You are a helpful travel assistant agent with access to flight booking tools and skills.",
	})

	// Create tools
	searchTool := agent.NewFlightSearchTool()
	reservationTool := agent.NewFlightReservationTool()

	// Create skills registry and register high-level skills
	skills := agent.NewSkillRegistry()
	flightBookingSkill := agent.NewFlightBookingSkill(searchTool, reservationTool)
	skills.Register(flightBookingSkill)

	// Create agent with both tools and skills
	ag, err := agent.NewAgent(agent.Config{
		Session: session,
		Tools: []agent.Tool{
			searchTool,
			reservationTool,
		},
		Skills: skills,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Store some example preferences in memory
	ag.Memory().Store("preferred_airline", "Delta Airlines")
	ag.Memory().Store("user_name", "John Doe")

	fmt.Println("Agent mode enabled with Skills support.")
	fmt.Println("Available Skills: flight_booking (end-to-end booking)")
	fmt.Println("Available Tools: flight_search, flight_reservation")
	fmt.Println("Example preferences stored: preferred_airline=Delta Airlines, user_name=John Doe")
	fmt.Println()

	p := prompt.New(
		func(text string) {
			text = strings.TrimSpace(text)
			if strings.HasPrefix(text, "/help") {
				fmt.Println("Enter your message to interact with the agent.")
				fmt.Println("Commands:")
				fmt.Println("  /help     - Show this help message")
				fmt.Println("  /reset    - Clear the session")
				fmt.Println("  /memory   - Show current memory facts")
				fmt.Println("  /store    - Store a fact (usage: /store key=value)")
				fmt.Println("  /bye      - Exit the program")
				return
			}
			if strings.HasPrefix(text, "/reset") {
				session.Reset()
				fmt.Println("Session cleared.")
				return
			}
			if strings.HasPrefix(text, "/memory") {
				facts := ag.Memory().All()
				if len(facts) == 0 {
					fmt.Println("No facts stored in memory.")
				} else {
					fmt.Println("Memory contents:")
					for _, fact := range facts {
						fmt.Printf("  %s: %s\n", fact.Key, fact.Value)
					}
				}
				return
			}
			if strings.HasPrefix(text, "/store ") {
				parts := strings.SplitN(strings.TrimPrefix(text, "/store "), "=", 2)
				if len(parts) != 2 {
					fmt.Println("Usage: /store key=value")
					return
				}
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				ag.Memory().Store(key, value)
				fmt.Printf("Stored: %s = %s\n", key, value)
				return
			}
			if strings.HasPrefix(text, "/bye") {
				fmt.Println("Goodbye!")
				os.Exit(0)
			}

			response, err := ag.Execute(ctx, text)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return
			}

			fmt.Println(response)
		},
		func(in prompt.Document) []prompt.Suggest {
			s := []prompt.Suggest{
				{Text: "/help", Description: "Show the help message"},
				{Text: "/reset", Description: "Clear the current session"},
				{Text: "/memory", Description: "Show memory facts"},
				{Text: "/store ", Description: "Store a fact (key=value)"},
				{Text: "/bye", Description: "Exit the program"},
			}
			return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
		},
		prompt.OptionPrefix("> "),
	)
	p.Run()
}
