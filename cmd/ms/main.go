package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/c-bata/go-prompt"
	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/agent/tools"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/session"
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
	llm, err := llm.NewLLM(
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
	session := session.NewSession(llm,
		session.WithSystemPrompt("You are a helpful coding assistant with access to shell commands and file operations."),
	)

	skillList := []agent.Skill{}

	for _, skillsDir := range []string{os.Getenv("HOME") + "/.agents/skills", ".agents/skills"} {
		loader := skills.NewSkillLoader(skillsDir)
		skillsList, err := loader.LoadAllSkills()
		if err != nil {
			slog.Warn("Failed to load skills from directory", "dir", skillsDir, "error", err)
			os.Exit(1)
		}
		for _, skill := range skillsList {
			skillList = append(skillList, skill)
		}
	}

	ag, err := agent.NewAgent(
		session,
		agent.WithTools(tools.NewBashTool()),
		agent.WithSkills(skillList...),
		agent.WithMaxRetries(20),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	p := prompt.New(
		func(text string) {
			text = strings.TrimSpace(text)
			if text == "" {
				return
			}
			if strings.HasPrefix(text, "/") {
				if strings.HasPrefix(text, "/help") {
					fmt.Println("Enter your message to interact with the agent.")
					fmt.Println("Commands:")
					fmt.Println("  /help     - Show this help message")
					fmt.Println("  /reset    - Clear the session")
					fmt.Println("  /bye      - Exit the program")
					return
				} else if strings.HasPrefix(text, "/reset") {
					session.Reset()
					fmt.Println("Session cleared.")
					return
				} else if strings.HasPrefix(text, "/bye") {
					fmt.Println("Goodbye!")
					os.Exit(0)
				} else {
					fmt.Println("Unknown command. Type /help for a list of commands.")
					return
				}
			}

			response, err := ag.Execute(ctx, text)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				return
			}

			fmt.Println(response)
		},
		func(in prompt.Document) []prompt.Suggest {
			if in.Text == "" || !strings.HasPrefix(in.Text, "/") {
				return nil
			}
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
