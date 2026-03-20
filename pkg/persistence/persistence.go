package persistence

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wzshiming/MachineSpirit/pkg/persistence/i18n"
)

// PersistenceManager handles loading and saving persistence files
type PersistenceManager struct {
	baseDir string
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

// CheckBootstrap reports whether a BOOTSTRAP.md file exists in the workspace.
func (pm *PersistenceManager) CheckBootstrap() (bool, error) {
	baseDir := pm.GetBaseDir()
	bootstrapPath := filepath.Join(baseDir, i18n.FileBootstrap)

	info, err := os.Stat(bootstrapPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check bootstrap file: %w", err)
	}

	if info.IsDir() {
		return false, nil
	}

	return true, nil
}

// ReadFile reads a persistence file by name, respecting locale settings.
// It returns the trimmed content and any error.
func (pm *PersistenceManager) ReadFile(filename string) (string, error) {
	path := pm.getFilePath(filename)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}
