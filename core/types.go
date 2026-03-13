package core

import "time"

// Role represents the speaker of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// Event is an inbound channel event that should be routed to a session.
type Event struct {
	SessionID string
	Sender    string
	Channel   string
	Content   string
	Timestamp time.Time
}

// Message is a transcript entry exchanged within a session.
type Message struct {
	Role      Role
	Content   string
	Timestamp time.Time
	Channel   string
	Sender    string
}

// PresenceStatus describes the activity state for a session.
type PresenceStatus string

const (
	PresenceTyping   PresenceStatus = "typing"
	PresenceActive   PresenceStatus = "active"
	PresenceInactive PresenceStatus = "inactive"
)

// PresenceUpdate signals a change in presence or typing activity.
type PresenceUpdate struct {
	Status PresenceStatus
	At     time.Time
}

// ResponseEnvelope is emitted back to the gateway after handling an event.
type ResponseEnvelope struct {
	SessionID      string
	Presence       []PresenceUpdate
	Messages       []Message
	Dropped        bool
	DropReason     string
	TranscriptSize int
	Error          string
}

// SessionSnapshot exposes a read-only view of a session for monitoring or tests.
type SessionSnapshot struct {
	ID          string
	Transcript  []Message
	LastActive  time.Time
	PendingWork int
}
