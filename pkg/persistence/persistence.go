package persistence

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/wzshiming/MachineSpirit/pkg/persistence/i18n"
)

// PersistenceManager handles loading and saving persistence files
type PersistenceManager struct {
	baseDir string
	items   []string
	locale  string // e.g., "en", "zh" - empty means default/no locale
}

// NewPersistenceManager creates a new persistence manager
// If baseDir is empty, uses the current working directory
func NewPersistenceManager(baseDir string) (*PersistenceManager, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("base directory is required for persistence manager")
	}

	return &PersistenceManager{
		baseDir: baseDir,
		items: []string{
			i18n.FileAgents,
		},
	}, nil
}

// SetLocale sets the locale for internationalized prompt loading
// Only "en" and "zh" locales are currently supported
func (pm *PersistenceManager) SetLocale(locale string) error {
	if err := i18n.ValidateLocale(locale); err != nil {
		return err
	}
	pm.locale = locale
	return nil
}

// GetLocale returns the current locale setting
func (pm *PersistenceManager) GetLocale() string {
	return pm.locale
}

// GetBaseDir returns the base directory
func (pm *PersistenceManager) GetBaseDir() string {
	return pm.baseDir
}

func (pm *PersistenceManager) getFilePath(filename string) string {
	return i18n.GetLocalizedFilePath(pm.baseDir, filename, pm.locale)
}

// BuildSystemPrompt constructs a system prompt from persistence files
func (pm *PersistenceManager) BuildSystemPrompt(basePrompt string) string {
	var parts []string

	if basePrompt != "" {
		parts = append(parts, basePrompt)
	}

	now := time.Now()

	zone, offset := now.Zone()

	parts = append(parts, fmt.Sprintf("Current time %s, zone %s (UTC%+d)", now.Format(time.RFC3339), zone, offset/3600))

	parts = append(parts, fmt.Sprintf("Workspace %s", pm.baseDir))

	// files of baseDir
	entrys, err := os.ReadDir(pm.baseDir)
	if err != nil {
		slog.Warn("Failed to read workspace directory", "dir", pm.baseDir, "error", err)
	}

	list := make([]string, 0, len(entrys))
	for _, entry := range entrys {
		if entry.IsDir() {
			list = append(list, entry.Name()+"/")
		} else {
			list = append(list, entry.Name())
		}
	}

	parts = append(parts, "Workspace files:\n"+strings.Join(list, "\n"))

	for _, item := range pm.items {

		path := pm.getFilePath(item)
		raw, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				slog.Warn("Failed to read persistence file", "path", path, "error", err)
			}
			continue
		}

		content := string(raw)
		content = strings.TrimSpace(content)
		if content == "" {
			continue
		}

		parts = append(parts, content)

	}

	return strings.Join(parts, "\n\n")
}
