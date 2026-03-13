package session

import (
	"fmt"
	"strings"

	"github.com/wzshiming/MachineSpirit/pkg/model"
)

// Summarizer produces a discussion summary suitable for memory storage.
type Summarizer interface {
	Summarize(user model.Message, assistant model.Message) string
}

// SimpleSummarizer condenses a single user/assistant turn into a dated summary line.
type SimpleSummarizer struct{}

func (SimpleSummarizer) Summarize(user model.Message, assistant model.Message) string {
	day := user.Timestamp.UTC().Format("2006-01-02")
	userText := strings.TrimSpace(user.Content)
	asstText := strings.TrimSpace(assistant.Content)
	return fmt.Sprintf("%s: user=\"%s\" assistant=\"%s\"", day, userText, asstText)
}
