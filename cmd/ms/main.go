package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/Xuanwo/go-locale"
	"github.com/c-bata/go-prompt"
	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/agent/tools"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/persistence/i18n"
	"github.com/wzshiming/MachineSpirit/pkg/session"
)

var (
	Name         string
	Model        string
	APIKey       string
	BaseURL      string
	WorkspaceDir string
	Locale       string
)

func init() {
	dir, _ := os.Getwd()
	WorkspaceDir = dir

	flag.StringVar(&Name, "provider", "openai", "LLM provider: openai, anthropic, or ollama")
	flag.StringVar(&Model, "model", "", "Model name (optional, provider default used if empty)")
	flag.StringVar(&APIKey, "api-key", "", "API key for the provider (env fallback OPENAI_API_KEY or ANTHROPIC_API_KEY; not needed for ollama)")
	flag.StringVar(&BaseURL, "base-url", "", "Optional base URL for the provider API (default http://localhost:11434 for ollama)")
	flag.StringVar(&WorkspaceDir, "workspace", WorkspaceDir, "Path to workspace directory (optional)")
	flag.StringVar(&Locale, "locale", "", "Language/locale for internationalized prompts ('en' or 'zh'). Auto-detected from USER.md if not specified.")
	flag.Parse()
}

func main() {
	pm, err := persistence.NewPersistenceManager(WorkspaceDir)
	if err != nil {
		slog.Error("Failed to initialize persistence manager", "error", err)
		os.Exit(1)
	}

	// Initialize workspace with templates if needed
	var detectedLocale string
	if Locale == "" {
		// Auto-detect system locale
		tag, err := locale.Detect()
		if err != nil {
			slog.Warn("Failed to detect system locale, defaulting to English", "error", err)
			detectedLocale = "en"
		} else {
			// Get the language tag base (e.g., "zh" from "zh-CN")
			lang := tag.String()
			// Map to supported locales: zh for Chinese, en for everything else
			if strings.HasPrefix(lang, "zh") {
				detectedLocale = "zh"
			} else {
				detectedLocale = "en"
			}
			slog.Info("Detected system locale", "language", lang, "mapped_to", detectedLocale)
		}
	} else {
		detectedLocale = Locale
	}

	// Initialize workspace files from templates if they don't exist
	if err := i18n.InitializeWorkspace(pm.GetBaseDir(), detectedLocale); err != nil {
		slog.Warn("Failed to initialize workspace templates", "error", err)
	}

	// Set up locale for i18n
	if Locale != "" {
		// Explicit locale flag takes precedence
		if err := pm.SetLocale(Locale); err != nil {
			slog.Error("Invalid locale", "locale", Locale, "error", err)
			os.Exit(1)
		}
		slog.Info("Using locale from command line flag", "locale", Locale)
	}

	// Initialize LLM
	llm, err := llm.NewLLM(
		llm.WithProvider(Name),
		llm.WithModel(Model),
		llm.WithAPIKey(APIKey),
		llm.WithBaseURL(BaseURL),
	)
	if err != nil {
		slog.Error("Failed to initialize LLM", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	session := session.NewSession(llm)

	toolsList := []agent.Tool{
		tools.NewBashTool(),
		tools.NewWriteTool(),
		tools.NewReadTool(),
	}
	skillsList := skills.NewSkills(os.Getenv("HOME")+"/.agents/skills", ".agents/skills")

	ag, err := agent.NewAgent(
		session,
		agent.WithPersistenceManager(pm),
		agent.WithTools(toolsList...),
		agent.WithSkills(skillsList),
		agent.WithMaxRetries(20),
	)
	if err != nil {
		slog.Error("Failed to create agent", "error", err)
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
					fmt.Println("  /skills   - List available skills")
					fmt.Println("  /tools    - List available tools")
					return
				} else if strings.HasPrefix(text, "/reset") {
					session.Reset()
					fmt.Println("Session cleared.")
					return
				} else if strings.HasPrefix(text, "/bye") {
					fmt.Println("Goodbye!")
					os.Exit(0)
				} else if strings.HasPrefix(text, "/skills") {
					fmt.Println("Available Skills:")
					for _, skill := range skillsList.List() {
						fmt.Printf("- %s: %s\n", skill.Path(), skill.Description())
					}
					return
				} else if strings.HasPrefix(text, "/tools") {
					fmt.Println("Available Tools:")
					for _, tool := range toolsList {
						fmt.Printf("- %s: %s\n", tool.Name(), tool.Description())
					}
				} else if strings.HasPrefix(text, "/system-prompt") {
					prompt := pm.BuildSystemPrompt("")
					fmt.Println("Current System Prompt:")
					fmt.Println(prompt)
					return
				} else {
					fmt.Println("Unknown command. Type /help for a list of commands.")
					return
				}
			}

			response, err := ag.Execute(ctx, text)
			if err != nil {
				slog.Error("Agent execution error", "error", err)
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
				{Text: "/skills", Description: "List available skills"},
				{Text: "/tools", Description: "List available tools"},
				{Text: "/system-prompt", Description: "Show the current system prompt"},
			}
			return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
		},
		prompt.OptionPrefix("> "),
	)
	p.Run()
}
