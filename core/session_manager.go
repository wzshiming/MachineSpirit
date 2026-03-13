package core

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrSessionOverloaded = errors.New("session overloaded")
	ErrSessionInactive   = errors.New("session inactive")
	ErrSessionMissingID  = errors.New("session id required")
)

type SessionManager struct {
	mu         sync.Mutex
	sessions   map[string]*sessionState
	agent      Agent
	maxPending int
	pruneAfter time.Duration
	now        func() time.Time
}

type sessionState struct {
	id         string
	transcript []Message
	lastActive time.Time
	pending    int
}

type Option func(m *SessionManager)

func WithMaxPending(limit int) Option {
	return func(m *SessionManager) {
		if limit > 0 {
			m.maxPending = limit
		}
	}
}

func WithPruneAfter(d time.Duration) Option {
	return func(m *SessionManager) {
		if d > 0 {
			m.pruneAfter = d
		}
	}
}

func WithClock(now func() time.Time) Option {
	return func(m *SessionManager) {
		if now != nil {
			m.now = now
		}
	}
}

func NewSessionManager(agent Agent, opts ...Option) *SessionManager {
	manager := &SessionManager{
		sessions:   make(map[string]*sessionState),
		agent:      agent,
		maxPending: 4,
		pruneAfter: 30 * time.Minute,
		now:        time.Now,
	}
	if manager.agent == nil {
		manager.agent = AgentLoop{
			Planner:     EchoPlanner{},
			ToolInvoker: NoopToolInvoker{},
			Composer:    SimpleComposer{},
		}
	}
	for _, opt := range opts {
		opt(manager)
	}
	return manager
}

// HandleEvent processes an inbound event, routing it through the agent loop and returning a response envelope.
func (m *SessionManager) HandleEvent(ctx context.Context, event Event) (ResponseEnvelope, error) {
	var envelope ResponseEnvelope
	if event.SessionID == "" {
		return envelope, ErrSessionMissingID
	}

	now := m.now()
	if event.Timestamp.IsZero() {
		event.Timestamp = now
	}

	userMessage := Message{
		Role:      RoleUser,
		Content:   event.Content,
		Timestamp: event.Timestamp,
		Channel:   event.Channel,
		Sender:    event.Sender,
	}

	m.mu.Lock()
	session := m.ensureSession(event.SessionID, now)

	if m.pruneAfter > 0 && now.Sub(session.lastActive) > m.pruneAfter {
		m.mu.Unlock()
		return ResponseEnvelope{
			SessionID:  event.SessionID,
			Dropped:    true,
			DropReason: "session inactive",
		}, ErrSessionInactive
	}

	if session.pending >= m.maxPending {
		m.mu.Unlock()
		return ResponseEnvelope{
			SessionID:  event.SessionID,
			Dropped:    true,
			DropReason: "queue limit reached",
		}, ErrSessionOverloaded
	}

	session.pending++
	session.lastActive = now
	presence := []PresenceUpdate{{Status: PresenceTyping, At: now}}
	transcriptCopy := append([]Message(nil), session.transcript...)
	m.mu.Unlock()

	reply, err := m.agent.Respond(ctx, AgentInput{
		Event:      event,
		Transcript: transcriptCopy,
	})

	m.mu.Lock()
	defer m.mu.Unlock()

	session.pending--
	session.transcript = append(session.transcript, userMessage)
	envelope.SessionID = event.SessionID
	envelope.Presence = append(envelope.Presence, presence...)

	if err != nil {
		session.lastActive = m.now()
		envelope.Presence = append(envelope.Presence, PresenceUpdate{
			Status: PresenceActive,
			At:     session.lastActive,
		})
		envelope.Error = err.Error()
		envelope.TranscriptSize = len(session.transcript)
		return envelope, err
	}

	if reply.Role == "" {
		reply.Role = RoleAssistant
	}
	if reply.Timestamp.IsZero() {
		reply.Timestamp = m.now()
	}

	session.transcript = append(session.transcript, reply)
	session.lastActive = reply.Timestamp

	envelope.Messages = []Message{reply}
	envelope.Presence = append(envelope.Presence, PresenceUpdate{
		Status: PresenceActive,
		At:     session.lastActive,
	})
	envelope.TranscriptSize = len(session.transcript)

	return envelope, nil
}

func (m *SessionManager) PruneInactive() []string {
	if m.pruneAfter <= 0 {
		return nil
	}

	cutoff := m.now().Add(-m.pruneAfter)
	m.mu.Lock()
	defer m.mu.Unlock()

	var removed []string
	for id, session := range m.sessions {
		if session.pending > 0 {
			continue
		}
		if session.lastActive.Before(cutoff) {
			delete(m.sessions, id)
			removed = append(removed, id)
		}
	}
	return removed
}

// StartPruneLoop launches a goroutine that periodically prunes inactive sessions until ctx is cancelled.
func (m *SessionManager) StartPruneLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = m.pruneAfter
	}
	if interval <= 0 {
		return
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.PruneInactive()
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (m *SessionManager) Snapshot(sessionID string) (SessionSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	session, ok := m.sessions[sessionID]
	if !ok {
		return SessionSnapshot{}, false
	}

	transcript := append([]Message(nil), session.transcript...)
	return SessionSnapshot{
		ID:          sessionID,
		Transcript:  transcript,
		LastActive:  session.lastActive,
		PendingWork: session.pending,
	}, true
}

func (m *SessionManager) ensureSession(id string, now time.Time) *sessionState {
	session := m.sessions[id]
	if session == nil {
		session = &sessionState{
			id:         id,
			lastActive: now,
		}
		m.sessions[id] = session
	}
	return session
}
