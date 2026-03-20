package main

import (
	"context"
	"flag"
	"fmt"
	"io"
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
	"golang.org/x/term"
)

var (
	Name         string
	Model        string
	APIKey       string
	BaseURL      string
	WorkspaceDir string
	Locale       string
	MaxRetries   int = 30
)

func init() {
	dir, _ := os.Getwd()
	WorkspaceDir = dir

	flag.StringVar(&Name, "provider", "openai", "LLM provider: openai or anthropic")
	flag.StringVar(&Model, "model", "", "Model name (optional, provider default used if empty)")
	flag.StringVar(&APIKey, "api-key", "", "API key for the provider (env fallback OPENAI_API_KEY or ANTHROPIC_API_KEY)")
	flag.StringVar(&BaseURL, "base-url", "", "Optional base URL for the provider API")
	flag.StringVar(&WorkspaceDir, "workspace", WorkspaceDir, "Path to workspace directory (optional)")
	flag.StringVar(&Locale, "locale", "", "Language/locale for internationalized prompts ('en' or 'zh')")
	flag.IntVar(&MaxRetries, "max-retries", MaxRetries, "Maximum number of retries for tool execution")
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
		// Set the detected locale in the persistence manager
		if err := pm.SetLocale(detectedLocale); err != nil {
			slog.Error("Failed to set detected locale", "locale", detectedLocale, "error", err)
			os.Exit(1)
		}
	} else {
		// Explicit locale flag takes precedence
		if err := pm.SetLocale(Locale); err != nil {
			slog.Error("Invalid locale", "locale", Locale, "error", err)
			os.Exit(1)
		}
		detectedLocale = Locale
	}

	// Initialize workspace files from templates if they don't exist
	if err := i18n.InitializeWorkspace(pm.GetBaseDir(), detectedLocale); err != nil {
		slog.Warn("Failed to initialize workspace templates", "error", err)
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

	session := session.NewSession(llm,
		session.WithPersistenceManager(pm),
		session.WithSave("current"),
	)

	// Attempt to load the previous session if it exists
	if err := session.Load("current"); err != nil {
		if os.IsNotExist(err) {
			// No previous session file found; start with a fresh session
			slog.Debug("No previous session found, starting a new one", "error", err)
		} else {
			// Session file exists but failed to load; warn the user and start fresh
			slog.Warn("Failed to load previous session; starting with a new one", "error", err)
		}
	} else {
		slog.Info("Loaded previous session from session/current.ndjson")
	}

	toolsList := []agent.Tool{
		tools.NewBashTool(),
		tools.NewWriteTool(),
		tools.NewReadTool(),
		tools.NewEditTool(),
		tools.NewCompressTool(session),
	}
	skillsList := skills.NewSkills(os.Getenv("HOME")+"/.agents/skills", ".agents/skills")

	ag, err := agent.NewAgent(
		session,
		agent.WithPersistenceManager(pm),
		agent.WithTools(toolsList...),
		agent.WithSkills(skillsList),
		agent.WithMaxRetries(MaxRetries),
	)
	if err != nil {
		slog.Error("Failed to create agent", "error", err)
		os.Exit(1)
	}

	if !isTty() {
		text, err := io.ReadAll(os.Stdin)
		if err != nil {
			slog.Error("Read stdint error", "error", err)
			os.Exit(1)
		}
		response, err := ag.Execute(ctx, string(text))
		if err != nil {
			slog.Error("Agent execution error", "error", err)
			os.Exit(1)
		}

		fmt.Println(response)
		return
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
					fmt.Println("  /save     - Save the current session to a file")
					fmt.Println("  /load     - Load a session from a file")
					fmt.Println("  /bye      - Exit the program")
					fmt.Println("  /skills   - List available skills")
					fmt.Println("  /tools    - List available tools")
					return
				} else if strings.HasPrefix(text, "/reset") {
					session.Reset()
					fmt.Println("Session cleared.")
					return
				} else if strings.HasPrefix(text, "/save") {
					// Extract filename from command
					parts := strings.Fields(text)
					filename := "session"
					if len(parts) > 1 {
						filename = parts[1]
					}
					err := session.Save(filename)
					if err != nil {
						slog.Error("Failed to save session", "error", err)
						fmt.Printf("Error: Failed to save session: %v\n", err)
					} else {
						displayFilename := filename
						if !strings.HasSuffix(displayFilename, ".ndjson") {
							displayFilename += ".ndjson"
						}
						fmt.Printf("Session saved to session/%s\n", displayFilename)
					}
					return
				} else if strings.HasPrefix(text, "/load") {
					// Extract filename from command
					parts := strings.Fields(text)
					if len(parts) < 2 {
						fmt.Println("Usage: /load <filename>")
						return
					}
					filename := parts[1]
					err := session.Load(filename)
					if err != nil {
						slog.Error("Failed to load session", "error", err)
						fmt.Printf("Error: Failed to load session: %v\n", err)
					} else {
						displayFilename := filename
						if !strings.HasSuffix(displayFilename, ".ndjson") {
							displayFilename += ".ndjson"
						}
						fmt.Printf("Session loaded from session/%s\n", displayFilename)
					}
					return
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
				} else if strings.HasPrefix(text, "/bye") {
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
				{Text: "/save", Description: "Save the current session to a file"},
				{Text: "/load", Description: "Load a session from a file"},
				{Text: "/bye", Description: "Exit the program"},
				{Text: "/skills", Description: "List available skills"},
				{Text: "/tools", Description: "List available tools"},
				{Text: "/system-prompt", Description: "Show the current system prompt"},
			}
			return prompt.FilterHasPrefix(s, in.GetWordBeforeCursor(), true)
		},
		prompt.OptionPrefix("> "),
		prompt.OptionSetExitCheckerOnInput(func(in string, breakline bool) bool {
			exit := breakline && strings.TrimSpace(in) == "/bye"
			return exit
		}),
	)
	p.Run()
}

func isTty() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
