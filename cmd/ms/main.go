package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

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
		session.WithBaseDir(pm.GetBaseDir()),
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

	baseTools := []agent.Tool{
		tools.NewBashTool(),
		tools.NewWriteTool(),
		tools.NewReadTool(),
	}

	subSession := tools.NewSubSessionTool(llm, pm, session, func() []agent.Tool {
		return baseTools
	})

	toolsList := append(baseTools,
		tools.NewCompressTool(session),
		subSession,
	)
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
		err = ag.Execute(ctx, string(text), os.Stdout)
		if err != nil {
			slog.Error("Agent execution error", "error", err)
			os.Exit(1)
		}

		// Process any pending sub-session results
		processQueuedInputs(ctx, ag, session)
		return
	}

	histPath := historyFilePath(pm.GetBaseDir())
	histLines, err := loadHistory(histPath)
	if err != nil {
		slog.Warn("Failed to load input history", "error", err)
	}

	// execMu serializes agent execution so that the background goroutine
	// (processing sub-session results) and the prompt executor never call
	// ag.Execute concurrently.
	var execMu sync.Mutex

	// Background goroutine: react to sub-session completions even when the
	// user has not entered new input. This goroutine is intentionally
	// long-lived and exits when the process terminates.
	go func() {
		for range session.InputNotify() {
			execMu.Lock()
			processQueuedInputs(ctx, ag, session)
			execMu.Unlock()
		}
	}()

	p := prompt.New(
		func(text string) {
			text = strings.TrimSpace(text)
			if text == "" {
				return
			}
			appendHistory(histPath, text)
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
					prompt := ag.BuildSystemPrompt()
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

			execMu.Lock()
			err := ag.Execute(ctx, text, os.Stdout)
			if err != nil {
				slog.Error("Agent execution error", "error", err)
				execMu.Unlock()
				return
			}

			// Process any pending sub-session results
			processQueuedInputs(ctx, ag, session)
			execMu.Unlock()
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
		prompt.OptionHistory(histLines),
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

// historyFilePath returns the path to the input history file within the
// workspace's session directory, creating the directory if necessary.
func historyFilePath(baseDir string) string {
	return filepath.Join(baseDir, ".ms_history")
}

// loadHistory reads newline-delimited input history from path.
// It returns nil (no error) when the file does not exist.
func loadHistory(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if line := scanner.Text(); line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}

// appendHistory appends a single line to the history file, creating the
// file and its parent directory if they do not exist.
func appendHistory(path, line string) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		slog.Warn("Failed to create history directory", "error", err)
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		slog.Warn("Failed to open history file", "error", err)
		return
	}
	defer f.Close()
	if _, err := fmt.Fprintln(f, line); err != nil {
		slog.Warn("Failed to write to history file", "error", err)
	}
}

// processQueuedInputs drains all pending messages from the session's input
// queue (e.g. results from completed sub-sessions) and feeds each one to the
// agent for processing.  It loops up to maxDrainRounds times to pick up
// results from sub-sessions that may complete while earlier results are being
// processed, preventing unbounded recursion.
func processQueuedInputs(ctx context.Context, ag *agent.Agent, sess *session.Session) {
	const maxDrainRounds = 3
	for round := range maxDrainRounds {
		msgs := sess.DrainInputs()
		if len(msgs) == 0 {
			break
		}
		_ = round
		for _, msg := range msgs {
			fmt.Printf("\n[Queued input]: %s\n", msg.Content)
			err := ag.Execute(ctx, msg.Content, os.Stdout)
			if err != nil {
				slog.Error("Failed to process queued input", "error", err)
				continue
			}
		}
	}
}
