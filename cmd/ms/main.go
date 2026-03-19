package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Xuanwo/go-locale"
	"github.com/c-bata/go-prompt"
	"github.com/wzshiming/MachineSpirit/pkg/agent"
	"github.com/wzshiming/MachineSpirit/pkg/agent/skills"
	"github.com/wzshiming/MachineSpirit/pkg/agent/tools"
	"github.com/wzshiming/MachineSpirit/pkg/llm"
	"github.com/wzshiming/MachineSpirit/pkg/persistence"
	"github.com/wzshiming/MachineSpirit/pkg/persistence/i18n"
	"github.com/wzshiming/MachineSpirit/pkg/scheduler"
	"github.com/wzshiming/MachineSpirit/pkg/session"
	"golang.org/x/term"
)

var (
	Name              string
	Model             string
	APIKey            string
	BaseURL           string
	WorkspaceDir      string
	Locale            string
	MaxRetries        int = 30
	HeartbeatInterval string
	HeartbeatMessage  string
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
	flag.StringVar(&HeartbeatInterval, "heartbeat", "", "Periodic heartbeat interval (e.g. '1m', '30s'). Triggers the main agent periodically.")
	flag.StringVar(&HeartbeatMessage, "heartbeat-message", "heartbeat: check status and report", "Message sent to the agent on each heartbeat tick")
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
	provider, err := llm.NewLLM(
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

	mainSession := session.NewSession(provider,
		session.WithPersistenceManager(pm),
	)

	// Create scheduler with file persistence and sub-agent callback.
	// When a cron job fires, a fresh sub-agent (with its own session)
	// executes the task independently, then feeds results back to the
	// main session so the main agent knows what happened.
	crontabFile := filepath.Join(pm.GetBaseDir(), "CRONTAB")
	sched := scheduler.New(func(schedCtx context.Context, message string) {
		// Create a fresh session for the sub-agent so it does not
		// pollute the main conversation transcript during execution.
		subSession := session.NewSession(provider,
			session.WithPersistenceManager(pm),
		)
		// Sub-agent gets execution tools only (no scheduling tools)
		subTools := []agent.Tool{
			tools.NewBashTool(),
			tools.NewWriteTool(),
			tools.NewReadTool(),
		}
		subAgent, err := agent.NewAgent(
			subSession,
			agent.WithPersistenceManager(pm),
			agent.WithTools(subTools...),
			agent.WithMaxRetries(MaxRetries),
		)
		if err != nil {
			slog.Error("Failed to create sub-agent for scheduled task", "error", err)
			return
		}
		response, err := subAgent.Execute(schedCtx, message)
		if err != nil {
			slog.Error("Scheduled task error", "error", err)
			// Feed error back to main session
			mainSession.AddMessages(llm.Message{
				Role:    llm.RoleUser,
				Content: fmt.Sprintf("[scheduled task] %s", message),
			}, llm.Message{
				Role:    llm.RoleAssistant,
				Content: fmt.Sprintf("Error executing scheduled task: %v", err),
			})
			return
		}
		fmt.Printf("\n[scheduled] %s\n", response)
		// Feed sub-agent's work back to main session so the main agent
		// knows what the scheduled task did.
		mainSession.AddMessages(llm.Message{
			Role:    llm.RoleUser,
			Content: fmt.Sprintf("[scheduled task] %s", message),
		}, llm.Message{
			Role:    llm.RoleAssistant,
			Content: response,
		})
	}, crontabFile)
	defer sched.Stop()

	// Restore persisted cron jobs from previous runs
	if err := sched.LoadFromFile(); err != nil {
		slog.Warn("Failed to load persisted crontab", "error", err)
	}

	toolsList := []agent.Tool{
		tools.NewBashTool(),
		tools.NewWriteTool(),
		tools.NewReadTool(),
		tools.NewCompressTool(mainSession),
		tools.NewCronTool(sched),
	}
	skillsList := skills.NewSkills(os.Getenv("HOME")+"/.agents/skills", ".agents/skills")

	ag, err := agent.NewAgent(
		mainSession,
		agent.WithPersistenceManager(pm),
		agent.WithTools(toolsList...),
		agent.WithSkills(skillsList),
		agent.WithMaxRetries(MaxRetries),
	)
	if err != nil {
		slog.Error("Failed to create agent", "error", err)
		os.Exit(1)
	}

	// Heartbeat: periodic trigger for the main agent
	if HeartbeatInterval != "" {
		interval, err := time.ParseDuration(HeartbeatInterval)
		if err != nil {
			slog.Error("Invalid heartbeat interval", "interval", HeartbeatInterval, "error", err)
			os.Exit(1)
		}
		if interval <= 0 {
			slog.Error("Heartbeat interval must be positive", "interval", HeartbeatInterval)
			os.Exit(1)
		}
		go func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					response, err := ag.Execute(ctx, HeartbeatMessage)
					if err != nil {
						slog.Error("Heartbeat execution error", "error", err)
						continue
					}
					fmt.Printf("\n[heartbeat] %s\n", response)
				}
			}
		}()
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
					fmt.Println("  /bye      - Exit the program")
					fmt.Println("  /skills   - List available skills")
					fmt.Println("  /tools    - List available tools")
					return
				} else if strings.HasPrefix(text, "/reset") {
					mainSession.Reset()
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

func isTty() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}
