package i18n

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	// FileMemory stores persistent information and context
	FileMemory = "MEMORY.md"
	// FileIdentity stores agent identity and personality
	FileIdentity = "IDENTITY.md"
	// FileAgents stores information about other agents
	FileAgents = "AGENTS.md"
	// FileSoul stores agent identity and personality
	FileSoul = "SOUL.md"
	// FileUser stores user information and preferences
	FileUser = "USER.md"
	// FileBootstrap stores initial greeting and setup instructions (deleted after boot)
	FileBootstrap = "BOOTSTRAP.md"
	// FileTools stores information about available tools (optional, can be generated from agent configuration)
	FileTools = "TOOLS.md"
)

//go:embed en/*.md zh/*.md
var templatesFS embed.FS

// SupportedLocales lists the locales that are currently supported
var SupportedLocales = []string{"en", "zh"}

// ValidateLocale checks if a locale is supported
func ValidateLocale(locale string) error {
	if locale == "" {
		return nil // empty locale is valid (means use default)
	}

	if slices.Contains(SupportedLocales, locale) {
		return nil
	}

	return fmt.Errorf("unsupported locale '%s', only %v are supported", locale, SupportedLocales)
}

// GetLocalizedFilePath returns the path to a localized file if it exists,
// otherwise returns the default file path.
// For example, if locale is "zh" and filename is "SOUL.md", it will try
// "SOUL.zh.md" first, then fall back to "SOUL.md".
func GetLocalizedFilePath(baseDir, filename, locale string) string {
	// If no locale is set, return default path
	if locale == "" {
		return filepath.Join(baseDir, filename)
	}

	// Try locale-specific file first (e.g., SOUL.zh.md for zh locale)
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)
	localizedPath := filepath.Join(baseDir, nameWithoutExt+"."+locale+ext)

	// Check if localized file exists
	if _, err := os.Stat(localizedPath); err == nil {
		return localizedPath
	}

	// Fall back to default file
	return filepath.Join(baseDir, filename)
}

// GetTemplateContent returns the content of a template file for a given locale.
// It first tries to read from the embedded filesystem, then falls back to en.
func GetTemplateContent(filename, locale string) (string, error) {
	if locale == "" {
		locale = "en"
	}

	// Validate locale
	if err := ValidateLocale(locale); err != nil {
		return "", err
	}

	// Try to read from the specified locale
	path := filepath.Join(locale, filename)
	content, err := fs.ReadFile(templatesFS, path)
	if err == nil {
		return string(content), nil
	}

	// Fall back to English if file not found in specified locale
	if locale != "en" {
		path = filepath.Join("en", filename)
		content, err = fs.ReadFile(templatesFS, path)
		if err == nil {
			return string(content), nil
		}
	}

	return "", fmt.Errorf("template file %s not found for locale %s", filename, locale)
}

// InitializeWorkspace creates the workspace files from templates if they don't exist
func InitializeWorkspace(baseDir, locale string) error {
	if locale == "" {
		locale = "en"
	}

	if err := ValidateLocale(locale); err != nil {
		return err
	}

	// List of template files to copy
	templates := []string{
		FileSoul,
		FileAgents,
		FileIdentity,
		FileUser,
		FileTools,
		FileBootstrap,
	}

	size := 0
	for _, template := range templates {
		targetPath := filepath.Join(baseDir, template)

		// Skip if file already exists
		if _, err := os.Stat(targetPath); err == nil {
			size++
			continue
		}

		if template == FileBootstrap && size != 0 {
			continue
		}

		// Get template content
		content, err := GetTemplateContent(template, locale)
		if err != nil {
			return fmt.Errorf("failed to get template %s: %w", template, err)
		}

		// Write to file
		if err := os.WriteFile(targetPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", targetPath, err)
		}
	}

	return nil
}
